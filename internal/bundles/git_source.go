package bundles

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"agent-vivy/internal/bundlecontract"
)

var gitRefPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,255}$`)

type GitEnvironment func(context.Context, string) ([]string, error)

func safeOrigin(s string) (string, error) {
	if s == "" || strings.HasPrefix(s, "-") || strings.ContainsAny(s, "\x00\n\r") {
		return "", bundlecontract.Invalid("git_origin")
	}
	// A native absolute path can contain a Windows drive colon, which URL
	// parsing would mistake for a scheme. Keep local provenance to the basename.
	if filepath.IsAbs(s) {
		return filepath.Base(filepath.Clean(s)), nil
	}
	u, err := url.Parse(s)
	if err == nil && u.Scheme != "" {
		if (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return "", bundlecontract.Invalid("git_origin")
		}
		return u.Scheme + "://" + u.Host + u.EscapedPath(), nil
	}
	return filepath.Base(filepath.Clean(s)), nil
}
func sanitizedEnv(extra []string) []string {
	env := make([]string, 0, len(os.Environ())+len(extra))
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if strings.HasPrefix(strings.ToUpper(key), "GIT_") {
			continue
		}
		if strings.EqualFold(key, "GIT_CONFIG_COUNT") {
			continue
		}
		env = append(env, item)
	}
	env = append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_LFS_SKIP_SMUDGE=1", "GIT_OPTIONAL_LOCKS=0")
	return append(env, extra...)
}
func gitRun(ctx context.Context, dir string, extra []string, args ...string) ([]byte, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return nil, artifactError("git_unavailable", "git", err)
	}
	cmd := exec.CommandContext(ctx, git, args...)
	cmd.Dir = dir
	cmd.Env = sanitizedEnv(extra)
	var out limitedBuffer
	out.limit = 128 << 20
	cmd.Stdout = &out
	var stderr limitedBuffer
	stderr.limit = 64 << 10
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, artifactError("source_unavailable", "git", fmt.Errorf("git acquisition failed"))
	}
	if out.exceeded {
		return nil, bundlecontract.Invalid("git_tree_limit")
	}
	return out.Bytes(), nil
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if b.Len()+n > b.limit {
		remaining := b.limit - b.Len()
		if remaining > 0 {
			_, _ = b.Buffer.Write(p[:remaining])
		}
		b.exceeded = true
		return n, nil
	}
	return b.Buffer.Write(p)
}

func acquireGit(ctx context.Context, source bundlecontract.SourceSpec, stage string, credentials GitEnvironment) (string, bundlecontract.SourceLock, error) {
	var lock bundlecontract.SourceLock
	origin, err := safeOrigin(source.Location)
	if err != nil {
		return "", lock, err
	}
	if !gitRefPattern.MatchString(source.Ref) || strings.Contains(source.Ref, "..") || strings.Contains(source.Ref, "@{") || strings.HasSuffix(source.Ref, "/") || strings.HasSuffix(source.Ref, ".") || strings.HasSuffix(source.Ref, ".lock") {
		return "", lock, bundlecontract.Invalid("git_ref")
	}
	lock = bundlecontract.SourceLock{Kind: "git", Origin: origin, RequestedRef: source.Ref, ValidatorVersion: bundlecontract.APIVersion}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	repo := filepath.Join(stage, "git")
	hooks := filepath.Join(stage, "empty-hooks")
	if err := os.Mkdir(hooks, 0700); err != nil {
		return "", lock, err
	}
	if _, err := gitRun(ctx, stage, nil, "-c", "core.hooksPath="+hooks, "init", "--bare", "--template=", repo); err != nil {
		return "", lock, err
	}
	extra := []string{"GIT_CONFIG_COUNT=4", "GIT_CONFIG_KEY_0=core.hooksPath", "GIT_CONFIG_VALUE_0=" + hooks, "GIT_CONFIG_KEY_1=filter.lfs.smudge", "GIT_CONFIG_VALUE_1=", "GIT_CONFIG_KEY_2=filter.lfs.process", "GIT_CONFIG_VALUE_2=", "GIT_CONFIG_KEY_3=credential.helper", "GIT_CONFIG_VALUE_3="}
	if credentials != nil {
		supplied, e := credentials(ctx, source.Location)
		if e != nil {
			return "", lock, artifactError("source_unavailable", "credentials", nil)
		}
		for _, v := range supplied {
			key, _, ok := strings.Cut(v, "=")
			if !ok || (key != "GIT_ASKPASS" && key != "GIT_SSH_COMMAND" && key != "SSH_AUTH_SOCK") {
				return "", lock, bundlecontract.Invalid("credential_environment")
			}
		}
		extra = append(extra, supplied...)
	}
	// GIT_CONFIG_COUNT values are process-scoped overrides; no source-controlled
	// config or user-global credential helper can change execution behavior.
	if _, err := gitRun(ctx, stage, extra, "--git-dir", repo, "remote", "add", "origin", source.Location); err != nil {
		return "", lock, err
	}
	if _, err := gitRun(ctx, stage, extra, "--git-dir", repo, "-c", "protocol.file.allow=always", "fetch", "--no-tags", "--no-recurse-submodules", "origin", source.Ref); err != nil {
		return "", lock, err
	}
	commitBytes, err := gitRun(ctx, stage, extra, "--git-dir", repo, "rev-parse", "--verify", "FETCH_HEAD^{commit}")
	if err != nil {
		return "", lock, err
	}
	commit := strings.TrimSpace(string(commitBytes))
	if len(commit) != 40 && len(commit) != 64 {
		return "", lock, bundlecontract.Invalid("git_commit")
	}
	lock.ResolvedCommit = commit
	manifest, err := gitRun(ctx, stage, extra, "--git-dir", repo, "show", commit+":"+bundlecontract.ManifestFile)
	if err != nil {
		return "", lock, artifactError("unsupported_format", "native_manifest", err)
	}
	return extractDeclared(ctx, stage, repo, commit, manifest, extra, lock)
}

func extractDeclared(ctx context.Context, stage, repo, commit string, raw []byte, extra []string, lock bundlecontract.SourceLock) (string, bundlecontract.SourceLock, error) {
	m, err := bundlecontract.DecodeManifest(raw)
	if err != nil {
		return "", lock, err
	}
	tree, err := gitRun(ctx, stage, extra, "--git-dir", repo, "ls-tree", "-r", "-z", "--full-tree", commit)
	if err != nil {
		return "", lock, err
	}
	entries := map[string]string{}
	for _, row := range bytes.Split(tree, []byte{0}) {
		if len(row) == 0 {
			continue
		}
		meta, path, ok := bytes.Cut(row, []byte{'\t'})
		if !ok || !utf8.Valid(path) {
			return "", lock, bundlecontract.Invalid("git_tree")
		}
		fields := strings.Fields(string(meta))
		if len(fields) != 3 {
			return "", lock, bundlecontract.Invalid("git_tree")
		}
		entries[string(path)] = fields[0]
	}
	out := filepath.Join(stage, "source-tree")
	if err := os.Mkdir(out, 0700); err != nil {
		return "", lock, err
	}
	if err := os.WriteFile(filepath.Join(out, bundlecontract.ManifestFile), raw, 0600); err != nil {
		return "", lock, err
	}
	for _, p := range bundlecontract.DeclaredPaths(m) {
		mode, ok := entries[p]
		if !ok || mode != "100644" && mode != "100755" {
			return "", lock, bundlecontract.Invalid("git_declared_file")
		}
		b, err := gitRun(ctx, stage, extra, "--git-dir", repo, "cat-file", "blob", commit+":"+p)
		if err != nil {
			return "", lock, err
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(out, filepath.FromSlash(p))), 0700); err != nil {
			return "", lock, err
		}
		if err := os.WriteFile(filepath.Join(out, filepath.FromSlash(p)), b, 0600); err != nil {
			return "", lock, err
		}
	}
	return out, lock, nil
}
