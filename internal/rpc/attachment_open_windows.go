//go:build windows

package rpc

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32GetFinalPathNameByHandleW = syscall.NewLazyDLL("kernel32.dll").NewProc("GetFinalPathNameByHandleW")
	errFinalPathOutsideRoot           = errors.New("opened attachment resolves outside project root")
)

func openAttachmentFile(root *os.Root, rootPath, clean string) (*os.File, string, error) {
	file, err := root.Open(clean)
	if err != nil {
		return nil, "", err
	}
	rootFile, err := os.Open(rootPath)
	if err != nil {
		_ = file.Close()
		return nil, "", err
	}
	rootFinal, err := finalPathByHandle(rootFile)
	_ = rootFile.Close()
	if err != nil {
		_ = file.Close()
		return nil, "", err
	}
	fileFinal, err := finalPathByHandle(file)
	if err != nil {
		_ = file.Close()
		return nil, "", err
	}
	if !pathWithin(rootFinal, fileFinal) {
		_ = file.Close()
		return nil, "", errFinalPathOutsideRoot
	}
	rel, err := filepath.Rel(rootFinal, fileFinal)
	if err != nil {
		_ = file.Close()
		return nil, "", err
	}
	return file, rel, nil
}

func finalPathByHandle(file *os.File) (string, error) {
	buffer := make([]uint16, 512)
	for {
		length, _, callErr := kernel32GetFinalPathNameByHandleW.Call(file.Fd(), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0)
		if length == 0 {
			return "", callErr
		}
		if length < uintptr(len(buffer)) {
			raw := syscall.UTF16ToString(buffer[:length])
			path := raw
			if strings.HasPrefix(raw, `\\?\UNC\`) {
				path = `\\` + strings.TrimPrefix(raw, `\\?\UNC\`)
			} else {
				path = strings.TrimPrefix(raw, `\\?\`)
			}
			return filepath.Clean(path), nil
		}
		buffer = make([]uint16, length+1)
	}
}
