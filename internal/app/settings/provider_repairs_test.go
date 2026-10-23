package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProviderRepairDefaultModelReconciliation(t *testing.T) {
	for _, tc := range []struct {
		name, def    string
		models, want []string
	}{
		{"missing", " default ", []string{"first"}, []string{"first", "default"}},
		{"listed", "default", []string{"first", " default "}, []string{"first", " default "}},
		{"empty", " ", []string{"first"}, []string{"first"}},
		{"nil list", "default", nil, []string{"default"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storage := make([]string, len(tc.models), len(tc.models)+1)
			copy(storage, tc.models)
			backing := storage[:cap(storage)]
			backing[len(storage)] = "sentinel"
			e := ProviderEntry{ID: "custom", DisplayName: "Gateway", Bundle: ProviderOpenAI, BaseURL: "https://gateway.example/v1", DefaultModel: tc.def, Models: storage}
			for _, original := range []Settings{{}, {Providers: []ProviderEntry{e}}} {
				got := original.UpsertProvider(e)
				if !reflect.DeepEqual(got.Providers[0].Models, tc.want) {
					t.Errorf("models %v, want %v", got.Providers[0].Models, tc.want)
				}
				if backing[len(storage)] != "sentinel" {
					t.Error("upsert mutated caller backing array")
				}
				again := got.UpsertProvider(got.Providers[0])
				if !reflect.DeepEqual(again.Providers[0].Models, got.Providers[0].Models) {
					t.Error("not idempotent")
				}
			}
		})
	}
}

func TestProviderRepairLegacyDefaultModelLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	document := []byte("providers:\n  - id: custom\n    display_name: Gateway\n    bundle: openai\n    base_url: https://gateway.example/v1\n    default_model: preferred\n    models: [first]\n")
	if err := os.WriteFile(path, document, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"first", "preferred"}
	if !reflect.DeepEqual(loaded.Providers[0].Models, want) {
		t.Errorf("loaded models %v", loaded.Providers[0].Models)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != string(document) {
		t.Error("Load rewrote document")
	}
	if _, err := Save(path, loaded); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reloaded.Providers[0].Models, want) {
		t.Error("round trip lost reconciliation")
	}
}
