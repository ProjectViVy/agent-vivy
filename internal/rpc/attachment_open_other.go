//go:build !windows && !unix

package rpc

import "os"

func openAttachmentFile(root *os.Root, _ string, clean string) (*os.File, string, error) {
	file, err := root.Open(clean)
	return file, "", err
}
