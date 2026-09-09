//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package settings

import "errors"

// Unsupported platforms retain buildability but reject shared settings
// writes instead of silently offering a process-local-only transaction.
func acquireSettingsFileLock(string) (settingsFileLocker, error) {
	return nil, errors.New("cross-process settings locking is unsupported on this platform")
}
