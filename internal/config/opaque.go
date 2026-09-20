package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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
	value, err := opaqueYAMLValue(node)
	if err != nil {
		return nil, fmt.Errorf("decode settings node: %w", err)
	}
	out, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode settings to json: %w", err)
	}
	return out, nil
}

func opaqueYAMLValue(node yaml.Node) (any, error) {
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) != 1 {
			return nil, fmt.Errorf("document must contain exactly one value")
		}
		return opaqueYAMLValue(*node.Content[0])
	}
	switch node.Kind {
	case yaml.MappingNode:
		if len(node.Content)%2 != 0 {
			return nil, fmt.Errorf("mapping has an unmatched key")
		}
		out := make(map[string]any, len(node.Content)/2)
		for index := 0; index < len(node.Content); index += 2 {
			key := node.Content[index]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
				return nil, fmt.Errorf("mapping key %q is not a string", key.Value)
			}
			value, err := opaqueYAMLValue(*node.Content[index+1])
			if err != nil {
				return nil, err
			}
			out[key.Value] = value
		}
		return out, nil
	case yaml.SequenceNode:
		out := make([]any, 0, len(node.Content))
		for _, child := range node.Content {
			value, err := opaqueYAMLValue(*child)
			if err != nil {
				return nil, err
			}
			out = append(out, value)
		}
		return out, nil
	case yaml.AliasNode:
		if node.Alias == nil {
			return nil, fmt.Errorf("alias has no target")
		}
		return opaqueYAMLValue(*node.Alias)
	case yaml.ScalarNode:
		switch node.Tag {
		case "!!null":
			return nil, nil
		case "!!bool":
			value, err := strconv.ParseBool(node.Value)
			if err != nil {
				return nil, err
			}
			return value, nil
		case "!!int", "!!float":
			return opaqueYAMLNumber(node)
		default:
			return node.Value, nil
		}
	default:
		return nil, fmt.Errorf("unsupported YAML node kind %d", node.Kind)
	}
}

func opaqueYAMLNumber(node yaml.Node) (json.Number, error) {
	lexeme := strings.ReplaceAll(node.Value, "_", "")
	if json.Valid([]byte(lexeme)) {
		var number json.Number
		decoder := json.NewDecoder(strings.NewReader(lexeme))
		decoder.UseNumber()
		if err := decoder.Decode(&number); err == nil {
			return number, nil
		}
	}
	if node.Tag == "!!int" {
		if value, err := strconv.ParseInt(lexeme, 0, 64); err == nil {
			return json.Number(strconv.FormatInt(value, 10)), nil
		}
		if value, err := strconv.ParseUint(lexeme, 0, 64); err == nil {
			return json.Number(strconv.FormatUint(value, 10)), nil
		}
	}
	value, err := strconv.ParseFloat(lexeme, 64)
	if err != nil {
		return "", fmt.Errorf("invalid numeric scalar %q", node.Value)
	}
	return json.Number(strconv.FormatFloat(value, 'g', -1, 64)), nil
}
