package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
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

// ValidateArgs applies the manifest-level schema before a tool is invoked.
// Individual tools may apply narrower domain validation afterwards, but an
// invalid shape never reaches a side effect.
func ValidateArgs(spec domain.ToolSpec, args json.RawMessage) error {
	trimmed := bytes.TrimSpace(args)
	if len(trimmed) == 0 && len(spec.Params) == 0 {
		return nil
	}
	var fields map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	if err := dec.Decode(&fields); err != nil || fields == nil {
		if err == nil {
			err = fmt.Errorf("must be a JSON object")
		}
		return &ArgError{Field: "args", Reason: fmt.Sprintf("must be a JSON object: %v", err)}
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return &ArgError{Field: "args", Reason: fmt.Sprintf("must contain one JSON object: %v", err)}
	}
	for name, value := range fields {
		if _, ok := spec.Params[name]; !ok {
			return &ArgError{Field: name, Reason: "is not declared by the tool schema"}
		}
		if string(value) == "null" {
			return &ArgError{Field: name, Reason: "must be a string"}
		}
		var text string
		if err := json.Unmarshal(value, &text); err != nil {
			return &ArgError{Field: name, Reason: "must be a string"}
		}
	}
	for name, param := range spec.Params {
		if param.Required {
			if _, ok := fields[name]; !ok {
				return &ArgError{Field: name, Reason: "is required"}
			}
		}
	}
	return nil
}

// Selection is the request-scoped tool surface. It preserves registry order
// so provider schemas and prompts remain deterministic.
type Selection struct {
	Tools []Tool
	Specs []domain.ToolSpec
}

// Names returns a copy of the selected tool names for context propagation.
func (s Selection) Names() []string {
	out := make([]string, 0, len(s.Specs))
	for _, spec := range s.Specs {
		out = append(out, spec.Name)
	}
	return out
}

// Selector performs conservative, deterministic request routing. A tool is
// selected only when the request contains one of its explicit keywords or a
// name component; unrelated requests receive no callable tools.
type Selector struct {
	tools []Tool
}

// NewSelector builds a selector over the already config-filtered tools.
func NewSelector(ts []Tool) *Selector {
	copyTools := append([]Tool(nil), ts...)
	return &Selector{tools: copyTools}
}

// Select returns the tools whose manifest terms occur in the request. Token
// matching is case-insensitive and uses explicit manifest keywords, which
// keeps routing stable without an embedding or model call.
func (s *Selector) Select(request string) Selection {
	query := tokenSet(request)
	selection := Selection{}
	if len(query) == 0 {
		return selection
	}
	maxScore := 0
	scores := make([]int, len(s.tools))
	for i, tool := range s.tools {
		spec := tool.Spec()
		score := toolScore(spec, query)
		scores[i] = score
		if score > maxScore {
			maxScore = score
		}
	}
	if maxScore == 0 {
		return selection
	}
	for i, tool := range s.tools {
		if scores[i] != maxScore {
			continue
		}
		selection.Tools = append(selection.Tools, tool)
		selection.Specs = append(selection.Specs, tool.Spec())
	}
	return selection
}

func toolScore(spec domain.ToolSpec, query map[string]struct{}) int {
	terms := append([]string(nil), spec.Keywords...)
	if len(terms) == 0 {
		// Custom tools without explicit routing hints fall back to their
		// name. Builtins provide keywords so generic words such as "note"
		// do not accidentally select several note tools at once.
		terms = strings.FieldsFunc(spec.Name, func(r rune) bool { return r == '_' || r == '-' })
	}
	score := 0
	for _, term := range terms {
		if _, ok := query[normalizeToken(term)]; ok {
			weight := 2
			if token := normalizeToken(term); token == "note" || token == "notes" {
				weight = 1
			}
			score += weight
		}
	}
	return score
}

func tokenSet(value string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, raw := range strings.FieldsFunc(value, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		token := normalizeToken(raw)
		if token != "" {
			set[token] = struct{}{}
		}
	}
	return set
}

func normalizeToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
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

// Builtin returns the registry of shipped tools: one read-only
// auto-execute probe tool and the notebook trio (MA-3) — read-only
// list/read tools plus the effectful approval-gated write tool (D-012).
// notes backs the trio; a nil store keeps the tools resolvable but
// failing fast at call time, which is how tests that only use other
// tools wire it.
func Builtin(notes storage.NoteStore) *Registry {
	return NewRegistry(NewEchoInfo(), NewWriteNote(notes), NewListNotes(notes), NewReadNote(notes))
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
