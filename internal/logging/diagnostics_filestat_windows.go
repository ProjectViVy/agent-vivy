package logging

import (
	"os"
	"syscall"
)

// The file index is stable across appends and changes when the file is replaced.
// FileInfo.Sys() exposes timestamps, so query the opened handle instead.
func fileIdentity(_ os.FileInfo, f *os.File) (uint64, error) {
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(f.Fd()), &info); err != nil {
		return 0, err
	}
	return uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow), nil
}
