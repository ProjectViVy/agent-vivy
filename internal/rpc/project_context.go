package rpc

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"agent-vivy/internal/domain"
)

// Project-file context is intentionally smaller than image attachments. The
// body is a text snapshot captured before RunWithOptions; history RPCs expose
// only the metadata projection.
const (
	maxProjectContextBytes = 1 << 20
	maxProjectContextCount = 8
	maxProjectContextTotal = 4 << 20
	maxProjectContextList  = 200
)

type projectContext struct {
	Path    string
	Name    string
	Size    int64
	Content []byte
}

type projectContextResult struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type projectContextPathsParams struct {
	Paths []string `json:"paths"`
}

type projectContextListParams struct {
	Prefix string `json:"prefix,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type projectContextPathError struct {
	index  int
	public string
	cause  error
}

func (e *projectContextPathError) Error() string {
	if e == nil {
		return "invalid project file context"
	}
	if e.index < 0 {
		if e.public != "" {
			return e.public
		}
		return "project file context is unavailable"
	}
	return fmt.Sprintf("context %d: %s", e.index+1, e.public)
}

func (e *projectContextPathError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

var (
	errProjectContextRoot       = errors.New("project root is not configured")
	errProjectContextEmpty      = errors.New("path is empty")
	errProjectContextNUL        = errors.New("path contains NUL")
	errProjectContextAbsolute   = errors.New("path must be project-relative")
	errProjectContextTraversal  = errors.New("path traversal is not allowed")
	errProjectContextSensitive  = errors.New("path is sensitive")
	errProjectContextMissing    = errors.New("file cannot be opened")
	errProjectContextDirectory  = errors.New("path is not a regular file")
	errProjectContextSymlink    = errors.New("symlink paths are not allowed")
	errProjectContextTooLarge   = errors.New("file exceeds the size limit")
	errProjectContextBinary     = errors.New("file content is not UTF-8 text")
	errProjectContextListPrefix = errors.New("invalid project context prefix")
)

func (h *controlHandler) resolveProjectContext(request Request) (any, *Error) {
	var params projectContextPathsParams
	if rpcErr := decodeParams(request, &params); rpcErr != nil {
		return nil, rpcErr
	}
	if len(params.Paths) == 0 {
		return nil, &Error{Code: InvalidParams, Message: "paths is required"}
	}
	resolved, err := resolveProjectContexts(h.deps.ProjectRoot, params.Paths)
	if err != nil {
		return nil, &Error{Code: InvalidParams, Message: err.Error()}
	}
	return map[string]any{"contexts": projectContextMetadata(resolved)}, nil
}

func (h *controlHandler) listProjectContext(request Request) (any, *Error) {
	var params projectContextListParams
	if rpcErr := decodeParams(request, &params); rpcErr != nil {
		return nil, rpcErr
	}
	if strings.TrimSpace(h.deps.ProjectRoot) == "" {
		return nil, &Error{Code: MethodNotFound, Message: "project file context is not configured"}
	}
	limit := params.Limit
	if limit <= 0 || limit > maxProjectContextList {
		limit = maxProjectContextList
	}
	contexts, truncated, err := listProjectContexts(h.deps.ProjectRoot, params.Prefix, limit)
	if err != nil {
		message := "project context listing failed"
		if errors.Is(err, errProjectContextListPrefix) {
			message = "invalid project context prefix"
		}
		return nil, &Error{Code: InvalidParams, Message: message}
	}
	return map[string]any{"contexts": projectContextMetadata(contexts), "truncated": truncated}, nil
}

func projectContextMetadata(items []projectContext) []projectContextResult {
	if len(items) == 0 {
		return []projectContextResult{}
	}
	out := make([]projectContextResult, 0, len(items))
	for _, item := range items {
		out = append(out, projectContextResult{Path: item.Path, Name: item.Name, Size: item.Size})
	}
	return out
}

func projectContextDomainValues(items []projectContext) []domain.FileContext {
	if len(items) == 0 {
		return nil
	}
	out := make([]domain.FileContext, 0, len(items))
	for _, item := range items {
		out = append(out, domain.FileContext{
			Path: item.Path, Name: item.Name, Size: item.Size,
			Content: cloneProjectContextBytes(item.Content),
		})
	}
	return out
}

// resolveProjectContexts is the sole project-file filesystem seam. It opens
// the canonical root once, validates every path lexically and after symlink
// evaluation, then uses os.Root.Open for the race-resistant containment
// boundary. No caller-supplied path or body is echoed in errors/metadata.
func resolveProjectContexts(root string, paths []string) ([]projectContext, error) {
	if strings.TrimSpace(root) == "" {
		return nil, &projectContextPathError{index: -1, public: "project file context is unavailable", cause: errProjectContextRoot}
	}
	if len(paths) == 0 {
		return nil, nil
	}
	if len(paths) > maxProjectContextCount {
		return nil, &projectContextPathError{index: -1, public: fmt.Sprintf("at most %d project files are allowed", maxProjectContextCount), cause: errProjectContextTooLarge}
	}
	rootAbs, rootReal, rootHandle, err := openProjectContextRoot(root)
	if err != nil {
		return nil, &projectContextPathError{index: -1, public: "project file context is unavailable", cause: err}
	}
	defer rootHandle.Close()

	out := make([]projectContext, 0, len(paths))
	total := 0
	for index, raw := range paths {
		clean, err := cleanProjectContextPath(raw)
		if err != nil {
			return nil, &projectContextPathError{index: index, public: publicProjectContextPathError(err), cause: err}
		}
		if sensitiveProjectContextPath(clean) {
			return nil, &projectContextPathError{index: index, public: "path is sensitive", cause: errProjectContextSensitive}
		}
		// This preflight produces a stable error for a symlink/junction that
		// points outside the root. os.Root.Open below remains authoritative if
		// a path is renamed between the checks.
		candidateReal, err := filepath.EvalSymlinks(filepath.Join(rootAbs, clean))
		if err != nil {
			return nil, &projectContextPathError{index: index, public: "file cannot be opened", cause: fmt.Errorf("resolve project context: %w", err)}
		}
		candidateReal, err = filepath.Abs(candidateReal)
		if err != nil || !pathWithin(rootReal, candidateReal) {
			if err == nil {
				err = errors.New("resolved project context is outside project root")
			}
			return nil, &projectContextPathError{index: index, public: "path escapes the project", cause: err}
		}
		canonicalRel, err := filepath.Rel(rootReal, candidateReal)
		if err != nil {
			return nil, &projectContextPathError{index: index, public: "file cannot be opened", cause: fmt.Errorf("relativize project context: %w", err)}
		}
		if sensitiveProjectContextPath(canonicalRel) {
			return nil, &projectContextPathError{index: index, public: "path is sensitive", cause: errProjectContextSensitive}
		}
		lexicalReal, err := filepath.Abs(filepath.Join(rootReal, clean))
		if err != nil || !sameProjectContextPath(lexicalReal, candidateReal) {
			if err == nil {
				err = errProjectContextSymlink
			}
			return nil, &projectContextPathError{index: index, public: "symlink paths are not allowed", cause: err}
		}
		file, err := rootHandle.Open(clean)
		if err != nil {
			return nil, &projectContextPathError{index: index, public: "file cannot be opened", cause: fmt.Errorf("open project context within root: %w", err)}
		}
		info, err := file.Stat()
		if err != nil {
			_ = file.Close()
			return nil, &projectContextPathError{index: index, public: "file cannot be opened", cause: fmt.Errorf("stat project context: %w", err)}
		}
		if !info.Mode().IsRegular() {
			_ = file.Close()
			return nil, &projectContextPathError{index: index, public: "path is not a regular file", cause: errProjectContextDirectory}
		}
		if info.Size() > maxProjectContextBytes {
			_ = file.Close()
			return nil, &projectContextPathError{index: index, public: "file exceeds the 1 MiB limit", cause: errProjectContextTooLarge}
		}
		data, readErr := io.ReadAll(io.LimitReader(file, maxProjectContextBytes+1))
		closeErr := file.Close()
		if readErr != nil {
			return nil, &projectContextPathError{index: index, public: "file cannot be opened", cause: fmt.Errorf("read project context: %w", readErr)}
		}
		if closeErr != nil {
			return nil, &projectContextPathError{index: index, public: "file cannot be opened", cause: fmt.Errorf("close project context: %w", closeErr)}
		}
		// Re-check the named entry after reading and compare it with the opened
		// file identity. This rejects a concurrent replacement instead of ever
		// accepting bytes read through a transient sensitive symlink.
		postReal, postErr := filepath.EvalSymlinks(filepath.Join(rootReal, clean))
		if postErr != nil {
			return nil, &projectContextPathError{index: index, public: "file changed while reading", cause: fmt.Errorf("recheck project context: %w", postErr)}
		}
		postReal, postErr = filepath.Abs(postReal)
		postInfo, statErr := os.Stat(filepath.Join(rootReal, clean))
		postRel, relErr := filepath.Rel(rootReal, postReal)
		if postErr != nil || statErr != nil || relErr != nil || !sameProjectContextPath(lexicalReal, postReal) || sensitiveProjectContextPath(postRel) || !os.SameFile(info, postInfo) {
			cause := errProjectContextSymlink
			if postErr != nil {
				cause = postErr
			} else if statErr != nil {
				cause = statErr
			} else if relErr != nil {
				cause = relErr
			}
			return nil, &projectContextPathError{index: index, public: "file changed while reading", cause: cause}
		}
		if len(data) > maxProjectContextBytes {
			return nil, &projectContextPathError{index: index, public: "file exceeds the 1 MiB limit", cause: errProjectContextTooLarge}
		}
		if !validProjectContextText(data) {
			return nil, &projectContextPathError{index: index, public: "file content is not UTF-8 text", cause: errProjectContextBinary}
		}
		total += len(data)
		if total > maxProjectContextTotal {
			return nil, &projectContextPathError{index: index, public: "project file context total exceeds the 4 MiB limit", cause: errProjectContextTooLarge}
		}
		out = append(out, projectContext{
			Path: filepath.ToSlash(clean), Name: safeProjectContextName(filepath.Base(clean)),
			Size: int64(len(data)), Content: cloneProjectContextBytes(data),
		})
	}
	return out, nil
}

func openProjectContextRoot(root string) (string, string, *os.Root, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", "", nil, fmt.Errorf("resolve project root: %w", err)
	}
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", "", nil, fmt.Errorf("resolve project root symlinks: %w", err)
	}
	rootReal, err = filepath.Abs(rootReal)
	if err != nil {
		return "", "", nil, fmt.Errorf("resolve project root path: %w", err)
	}
	info, err := os.Stat(rootReal)
	if err != nil {
		return "", "", nil, fmt.Errorf("inspect project root: %w", err)
	}
	if !info.IsDir() {
		return "", "", nil, errors.New("project root is not a directory")
	}
	handle, err := os.OpenRoot(rootReal)
	if err != nil {
		return "", "", nil, fmt.Errorf("open project root: %w", err)
	}
	return rootAbs, rootReal, handle, nil
}

func cleanProjectContextPath(raw string) (string, error) {
	if raw == "" {
		return "", errProjectContextEmpty
	}
	if strings.IndexByte(raw, 0) >= 0 {
		return "", errProjectContextNUL
	}
	if filepath.IsAbs(raw) || filepath.VolumeName(raw) != "" || strings.Contains(raw, ":") || strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, `\`) {
		return "", errProjectContextAbsolute
	}
	normalized := strings.ReplaceAll(raw, `\`, "/")
	parts := strings.Split(normalized, "/")
	for _, part := range parts {
		if part == ".." {
			return "", errProjectContextTraversal
		}
	}
	clean := filepath.Clean(filepath.FromSlash(normalized))
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errProjectContextAbsolute
	}
	return clean, nil
}

func publicProjectContextPathError(err error) string {
	switch {
	case errors.Is(err, errProjectContextEmpty):
		return "path is required"
	case errors.Is(err, errProjectContextNUL):
		return "path contains an invalid character"
	case errors.Is(err, errProjectContextAbsolute):
		return "path must be project-relative"
	case errors.Is(err, errProjectContextTraversal):
		return "path traversal is not allowed"
	default:
		return "invalid project-relative path"
	}
}

func sensitiveProjectContextPath(clean string) bool {
	parts := strings.FieldsFunc(clean, func(r rune) bool { return r == '/' || r == '\\' })
	for _, part := range parts {
		lower := strings.ToLower(part)
		if lower == ".git" || lower == ".hg" || lower == ".svn" || lower == "node_modules" || lower == ".ssh" || lower == ".aws" {
			return true
		}
		if lower == ".env" || strings.HasPrefix(lower, ".env.") || lower == ".netrc" || lower == ".npmrc" || lower == ".pypirc" || lower == "secrets" || lower == "secret" || lower == "credential" || lower == "credentials" || lower == "password" || lower == "token" || lower == "keys" || lower == "keys.txt" {
			return true
		}
		for _, prefix := range []string{"secret.", "secret-", "secret_", "credential.", "credential-", "credential_", "password.", "password-", "password_", "token.", "token-", "token_"} {
			if strings.HasPrefix(lower, prefix) {
				return true
			}
		}
	}
	base := strings.ToLower(filepath.Base(clean))
	for _, suffix := range []string{".pem", ".key", ".p12", ".pfx", ".der", ".secret", ".secrets", ".credential", ".credentials", ".password", ".token"} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	for _, name := range []string{"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519", "credentials.json", "service-account.json"} {
		if base == name {
			return true
		}
	}
	return false
}

func validProjectContextText(data []byte) bool {
	if !utf8.Valid(data) {
		return false
	}
	for _, r := range string(data) {
		if r == 0 || (unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' && r != '\f') {
			return false
		}
	}
	return true
}

func cloneProjectContextBytes(data []byte) []byte {
	out := make([]byte, len(data))
	copy(out, data)
	return out
}

func sameProjectContextPath(a, b string) bool {
	if filepath.Separator == '\\' {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func safeProjectContextName(name string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(name))
	runes := []rune(name)
	if len(runes) > 256 {
		name = string(runes[:256])
	}
	if strings.TrimSpace(name) == "" {
		return "file"
	}
	return name
}

func listProjectContexts(root, prefix string, limit int) ([]projectContext, bool, error) {
	_, rootReal, rootHandle, err := openProjectContextRoot(root)
	if err != nil {
		return nil, false, err
	}
	_ = rootHandle.Close()
	cleanPrefix := ""
	if strings.TrimSpace(prefix) != "" {
		cleanPrefix, err = cleanProjectContextPath(prefix)
		if err != nil || sensitiveProjectContextPath(cleanPrefix) {
			return nil, false, errProjectContextListPrefix
		}
		info, statErr := os.Stat(filepath.Join(rootReal, cleanPrefix))
		if statErr != nil || !info.IsDir() {
			return nil, false, errProjectContextListPrefix
		}
	}
	items := make([]projectContext, 0, min(limit, 32))
	total := 0
	truncated := false
	walkErr := filepath.WalkDir(rootReal, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != rootReal && cleanPrefix != "" {
			rel, relErr := filepath.Rel(rootReal, path)
			if relErr != nil {
				return relErr
			}
			prefixPath := filepath.FromSlash(cleanPrefix)
			if rel != prefixPath && !strings.HasPrefix(rel, prefixPath+string(filepath.Separator)) {
				if entry.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			rel, relErr := filepath.Rel(rootReal, path)
			if relErr != nil {
				return relErr
			}
			if rel != "." && sensitiveProjectContextPath(rel) {
				return fs.SkipDir
			}
			return nil
		}
		if len(items) >= limit {
			truncated = true
			return fs.SkipAll
		}
		rel, relErr := filepath.Rel(rootReal, path)
		if relErr != nil {
			return relErr
		}
		resolved, resolveErr := resolveProjectContexts(root, []string{rel})
		if resolveErr != nil {
			// Listing is metadata discovery: unreadable, sensitive, binary,
			// oversized, and escaping entries are omitted rather than making a
			// whole directory unusable. Explicit resolve remains fail-closed.
			return nil
		}
		if total+len(resolved[0].Content) > maxProjectContextTotal {
			truncated = true
			return fs.SkipAll
		}
		total += len(resolved[0].Content)
		items = append(items, resolved[0])
		return nil
	})
	if walkErr != nil {
		return nil, false, walkErr
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	return items, truncated, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
