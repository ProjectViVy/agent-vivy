//go:build windows

package settings

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

type settingsFileLock struct {
	file       *os.File
	overlapped windows.Overlapped
}

func acquireSettingsFileLock(path string) (settingsFileLocker, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	lock := &settingsFileLock{file: file}
	err = windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK,
		0,
		1,
		0,
		&lock.overlapped,
	)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return lock, nil
}

func (l *settingsFileLock) unlock() error {
	unlockErr := windows.UnlockFileEx(
		windows.Handle(l.file.Fd()),
		0,
		1,
		0,
		&l.overlapped,
	)
	return errors.Join(unlockErr, l.file.Close())
}
