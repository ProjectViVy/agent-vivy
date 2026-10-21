package assembly

import (
	"agent-vivy/sdk/module"
	"bytes"
	"testing"
)

func TestInspectManifestRejectsDuplicateJSONFields(t *testing.T) {
	_, raw, err := SealManifest(manifestTestPlan(), SealInputs{GenerationID: "authoring-test", SpecificationVersion: "vivy.module/v1", CompilerVersion: "test", SDKVersion: "v1", CanonicalRecipe: []byte(`{}`), CapabilityStates: map[string]CapabilityState{"channels": CapabilityReady}})
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"invalid UTF-8":         bytes.Replace(raw, []byte("authoring-test"), []byte{0xff}, 1),
		"identity":              append([]byte(`{"generationId":"foreign",`), raw[1:]...),
		"case-variant identity": append([]byte(`{"GenerationID":"foreign",`), raw[1:]...),
		"case-variant version":  append([]byte(`{"SpecificationVersion":"foreign",`), raw[1:]...),
		"escaped identity":      append([]byte(`{"generation\u0049d":"foreign",`), raw[1:]...),
		"nested state":          bytes.Replace(raw, []byte(`"channels":"READY"`), []byte(`"channels":"INACTIVE","channels":"READY"`), 1),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if bytes.Equal(body, raw) {
				t.Fatal("fixture did not add a duplicate")
			}
			if _, err := InspectManifest(body); err == nil {
				t.Fatal("ambiguous manifest accepted")
			}
		})
	}
}

func TestManifestSchemaPreservesCaseSensitiveMapKeys(t *testing.T) {
	_, raw, err := SealManifest(manifestTestPlan(), SealInputs{GenerationID: "authoring-map", SpecificationVersion: "vivy.module/v1", CompilerVersion: "test", SDKVersion: "v1", CanonicalRecipe: []byte(`{}`), CapabilityStates: map[string]CapabilityState{"channels": CapabilityReady, "Channels": CapabilityReady}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InspectManifest(raw); err != nil {
		t.Fatalf("case-sensitive map keys refused: %v", err)
	}
}

func TestManifestSchemaRoundTripsEmbeddedRequirement(t *testing.T) {
	plan := manifestTestPlan()
	plan.Modules[0].Descriptor.Requires = []module.Requirement{{PortRef: module.PortRef{Port: "core/tool-host@v1", ID: "tools"}, Provider: "vivy/tool-host"}}
	_, raw, err := SealManifest(plan, SealInputs{GenerationID: "authoring-requirement", SpecificationVersion: "vivy.module/v1", CompilerVersion: "test", SDKVersion: "v1", CanonicalRecipe: []byte(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InspectManifest(raw); err != nil {
		t.Fatalf("valid embedded Requirement refused: %v", err)
	}
}
