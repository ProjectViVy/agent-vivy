//go:build !unix && !windows

package logging

import "os"

func fileIdentity(info os.FileInfo) uint64 {
	return 0
}
