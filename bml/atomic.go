// atomic.go ports Diva's atomic.rs: same-directory temp file + fsync +
// rename so a manifest is never observed half-written.
package bml

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// atomicWrite writes data to path atomically: create an O_EXCL temp file
// in the same directory, write + fsync, rename over the target, fsync the
// parent directory.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func(err error) error {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return cleanup(err)
	}
	if err := tmp.Sync(); err != nil {
		return cleanup(err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	// Parent-directory fsync is best-effort like upstream sync_parent_dir:
	// platforms that cannot fsync a directory (Windows) must not fail the
	// write — the file data was already synced before the rename.
	if dirHandle, err := os.Open(dir); err == nil {
		_ = dirHandle.Sync()
		_ = dirHandle.Close()
	}
	return nil
}

// atomicWriteJSON persists value as pretty JSON (serde_json
// to_string_pretty equivalent: two-space indent).
func atomicWriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, data)
}
