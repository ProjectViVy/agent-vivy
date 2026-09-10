package assembly

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestP1P2EvidenceReferencesRepositoryFiles(t *testing.T) {
	for portID, evidence := range P1P2PortEvidence() {
		for _, reference := range evidence.References {
			path, _, ok := strings.Cut(reference.ID, "#")
			if !ok || path == "" {
				t.Fatalf("%s %s evidence has no file anchor: %q", portID, reference.Kind, reference.ID)
			}
			if _, err := os.Stat(filepath.Join("..", "..", "..", path)); err != nil {
				t.Fatalf("%s %s evidence path %q: %v", portID, reference.Kind, path, err)
			}
		}
	}
}
