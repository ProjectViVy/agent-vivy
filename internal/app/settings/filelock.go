package settings

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type settingsFileLocker interface {
	unlock() error
}

// withSettingsFileLock serializes a settings write transaction with other
// Vivy processes that share the same workspace settings path. The lock file
// is stable and intentionally retained; the operating system releases the
// actual advisory lock when the handle closes, including after a crash.
func withSettingsFileLock(path string, fn func() (Settings, error)) (Settings, error) {
	dir := filepath.Dir(path)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return Settings{}, fmt.Errorf("settings: create dir: %w", err)
		}
	}
	lock, err := acquireSettingsFileLock(path + ".lock")
	if err != nil {
		return Settings{}, fmt.Errorf("settings: lock %s: %w", path, err)
	}
	result, runErr := fn()
	unlockErr := lock.unlock()
	if runErr != nil {
		if unlockErr != nil {
			return Settings{}, errors.Join(runErr, fmt.Errorf("settings: unlock %s: %w", path, unlockErr))
		}
		return Settings{}, runErr
	}
	if unlockErr != nil {
		return Settings{}, fmt.Errorf("settings: unlock %s: %w", path, unlockErr)
	}
	return result, nil
}
