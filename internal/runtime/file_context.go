package runtime

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"agent-vivy/internal/domain"
)

// File-context limits are deliberately repeated at the runtime boundary even
// though the RPC resolver applies the same values. RunWithOptions is also used
// by in-process callers, so it must not trust a caller-provided snapshot to
// bypass the model-context bound or binary-text guard.
const (
	maxFileContextBytes = 1 << 20
	maxFileContextCount = 8
	maxFileContextTotal = 4 << 20
)

var (
	errFileContextPath      = errors.New("runtime: file context path must be project-relative")
	errFileContextTooMany   = errors.New("runtime: too many file contexts")
	errFileContextTooLarge  = errors.New("runtime: file context exceeds the size limit")
	errFileContextTotal     = errors.New("runtime: file context total exceeds the size limit")
	errFileContextBinary    = errors.New("runtime: file context is not UTF-8 text")
	errFileContextSensitive = errors.New("runtime: sensitive file context is not allowed")
)

// normalizeFileContexts validates and defensively copies server-resolved
// snapshots before they become durable message state. It does not touch the
// host filesystem: path containment and reading belong to the RPC resolver.
func normalizeFileContexts(in []domain.FileContext) ([]domain.FileContext, error) {
	if len(in) == 0 {
		return nil, nil
	}
	if len(in) > maxFileContextCount {
		return nil, errFileContextTooMany
	}
	out := make([]domain.FileContext, 0, len(in))
	total := 0
	for _, item := range in {
		if !validFileContextPath(item.Path) {
			return nil, errFileContextPath
		}
		if sensitiveFileContextPath(item.Path) {
			return nil, errFileContextSensitive
		}
		if item.Size < 0 || item.Size != int64(len(item.Content)) || len(item.Content) > maxFileContextBytes {
			return nil, errFileContextTooLarge
		}
		total += len(item.Content)
		if total > maxFileContextTotal {
			return nil, errFileContextTotal
		}
		if !validTextSnapshot(item.Content) {
			return nil, errFileContextBinary
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = filepath.Base(filepath.FromSlash(item.Path))
		}
		if !validDisplayName(name) {
			return nil, fmt.Errorf("runtime: invalid file context name")
		}
		out = append(out, domain.FileContext{
			Path:    filepath.ToSlash(filepath.Clean(filepath.FromSlash(item.Path))),
			Name:    name,
			Size:    int64(len(item.Content)),
			Content: cloneFileContextBytes(item.Content),
		})
	}
	return out, nil
}

func validFileContextPath(raw string) bool {
	if raw == "" || strings.IndexByte(raw, 0) >= 0 || strings.Contains(raw, ":") {
		return false
	}
	if filepath.IsAbs(raw) || strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, `\`) {
		return false
	}
	normalized := strings.ReplaceAll(raw, `\`, "/")
	for _, part := range strings.Split(normalized, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	clean := filepath.Clean(filepath.FromSlash(normalized))
	return clean != "." && !filepath.IsAbs(clean) && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

// Keep the runtime boundary fail-closed even when an in-process caller skips
// RPC resolution. The RPC resolver applies the same policy before it reads
// the project root; this duplicate check protects direct RunWithOptions users
// from persisting a caller-supplied secret snapshot.
func sensitiveFileContextPath(path string) bool {
	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	for _, part := range parts {
		lower := strings.ToLower(part)
		if lower == ".git" || lower == ".hg" || lower == ".svn" || lower == "node_modules" || lower == ".ssh" || lower == ".aws" {
			return true
		}
		if lower == ".env" || strings.HasPrefix(lower, ".env.") || lower == ".netrc" || lower == ".npmrc" || lower == ".pypirc" || lower == "secret" || lower == "secrets" || lower == "credential" || lower == "credentials" || lower == "password" || lower == "token" || lower == "keys" || lower == "keys.txt" {
			return true
		}
		for _, prefix := range []string{"secret.", "secret-", "secret_", "credential.", "credential-", "credential_", "password.", "password-", "password_", "token.", "token-", "token_"} {
			if strings.HasPrefix(lower, prefix) {
				return true
			}
		}
	}
	base := strings.ToLower(filepath.Base(filepath.FromSlash(path)))
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

func validTextSnapshot(data []byte) bool {
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

func validDisplayName(name string) bool {
	if name == "" || len([]rune(name)) > 256 {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func cloneFileContexts(in []domain.FileContext) []domain.FileContext {
	if len(in) == 0 {
		return nil
	}
	out := make([]domain.FileContext, len(in))
	for i, item := range in {
		out[i] = item
		out[i].Content = cloneFileContextBytes(item.Content)
	}
	return out
}

func cloneFileContextBytes(data []byte) []byte {
	out := make([]byte, len(data))
	copy(out, data)
	return out
}
