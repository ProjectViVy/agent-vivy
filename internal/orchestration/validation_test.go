package orchestration

import (
	"strings"
	"testing"
)

func validDescriptor() Descriptor {
	return Descriptor{
		SchemaVersion: SchemaVersion,
		StartNodes:    []string{"a", "b"},
		Nodes: []Node{
			{Key: "a", Task: "read the first source", ToolNames: []string{"read_file"}},
			{Key: "b", Task: "read the second source"},
			{Key: "join", Task: "combine the approved results"},
		},
		Edges:   []Edge{{From: "a", To: "join", InputKey: "first"}, {From: "b", To: "join", InputKey: "second"}},
		Outputs: []string{"join"},
	}
}

func TestValidateDescriptorRejectsUnsafeGraphs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Descriptor)
		tools  []string
		path   string
	}{
		{name: "empty", mutate: func(d *Descriptor) { d.Nodes = nil }, path: "nodes"},
		{name: "duplicate node", mutate: func(d *Descriptor) { d.Nodes = append(d.Nodes, d.Nodes[0]) }, path: "nodes["},
		{name: "unknown edge source", mutate: func(d *Descriptor) { d.Edges[0].From = "missing" }, path: "edges["},
		{name: "self edge", mutate: func(d *Descriptor) { d.Edges = append(d.Edges, Edge{From: "a", To: "a"}) }, path: "edges["},
		{name: "cycle", mutate: func(d *Descriptor) {
			d.StartNodes = []string{"b"}
			d.Edges = append(d.Edges, Edge{From: "join", To: "a"})
		}, path: "edges"},
		{name: "unreachable", mutate: func(d *Descriptor) { d.Nodes = append(d.Nodes, Node{Key: "orphan", Task: "do work"}) }, path: "start_nodes"},
		{name: "unconsumed output", mutate: func(d *Descriptor) { d.Outputs = []string{"a"} }, path: "outputs"},
		{name: "oversized task", mutate: func(d *Descriptor) { d.Nodes[0].Task = strings.Repeat("x", MaxTaskBytes+1) }, path: "nodes["},
		{name: "too many nodes", mutate: func(d *Descriptor) {
			for i := 0; i < MaxNodes; i++ {
				d.Nodes = append(d.Nodes, Node{Key: "extra" + string(rune('a'+i)), Task: "task"})
			}
		}, path: "nodes"},
		{name: "too deep", mutate: func(d *Descriptor) {
			d.StartNodes = []string{"a"}
			d.Nodes = []Node{{Key: "a", Task: "a"}, {Key: "b", Task: "b"}, {Key: "c", Task: "c"}, {Key: "d", Task: "d"}, {Key: "e", Task: "e"}, {Key: "f", Task: "f"}, {Key: "g", Task: "g"}, {Key: "h", Task: "h"}, {Key: "i", Task: "i"}}
			d.Edges = []Edge{{From: "a", To: "b"}, {From: "b", To: "c"}, {From: "c", To: "d"}, {From: "d", To: "e"}, {From: "e", To: "f"}, {From: "f", To: "g"}, {From: "g", To: "h"}, {From: "h", To: "i"}}
			d.Outputs = []string{"i"}
		}, path: "depth"},
		{name: "too wide", mutate: func(d *Descriptor) {
			d.StartNodes = []string{"a", "b", "c", "d", "e"}
			d.Nodes = []Node{{Key: "a", Task: "a"}, {Key: "b", Task: "b"}, {Key: "c", Task: "c"}, {Key: "d", Task: "d"}, {Key: "e", Task: "e"}, {Key: "join", Task: "join"}}
			d.Edges = []Edge{{From: "a", To: "join"}, {From: "b", To: "join"}, {From: "c", To: "join"}, {From: "d", To: "join"}, {From: "e", To: "join"}}
			d.Outputs = []string{"join"}
		}, path: "width"},
		{name: "tool widening", mutate: func(d *Descriptor) { d.Nodes[0].ToolNames = []string{"write_file"} }, tools: []string{"read_file"}, path: "tool_names"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := validDescriptor()
			tt.mutate(&d)
			parentTools := tt.tools
			if parentTools == nil {
				parentTools = []string{"read_file"}
			}
			_, err := Validate(d, parentTools)
			if err == nil {
				t.Fatal("Validate() unexpectedly succeeded")
			}
			if !strings.Contains(err.Error(), tt.path) {
				t.Fatalf("error = %q, want path containing %q", err, tt.path)
			}
		})
	}
}

func TestValidateDescriptorCanonicalizesStableIdentity(t *testing.T) {
	left := validDescriptor()
	right := validDescriptor()
	left.Nodes[0], left.Nodes[2] = left.Nodes[2], left.Nodes[0]
	left.Edges[0], left.Edges[1] = left.Edges[1], left.Edges[0]
	left.StartNodes = []string{"b", "a"}
	right.Nodes[0].ToolNames = []string{"read_file"}
	validatedLeft, err := Validate(left, []string{"read_file", "list_files"})
	if err != nil {
		t.Fatal(err)
	}
	validatedRight, err := Validate(right, []string{"list_files", "read_file"})
	if err != nil {
		t.Fatal(err)
	}
	if validatedLeft.Digest != validatedRight.Digest {
		t.Fatalf("digests differ: %s != %s", validatedLeft.Digest, validatedRight.Digest)
	}
	if string(validatedLeft.CanonicalJSON) != string(validatedRight.CanonicalJSON) {
		t.Fatalf("canonical JSON differs:\n%s\n%s", validatedLeft.CanonicalJSON, validatedRight.CanonicalJSON)
	}
}

func TestDecodeValidatedRejectsNonCanonicalOrMutatedDescriptor(t *testing.T) {
	validated, err := Validate(validDescriptor(), []string{"read_file"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeValidated(validated.CanonicalJSON, validated.Digest, []string{"read_file"}); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeValidated(validated.CanonicalJSON, strings.Repeat("0", 64), []string{"read_file"}); err == nil {
		t.Fatal("digest mismatch was accepted")
	}
	if _, err := DecodeValidated([]byte(`{"schema_version":1,"nodes":[],"edges":[],"outputs":[]}`), validated.Digest, nil); err == nil {
		t.Fatal("invalid mutated descriptor was accepted")
	}
}
