package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

// FileVersionRecorder adapts the storage file-version store to the
// tools.FileVersionRecorder seam (RB-1 record side). Recording is
// best-effort: failures are logged and dropped so file mutations never
// block on history writes.
type FileVersionRecorder struct {
	store  storage.FileVersionStore
	logger *slog.Logger
}

var _ tools.FileVersionRecorder = (*FileVersionRecorder)(nil)

// NewFileVersionRecorder binds the recorder to a durable store. A nil
// logger falls back to slog.Default().
func NewFileVersionRecorder(store storage.FileVersionStore, logger *slog.Logger) *FileVersionRecorder {
	if logger == nil {
		logger = slog.Default()
	}
	return &FileVersionRecorder{store: store, logger: logger}
}

// RecordMutation archives the pre/post-mutation contents onto the session's
// version chain; failures are logged, never propagated.
func (r *FileVersionRecorder) RecordMutation(ctx context.Context, sessionID domain.SessionID, runID domain.RunID, path string, oldContent, newContent []byte) {
	if r == nil || r.store == nil || sessionID == "" || path == "" {
		return
	}
	if err := r.store.RecordFileMutation(ctx, sessionID, runID, path, oldContent, newContent); err != nil {
		r.logger.Warn("file version archive failed (best-effort)",
			"session", string(sessionID), "run", string(runID), "path", path, "error", err)
	}
}

// TrackAccess moves the stale-read marker forward; failures are logged,
// never propagated.
func (r *FileVersionRecorder) TrackAccess(ctx context.Context, sessionID domain.SessionID, workspaceRoot, path string, state domain.FileReadState) {
	if r == nil || r.store == nil || sessionID == "" || path == "" {
		return
	}
	key, err := fileReadKey(workspaceRoot, path)
	if err == nil {
		err = r.store.TrackFileAccess(ctx, sessionID, key, state)
	}
	if err != nil {
		r.logger.Warn("file access tracking failed (best-effort)",
			"session", string(sessionID), "path", path, "error", err)
	}
}

// LastAccess returns exact state. Lookup errors and unavailable old state
// cannot authorize an overwrite; the backend requests another read.
func (r *FileVersionRecorder) LastAccess(ctx context.Context, sessionID domain.SessionID, workspaceRoot, path string) (*domain.FileReadState, bool, error) {
	if r == nil || r.store == nil || sessionID == "" || path == "" {
		return nil, false, nil
	}
	key, err := fileReadKey(workspaceRoot, path)
	if err != nil {
		return nil, false, err
	}
	state, tracked, err := r.store.LastFileAccess(ctx, sessionID, key)
	if err != nil || tracked {
		return state, tracked, err
	}
	// Converted relative-path rows cannot prove their workspace. Preserve the
	// unavailable fence until an actual read binds this root; never copy bytes.
	_, tracked, err = r.store.LastFileAccess(ctx, sessionID, filepath.ToSlash(filepath.Clean(path)))
	return nil, tracked, err
}

func fileReadKey(root, name string) (string, error) {
	if !filepath.IsAbs(root) || filepath.IsAbs(name) || strings.ContainsRune(name, '\x00') {
		return "", fmt.Errorf("file history: invalid workspace read identity")
	}
	key := filepath.Join(root, filepath.FromSlash(name))
	rel, err := filepath.Rel(root, key)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("file history: read identity escapes workspace")
	}
	return filepath.ToSlash(key), nil
}
