//go:build unix

package bundles

import (
	"os"
	"syscall"
)

const noFollowFlag = syscall.O_NOFOLLOW

func singleLink(_ *os.File, info os.FileInfo) error {
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok || s.Nlink != 1 {
		return artifactError("invalid_manifest", "hardlink", nil)
	}
	return nil
}
func syncDirectory(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func promoteDirectory(from, to string) error { return os.Rename(from, to) }
