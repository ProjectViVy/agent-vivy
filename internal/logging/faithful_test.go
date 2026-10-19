package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoggingPreservesAuthorizedSyntheticText(t *testing.T) {
	dir := t.TempDir()
	logger, _, closer, err := Setup(Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	text := "alice@example.com password=synthetic sk-test-abcdefghijkl [REDACTED]"
	logger.Info(text, "token_count", 17)
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(dir, FilePrefix+".*"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("logs = %v, err = %v", paths, err)
	}
	raw, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), text) || !strings.Contains(string(raw), `"token_count":17`) {
		t.Fatalf("authorized synthetic content changed: %s", raw)
	}
}
