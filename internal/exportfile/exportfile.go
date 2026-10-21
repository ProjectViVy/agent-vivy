// Package exportfile owns bounded, write-once artifacts in the instance exports
// root. Names are opaque identities, never reused by its writers. This protects
// publication and reads; it is not a retained baseline for external filesystem
// edits made before a read begins.
package exportfile

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const MaxBytes = 32 << 20

var (
	ErrName        = errors.New("export: invalid artifact name")
	ErrTooLarge    = errors.New("export: artifact exceeds byte limit")
	ErrUnavailable = errors.New("export: artifact unavailable or changed during read")
)

// ValidName admits only bare export-owner basenames.
func ValidName(name string) bool {
	if name == "" || name == "." || name == ".." || len(name) > 128 || filepath.Base(name) != name {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// Write publishes a new artifact with exclusive creation. The name is returned
// only after the complete write and close; failed writes remove their own file.
// Existing names are never opened for replacement.
func Write(dir, prefix, suffix string, body []byte) (string, error) {
	if len(body) > MaxBytes {
		return "", ErrTooLarge
	}
	if len(prefix) > 64 {
		prefix = prefix[:64]
	}
	name := prefix + "-" + rand.Text() + suffix
	if !ValidName(name) {
		return "", ErrName
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("export: create root: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", fmt.Errorf("export: open root: %w", err)
	}
	defer root.Close()
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", fmt.Errorf("export: create artifact: %w", err)
	}
	_, writeErr := file.Write(body)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		_ = root.Remove(name)
		return "", fmt.Errorf("export: write artifact: %w", err)
	}
	return name, nil
}

// Read serves one complete bounded descriptor snapshot. All callers share the
// configured root; no basename is a bearer capability or a session authority.
func Read(ctx context.Context, dir, name string) ([]byte, error) { return read(ctx, dir, name, nil) }

func read(ctx context.Context, dir, name string, afterRead func()) ([]byte, error) {
	if !ValidName(name) {
		return nil, ErrName
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("export: open root: %w", err)
	}
	defer root.Close()
	before, err := root.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("export: stat artifact: %w", err)
	}
	if !before.Mode().IsRegular() {
		return nil, ErrUnavailable
	}
	if before.Size() > MaxBytes {
		return nil, ErrTooLarge
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, fmt.Errorf("export: open artifact: %w", err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("export: stat descriptor: %w", err)
	}
	if !sameState(before, opened) {
		return nil, ErrUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(contextReader{ctx: ctx, r: file}, MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("export: read artifact: %w", err)
	}
	if len(body) > MaxBytes {
		return nil, ErrTooLarge
	}
	if afterRead != nil {
		afterRead()
	}
	after, err := root.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	descriptor, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("export: stat descriptor: %w", err)
	}
	if !sameState(opened, after) || !sameState(opened, descriptor) || int64(len(body)) != opened.Size() {
		return nil, ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return body, nil
}

func sameState(a, b os.FileInfo) bool {
	return a.Mode().IsRegular() && b.Mode().IsRegular() && os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
