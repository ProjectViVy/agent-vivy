package tools

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

var spillNameUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// SpillFileName sanitizes a tool call id into a spill file name component.
// An empty id falls back to "output" so a file name is always well-formed.
func SpillFileName(id, stream string) string {
	name := spillNameUnsafe.ReplaceAllString(id, "_")
	if name == "" {
		name = "output"
	}
	return name + "." + stream + ".txt"
}

// spillWriter tees a process output stream into a lazily created file. While
// the total stays under threshold bytes nothing is written to disk; the
// first byte past it opens <dir>/<name>, backfills the retained head, and
// appends every subsequent byte — so the file always holds the full output
// even though the wrapped writer keeps only its own bounded excerpt.
type spillWriter struct {
	inner     io.Writer
	dir       string
	name      string
	threshold int64

	mu    sync.Mutex
	head  bytes.Buffer
	file  *os.File
	total int64
	err   error
}

func newSpillWriter(inner io.Writer, dir, name string, threshold int64) *spillWriter {
	return &spillWriter{inner: inner, dir: dir, name: name, threshold: threshold}
}

func (w *spillWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.total += int64(len(p))
	if w.file == nil && w.err == nil && w.dir != "" && w.total > w.threshold {
		if err := os.MkdirAll(w.dir, 0o700); err == nil {
			if file, openErr := os.OpenFile(filepath.Join(w.dir, w.name), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600); openErr == nil {
				w.file = file
				_, w.err = w.file.Write(w.head.Bytes())
			} else {
				w.err = openErr
			}
		} else {
			w.err = err
		}
	}
	if w.file != nil {
		if _, err := w.file.Write(p); err != nil && w.err == nil {
			w.err = err
		}
	} else if w.err == nil {
		w.head.Write(p)
	}
	return w.inner.Write(p)
}

// finish reports the spill path, the total stream bytes, and whether
// spilling happened at all. closeFile=true (terminal collect) also closes
// the file; a running job keeps appending and passes false.
func (w *spillWriter) finish(closeFile bool) (path string, total int64, spilled bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return "", w.total, false
	}
	if closeFile {
		_ = w.file.Close()
	}
	return w.file.Name(), w.total, true
}
