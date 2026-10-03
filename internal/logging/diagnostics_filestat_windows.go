package logging

import (
	"os"
	"syscall"
)

// Windows FileInfo.Sys() carries no file index; creation time stands in:
// a path that is rotated/replaced always gets a new creation timestamp.
func fileIdentity(info os.FileInfo) uint64 {
	if st, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return uint64(st.CreationTime.Nanoseconds())
	}
	return 0
}
