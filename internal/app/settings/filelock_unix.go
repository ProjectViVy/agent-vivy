//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package settings

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

type settingsFileLock struct {
	file *os.File
}

func acquireSettingsFileLock(path string) (settingsFileLocker, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, err
	}
	return &settingsFileLock{file: file}, nil
}

func (l *settingsFileLock) unlock() error {
	unlockErr := unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
	return errors.Join(unlockErr, l.file.Close())
}
