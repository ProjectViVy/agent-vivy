//go:build !unix && !windows

package logging

import "os"

func fileIdentity(_ os.FileInfo, _ *os.File) (uint64, error) {
	return 0, nil
}
