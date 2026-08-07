package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-vivy/internal/domain"
)

// Tool is the Vivy-owned callable contract. The Eino wrapping
// (tool.BaseTool) happens in internal/runtime (C3/C6); this package never
// imports Eino. Readonly tools execute automatically; effectful tools are
// approval-gated before InvokableRun is ever called (D-012).
type Tool interface {
	Spec() domain.ToolSpec
	// InvokableRun executes one call. args is a JSON object whose shape is
	// fixed by Spec; invalid args yield *ArgError, never a panic.
	InvokableRun(ctx context.Context, args json.RawMessage) (string, error)
}

// ArgError is a structured argument validation failure, safe to surface in
// tool.finished payloads.
type ArgError struct {
	Field  string
	Reason string
}

func (e *ArgError) Error() string {
	return fmt.Sprintf("tool argument %q: %s", e.Field, e.Reason)
}

// Registry maps tool names to implementations. Registration is
// Vivy-owned, not Eino's.
type Registry struct {
	byName map[string]Tool
}

// NewRegistry builds a registry from the given tools; duplicate names are
// a programmer error and panic.
func NewRegistry(ts ...Tool) *Registry {
	r := &Registry{byName: make(map[string]Tool, len(ts))}
	for _, t := range ts {
		name := t.Spec().Name
		if _, dup := r.byName[name]; dup {
			panic(fmt.Sprintf("tools: duplicate registration of %q", name))
		}
		r.byName[name] = t
	}
	return r
}

// Builtin returns the registry of V0 shipped tools: one read-only
// auto-execute tool and one effectful approval-gated tool (D-012).
func Builtin() *Registry {
	return NewRegistry(NewEchoInfo(), NewWriteNote())
}

// Resolve selects the enabled tools by name, preserving order. An unknown
// name is a startup error (FR-10: config names must resolve).
func (r *Registry) Resolve(enabled []string) ([]Tool, error) {
	out := make([]Tool, 0, len(enabled))
	for _, name := range enabled {
		t, ok := r.byName[name]
		if !ok {
			return nil, fmt.Errorf("tools: unknown tool %q in tools.enabled", name)
		}
		out = append(out, t)
	}
	return out, nil
}
