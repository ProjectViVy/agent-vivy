//go:build unix

package rpc

import (
	"os"
	"syscall"
)

// O_NONBLOCK prevents a regular-file-to-FIFO/device replacement between the
// rooted Lstat and Open from hanging the control plane. The caller still
// verifies identity and regular-file mode on the opened handle.
func openAttachmentFile(root *os.Root, _ string, clean string) (*os.File, string, error) {
	file, err := root.OpenFile(clean, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	return file, "", err
}
