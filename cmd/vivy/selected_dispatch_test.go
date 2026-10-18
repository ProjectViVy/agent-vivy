package main

import (
	"context"
	"testing"

	genassembly "agent-vivy/internal/generated/assembly"
	plugin "agent-vivy/sdk/port/face"
)

// The default gateway, the packed TUI generation and every non-ACP face
// keep the existing vivy command surface; only the ACP face family claims
// the artifact's no-argument protocol mode (ACP-STDIO-FACE §288, §294).

type dispatchFakeFace struct{ def plugin.Definition }

func (f dispatchFakeFace) Definition() plugin.Definition { return f.def }
func (dispatchFakeFace) Construct(context.Context, plugin.Host) (plugin.Instance, error) {
	return nil, context.Canceled
}

func TestDefaultCLIUnchanged(t *testing.T) {
	if got := selectedFaceClaimsProtocol(genassembly.BuildDefault().Face); got {
		t.Fatal("the default gateway generation claims the protocol dispatch")
	}
	for _, test := range []struct {
		name string
		face plugin.FaceProvider
		want bool
	}{
		{"nil face", nil, false},
		{"tui face family", dispatchFakeFace{def: plugin.Definition{ID: "vivy.tui", Kind: "tui"}}, false},
		{"headless face family", dispatchFakeFace{def: plugin.Definition{ID: "vivy.headless", Kind: "headless"}}, false},
		{"acp face family", dispatchFakeFace{def: plugin.Definition{ID: "projectvivy.acp", Kind: "acp"}}, true},
	} {
		if got := selectedFaceClaimsProtocol(test.face); got != test.want {
			t.Fatalf("%s: selectedFaceClaimsProtocol = %v, want %v", test.name, got, test.want)
		}
	}
	// The run subcommand surface stays wired for normal generations.
	if code := runRun([]string{"--help"}); code != 0 {
		t.Fatalf("vivy run --help exit = %d, want 0", code)
	}
}
