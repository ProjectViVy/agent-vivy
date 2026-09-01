// Kernel-side capture for hosted plugin file writes (VC-3 record side).
// lsp_rename and any other write-effect plugin tool lands in the same
// file_versions chain as the kernel's own write tools, without any plugin
// cooperation or sdk/plugin surface change.

package pluginhost

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"time"

	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

// openRecordingWrite wraps the plain open so plugin file writes (lsp_rename
// et al) land in the same file_versions chain as kernel write tools (VC-3
// record side). The file is opened WITHOUT O_TRUNC: old content is
// snapshotted and stays on disk until the buffered writes are flushed on
// Close — a writer abandoned mid-call leaves the old file intact instead of
// an empty husk. Close is the destructive moment: truncate, flush, record
// the mutation, refresh the access marker. Best-effort by contract:
// capture problems degrade to an unrecorded write, never a failed plugin
// write.
func (e hostedEnv) openRecordingWrite(path string) (io.WriteCloser, error) {
	if e.recorder == nil {
		return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	}
	old, skip := snapshotOld(path)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	return &recordingWriter{file: f, env: e, display: e.displayPath(path), old: old, skip: skip}, nil
}

// snapshotOld reads the pre-write content, capped at the version limit.
// A missing file yields an empty baseline (the WriteFile convention); an
// unreadable or oversize file yields skip, because a truncated snapshot
// would poison the chain.
func snapshotOld(path string) (old []byte, skip bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, !os.IsNotExist(err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, storage.FileVersionMaxBytes+1))
	if err != nil || len(data) > storage.FileVersionMaxBytes {
		return nil, true
	}
	return data, false
}

// recordingWriter buffers plugin writes up to the version cap. A larger
// payload truncates immediately (the destructive moment moves as early as
// the buffer allows) and switches to pass-through; the chain entry is
// skipped rather than recorded from a truncated view. Close is idempotent
// and records at most once.
type recordingWriter struct {
	file    *os.File
	env     hostedEnv
	display string
	old     []byte
	skip    bool
	buf     bytes.Buffer
	direct  bool
	closed  bool
}

func (w *recordingWriter) Write(p []byte) (int, error) {
	if !w.direct {
		if w.buf.Len()+len(p) > storage.FileVersionMaxBytes {
			if err := w.file.Truncate(0); err != nil {
				return 0, err
			}
			if _, err := w.file.Seek(0, io.SeekStart); err != nil {
				return 0, err
			}
			if _, err := w.file.Write(w.buf.Bytes()); err != nil {
				return 0, err
			}
			w.direct = true
		} else {
			return w.buf.Write(p)
		}
	}
	return w.file.Write(p)
}

func (w *recordingWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	if !w.direct {
		if err := w.file.Truncate(0); err != nil {
			_ = w.file.Close()
			return err
		}
		if _, err := w.file.Seek(0, io.SeekStart); err != nil {
			_ = w.file.Close()
			return err
		}
		if _, err := w.file.Write(w.buf.Bytes()); err != nil {
			_ = w.file.Close()
			return err
		}
	}
	if err := w.file.Close(); err != nil {
		return err
	}
	w.record()
	return nil
}

// record runs only after a fully successful write. The access marker is
// refreshed on every plugin write (the agent's own mutation must not make
// its next edit stale, mirroring WriteFile); the chain entry additionally
// needs a complete old/new snapshot.
func (w *recordingWriter) record() {
	if w.env.recorder == nil {
		return
	}
	sessionID := tools.SessionIDFromContext(w.env.ctx)
	if sessionID == "" {
		return
	}
	if !w.skip && !w.direct {
		w.env.recorder.RecordMutation(w.env.ctx, sessionID, tools.RunIDFromContext(w.env.ctx), w.display, w.old, w.buf.Bytes())
	}
	w.env.recorder.TrackAccess(w.env.ctx, sessionID, w.display, time.Now().UnixMilli())
}

func (e hostedEnv) displayPath(path string) string {
	root, err := e.workspace()
	if err != nil {
		return filepath.ToSlash(path)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
