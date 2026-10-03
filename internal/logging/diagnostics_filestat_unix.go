//go:build unix

package logging

import (
	"os"
	"syscall"
)

func fileIdentity(info os.FileInfo) uint64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return st.Ino
	}
	return 0
}
