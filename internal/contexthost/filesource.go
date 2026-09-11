package contexthost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"agent-vivy/internal/domain"
	"agent-vivy/sdk/port/contextsource"
)

// FileSnapshotSource adapts already-authorized durable project file snapshots
// into the public Context Source shape. It never reads the filesystem itself;
// path authorization and race-resistant reads remain at the RPC boundary.
type FileSnapshotSource struct {
	id       string
	files    []domain.FileContext
	validate func(context.Context, []domain.FileContext) ([]domain.FileContext, error)
}

func NewFileSnapshotSource(id string, files []domain.FileContext) *FileSnapshotSource {
	return NewFileSnapshotSourceWithValidator(id, files, nil)
}

// NewFileSnapshotSourceWithValidator keeps first-party snapshots behind the
// same live request validation boundary used by Runtime and RPC. The
// validator receives a defensive copy and may reject legacy/stored rows.
func NewFileSnapshotSourceWithValidator(id string, files []domain.FileContext, validate func(context.Context, []domain.FileContext) ([]domain.FileContext, error)) *FileSnapshotSource {
	copyFiles := make([]domain.FileContext, len(files))
	for index, file := range files {
		copyFiles[index] = file
		copyFiles[index].Content = append([]byte(nil), file.Content...)
	}
	return &FileSnapshotSource{id: id, files: copyFiles, validate: validate}
}

func (source *FileSnapshotSource) ID() string { return source.id }

func (source *FileSnapshotSource) Query(ctx context.Context, request contextsource.Request) (contextsource.Page, error) {
	if err := ctx.Err(); err != nil {
		return contextsource.Page{}, err
	}
	files := source.files
	if source.validate != nil {
		validated, err := source.validate(ctx, cloneSnapshotFiles(source.files))
		if err != nil {
			return contextsource.Page{}, err
		}
		files = validated
	}
	limit := request.Limit
	if limit <= 0 || limit > len(files) {
		limit = len(files)
	}
	candidates := make([]contextsource.Candidate, 0, limit)
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return contextsource.Page{}, err
		}
		if len(candidates) >= limit {
			break
		}
		if !validSnapshotPath(file.Path) {
			continue
		}
		path := strings.ReplaceAll(file.Path, `\`, "/")
		if !validSnapshotPath(path) {
			continue
		}
		sum := sha256.Sum256(file.Content)
		candidates = append(candidates, contextsource.NewCandidate(contextsource.Candidate{
			SourceID:   source.id,
			ContentID:  path,
			MediaType:  "text/plain",
			Content:    string(file.Content),
			SizeHint:   len(file.Content),
			Confidence: 1,
			Version:    hex.EncodeToString(sum[:]),
			Metadata: map[string]string{
				"name": file.Name,
				"path": path,
			},
		}))
	}
	return contextsource.NewPage(candidates, ""), nil
}

func cloneSnapshotFiles(in []domain.FileContext) []domain.FileContext {
	if len(in) == 0 {
		return nil
	}
	out := make([]domain.FileContext, len(in))
	for i, item := range in {
		out[i] = item
		out[i].Content = append([]byte(nil), item.Content...)
	}
	return out
}

func validSnapshotPath(path string) bool {
	if path == "" || !utf8.ValidString(path) || strings.ContainsRune(path, 0) || strings.Contains(path, ":") || filepath.IsAbs(filepath.FromSlash(path)) || strings.HasPrefix(path, "/") {
		return false
	}
	for _, r := range path {
		if unicode.IsControl(r) || isPathBidiControl(r) {
			return false
		}
	}
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	return clean == path && clean != "." && !strings.HasPrefix(clean, "../")
}

func isPathBidiControl(r rune) bool {
	return r == '\u061c' || r == '\u200e' || r == '\u200f' || (r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069')
}
