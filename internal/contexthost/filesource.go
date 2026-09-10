package contexthost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/sdk/port/contextsource"
)

// FileSnapshotSource adapts already-authorized durable project file snapshots
// into the public Context Source shape. It never reads the filesystem itself;
// path authorization and race-resistant reads remain at the RPC boundary.
type FileSnapshotSource struct {
	id    string
	files []domain.FileContext
}

func NewFileSnapshotSource(id string, files []domain.FileContext) *FileSnapshotSource {
	copyFiles := make([]domain.FileContext, len(files))
	for index, file := range files {
		copyFiles[index] = file
		copyFiles[index].Content = append([]byte(nil), file.Content...)
	}
	return &FileSnapshotSource{id: strings.TrimSpace(id), files: copyFiles}
}

func (source *FileSnapshotSource) ID() string { return source.id }

func (source *FileSnapshotSource) Query(_ context.Context, request contextsource.Request) (contextsource.Page, error) {
	limit := request.Limit
	if limit <= 0 || limit > len(source.files) {
		limit = len(source.files)
	}
	candidates := make([]contextsource.Candidate, 0, limit)
	for _, file := range source.files {
		if len(candidates) >= limit {
			break
		}
		path := strings.ReplaceAll(strings.TrimSpace(file.Path), `\`, "/")
		if !validSnapshotPath(path) || len(file.Content) == 0 {
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
				"name": strings.TrimSpace(file.Name),
				"path": path,
			},
		}))
	}
	return contextsource.NewPage(candidates, ""), nil
}

func validSnapshotPath(path string) bool {
	if path == "" || strings.ContainsRune(path, 0) || strings.Contains(path, ":") || filepath.IsAbs(filepath.FromSlash(path)) || strings.HasPrefix(path, "/") {
		return false
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
