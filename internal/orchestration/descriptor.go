// Package orchestration contains the host-owned, versioned workflow graph
// contract. It deliberately has no dependency on Eino or runtime execution.
package orchestration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"agent-vivy/internal/domain"
)

const (
	SchemaVersion     = 1
	MaxNodes          = 12
	MaxEdges          = 24
	MaxDepth          = 6
	MaxWidth          = 4
	MaxOutputs        = 4
	MaxTaskBytes      = 4 << 10
	MaxTotalTaskBytes = 16 << 10
	MaxOutputBytes    = 8 << 10
)

var nodeKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,47}$`)

type Descriptor struct {
	SchemaVersion int      `json:"schema_version"`
	StartNodes    []string `json:"start_nodes"`
	Nodes         []Node   `json:"nodes"`
	Edges         []Edge   `json:"edges"`
	Outputs       []string `json:"outputs"`
}

type Node struct {
	Key       string   `json:"key"`
	Task      string   `json:"task"`
	ToolNames []string `json:"tool_names,omitempty"`
}

// Edge.InputKey maps the predecessor's bounded textual result into a named
// dependency input. An empty key is an order-only dependency.
type Edge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	InputKey string `json:"input_key,omitempty"`
}

type ValidatedDescriptor struct {
	Descriptor
	CanonicalJSON []byte
	Digest        string
	Topological   []string
	Layers        [][]string
}

// ValidationError carries the descriptor path the author can repair.
type ValidationError struct {
	Path   string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("workflow descriptor %s: %s", e.Path, e.Reason)
}

// Validate checks graph shape, resource bounds and per-node tool narrowing,
// then returns a deterministic immutable representation and SHA-256 digest.
func Validate(descriptor Descriptor, allowedTools []string) (ValidatedDescriptor, error) {
	if descriptor.SchemaVersion != SchemaVersion {
		return ValidatedDescriptor{}, invalid("schema_version", "unsupported schema version")
	}
	if len(descriptor.Nodes) == 0 || len(descriptor.Nodes) > MaxNodes {
		return ValidatedDescriptor{}, invalid("nodes", fmt.Sprintf("must contain between 1 and %d nodes", MaxNodes))
	}
	if len(descriptor.Edges) > MaxEdges {
		return ValidatedDescriptor{}, invalid("edges", fmt.Sprintf("must contain at most %d edges", MaxEdges))
	}
	if len(descriptor.Outputs) == 0 || len(descriptor.Outputs) > MaxOutputs {
		return ValidatedDescriptor{}, invalid("outputs", fmt.Sprintf("must contain between 1 and %d node keys", MaxOutputs))
	}

	allowed, err := canonicalTools(allowedTools, "allowed_tools")
	if err != nil {
		return ValidatedDescriptor{}, err
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, name := range allowed {
		allowedSet[name] = struct{}{}
	}

	d := Descriptor{SchemaVersion: SchemaVersion}
	d.Nodes = make([]Node, len(descriptor.Nodes))
	nodes := make(map[string]Node, len(descriptor.Nodes))
	totalTaskBytes := 0
	for i, node := range descriptor.Nodes {
		path := fmt.Sprintf("nodes[%d]", i)
		if !nodeKeyPattern.MatchString(node.Key) {
			return ValidatedDescriptor{}, invalid(path+".key", "must be a stable lowercase key of at most 48 characters")
		}
		if _, exists := nodes[node.Key]; exists {
			return ValidatedDescriptor{}, invalid(path+".key", "duplicate node key")
		}
		if strings.TrimSpace(node.Task) == "" || len([]byte(node.Task)) > MaxTaskBytes {
			return ValidatedDescriptor{}, invalid(path+".task", fmt.Sprintf("must be non-empty and at most %d bytes", MaxTaskBytes))
		}
		totalTaskBytes += len([]byte(node.Task))
		if totalTaskBytes > MaxTotalTaskBytes {
			return ValidatedDescriptor{}, invalid("nodes", fmt.Sprintf("combined task text exceeds %d bytes", MaxTotalTaskBytes))
		}
		toolNames, err := canonicalTools(node.ToolNames, path+".tool_names")
		if err != nil {
			return ValidatedDescriptor{}, err
		}
		for _, toolName := range toolNames {
			if _, ok := allowedSet[toolName]; !ok {
				return ValidatedDescriptor{}, invalid(path+".tool_names", fmt.Sprintf("tool %q widens parent authority", toolName))
			}
		}
		node.ToolNames = toolNames
		nodes[node.Key] = node
		d.Nodes[i] = node
	}

	startSet, starts, err := uniqueKnownKeys(descriptor.StartNodes, nodes, "start_nodes")
	if err != nil {
		return ValidatedDescriptor{}, err
	}
	if len(starts) == 0 {
		return ValidatedDescriptor{}, invalid("start_nodes", "must name every root node")
	}
	_, outputs, err := uniqueKnownKeys(descriptor.Outputs, nodes, "outputs")
	if err != nil {
		return ValidatedDescriptor{}, err
	}

	indegree := make(map[string]int, len(nodes))
	children := make(map[string][]string, len(nodes))
	parents := make(map[string][]string, len(nodes))
	edgeIDs := make(map[string]struct{}, len(descriptor.Edges))
	inputKeys := make(map[string]struct{}, len(descriptor.Edges))
	d.Edges = append([]Edge(nil), descriptor.Edges...)
	for i, edge := range descriptor.Edges {
		path := fmt.Sprintf("edges[%d]", i)
		if _, ok := nodes[edge.From]; !ok {
			return ValidatedDescriptor{}, invalid(path+".from", "references an unknown node")
		}
		if _, ok := nodes[edge.To]; !ok {
			return ValidatedDescriptor{}, invalid(path+".to", "references an unknown node")
		}
		if edge.From == edge.To {
			return ValidatedDescriptor{}, invalid(path, "self edges are not allowed")
		}
		if edge.InputKey != "" && !nodeKeyPattern.MatchString(edge.InputKey) {
			return ValidatedDescriptor{}, invalid(path+".input_key", "must be a stable lowercase key")
		}
		id := edge.From + "\x00" + edge.To
		if _, ok := edgeIDs[id]; ok {
			return ValidatedDescriptor{}, invalid(path, "duplicate dependency")
		}
		edgeIDs[id] = struct{}{}
		if edge.InputKey != "" {
			mappingID := edge.To + "\x00" + edge.InputKey
			if _, ok := inputKeys[mappingID]; ok {
				return ValidatedDescriptor{}, invalid(path+".input_key", "duplicate input mapping on target node")
			}
			inputKeys[mappingID] = struct{}{}
		}
		indegree[edge.To]++
		children[edge.From] = append(children[edge.From], edge.To)
		parents[edge.To] = append(parents[edge.To], edge.From)
	}

	for key := range nodes {
		if indegree[key] == 0 {
			if _, declared := startSet[key]; !declared {
				return ValidatedDescriptor{}, invalid("start_nodes", fmt.Sprintf("root node %q is not declared", key))
			}
		} else if _, declared := startSet[key]; declared {
			return ValidatedDescriptor{}, invalid("start_nodes", fmt.Sprintf("node %q has dependencies and cannot be a root", key))
		}
	}

	depth := make(map[string]int, len(nodes))
	for _, key := range starts {
		depth[key] = 1
	}
	layers := make([][]string, 0)
	order := make([]string, 0, len(nodes))
	remaining := make(map[string]int, len(indegree))
	for key, count := range indegree {
		remaining[key] = count
	}
	ready := append([]string(nil), starts...)
	for len(ready) > 0 {
		sort.Strings(ready)
		if len(ready) > MaxWidth {
			return ValidatedDescriptor{}, invalid("width", fmt.Sprintf("parallel layer exceeds %d nodes", MaxWidth))
		}
		layer := append([]string(nil), ready...)
		layers = append(layers, layer)
		next := make([]string, 0)
		for _, key := range ready {
			order = append(order, key)
			for _, child := range children[key] {
				if depth[child] < depth[key]+1 {
					depth[child] = depth[key] + 1
				}
				remaining[child]--
				if remaining[child] == 0 {
					next = append(next, child)
				}
			}
		}
		ready = next
	}
	if len(order) != len(nodes) {
		return ValidatedDescriptor{}, invalid("edges", "contains a dependency cycle")
	}
	for key, level := range depth {
		if level > MaxDepth {
			return ValidatedDescriptor{}, invalid("depth", fmt.Sprintf("node %q exceeds maximum depth %d", key, MaxDepth))
		}
	}

	reachable := make(map[string]struct{}, len(nodes))
	queue := append([]string(nil), starts...)
	for len(queue) > 0 {
		key := queue[0]
		queue = queue[1:]
		if _, seen := reachable[key]; seen {
			continue
		}
		reachable[key] = struct{}{}
		queue = append(queue, children[key]...)
	}
	if len(reachable) != len(nodes) {
		return ValidatedDescriptor{}, invalid("nodes", "contains nodes unreachable from the declared roots")
	}

	consumed := make(map[string]struct{}, len(nodes))
	queue = append(queue[:0], outputs...)
	for len(queue) > 0 {
		key := queue[0]
		queue = queue[1:]
		if _, seen := consumed[key]; seen {
			continue
		}
		consumed[key] = struct{}{}
		queue = append(queue, parents[key]...)
	}
	if len(consumed) != len(nodes) {
		return ValidatedDescriptor{}, invalid("outputs", "every node must contribute by dependency to a declared output")
	}

	d.StartNodes = starts
	d.Outputs = outputs
	sort.Slice(d.Nodes, func(i, j int) bool { return d.Nodes[i].Key < d.Nodes[j].Key })
	sort.Slice(d.Edges, func(i, j int) bool {
		if d.Edges[i].From != d.Edges[j].From {
			return d.Edges[i].From < d.Edges[j].From
		}
		if d.Edges[i].To != d.Edges[j].To {
			return d.Edges[i].To < d.Edges[j].To
		}
		return d.Edges[i].InputKey < d.Edges[j].InputKey
	})
	encoded, err := json.Marshal(d)
	if err != nil {
		return ValidatedDescriptor{}, fmt.Errorf("workflow descriptor: encode canonical JSON: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return ValidatedDescriptor{
		Descriptor: d, CanonicalJSON: encoded, Digest: hex.EncodeToString(sum[:]),
		Topological: order, Layers: layers,
	}, nil
}

// DecodeValidated verifies stored descriptor bytes are canonical and still
// satisfy the current schema, bounds and authority ceiling.
func DecodeValidated(encoded []byte, expectedDigest string, allowedTools []string) (ValidatedDescriptor, error) {
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.DisallowUnknownFields()
	var descriptor Descriptor
	if err := decoder.Decode(&descriptor); err != nil {
		return ValidatedDescriptor{}, invalid("descriptor", "stored JSON is invalid")
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return ValidatedDescriptor{}, invalid("descriptor", "contains trailing JSON data")
	}
	validated, err := Validate(descriptor, allowedTools)
	if err != nil {
		return ValidatedDescriptor{}, err
	}
	if validated.Digest != expectedDigest || string(validated.CanonicalJSON) != string(encoded) {
		return ValidatedDescriptor{}, invalid("digest", "does not match canonical descriptor contents")
	}
	return validated, nil
}

func canonicalTools(names []string, path string) ([]string, error) {
	namesCopy := append([]string(nil), names...)
	canonical, err := domain.CanonicalToolNames(namesCopy)
	if err != nil {
		return nil, invalid(path, "contains an invalid or duplicate tool name")
	}
	return canonical, nil
}

func uniqueKnownKeys(values []string, nodes map[string]Node, path string) (map[string]struct{}, []string, error) {
	set := make(map[string]struct{}, len(values))
	for i, key := range values {
		if _, exists := nodes[key]; !exists {
			return nil, nil, invalid(fmt.Sprintf("%s[%d]", path, i), "references an unknown node")
		}
		if _, exists := set[key]; exists {
			return nil, nil, invalid(fmt.Sprintf("%s[%d]", path, i), "contains a duplicate key")
		}
		set[key] = struct{}{}
	}
	canonical := make([]string, 0, len(set))
	for key := range set {
		canonical = append(canonical, key)
	}
	sort.Strings(canonical)
	return set, canonical, nil
}

func invalid(path, reason string) error { return &ValidationError{Path: path, Reason: reason} }
