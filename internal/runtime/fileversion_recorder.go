package runtime

import (
	"context"
	"log/slog"

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
func (r *FileVersionRecorder) TrackAccess(ctx context.Context, sessionID domain.SessionID, path string, at int64) {
	if r == nil || r.store == nil || sessionID == "" || path == "" {
		return
	}
	if err := r.store.TrackFileAccess(ctx, sessionID, path, at); err != nil {
		r.logger.Warn("file access tracking failed (best-effort)",
			"session", string(sessionID), "path", path, "error", err)
	}
}

// LastAccess reports the marker timestamp; a failed lookup reads as "never
// tracked" so the stale-read guard fails open.
func (r *FileVersionRecorder) LastAccess(ctx context.Context, sessionID domain.SessionID, path string) (int64, bool, error) {
	if r == nil || r.store == nil || sessionID == "" || path == "" {
		return 0, false, nil
	}
	return r.store.LastFileAccess(ctx, sessionID, path)
}
