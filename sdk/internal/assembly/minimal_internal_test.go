package assembly

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"agent-vivy/internal/modules/defaults"
	"agent-vivy/sdk/port"
)

func TestMinimalRecipeOmitsOptionalCapabilities(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..")
	records, err := defaults.Catalog(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	sources := make([]SourceRecord, 0, len(records))
	for _, record := range records {
		sources = append(sources, SourceRecord{
			Descriptor: record.Descriptor, Trust: TrustT1,
			Binding: GoBinding{
				ImportPath: record.Binding.ImportPath, Package: record.Binding.Package,
				Constructor: record.Binding.Constructor, ProviderConstructor: record.Binding.ProviderConstructor,
				ProviderCollection: record.Binding.ProviderCollection, ContextSourceProvider: record.Binding.ContextSourceProvider,
				SkillSourceProvider: record.Binding.SkillSourceProvider, MCPHostProvider: record.Binding.MCPHostProvider,
			},
		})
	}
	catalog, err := NewSourceCatalog(sources)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(repoRoot, "recipes", "minimal.vivy.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var recipe Recipe
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&recipe); err != nil {
		t.Fatal(err)
	}
	plan, err := (Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: SupportedPortEvidence()}).Compile(context.Background(), recipe)
	if err != nil {
		t.Fatal(err)
	}
	for _, resolved := range plan.Modules {
		if strings.HasSuffix(resolved.Descriptor.Module.ID, "-host") && resolved.Descriptor.Module.ID != "vivy/tool-host" {
			t.Fatalf("minimal Assembly retained optional Host %s", resolved.Descriptor.Module.ID)
		}
		for _, provided := range resolved.Descriptor.Provides {
			if strings.HasPrefix(provided.Port, "std/") {
				t.Fatalf("minimal Assembly retained public capability %s from %s", provided.Port, resolved.Descriptor.Module.ID)
			}
		}
	}
}
