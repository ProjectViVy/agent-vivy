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
