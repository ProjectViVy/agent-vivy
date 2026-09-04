//go:build windows

package rpc

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

var kernel32GetShortPathNameW = syscall.NewLazyDLL("kernel32.dll").NewProc("GetShortPathNameW")

func TestResolveProjectAttachmentsRejectsSensitiveEightDotThreeAlias(t *testing.T) {
	root := t.TempDir()
	longPath := filepath.Join(root, "credentials.json")
	if err := os.WriteFile(longPath, []byte{0x89, 'P', 'N', 'G'}, 0o600); err != nil {
		t.Fatal(err)
	}
	shortPath, err := shortPathName(longPath)
	if err != nil {
		t.Skipf("8.3 aliases unavailable: %v", err)
	}
	alias := filepath.Base(shortPath)
	if strings.EqualFold(alias, filepath.Base(longPath)) {
		t.Skip("8.3 aliases are disabled on this volume")
	}

	resolved, err := resolveProjectAttachments(root, []string{alias})
	if err == nil || !errors.Is(err, errAttachmentPathSensitive) || len(resolved) != 0 {
		t.Fatalf("sensitive 8.3 alias resolved: attachments=%d err=%v alias=%q", len(resolved), err, alias)
	}
}

func shortPathName(path string) (string, error) {
	input, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	buffer := make([]uint16, 512)
	for {
		length, _, callErr := kernel32GetShortPathNameW.Call(uintptr(unsafe.Pointer(input)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
		if length == 0 {
			return "", callErr
		}
		if length < uintptr(len(buffer)) {
			return syscall.UTF16ToString(buffer[:length]), nil
		}
		buffer = make([]uint16, length+1)
	}
}
