package runtime

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"agent-vivy/internal/tools"
)

var _ tools.GrepOperations = (*EinoFilesystemBackend)(nil)

const (
	// grepTimeout bounds one ripgrep subprocess; the pure-Go fallback walk
	// is inherently bounded by workspace size and result caps.
	grepTimeout = 30 * time.Second
	// grepMaxOutputBytes caps the ripgrep stdout buffer before parsing.
	grepMaxOutputBytes = 8 << 20
)

// Grep searches workspace content: ripgrep first (native .gitignore and
// hidden-file handling), the bounded pure-Go walk on any rg failure.
func (b *EinoFilesystemBackend) Grep(ctx context.Context, req tools.GrepRequest) (tools.GrepResult, error) {
	root, base, err := b.searchRoot(ctx, req.Path)
	if err != nil {
		return tools.GrepResult{}, err
	}
	re, err := regexp.Compile(req.Pattern)
	if err != nil {
		return tools.GrepResult{}, fmt.Errorf("filesystem: invalid grep pattern: %w", err)
	}
	if b.rgPath != "" {
		if res, err := b.rgGrep(ctx, base, root, req); err == nil {
			return res, nil
		} else if ctx.Err() != nil {
			return tools.GrepResult{}, err
		}
	}
	return b.goGrep(base, root, re, req.Include)
}

// Glob matches workspace paths against a doublestar pattern (`**` recursion),
// newest first, never descending into ignored directories.
func (b *EinoFilesystemBackend) Glob(ctx context.Context, req tools.GlobRequest) (tools.GlobResult, error) {
	root, base, err := b.searchRoot(ctx, req.Path)
	if err != nil {
		return tools.GlobResult{}, err
	}
	for _, part := range strings.Split(filepath.ToSlash(req.Pattern), "/") {
		if part == ".." {
			return tools.GlobResult{}, fmt.Errorf("filesystem: glob pattern must stay inside the workspace")
		}
	}
	pattern := filepath.ToSlash(filepath.Clean("/" + req.Pattern))[1:]
	if pattern == "" || pattern == "." || strings.HasPrefix(pattern, "../") || pattern == ".." {
		return tools.GlobResult{}, fmt.Errorf("filesystem: glob pattern must stay inside the workspace")
	}
	type hit struct {
		info tools.GlobFileInfo
		mod  int64
	}
	var hits []hit
	truncated := false
	err = filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			if path != base && isIgnoredDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel := displayPath(root, path)
		if !globPatternMatches(pattern, filepath.ToSlash(rel), entry.Name()) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		hits = append(hits, hit{
			info: tools.GlobFileInfo{Path: rel, Size: info.Size(), ModifiedAt: info.ModTime().UTC().Format("2006-01-02T15:04:05Z07:00")},
			mod:  info.ModTime().UnixNano(),
		})
		return nil
	})
	if err != nil && !errors.Is(err, errSearchLimit) {
		return tools.GlobResult{}, fmt.Errorf("filesystem: glob: %w", err)
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].mod != hits[j].mod {
			return hits[i].mod > hits[j].mod
		}
		return hits[i].info.Path < hits[j].info.Path
	})
	if len(hits) > b.maxResults {
		hits = hits[:b.maxResults]
		truncated = true
	}
	out := make([]tools.GlobFileInfo, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.info)
	}
	return tools.GlobResult{Files: out, Truncated: truncated}, nil
}

func (b *EinoFilesystemBackend) searchRoot(ctx context.Context, path string) (string, string, error) {
	if err := b.validateSandboxPath(ctx, filesystemRunID(ctx), path, FileOpRead, false); err != nil {
		return "", "", err
	}
	return b.resolve(ctx, filesystemRunID(ctx), path, false)
}

// rgGrep runs one ripgrep invocation with cwd at the search root and parses
// its `path:line:content` stream. Errors surface to the caller for fallback.
func (b *EinoFilesystemBackend) rgGrep(ctx context.Context, base, root string, req tools.GrepRequest) (tools.GrepResult, error) {
	gctx, cancel := context.WithTimeout(ctx, grepTimeout)
	defer cancel()
	args := []string{"-n", "--no-heading", "--no-messages", "--no-require-git", "--path-separator", "/"}
	if req.Include != "" {
		args = append(args, "--glob", req.Include)
	}
	args = append(args, "--", req.Pattern)
	cmd := exec.CommandContext(gctx, b.rgPath, args...)
	cmd.Dir = base
	env, err := safeCommandEnv(nil)
	if err != nil {
		return tools.GrepResult{}, err
	}
	cmd.Env = env
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return tools.GrepResult{}, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return tools.GrepResult{}, err
	}
	out := make([]tools.GrepMatch, 0, 16)
	truncated := false
	stopped := false
	counting := &countingReader{r: stdout}
	scanner := bufio.NewScanner(counting)
	for scanner.Scan() {
		if stopped {
			continue // keep draining so rg never blocks on a full pipe
		}
		if len(out) >= b.maxResults || counting.n >= grepMaxOutputBytes {
			truncated = true
			stopped = true
			cancel() // stop rg early; the pipe keeps draining below
			continue
		}
		if match, ok := parseRgLine(scanner.Text()); ok {
			out = append(out, match)
		}
	}
	if err := scanner.Err(); err != nil {
		return tools.GrepResult{}, err
	}
	waitErr := cmd.Wait()
	if waitErr != nil && !isRgNoMatches(waitErr) && !stopped {
		return tools.GrepResult{}, fmt.Errorf("filesystem: rg: %v (%s)", waitErr, strings.TrimSpace(stderr.String()))
	}
	for i := range out {
		out[i].Path = workspaceRelative(root, base, out[i].Path)
	}
	return tools.GrepResult{Matches: out, Truncated: truncated}, nil
}

// countingReader tracks drained bytes so a full stdout buffer reads as
// truncation without touching the pipe after Wait.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func parseRgLine(line string) (tools.GrepMatch, bool) {
	parts := strings.SplitN(line, ":", 3)
	if len(parts) != 3 {
		return tools.GrepMatch{}, false
	}
	lineNo := 0
	for _, ch := range parts[1] {
		if ch < '0' || ch > '9' {
			return tools.GrepMatch{}, false
		}
		lineNo = lineNo*10 + int(ch-'0')
	}
	return tools.GrepMatch{Path: parts[0], Line: lineNo, Content: trimResultLine(parts[2])}, lineNo > 0
}

// isRgNoMatches distinguishes rg's "no matches" exit (1) from real errors.
func isRgNoMatches(err error) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode() == 1
	}
	return false
}

// workspaceRelative maps an rg-reported path (relative to the search root)
// to a workspace-relative display path.
func workspaceRelative(root, base, rgPath string) string {
	if filepath.IsAbs(rgPath) {
		return displayPath(root, rgPath)
	}
	joined := filepath.Join(base, filepath.FromSlash(rgPath))
	return displayPath(root, joined)
}

// goGrep is the bounded pure-Go fallback walk: ignored directories are
// pruned, binaries and oversized files are skipped, matches are capped.
func (b *EinoFilesystemBackend) goGrep(base, root string, re *regexp.Regexp, include string) (tools.GrepResult, error) {
	out := make([]tools.GrepMatch, 0)
	truncated := false
	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != base && isIgnoredDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel := displayPath(root, path)
		if include != "" && !globPatternMatches(filepath.ToSlash(include), filepath.ToSlash(rel), entry.Name()) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) > b.maxFileBytes || isBinary(data) {
			return nil
		}
		for lineNo, line := range strings.Split(string(data), "\n") {
			if !re.MatchString(line) {
				continue
			}
			if len(out) >= b.maxResults {
				truncated = true
				return errSearchLimit
			}
			out = append(out, tools.GrepMatch{Path: rel, Line: lineNo + 1, Content: trimResultLine(line)})
		}
		return nil
	})
	if err != nil && !errors.Is(err, errSearchLimit) {
		return tools.GrepResult{}, fmt.Errorf("filesystem: grep: %w", err)
	}
	return tools.GrepResult{Matches: out, Truncated: truncated}, nil
}

// globPatternMatches applies doublestar to the full relative path and, for
// slash-free patterns, the base name — `*.go` behaves like rg's glob filter.
func globPatternMatches(pattern, relative, base string) bool {
	if ok, _ := doublestar.Match(pattern, relative); ok {
		return true
	}
	if !strings.Contains(pattern, "/") {
		if ok, _ := doublestar.Match(pattern, base); ok {
			return true
		}
		return false
	}
	return false
}
