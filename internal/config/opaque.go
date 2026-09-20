package config

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// OpaqueYAMLToJSON serializes an opaque YAML value without interpreting its
// keys. Absent and null values become an empty JSON object.
func OpaqueYAMLToJSON(node yaml.Node) (json.RawMessage, error) {
	if node.Kind == 0 {
		return json.RawMessage("{}"), nil
	}
	if node.Kind == yaml.ScalarNode && (node.Tag == "!!null" || node.Value == "") {
		return json.RawMessage("{}"), nil
	}
	var value any
	if err := node.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode settings node: %w", err)
	}
	out, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode settings to json: %w", err)
	}
	return out, nil
}
