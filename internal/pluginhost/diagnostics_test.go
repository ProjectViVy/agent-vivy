package pluginhost

import (
	"context"
	"testing"

	"agent-vivy/sdk/plugin"
)

type observerStub struct {
	name  string
	seam  plugin.Seam
	lines []string
}

func (s *observerStub) Name() string                                                { return s.name }
func (s *observerStub) Seam() plugin.Seam                                           { return s.seam }
func (s *observerStub) Grants() []plugin.Grant                                      { return nil }
func (s *observerStub) Tools() []plugin.Tool                                        { return nil }
func (s *observerStub) ObserveWrite(context.Context, plugin.Env, []string) []string { return s.lines }

type plainStub struct{ name string }

func (s *plainStub) Name() string           { return s.name }
func (s *plainStub) Seam() plugin.Seam      { return plugin.SeamToolWorld }
func (s *plainStub) Grants() []plugin.Grant { return nil }
func (s *plainStub) Tools() []plugin.Tool   { return nil }

func TestDiagnosticBridgeCollectsToolWorldObservers(t *testing.T) {
	first := &observerStub{name: "lsp", seam: plugin.SeamToolWorld, lines: []string{"a.go:1:1: error: x", ""}}
	second := &observerStub{name: "other", seam: plugin.SeamToolWorld, lines: []string{"b.go:2:1: warning: y"}}
	channel := &observerStub{name: "chan", seam: plugin.SeamChannel, lines: []string{"must not appear"}}

	bridge := NewDiagnosticBridge([]plugin.Plugin{first, channel, &plainStub{name: "plain"}, second}, nil)
	got := bridge.WriteDiagnostics(context.Background(), []string{"a.go"})
	want := []string{"a.go:1:1: error: x", "b.go:2:1: warning: y"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDiagnosticBridgeNilAndEmptyInputs(t *testing.T) {
	var nilBridge *DiagnosticBridge
	if got := nilBridge.WriteDiagnostics(context.Background(), []string{"a.go"}); got != nil {
		t.Fatalf("nil bridge = %v", got)
	}
	bridge := NewDiagnosticBridge(nil, nil)
	if got := bridge.WriteDiagnostics(context.Background(), []string{"a.go"}); got != nil {
		t.Fatalf("no observers = %v", got)
	}
	if got := bridge.WriteDiagnostics(context.Background(), nil); got != nil {
		t.Fatalf("no paths = %v", got)
	}
}
