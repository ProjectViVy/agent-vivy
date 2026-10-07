package app

import (
	"testing"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
)

// Per-model overrides resolve against the active route's model ID: the
// settings overlay merges per key over the config default, and For() folds
// the matching entry into the engine-visible snapshot.
func TestCompactionPolicyForPerModelResolution(t *testing.T) {
	cfg := config.Config{}
	cfg.Runtime.Compaction = config.CompactionConfig{
		Enabled: true, MaxTokens: 1000, TriggerPercent: 50, KeepRecent: 10,
		PerModel: map[string]config.CompactionOverride{
			"m-x": {TriggerPercent: 20},
			"m-y": {KeepRecent: 3},
		},
	}
	overlay := &settings.CompactionSettings{
		PerModel: map[string]settings.CompactionOverride{
			"m-x": {MaxTokens: 2000},
		},
	}

	got := compactionPolicyFor(cfg, overlay, 0, "m-x")
	if got.MaxTokens != 2000 || got.TriggerPercent != 20 || got.KeepRecent != 10 {
		t.Fatalf("policy for m-x = %+v, want config+overlay merged override", got)
	}
	if got.PerModel != nil {
		t.Fatal("resolved policy must not carry the unresolved map")
	}

	other := compactionPolicyFor(cfg, overlay, 0, "m-z")
	if other.MaxTokens != 1000 || other.TriggerPercent != 50 || other.KeepRecent != 10 {
		t.Fatalf("policy for m-z = %+v, want global values", other)
	}

	// Overlay keys merge into (not replace) the config map on the startup path.
	merged := mergedCompactionConfig(cfg.Runtime.Compaction, overlay)
	if len(merged.PerModel) != 2 || merged.PerModel["m-x"].MaxTokens != 2000 || merged.PerModel["m-x"].TriggerPercent != 20 {
		t.Fatalf("merged per_model = %+v", merged.PerModel)
	}
}

// The max_tokens fallback still lands after the per-model overlay, so an
// override that raises the window wins over the catalog window.
func TestCompactionPolicyForOverrideBeatsCatalogWindow(t *testing.T) {
	cfg := config.Config{}
	cfg.Runtime.Compaction = config.CompactionConfig{
		Enabled: true, MaxTokens: 0, TriggerPercent: 80,
		PerModel: map[string]config.CompactionOverride{"m-x": {MaxTokens: 64000}},
	}
	got := compactionPolicyFor(cfg, nil, 128000, "m-x")
	if got.MaxTokens != 64000 {
		t.Fatalf("MaxTokens = %d, want per-model override over catalog window", got.MaxTokens)
	}
	if got.TriggerTokens(0) != 51200 {
		t.Fatalf("TriggerTokens = %d, want 80%% of 64000", got.TriggerTokens(0))
	}
}
