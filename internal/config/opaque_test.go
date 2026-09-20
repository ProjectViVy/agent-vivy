package config

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOpaqueYAMLToJSON(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{name: "absent", want: `{}`},
		{name: "null", yaml: `null`, want: `{}`},
		{name: "mapping", yaml: "nested:\n  deep: 7\n", want: `{"nested":{"deep":7}}`},
		{name: "sequence", yaml: `[one, two]`, want: `["one","two"]`},
		{name: "scalar", yaml: `value`, want: `"value"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var node yaml.Node
			if test.yaml != "" {
				if err := yaml.Unmarshal([]byte(test.yaml), &node); err != nil {
					t.Fatal(err)
				}
				if node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
					node = *node.Content[0]
				}
			}
			got, err := OpaqueYAMLToJSON(node)
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid(got) || string(got) != test.want {
				t.Fatalf("got %s, want %s", got, test.want)
			}
		})
	}
}

func TestOpaqueYAMLToJSONRejectsNonStringMapKey(t *testing.T) {
	var node yaml.Node
	if err := yaml.Unmarshal([]byte("1: value\n"), &node); err != nil {
		t.Fatal(err)
	}
	if _, err := OpaqueYAMLToJSON(*node.Content[0]); err == nil {
		t.Fatal("non-string mapping key must fail")
	}
}
