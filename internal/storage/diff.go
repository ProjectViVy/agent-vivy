package storage

import (
	"strings"

	udiff "github.com/aymanbagabas/go-udiff"
)

// LineDiffStats reports unified-diff additions and deletions. File-version
// contents are bounded by FileVersionMaxBytes before this helper is called;
// malformed UTF-8 is still handled deterministically by string conversion.
func LineDiffStats(oldContent, newContent []byte) FileDiffStats {
	if string(oldContent) == string(newContent) {
		return FileDiffStats{}
	}
	diff := udiff.Unified("a/file", "b/file", string(oldContent), string(newContent))
	if diff == "" {
		return FileDiffStats{}
	}
	var out FileDiffStats
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			out.Additions++
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			out.Deletions++
		}
	}
	return out
}
