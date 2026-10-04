package sdk

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	providerconformance "agent-vivy/sdk/conformance"
	"agent-vivy/sdk/generation"
	assemblyv1 "agent-vivy/sdk/internal/assembly"
	generationconformance "agent-vivy/sdk/internal/conformance"
)

// go-host pack target (W3-4): build an external desktop host that embeds the
// sealed VIVY runtime through sdk/host/v1 instead of building agent-vivy's
// own executable. The embedder module is compiled under a generated consumer
// modfile that reproduces VIVY's local replacement closure.

const (
	packTargetGoHost     = "go-host"
	goHostLockSchema     = "diva.go-host-inputs/v1"
	goHostManifestSchema = "vivy.go-host/v1"
)

// goHostInputs pins the external host build inputs parsed at pack time.
type goHostInputs struct {
	Dir     string
	Package string
	Assets  string
	Lock    string
	LockDoc goHostLock
}

// goHostLock is the typed projection of build/vivy-sources.lock.json used by
// the pack target. The DIVA wrapper owns the tracked file; the SDK only reads
// it, verifies the staged reality matches, and seals its digest.
type goHostLock struct {
	Schema  string `json:"schema"`
	Release bool   `json:"release"`
	Host    struct {
		ModulePath string `json:"modulePath"`
		Repository string `json:"repository"`
		// Commit and TreeSHA256 are optional: the tracked lock must not pin
		// its own DIVA commit — pack resolves them from the live checkout.
		Commit     string `json:"commit"`
		TreeSHA256 string `json:"treeSHA256"`
	} `json:"host"`
	Sources struct {
		Vivy   goHostLockSource `json:"vivy"`
		Laputa goHostLockSource `json:"laputa"`
		Inofy  struct {
			Module  string `json:"module"`
			Version string `json:"version"`
			Commit  string `json:"commit"`
		} `json:"inofy"`
	} `json:"sources"`
	Recipe struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	} `json:"recipe"`
	// Locks records digests of dependency lockfiles (go.mod/go.sum/pnpm).
	Locks map[string]string `json:"locks"`
	// Tools records toolchain pins (go, wails, goos/goarch/cgo, keyring
	// capability evidence). Field names are wrapper-owned; the SDK only
	// echoes them into the build report.
	Tools   map[string]any `json:"tools"`
	Keyring map[string]any `json:"keyring"`
}

type goHostLockSource struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	TreeSHA256 string `json:"treeSHA256"`
}

func isHexString(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, r := range value {
		if !('0' <= r && r <= '9') && !('a' <= r && r <= 'f') {
			return false
		}
	}
	return true
}

// isLocalRelative reports whether value is a non-empty repository-relative
// path that cannot escape its documented root. "." is allowed (the root
// itself); absolute paths, drive prefixes and parent traversal are not.
func isLocalRelative(value string) bool {
	if value == "" || filepath.IsAbs(value) || strings.ContainsRune(value, 0) {
		return false
	}
	cleaned := filepath.Clean(filepath.ToSlash(value))
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return false
	}
	volume := filepath.VolumeName(value)
	return volume == ""
}

// normalizeHostRelative validates a host-dir-relative input and returns its
// slash-cleaned form for hashing/reporting (leading "./" removed).
func normalizeHostRelative(label, value string) (string, error) {
	if !isLocalRelative(value) {
		return "", fmt.Errorf("%s must be a relative path inside --host-dir: %q", label, value)
	}
	cleaned := filepath.ToSlash(filepath.Clean(value))
	return strings.TrimPrefix(cleaned, "./"), nil
}

func parseGoHostLock(path string) (goHostLock, error) {
	var lock goHostLock
	raw, err := os.ReadFile(path)
	if err != nil {
		return lock, fmt.Errorf("sdk: read --host-lock: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&lock); err != nil {
		return lock, fmt.Errorf("sdk: decode --host-lock %s: %w", path, err)
	}
	if lock.Schema != goHostLockSchema {
		return lock, fmt.Errorf("sdk: --host-lock schema is %q, want %s", lock.Schema, goHostLockSchema)
	}
	if lock.Host.ModulePath == "" {
		return lock, fmt.Errorf("sdk: --host-lock missing host.modulePath")
	}
	if lock.Host.Commit != "" && !isHexString(lock.Host.Commit, 40) {
		return lock, fmt.Errorf("sdk: --host-lock host.commit is not a full commit SHA")
	}
	if lock.Host.TreeSHA256 != "" && !isHexString(lock.Host.TreeSHA256, 64) {
		return lock, fmt.Errorf("sdk: --host-lock host.treeSHA256 is not a SHA-256 digest")
	}
	if !isHexString(lock.Sources.Vivy.TreeSHA256, 64) || !isHexString(lock.Sources.Laputa.TreeSHA256, 64) {
		return lock, fmt.Errorf("sdk: --host-lock source treeSHA256 fields are required")
	}
	if !isHexString(lock.Sources.Vivy.Commit, 40) || !isHexString(lock.Sources.Laputa.Commit, 40) {
		return lock, fmt.Errorf("sdk: --host-lock vivy/laputa commits must be full SHAs")
	}
	if lock.Release {
		if lock.Sources.Inofy.Commit == "" || lock.Sources.Inofy.Version == "" {
			return lock, fmt.Errorf("sdk: --host-lock release inputs must pin inofy version and commit")
		}
		if lock.Host.Repository == "" || lock.Sources.Vivy.Repository == "" || lock.Sources.Laputa.Repository == "" {
			return lock, fmt.Errorf("sdk: --host-lock release inputs must record repository URLs")
		}
	}
	if !isHexString(lock.Recipe.SHA256, 64) {
		return lock, fmt.Errorf("sdk: --host-lock recipe.sha256 is required")
	}
	return lock, nil
}

// validateGoHostArgs normalizes the host inputs shared by flag parsing and
// Pack so the pack target never sees unvalidated paths.
func validateGoHostArgs(o packOptions) (goHostInputs, error) {
	var inputs goHostInputs
	missing := make([]string, 0, 4)
	if o.HostDir == "" {
		missing = append(missing, "--host-dir")
	}
	if o.HostPackage == "" {
		missing = append(missing, "--host-package")
	}
	if o.HostAssets == "" {
		missing = append(missing, "--host-assets")
	}
	if o.HostLock == "" {
		missing = append(missing, "--host-lock")
	}
	if len(missing) > 0 {
		return inputs, fmt.Errorf("sdk: --target go-host requires %s", strings.Join(missing, ", "))
	}
	hostDir, err := filepath.Abs(o.HostDir)
	if err != nil {
		return inputs, err
	}
	if _, err := os.Stat(filepath.Join(hostDir, "go.mod")); err != nil {
		return inputs, fmt.Errorf("sdk: --host-dir has no go.mod: %w", err)
	}
	pkg, err := normalizeHostRelative("--host-package", o.HostPackage)
	if err != nil {
		return inputs, err
	}
	assets, err := normalizeHostRelative("--host-assets", o.HostAssets)
	if err != nil {
		return inputs, err
	}
	if info, statErr := os.Stat(filepath.Join(hostDir, filepath.FromSlash(assets))); statErr != nil || !info.IsDir() {
		return inputs, fmt.Errorf("sdk: --host-assets is not a built directory inside --host-dir: %q", o.HostAssets)
	}
	lock, err := parseGoHostLock(o.HostLock)
	if err != nil {
		return inputs, err
	}
	if pkg == "." {
		return inputs, errors.New("sdk: --host-package must name a package directory, not the host root")
	}
	return goHostInputs{Dir: hostDir, Package: pkg, Assets: assets, Lock: o.HostLock, LockDoc: lock}, nil
}

// git plumbing ----------------------------------------------------------------

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s in %s: %w: %s", strings.Join(args, " "), dir, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func gitHead(dir string) (string, error) { return gitOutput(dir, "rev-parse", "HEAD") }

// gitDirtyFiles lists worktree paths differing from HEAD (staged, modified,
// untracked). An empty result means a clean checkout.
func gitDirtyFiles(dir string) ([]string, error) {
	out, err := gitOutput(dir, "status", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	var dirty []string
	for _, entry := range strings.Split(strings.TrimRight(out, "\x00"), "\x00") {
		if entry == "" {
			continue
		}
		if len(entry) > 3 {
			entry = entry[3:]
		}
		dirty = append(dirty, entry)
	}
	sort.Strings(dirty)
	return dirty, nil
}

func gitRemoteURL(dir string) string {
	out, err := gitOutput(dir, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return out
}

// gitTrackedFiles returns every version-controlled file of dir (the canonical
// source set: .git metadata, ignored outputs and untracked build artifacts are
// excluded by construction).
func gitTrackedFiles(dir string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git ls-files in %s: %w: %s", dir, err, strings.TrimSpace(string(out)))
	}
	var files []string
	for _, rel := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		if rel != "" {
			files = append(files, rel)
		}
	}
	sort.Strings(files)
	return files, nil
}

func findGitRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if info, err := os.Stat(filepath.Join(abs, ".git")); err == nil && info != nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("no git repository above %s", dir)
		}
		abs = parent
	}
}

// hashSourceTree computes the canonical tracked-source digest: each tracked
// file contributes "<relpath>\x00<size>\x00<bytes>" to one sha256 stream.
// Worktree bytes are hashed so development snapshots still seal the exact
// content that was built; git's file list keeps .git and ignored outputs out.
func hashSourceTree(dir string) (string, error) {
	files, err := gitTrackedFiles(dir)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	for _, rel := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		info, err := os.Lstat(full)
		if err != nil {
			return "", fmt.Errorf("hash source file %s: %w", rel, err)
		}
		fmt.Fprintf(hash, "%s\x00%o\x00", rel, info.Mode().Perm())
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			link, linkErr := os.Readlink(full)
			if linkErr != nil {
				return "", linkErr
			}
			fmt.Fprintf(hash, "link\x00%s\x00", link)
		case info.IsDir():
			// gitlink (submodule): seal the commit it points at.
			sub, subErr := gitHead(full)
			if subErr != nil {
				fmt.Fprintf(hash, "gitlink\x00absent\x00")
			} else {
				fmt.Fprintf(hash, "gitlink\x00%s\x00", sub)
			}
		default:
			body, err := os.ReadFile(full)
			if err != nil {
				return "", fmt.Errorf("hash source file %s: %w", rel, err)
			}
			fmt.Fprintf(hash, "%d\x00", len(body))
			hash.Write(body)
		}
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// stageSourceTree copies only tracked files into destination so a snapshot
// never carries .git metadata, ignored outputs or untracked scratch.
func stageSourceTree(source, destination string) error {
	files, err := gitTrackedFiles(source)
	if err != nil {
		return err
	}
	for _, rel := range files {
		target := filepath.Join(destination, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		info, err := os.Lstat(filepath.Join(source, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			link, linkErr := os.Readlink(filepath.Join(source, filepath.FromSlash(rel)))
			if linkErr != nil {
				return linkErr
			}
			if err := os.Symlink(link, target); err != nil {
				return err
			}
		case info.IsDir():
			// gitlink (submodule): staged as an empty directory.
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
		default:
			body, err := os.ReadFile(filepath.Join(source, filepath.FromSlash(rel)))
			if err != nil {
				return fmt.Errorf("stage source file %s: %w", rel, err)
			}
			if err := os.WriteFile(target, body, info.Mode().Perm()); err != nil {
				return err
			}
		}
	}
	return nil
}

// go.mod closure --------------------------------------------------------------

type modReplace struct {
	Old, New string
	Version  string // non-empty for remote replaces (Old => New@Version)
}

// localModfileReplaces lists replace directives whose target is a local path
// (rather than a module@version pair).
func localModfileReplaces(modfile string) ([]modReplace, error) {
	cmd := exec.Command("go", "mod", "edit", "-json", "-modfile="+modfile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("sdk: decode modfile %s: %w: %s", modfile, err, strings.TrimSpace(string(out)))
	}
	var document struct {
		Replace []struct {
			Old struct {
				Path string `json:"Path"`
			} `json:"Old"`
			New struct {
				Path    string `json:"Path"`
				Version string `json:"Version"`
			} `json:"New"`
		} `json:"Replace"`
	}
	if err := json.Unmarshal(out, &document); err != nil {
		return nil, fmt.Errorf("sdk: parse modfile %s: %w", modfile, err)
	}
	var replaces []modReplace
	for _, entry := range document.Replace {
		newPath := entry.New.Path
		if entry.New.Version != "" {
			continue
		}
		replaces = append(replaces, modReplace{Old: entry.Old.Path, New: newPath, Version: entry.New.Version})
	}
	return replaces, nil
}

func goModEdit(dir, modfile string, edits ...string) error {
	args := append([]string{"mod", "edit", "-modfile=" + modfile}, edits...)
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sdk: edit consumer modfile: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// goHostStaged holds the staged snapshot layout for one pack invocation.
type goHostStaged struct {
	Root        string // temp root (caller removes)
	Vivy        string // staged agent-vivy tree
	Host        string // staged host module tree
	Deps        string // staged dependency repos parent
	Modfile     string // generated consumer modfile inside Host
	ResolvedSum string // consumer.sum path
}

// verifyGoHostLock checks the declared input lock against live trees and
// returns their observed identities for the build report. Release inputs
// additionally require clean worktrees at the pinned commits.
func verifyGoHostLock(inputs goHostInputs, repoRoot, laputaRoot string) error {
	lock := inputs.LockDoc
	type check struct {
		name   string
		dir    string
		want   goHostLockSource
		pinned bool
	}
	checks := []check{
		{"vivy", repoRoot, lock.Sources.Vivy, true},
		{"laputa", laputaRoot, lock.Sources.Laputa, true},
		{"host", inputs.Dir, goHostLockSource{Repository: lock.Host.Repository, Commit: lock.Host.Commit, TreeSHA256: lock.Host.TreeSHA256}, false},
	}
	for _, c := range checks {
		head, err := gitHead(c.dir)
		if err != nil {
			return fmt.Errorf("sdk: %s source is not a git worktree: %w", c.name, err)
		}
		// The host identity is resolved from the live checkout; a pinned
		// host.commit in the lock only adds a consistency check, it is never
		// required (the tracked lock must not pin its own DIVA commit).
		if c.want.Commit != "" && head != c.want.Commit {
			return fmt.Errorf("sdk: %s source commit is %s, lock pins %s", c.name, head, c.want.Commit)
		}
		tree, err := hashSourceTree(c.dir)
		if err != nil {
			return err
		}
		if c.pinned && tree != c.want.TreeSHA256 {
			return fmt.Errorf("sdk: %s source tree digest is %s, lock pins %s", c.name, tree, c.want.TreeSHA256)
		}
		if c.want.TreeSHA256 != "" && tree != c.want.TreeSHA256 {
			return fmt.Errorf("sdk: %s source tree digest is %s, lock pins %s", c.name, tree, c.want.TreeSHA256)
		}
		dirty, err := gitDirtyFiles(c.dir)
		if err != nil {
			return err
		}
		if lock.Release && len(dirty) > 0 {
			return fmt.Errorf("sdk: release packing rejects dirty %s source: %s", c.name, strings.Join(dirty[:min(3, len(dirty))], ", "))
		}
	}
	return nil
}

// pack pipeline ---------------------------------------------------------------

// goHostBinaryBase derives the published executable name from the selected
// host package directory (./cmd/diva -> diva); a root package falls back to
// the module path's final element.
func goHostBinaryBase(inputs goHostInputs) string {
	base := filepath.Base(filepath.FromSlash(inputs.Package))
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = filepath.Base(inputs.LockDoc.Host.ModulePath)
	}
	return base
}

func artifactHostBinaryName(goos, base string) string {
	if goos == "windows" {
		return base + ".exe"
	}
	return base
}

// goToolchainVersion reports the toolchain version used for the build.
func goToolchainVersion(dir string) (string, error) {
	cmd := exec.Command("go", "version")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("sdk: go toolchain probe: %w: %s", err, strings.TrimSpace(string(out)))
	}
	fields := strings.Fields(string(out))
	if len(fields) < 3 {
		return "", fmt.Errorf("sdk: unexpected go version output %q", strings.TrimSpace(string(out)))
	}
	return fields[2], nil
}

func goEnvValue(dir, name string) string {
	cmd := exec.Command("go", "env", name)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// laputaReplaceRoots returns the external local-replace repository roots the
// VIVY modfile depends on (currently the laputa monorepo submodules). Each
// returned root must be a git worktree declared in the host lock.
func laputaReplaceRoots(repoRoot string) ([]string, error) {
	replaces, err := localModfileReplaces(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		return nil, err
	}
	var roots []string
	seen := map[string]bool{}
	for _, replace := range replaces {
		target := replace.New
		if !filepath.IsAbs(target) {
			target = filepath.Join(repoRoot, target)
		}
		abs, err := filepath.Abs(target)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(abs, repoRoot+string(filepath.Separator)) || abs == repoRoot {
			continue
		}
		root, err := findGitRoot(abs)
		if err != nil {
			return nil, fmt.Errorf("sdk: external replacement %s => %s is not inside a git source repo: %w", replace.Old, replace.New, err)
		}
		if !seen[root] {
			seen[root] = true
			roots = append(roots, root)
		}
	}
	sort.Strings(roots)
	return roots, nil
}

// stageGoHostSources snapshots the tracked trees the consumer build resolves:
// the VIVY repo, the external sibling repos its local replaces need, and the
// host module itself.
func stageGoHostSources(repoRoot, laputaRoot string, inputs goHostInputs) (*goHostStaged, error) {
	root, err := os.MkdirTemp("", "vivy-go-host-sources-")
	if err != nil {
		return nil, err
	}
	staged := &goHostStaged{
		Root: root,
		Vivy: filepath.Join(root, "vivy"),
		Host: filepath.Join(root, "host"),
		Deps: filepath.Join(root, "deps"),
	}
	if err := stageSourceTree(repoRoot, staged.Vivy); err != nil {
		os.RemoveAll(root)
		return nil, fmt.Errorf("sdk: snapshot vivy sources: %w", err)
	}
	if err := stageSourceTree(laputaRoot, filepath.Join(staged.Deps, filepath.Base(laputaRoot))); err != nil {
		os.RemoveAll(root)
		return nil, fmt.Errorf("sdk: snapshot laputa sources: %w", err)
	}
	if err := stageSourceTree(inputs.Dir, staged.Host); err != nil {
		os.RemoveAll(root)
		return nil, fmt.Errorf("sdk: snapshot host sources: %w", err)
	}
	return staged, nil
}

// prepareConsumerModfile generates the temporary consumer modfile inside the
// staged host tree: the host's own requirements plus the full replacement
// closure an agent-vivy consumer needs but does not inherit.
func prepareConsumerModfile(staged *goHostStaged, repoRoot, laputaRoot string, inputs goHostInputs) error {
	modfile := filepath.Join(staged.Host, "consumer.mod")
	hostMod, err := os.ReadFile(filepath.Join(inputs.Dir, "go.mod"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(modfile, hostMod, 0o600); err != nil {
		return err
	}
	if sum, readErr := os.ReadFile(filepath.Join(inputs.Dir, "go.sum")); readErr == nil {
		if err := os.WriteFile(filepath.Join(staged.Host, "consumer.sum"), sum, 0o600); err != nil {
			return err
		}
	}
	// Drop any caller-supplied agent-vivy pin: pack owns that replacement.
	_ = goModEdit(staged.Host, modfile, "-dropreplace=agent-vivy")
	// Replacements are written relative to the staged host so the sealed
	// consumer modfile is identical no matter where the sources were staged.
	stagedRel := func(target string) (string, error) {
		rel, err := filepath.Rel(staged.Host, target)
		if err != nil {
			return "", err
		}
		return filepath.ToSlash(rel), nil
	}
	vivyRel, err := stagedRel(staged.Vivy)
	if err != nil {
		return err
	}
	var edits []string
	edits = append(edits, "-require=agent-vivy@v0.0.0", "-replace=agent-vivy="+vivyRel)
	vivyReplaces, err := localModfileReplaces(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		return err
	}
	laputaStaged := filepath.Join(staged.Deps, filepath.Base(laputaRoot))
	for _, replace := range vivyReplaces {
		target := replace.New
		if !filepath.IsAbs(target) {
			target = filepath.Join(repoRoot, target)
		}
		abs, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		var stagedTarget string
		switch {
		case abs == repoRoot:
			stagedTarget = staged.Vivy
		case strings.HasPrefix(abs, repoRoot+string(filepath.Separator)):
			rel, relErr := filepath.Rel(repoRoot, abs)
			if relErr != nil {
				return relErr
			}
			stagedTarget = filepath.Join(staged.Vivy, rel)
		default:
			root, rootErr := findGitRoot(abs)
			if rootErr != nil {
				return fmt.Errorf("sdk: replacement %s => %s escapes the declared source closure: %w", replace.Old, replace.New, rootErr)
			}
			if root != laputaRoot {
				return fmt.Errorf("sdk: replacement %s => %s resolves to undeclared source repo %s", replace.Old, replace.New, root)
			}
			rel, relErr := filepath.Rel(laputaRoot, abs)
			if relErr != nil {
				return relErr
			}
			stagedTarget = filepath.Join(laputaStaged, rel)
		}
		stagedTargetRel, relErr := stagedRel(stagedTarget)
		if relErr != nil {
			return relErr
		}
		edits = append(edits, "-replace="+replace.Old+"="+stagedTargetRel)
	}
	// The host's own local replaces may only point inside the host tree; they
	// are rewritten onto the staged snapshot so no path escapes the closure.
	hostReplaces, err := localModfileReplaces(modfile)
	if err != nil {
		return err
	}
	for _, replace := range hostReplaces {
		target := replace.New
		if !filepath.IsAbs(target) {
			target = filepath.Join(staged.Host, target)
		}
		abs, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		if abs != staged.Host && !strings.HasPrefix(abs, staged.Host+string(filepath.Separator)) {
			return fmt.Errorf("sdk: host replacement %s => %s escapes the staged host tree", replace.Old, replace.New)
		}
	}
	if err := goModEdit(staged.Host, modfile, edits...); err != nil {
		return err
	}
	staged.Modfile = modfile
	staged.ResolvedSum = filepath.Join(staged.Host, "consumer.sum")
	return nil
}

// resolveConsumerModules tidies the consumer modfile (adding the requires the
// host's imports need), downloads every module in the build list so
// consumer.sum is complete, and returns the resolved build list for sealing.
func resolveConsumerModules(staged *goHostStaged) ([]byte, error) {
	tidy := exec.Command("go", "mod", "tidy", "-modfile="+staged.Modfile)
	tidy.Dir = staged.Host
	if out, err := tidy.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("sdk: resolve consumer requires: %w: %s", err, strings.TrimSpace(string(out)))
	}
	download := exec.Command("go", "mod", "download", "-modfile="+staged.Modfile)
	download.Dir = staged.Host
	if out, err := download.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("sdk: resolve consumer dependencies: %w: %s", err, strings.TrimSpace(string(out)))
	}
	list := exec.Command("go", "list", "-mod=mod", "-modfile="+staged.Modfile, "-m", "-f", "{{.Path}} {{.Version}}", "all")
	list.Dir = staged.Host
	out, err := list.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("sdk: resolve consumer build list: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return bytes.TrimSpace(out), nil
}

// packGoHostArtifact publishes a sealed external-host Generation (W3-4):
// staged source closure, generated consumer modfile, sealed hostBuild
// provenance, headless compile of the real host package, then the artifact
// tree with build report and checksums.
func packGoHostArtifact(ctx context.Context, o packOptions, repoRoot string, recipe assemblyv1.Recipe, canonical []byte, uiInput assemblyv1.UIAssemblyInput, uiAssembly assemblyv1.UIAssembly, catalogs []assemblyv1.CatalogManifest, uiBuild builtWebUI, plan assemblyv1.AssemblyPlan, capabilityStates map[string]assemblyv1.CapabilityState, portSupport []generationconformance.PortSupportRecord, selectedConformanceResults []providerconformance.ConformanceResult, binder, runtimeSource []byte) (Artifact, error) {
	inputs := o.GoHost
	if inputs.Dir == "" {
		var err error
		inputs, err = validateGoHostArgs(o)
		if err != nil {
			return Artifact{}, err
		}
	}
	modulePath, err := goModulePath(inputs.Dir)
	if err != nil {
		return Artifact{}, fmt.Errorf("sdk: --host-dir module: %w", err)
	}
	if modulePath != inputs.LockDoc.Host.ModulePath {
		return Artifact{}, fmt.Errorf("sdk: --host-dir module is %s, lock declares %s", modulePath, inputs.LockDoc.Host.ModulePath)
	}
	laputaRoots, err := laputaReplaceRoots(repoRoot)
	if err != nil {
		return Artifact{}, err
	}
	if len(laputaRoots) != 1 {
		return Artifact{}, fmt.Errorf("sdk: expected exactly one external sibling source repo, found %d: %v", len(laputaRoots), laputaRoots)
	}
	laputaRoot := laputaRoots[0]
	if err := verifyGoHostLock(inputs, repoRoot, laputaRoot); err != nil {
		return Artifact{}, err
	}
	recipeRaw, err := os.ReadFile(o.Recipe)
	if err != nil {
		return Artifact{}, err
	}
	if sha256Hex(recipeRaw) != inputs.LockDoc.Recipe.SHA256 {
		return Artifact{}, fmt.Errorf("sdk: recipe content differs from --host-lock pin")
	}
	lockRaw, err := os.ReadFile(inputs.Lock)
	if err != nil {
		return Artifact{}, err
	}
	staged, err := stageGoHostSources(repoRoot, laputaRoot, inputs)
	if err != nil {
		return Artifact{}, err
	}
	defer os.RemoveAll(staged.Root)
	if err := prepareConsumerModfile(staged, repoRoot, laputaRoot, inputs); err != nil {
		return Artifact{}, err
	}
	buildList, err := resolveConsumerModules(staged)
	if err != nil {
		return Artifact{}, err
	}
	dependencyLocks, err := sealedFileInputs(staged.Host, "go.mod")
	if err != nil {
		return Artifact{}, err
	}
	if _, err := os.Stat(filepath.Join(staged.Host, "go.sum")); err == nil {
		hostSum, hashErr := sealedFileInputs(staged.Host, "go.sum")
		if hashErr != nil {
			return Artifact{}, hashErr
		}
		dependencyLocks["go.sum"] = hostSum["go.sum"]
	}
	canonicalMod, err := canonicalResolvedModfile(staged.Host, staged.Modfile)
	if err != nil {
		return Artifact{}, err
	}
	consumerSum, err := os.ReadFile(staged.ResolvedSum)
	if err != nil {
		return Artifact{}, fmt.Errorf("sdk: read resolved consumer sum: %w", err)
	}
	dependencyLocks["resolved-go.mod"] = sha256Hex(canonicalMod)
	dependencyLocks["resolved-go.sum"] = sha256Hex(consumerSum)
	dependencyLocks["resolved-build-list"] = sha256Hex(buildList)
	vivyLocks, err := sealedFileInputs(staged.Vivy, "go.mod", "go.sum")
	if err != nil {
		return Artifact{}, err
	}
	dependencyLocks["vivy-go.mod"] = vivyLocks["go.mod"]
	dependencyLocks["vivy-go.sum"] = vivyLocks["go.sum"]

	// The host identity is always resolved from the live checkout: the
	// tracked lock must not pin its own DIVA commit.
	hostCommit, err := gitHead(inputs.Dir)
	if err != nil {
		return Artifact{}, fmt.Errorf("sdk: resolve host commit: %w", err)
	}
	hostTree, err := hashSourceTree(inputs.Dir)
	if err != nil {
		return Artifact{}, fmt.Errorf("sdk: hash host source tree: %w", err)
	}

	assetsDir := filepath.Join(inputs.Dir, filepath.FromSlash(inputs.Assets))
	assetsDigest, err := hashUIArtifactTree(assetsDir)
	if err != nil {
		return Artifact{}, fmt.Errorf("sdk: hash --host-assets tree: %w", err)
	}
	goVersion, err := goToolchainVersion(staged.Host)
	if err != nil {
		return Artifact{}, err
	}
	hostBuild := &assemblyv1.HostBuild{
		Schema:     assemblyv1.HostBuildSchema,
		ModulePath: modulePath,
		Package:    "./" + inputs.Package,
		Source: assemblyv1.HostBuildSource{
			Repository: firstNonEmptyString(inputs.LockDoc.Host.Repository, gitRemoteURL(inputs.Dir)),
			Commit:     hostCommit,
			TreeSHA256: hostTree,
		},
		Assets:                assemblyv1.HostBuildAssets{Path: inputs.Assets, SHA256: assetsDigest},
		DependencyLockSHA256:  sha256Hex(lockRaw),
		ConsumerModfileSHA256: sha256Hex(canonicalMod),
		ConsumerSumSHA256:     sha256Hex(consumerSum),
		Tools: assemblyv1.HostBuildTools{
			Go:     goVersion,
			Wails:  lockToolVersion(inputs.LockDoc.Tools, "wails"),
			GOOS:   runtime.GOOS,
			GOARCH: runtime.GOARCH,
			CGO:    goEnvValue(staged.Host, "CGO_ENABLED") == "1",
		},
	}
	uiArtifacts := map[string]string{"ui/dist": uiBuild.Digest}
	for id, digest := range uiAssembly.Manifest.AssetHashes {
		uiArtifacts["ui/provider/"+id] = digest
	}
	manifest, manifestRaw, err := assemblyv1.SealManifest(plan, assemblyv1.SealInputs{
		SpecificationVersion: "vivy.module/v1", CompilerVersion: "plg-p9", SDKVersion: "v1",
		CanonicalRecipe: canonical, DependencyLocks: dependencyLocks, UIArtifacts: uiArtifacts,
		UI: &uiAssembly.Manifest, Catalogs: catalogs, CapabilityStates: capabilityStates,
		ContextSourcePolicies: assemblyv1.ContextSourcePoliciesForPlan(plan),
		RunObserverPolicies:   assemblyv1.RunObserverPoliciesForPlan(plan),
		ConformanceResults:    selectedConformanceResults,
		PortSupport:           portSupport,
		HostBuild:             hostBuild,
	})
	if err != nil {
		return Artifact{}, err
	}
	if _, err := os.Stat(o.Output); err == nil {
		return Artifact{}, fmt.Errorf("sdk: output already exists: %s", o.Output)
	} else if !os.IsNotExist(err) {
		return Artifact{}, err
	}
	parent := filepath.Dir(o.Output)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return Artifact{}, err
	}
	stage, err := os.MkdirTemp(parent, ".vivy-pack-")
	if err != nil {
		return Artifact{}, err
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(stage)
		}
	}()
	if err := os.WriteFile(filepath.Join(stage, "zz_assembly.go"), binder, 0o644); err != nil {
		return Artifact{}, err
	}
	if err := os.WriteFile(filepath.Join(stage, "ui-assembly.ts"), uiAssembly.Source, 0o644); err != nil {
		return Artifact{}, err
	}
	if err := os.WriteFile(filepath.Join(stage, "generation.json"), manifestRaw, 0o644); err != nil {
		return Artifact{}, err
	}
	frontendDir := filepath.Join(stage, "frontend")
	if err := copySourceTree(assetsDir, frontendDir); err != nil {
		return Artifact{}, fmt.Errorf("sdk: stage final host assets: %w", err)
	}
	// Publish the resolved consumer modfile/sum so the DIVA wrapper can run
	// Go builds and race tests against the exact sealed module closure
	// (replacements are staged-relative, so the file is location-independent).
	modfileRaw, err := os.ReadFile(staged.Modfile)
	if err != nil {
		return Artifact{}, err
	}
	if err := os.WriteFile(filepath.Join(stage, "consumer.mod"), modfileRaw, 0o644); err != nil {
		return Artifact{}, err
	}
	if err := os.WriteFile(filepath.Join(stage, "consumer.sum"), consumerSum, 0o644); err != nil {
		return Artifact{}, err
	}
	// Overlay the generated Assembly and sealed Manifest at the staged VIVY
	// paths the dependency compiler actually reads.
	overlayDir := filepath.Join(staged.Root, "overlay")
	if err := os.MkdirAll(overlayDir, 0o700); err != nil {
		return Artifact{}, err
	}
	overlayFile := filepath.Join(overlayDir, "overlay.json")
	zzStaged := filepath.Join(staged.Vivy, "internal/generated/assembly/zz_default.go")
	zzReplacement := filepath.Join(overlayDir, "zz_default.go")
	if err := os.WriteFile(zzReplacement, runtimeSource, 0o600); err != nil {
		return Artifact{}, err
	}
	overlayRaw, err := json.Marshal(map[string]any{"Replace": map[string]string{zzStaged: zzReplacement}})
	if err != nil {
		return Artifact{}, err
	}
	if err := os.WriteFile(overlayFile, overlayRaw, 0o600); err != nil {
		return Artifact{}, err
	}
	embedded := generation.FrameEmbeddedManifest(manifestRaw)
	manifestReplacement := filepath.Join(overlayDir, "manifest.go")
	if err := writeEmbeddedManifestOverlay(overlayFile, filepath.Join(staged.Vivy, "sdk/generation/manifest.go"), manifestReplacement, embedded); err != nil {
		return Artifact{}, fmt.Errorf("sdk: bind embedded Generation Manifest: %w", err)
	}
	binaryName := artifactHostBinaryName(runtime.GOOS, goHostBinaryBase(inputs))
	binary := filepath.Join(stage, binaryName)
	cmd := exec.CommandContext(ctx, "go", "build", "-modfile", staged.Modfile, "-mod=readonly", "-overlay", overlayFile, "-tags", "vivy_headless", "-o", binary, "./"+inputs.Package)
	cmd.Dir = staged.Host
	if output, buildErr := cmd.CombinedOutput(); buildErr != nil {
		return Artifact{}, fmt.Errorf("build go-host generation: %w: %s", buildErr, output)
	}
	report, err := goHostBuildReport(inputs, repoRoot, laputaRoot, staged, hostBuild, manifest.GenerationID)
	if err != nil {
		return Artifact{}, err
	}
	if err := os.WriteFile(filepath.Join(stage, "build-report.json"), report, 0o644); err != nil {
		return Artifact{}, err
	}
	if err := writeGoHostChecksums(stage, binaryName); err != nil {
		return Artifact{}, err
	}
	// Fail if any declared input moved while compiling.
	if err := verifyGoHostLock(inputs, repoRoot, laputaRoot); err != nil {
		return Artifact{}, fmt.Errorf("sdk: source changed during build: %w", err)
	}
	if err := os.Rename(stage, o.Output); err != nil {
		return Artifact{}, fmt.Errorf("publish generation: %w", err)
	}
	published = true
	return Artifact{Directory: o.Output, Binary: filepath.Join(o.Output, binaryName), Manifest: manifest}, nil
}

func lockToolVersion(tools map[string]any, name string) string {
	value, _ := tools[name].(string)
	return value
}

// goHostBuildReport renders the untracked resolved-identity report. The
// tracked source lock never pins its own host commit; the report records the
// identity resolved at build time instead.
func goHostBuildReport(inputs goHostInputs, repoRoot, laputaRoot string, staged *goHostStaged, build *assemblyv1.HostBuild, generationID string) ([]byte, error) {
	type sourceReport struct {
		Commit     string   `json:"commit"`
		TreeSHA256 string   `json:"treeSHA256"`
		Dirty      []string `json:"dirty,omitempty"`
	}
	source := func(dir string) (sourceReport, error) {
		head, err := gitHead(dir)
		if err != nil {
			return sourceReport{}, err
		}
		tree, err := hashSourceTree(dir)
		if err != nil {
			return sourceReport{}, err
		}
		dirty, err := gitDirtyFiles(dir)
		if err != nil {
			return sourceReport{}, err
		}
		return sourceReport{Commit: head, TreeSHA256: tree, Dirty: dirty}, nil
	}
	vivy, err := source(repoRoot)
	if err != nil {
		return nil, err
	}
	laputa, err := source(laputaRoot)
	if err != nil {
		return nil, err
	}
	host, err := source(inputs.Dir)
	if err != nil {
		return nil, err
	}
	report := map[string]any{
		"schema":       "vivy.go-host-report/v1",
		"release":      inputs.LockDoc.Release,
		"generationId": generationID,
		"resolved": map[string]any{
			"host":                  host,
			"vivy":                  vivy,
			"laputa":                laputa,
			"modulePath":            build.ModulePath,
			"package":               build.Package,
			"consumerModfileSHA256": build.ConsumerModfileSHA256,
			"consumerSumSHA256":     build.ConsumerSumSHA256,
			"dependencyLockSHA256":  build.DependencyLockSHA256,
			"tools":                 build.Tools,
		},
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// writeGoHostChecksums seals the artifact tree: binary, manifest and every
// staged frontend file get a sha256sum line. The checksum file itself and
// build-report.json stay out so neither becomes self-referential.
func writeGoHostChecksums(stage, binaryName string) error {
	var entries []string
	add := func(rel string) error {
		body, err := os.ReadFile(filepath.Join(stage, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		entries = append(entries, sha256Hex(body)+"  "+rel)
		return nil
	}
	for _, rel := range []string{binaryName, "generation.json", "ui-assembly.ts", "zz_assembly.go", "consumer.mod", "consumer.sum"} {
		if _, err := os.Stat(filepath.Join(stage, rel)); err == nil {
			if err := add(rel); err != nil {
				return err
			}
		}
	}
	frontend := filepath.Join(stage, "frontend")
	if err := filepath.WalkDir(frontend, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(stage, path)
		if err != nil {
			return err
		}
		return add(filepath.ToSlash(rel))
	}); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	sort.Strings(entries)
	return os.WriteFile(filepath.Join(stage, "checksums.sha256"), []byte(strings.Join(entries, "\n")+"\n"), 0o644)
}

// inspectGoHostArtifact verifies a sealed external Go-host artifact without
// executing its binary: the embedded Generation Manifest must match the
// sidecar byte-for-byte and the staged frontend tree must hash to the sealed
// hostBuild assets digest.
func inspectGoHostArtifact(dir string, manifestRaw []byte, manifest assemblyv1.GenerationManifest) (Artifact, error) {
	hostBuild := manifest.HostBuild
	base := filepath.Base(filepath.FromSlash(hostBuild.Package))
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = filepath.Base(hostBuild.ModulePath)
	}
	binary := filepath.Join(dir, artifactHostBinaryName(runtime.GOOS, base))
	binaryRaw, err := os.ReadFile(binary)
	if err != nil {
		return Artifact{}, fmt.Errorf("read go-host Generation Manifest binary: %w", err)
	}
	embeddedRaw, err := generation.ExtractEmbeddedManifest(binaryRaw)
	if err != nil {
		return Artifact{}, fmt.Errorf("inspect embedded Generation Manifest: %w", err)
	}
	embedded, err := assemblyv1.InspectManifest(embeddedRaw)
	if err != nil {
		return Artifact{}, err
	}
	if embedded.GenerationID != manifest.GenerationID || !bytes.Equal(embeddedRaw, manifestRaw) {
		return Artifact{}, fmt.Errorf("Generation Manifest is not bound to go-host executable")
	}
	if hostBuild.Assets.SHA256 != "" {
		actual, hashErr := hashUIArtifactTree(filepath.Join(dir, "frontend"))
		if hashErr != nil {
			return Artifact{}, fmt.Errorf("inspect go-host frontend artifact: %w", hashErr)
		}
		if actual != hostBuild.Assets.SHA256 {
			return Artifact{}, fmt.Errorf("go-host frontend hash mismatch: got %s, want %s", actual, hostBuild.Assets.SHA256)
		}
	}
	return Artifact{Directory: dir, Binary: binary, Manifest: manifest}, nil
}
