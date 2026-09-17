// Package workflow owns the pure WF-1 definition, expression, validation, and
// plan-compilation contracts. It deliberately has no Eino or storage import.
package workflow

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"agent-vivy/internal/domain"
)

type NodeKind = domain.NodeKind

const (
	NodeKindModel = domain.WorkflowNodeModel
	NodeKindAgent = domain.WorkflowNodeAgent
	NodeKindIO    = domain.WorkflowNodeIO
)

type Edge = domain.WorkflowEdge
type OutputBinding = domain.WorkflowOutputBinding

type InputParameter struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type Definition struct {
	APIVersion  string                    `json:"schema_version"`
	ID          string                    `json:"id"`
	Title       string                    `json:"title"`
	Description string                    `json:"description,omitempty"`
	Inputs      map[string]InputParameter `json:"inputs,omitempty"`
	Nodes       []Node                    `json:"nodes"`
	Edges       []Edge                    `json:"edges"`
	Outputs     []OutputBinding           `json:"outputs,omitempty"`
}

type Node struct {
	ID        string          `json:"id"`
	Kind      NodeKind        `json:"kind"`
	Config    json.RawMessage `json:"config"`
	TimeoutMS int64           `json:"timeout_ms"`
}

// CanonicalJSON removes insignificant whitespace and orders every object key
// through encoding/json's deterministic map encoding. Arrays retain their
// author-supplied order. Raw config values are normalized recursively as part
// of the same document, so the hash never depends on input formatting.
func CanonicalJSON(raw []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(raw)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("workflow definition is not valid JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return nil, errors.New("workflow definition must contain one JSON value")
	} else if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("workflow definition has trailing JSON: %w", err)
	}
	if _, ok := value.(map[string]any); !ok {
		return nil, errors.New("workflow definition must be a JSON object")
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("canonicalize workflow definition: %w", err)
	}
	return canonical, nil
}

// Decode canonicalizes and decodes one definition, returning its stable hash.
func Decode(raw []byte) (Definition, []byte, string, error) {
	canonical, err := CanonicalJSON(raw)
	if err != nil {
		return Definition{}, nil, "", err
	}
	var definition Definition
	if err := json.Unmarshal(canonical, &definition); err != nil {
		return Definition{}, nil, "", fmt.Errorf("decode workflow definition: %w", err)
	}
	return definition, canonical, hashCanonical(canonical), nil
}

func hashCanonical(canonical []byte) string {
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:])
}

func cloneJSON(raw json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), raw...)
}
