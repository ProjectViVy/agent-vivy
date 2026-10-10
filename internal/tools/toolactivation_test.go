package tools

import (
	"context"
	"testing"

	"agent-vivy/internal/domain"
)

func TestToolActivationSetAndContext(t *testing.T) {
	activation := NewToolActivation()
	activation.Activate("a", "b", "")
	if !activation.Has("a") || !activation.Has("b") {
		t.Fatalf("activation missing names: %v", activation.Snapshot())
	}
	activation.Deactivate("a")
	if activation.Has("a") || !activation.Has("b") {
		t.Fatalf("deactivate mismatch: %v", activation.Snapshot())
	}
	if got := activation.Snapshot(); len(got) != 1 || got[0] != "b" {
		t.Fatalf("snapshot = %v, want [b]", got)
	}
	ctx := WithToolActivation(context.Background(), activation)
	if ToolActivationFromContext(ctx) != activation {
		t.Fatal("context binding lost the tracker")
	}
	if ToolActivationFromContext(context.Background()) != nil {
		t.Fatal("unbound context reported a tracker")
	}
}

func TestResolveToolExposureDefaults(t *testing.T) {
	if len(fixedVisibleToolNames) != 11 {
		t.Fatalf("fixed-visible core size = %d, want 11", len(fixedVisibleToolNames))
	}
	if got := ResolveToolExposure(domain.ToolSpec{Name: ReadFileName}); got != domain.ToolExposureDirect {
		t.Fatalf("core tool exposure = %s, want direct", got)
	}
	if got := ResolveToolExposure(domain.ToolSpec{Name: "sequential_thinking"}); got != domain.ToolExposureDeferred {
		t.Fatalf("non-core tool exposure = %s, want deferred", got)
	}
	if got := ResolveToolExposure(domain.ToolSpec{Name: "x", Exposure: domain.ToolExposureHidden}); got != domain.ToolExposureHidden {
		t.Fatalf("explicit exposure = %s, want hidden", got)
	}
}
