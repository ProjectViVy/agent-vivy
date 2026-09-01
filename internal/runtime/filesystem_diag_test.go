package runtime

import (
	"context"
	"testing"
)

type stubDiagSource struct {
	calls int
	paths []string
}

func (s *stubDiagSource) WriteDiagnostics(_ context.Context, paths []string) []string {
	s.calls++
	s.paths = paths
	return []string{"a.go:1:1: error: x"}
}

func TestFilesystemBackendWriteDiagnosticsForwarding(t *testing.T) {
	backend := NewEinoFilesystemBackend(nil, nil)
	if got := backend.WriteDiagnostics(context.Background(), []string{"a.go"}); got != nil {
		t.Fatalf("unwired backend = %v", got)
	}
	src := &stubDiagSource{}
	backend.SetWriteDiagnostics(src)
	got := backend.WriteDiagnostics(context.Background(), []string{"a.go", "b.go"})
	if src.calls != 1 || len(src.paths) != 2 || src.paths[1] != "b.go" {
		t.Fatalf("forwarding = %d %v", src.calls, src.paths)
	}
	if len(got) != 1 || got[0] != "a.go:1:1: error: x" {
		t.Fatalf("lines = %v", got)
	}
}
