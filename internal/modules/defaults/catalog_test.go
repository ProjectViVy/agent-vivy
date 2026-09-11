package defaults

import (
	"path/filepath"
	"testing"
)

func TestDefaultCatalogHasOneRequiredInternalProvider(t *testing.T) {
	records, err := Catalog(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"core/loop-driver@v1", "core/chat-model-host@v1", "core/tool-host@v1", "core/storage-engine@v1", "core/checkpoint-store@v1", "core/credential-resolver@v1", "core/sandbox-backend@v1"}
	counts := map[string]int{}
	for _, record := range records {
		for _, p := range record.Descriptor.Provides {
			counts[p.Port]++
		}
	}
	for _, port := range want {
		if counts[port] != 1 {
			t.Errorf("providers for %s = %d", port, counts[port])
		}
	}
}

func TestDefaultCatalogBindsP4HostsAndSources(t *testing.T) {
	records, err := Catalog(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		port, provider string
		collection     bool
	}{
		"vivy/context-host":   {port: "core/context-host@v1"},
		"vivy/context-source": {port: "std/context-source@v1", provider: "ContextSourceProviders", collection: true},
		"vivy/skill-host":     {port: "core/skill-host@v1"},
		"vivy/skill-source":   {port: "std/skill-source@v1", provider: "SkillSourceProviders", collection: true},
		"vivy/mcp-host":       {port: "core/mcp-host@v1", provider: "NewMCPProvider"},
	}
	seen := make(map[string]bool, len(want))
	for _, record := range records {
		wantRecord, ok := want[record.Descriptor.Module.ID]
		if !ok {
			continue
		}
		seen[record.Descriptor.Module.ID] = true
		found := false
		for _, provided := range record.Descriptor.Provides {
			if provided.Port == wantRecord.port {
				found = true
			}
		}
		if !found {
			t.Errorf("%s does not provide %s", record.Descriptor.Module.ID, wantRecord.port)
		}
		if wantRecord.provider != "" && record.Binding.ProviderConstructor != wantRecord.provider {
			t.Errorf("%s provider constructor = %q, want %q", record.Descriptor.Module.ID, record.Binding.ProviderConstructor, wantRecord.provider)
		}
		if wantRecord.collection != record.Binding.ProviderCollection {
			t.Errorf("%s provider collection = %v, want %v", record.Descriptor.Module.ID, record.Binding.ProviderCollection, wantRecord.collection)
		}
	}
	for id := range want {
		if !seen[id] {
			t.Errorf("default catalog is missing %s", id)
		}
	}
}
