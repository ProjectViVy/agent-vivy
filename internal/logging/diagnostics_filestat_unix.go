//go:build unix

package logging

import (
	"os"
	"syscall"
)

func fileIdentity(info os.FileInfo, _ *os.File) (uint64, error) {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return st.Ino, nil
	}
	return 0, nil
}
