package rpc

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"agent-vivy/internal/domain"
)

// projectAttachment is the server-owned result of resolving one project
// relative path. Metadata is safe to return to a terminal face; Data is only
// used inside turn/start to build the existing domain.Attachment value.
type projectAttachment struct {
	Path     string
	Name     string
	MimeType string
	Size     int64
	Data     []byte
}

type attachmentPathsParams struct {
	AttachmentPaths []string `json:"attachment_paths"`
	// Paths is accepted as a compatibility spelling for read-only clients;
	// turn/start remains canonical on attachment_paths.
	Paths []string `json:"paths,omitempty"`
}

type attachmentPathResult struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

func (h *controlHandler) resolveAttachments(request Request) (any, *Error) {
	var params attachmentPathsParams
	if rpcErr := decodeParams(request, &params); rpcErr != nil {
		return nil, rpcErr
	}
	paths := params.AttachmentPaths
	if len(paths) == 0 {
		paths = params.Paths
	}
	if len(paths) == 0 {
		return nil, &Error{Code: InvalidParams, Message: "attachment_paths is required"}
	}
	resolved, err := resolveProjectAttachments(h.deps.ProjectRoot, paths)
	if err != nil {
		return nil, &Error{Code: InvalidParams, Message: err.Error()}
	}
	result := make([]attachmentPathResult, 0, len(resolved))
	for _, item := range resolved {
		result = append(result, attachmentPathResult{Path: item.Path, Name: item.Name, MimeType: item.MimeType, Size: item.Size})
	}
	return map[string]any{"attachments": result}, nil
}

// attachmentPathError deliberately contains no user supplied path in its
// public message. The wrapped cause is retained for callers and diagnostics,
// while RPC clients receive only a stable index/reason that cannot disclose
// host filesystem layout.
type attachmentPathError struct {
	index  int
	public string
	cause  error
}

func (e *attachmentPathError) Error() string {
	if e == nil {
		return "invalid image attachment path"
	}
	if e.index < 0 {
		if e.public != "" {
			return e.public
		}
		return "image attachments unavailable"
	}
	return fmt.Sprintf("attachment %d: %s", e.index+1, e.public)
}

func (e *attachmentPathError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

var (
	errAttachmentProjectRoot   = errors.New("project root is not configured")
	errAttachmentPathEmpty     = errors.New("path is empty")
	errAttachmentPathNUL       = errors.New("path contains NUL")
	errAttachmentPathAbsolute  = errors.New("path must be project-relative")
	errAttachmentPathTraversal = errors.New("path traversal is not allowed")
	errAttachmentPathMissing   = errors.New("file cannot be opened")
	errAttachmentPathDirectory = errors.New("path is not a regular file")
	errAttachmentPathTooLarge  = errors.New("image exceeds the size limit")
	errAttachmentPathNotImage  = errors.New("file content is not a supported image")
)

// resolveProjectAttachments is the one filesystem seam for TUI image
// attachments. Faces pass only relative names; the control plane resolves,
// validates, reads and sniffs them under the injected code project root.
// Neither the shared TUI nor a packed face receives a filesystem grant.
func resolveProjectAttachments(root string, paths []string) ([]projectAttachment, error) {
	if strings.TrimSpace(root) == "" {
		return nil, &attachmentPathError{index: -1, public: "image attachments unavailable", cause: errAttachmentProjectRoot}
	}
	if len(paths) == 0 {
		return nil, nil
	}
	if len(paths) > maxAttachmentCount {
		return nil, &attachmentPathError{index: -1, public: fmt.Sprintf("at most %d image attachments are allowed", maxAttachmentCount)}
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, &attachmentPathError{index: -1, public: "image attachments unavailable", cause: fmt.Errorf("resolve project root: %w", err)}
	}
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, &attachmentPathError{index: -1, public: "image attachments unavailable", cause: fmt.Errorf("resolve project root symlinks: %w", err)}
	}
	rootInfo, err := os.Stat(rootReal)
	if err != nil {
		return nil, &attachmentPathError{index: -1, public: "image attachments unavailable", cause: fmt.Errorf("inspect project root: %w", err)}
	}
	if !rootInfo.IsDir() {
		return nil, &attachmentPathError{index: -1, public: "image attachments unavailable", cause: errors.New("project root is not a directory")}
	}
	rootReal, _ = filepath.Abs(rootReal)
	rootHandle, err := os.OpenRoot(rootReal)
	if err != nil {
		return nil, &attachmentPathError{index: -1, public: "image attachments unavailable", cause: fmt.Errorf("open project root: %w", err)}
	}
	defer rootHandle.Close()

	out := make([]projectAttachment, 0, len(paths))
	for index, raw := range paths {
		clean, err := cleanProjectRelativePath(raw)
		if err != nil {
			return nil, &attachmentPathError{index: index, public: publicAttachmentPathError(err), cause: err}
		}
		// Preflight the current target only to produce a precise client error.
		// The subsequent os.Root.Open is still the authoritative race-safe
		// containment operation.
		candidateReal, err := filepath.EvalSymlinks(filepath.Join(rootAbs, clean))
		if err != nil {
			return nil, &attachmentPathError{index: index, public: "file cannot be opened", cause: fmt.Errorf("resolve attachment: %w", err)}
		}
		candidateReal, err = filepath.Abs(candidateReal)
		if err != nil {
			return nil, &attachmentPathError{index: index, public: "file cannot be opened", cause: fmt.Errorf("resolve attachment path: %w", err)}
		}
		if !pathWithin(rootReal, candidateReal) {
			return nil, &attachmentPathError{index: index, public: "path escapes the project", cause: errors.New("resolved attachment is outside project root")}
		}
		// os.Root resolves every component relative to an open directory handle
		// and refuses symlink/junction escapes, including rename races between
		// validation and open. This is the security boundary; lexical checks
		// above exist to provide stable, non-disclosing client errors.
		file, err := rootHandle.Open(clean)
		if err != nil {
			return nil, &attachmentPathError{index: index, public: "file cannot be opened", cause: fmt.Errorf("open attachment within project root: %w", err)}
		}
		info, err := file.Stat()
		if err != nil {
			_ = file.Close()
			return nil, &attachmentPathError{index: index, public: "file cannot be opened", cause: fmt.Errorf("stat attachment: %w", err)}
		}
		if !info.Mode().IsRegular() {
			_ = file.Close()
			return nil, &attachmentPathError{index: index, public: "path is not a regular file", cause: errAttachmentPathDirectory}
		}
		if info.Size() <= 0 {
			_ = file.Close()
			return nil, &attachmentPathError{index: index, public: "file content is not a supported image", cause: errAttachmentPathNotImage}
		}
		if info.Size() > maxAttachmentBytes {
			_ = file.Close()
			return nil, &attachmentPathError{index: index, public: "image exceeds the 5 MiB limit", cause: errAttachmentPathTooLarge}
		}
		data, readErr := io.ReadAll(io.LimitReader(file, maxAttachmentBytes+1))
		closeErr := file.Close()
		if readErr != nil {
			return nil, &attachmentPathError{index: index, public: "file cannot be opened", cause: fmt.Errorf("read attachment: %w", readErr)}
		}
		if closeErr != nil {
			return nil, &attachmentPathError{index: index, public: "file cannot be opened", cause: fmt.Errorf("close attachment: %w", closeErr)}
		}
		if len(data) > maxAttachmentBytes {
			return nil, &attachmentPathError{index: index, public: "image exceeds the 5 MiB limit", cause: errAttachmentPathTooLarge}
		}
		mime := sniffAttachmentMIME(data)
		if !attachmentMimeWhitelist[mime] {
			return nil, &attachmentPathError{index: index, public: "file content is not a supported image", cause: fmt.Errorf("%w: detected MIME %q is not allowed", errAttachmentPathNotImage, mime)}
		}

		out = append(out, projectAttachment{
			Path:     filepath.ToSlash(clean),
			Name:     safeAttachmentName(filepath.Base(clean)),
			MimeType: mime,
			Size:     int64(len(data)),
			Data:     data,
		})
	}
	return out, nil
}

// safeAttachmentName keeps filenames inert when projected into terminal
// chips or persisted history. The path used for the next server-side resolve
// remains exact and separate; only display metadata is sanitized and bounded.
func safeAttachmentName(name string) string {
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
		return "image"
	}
	return name
}

func cleanProjectRelativePath(raw string) (string, error) {
	if raw == "" {
		return "", errAttachmentPathEmpty
	}
	if strings.IndexByte(raw, 0) >= 0 {
		return "", errAttachmentPathNUL
	}
	// filepath.IsAbs/VolumeName are platform-aware, but packed faces may be
	// used against a server with different path syntax in tests or over a
	// remote seam. Reject both slash families and drive/UNC forms explicitly.
	if filepath.IsAbs(raw) || filepath.VolumeName(raw) != "" ||
		strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "\\") ||
		(len(raw) >= 2 && raw[1] == ':') {
		return "", errAttachmentPathAbsolute
	}
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return "", errAttachmentPathTraversal
		}
	}
	// Treat either separator as a separator for the server-side resolver. A
	// caller cannot smuggle a Windows-style traversal through a Unix host.
	normalized := strings.ReplaceAll(raw, "\\", "/")
	clean := filepath.Clean(filepath.FromSlash(normalized))
	if clean == "." || clean == string(filepath.Separator) || filepath.IsAbs(clean) {
		return "", errAttachmentPathAbsolute
	}
	return clean, nil
}

func publicAttachmentPathError(err error) string {
	switch {
	case errors.Is(err, errAttachmentPathEmpty):
		return "path is required"
	case errors.Is(err, errAttachmentPathNUL):
		return "path contains an invalid character"
	case errors.Is(err, errAttachmentPathAbsolute):
		return "path must be project-relative"
	case errors.Is(err, errAttachmentPathTraversal):
		return "path traversal is not allowed"
	default:
		return "invalid project-relative path"
	}
}

func pathWithin(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil || rel == ".." || rel == "." || filepath.IsAbs(rel) {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// sniffAttachmentMIME is content based. The extension and any client MIME
// claim are intentionally ignored, preventing a text file named *.png from
// entering the existing multimodal pipeline.
func sniffAttachmentMIME(data []byte) string {
	// The first four PNG bytes are enough to distinguish the format for the
	// existing inline attachment contract (which permits a short deterministic
	// fixture); project-path attachments still use this same content sniffing
	// seam and never trust an extension or client MIME claim.
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{0x89, 'P', 'N', 'G'}) {
		return "image/png"
	}
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "image/jpeg"
	}
	if len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))) {
		return "image/gif"
	}
	if len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return "image/webp"
	}
	// Keep net/http's maintained sniff table as a final guard for equivalent
	// signatures while still applying the explicit whitelist.
	detected := http.DetectContentType(data)
	if attachmentMimeWhitelist[detected] {
		return detected
	}
	return ""
}

func projectAttachmentDomainValues(items []projectAttachment) []domain.Attachment {
	if len(items) == 0 {
		return nil
	}
	out := make([]domain.Attachment, 0, len(items))
	for _, item := range items {
		out = append(out, domain.Attachment{Name: item.Name, MimeType: item.MimeType, Data: append([]byte(nil), item.Data...)})
	}
	return out
}
