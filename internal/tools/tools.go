package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

// ProposalProvider optionally prepares a reviewable mutation before the
// runtime opens an approval interrupt. Read-only tools do not implement it.
type ProposalProvider interface {
	PrepareProposal(context.Context, json.RawMessage) (domain.ToolProposal, error)
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
		param, ok := spec.Params[name]
		if !ok {
			return &ArgError{Field: name, Reason: "is not declared by the tool schema"}
		}
		if string(value) == "null" {
			return &ArgError{Field: name, Reason: "must not be null"}
		}
		if err := validateParamJSON(value, param.Type); err != nil {
			return &ArgError{Field: name, Reason: err.Error()}
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

func validateParamJSON(value json.RawMessage, paramType string) error {
	if paramType == "" || paramType == "string" {
		var text string
		if err := json.Unmarshal(value, &text); err != nil {
			return errors.New("must be a string")
		}
		return nil
	}
	var decoded any
	if err := json.Unmarshal(value, &decoded); err != nil {
		return errors.New("must be valid JSON")
	}
	valid := false
	switch paramType {
	case "integer":
		_, valid = decoded.(float64)
		if valid {
			valid = float64(int64(decoded.(float64))) == decoded.(float64)
		}
	case "number":
		_, valid = decoded.(float64)
	case "boolean":
		_, valid = decoded.(bool)
	case "object":
		_, valid = decoded.(map[string]any)
	case "array":
		_, valid = decoded.([]any)
	default:
		return fmt.Errorf("unsupported schema type %q", paramType)
	}
	if !valid {
		return errors.New("has the wrong JSON type")
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

// tokenSet splits a query into normalized tokens. It backs tool_search
// catalog matching; request binding no longer routes on it (every request
// binds the full enabled set).
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
	// order preserves registration order so catalog views (Settings tool
	// surface) are deterministic.
	order []string
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
		r.order = append(r.order, name)
	}
	return r
}

// Specs returns every registered manifest in registration order — the
// full active+hidden catalog, independent of the enabled list.
func (r *Registry) Specs() []domain.ToolSpec {
	out := make([]domain.ToolSpec, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.byName[name].Spec())
	}
	return out
}

// Builtin returns the registry of shipped non-filesystem tools. It preserves
// the V0 test surface; production wiring should use BuiltinWithFileOps so the
// complete workspace tool family is available.
func Builtin(notes storage.NoteStore) *Registry {
	return BuiltinWithFileOps(notes, nil)
}

// BuiltinWithFileOps returns the shipped tools plus the complete Vivy
// filesystem family. The backend is deliberately injected so this package
// stays independent of Eino and runtime workspace implementation details.
func BuiltinWithFileOps(notes storage.NoteStore, files FileOperations) *Registry {
	return NewRegistry(
		NewEchoInfo(), NewWriteNote(notes), NewListNotes(notes), NewReadNote(notes), NewAskUser(),
		NewListDir(files), NewReadFile(files), NewSearchFiles(files), NewWriteFile(files), NewPatch(files),
	)
}

// BuiltinWithCapabilities adds the Skill family while keeping the old
// BuiltinWithFileOps constructor source-compatible for existing callers.
func BuiltinWithCapabilities(notes storage.NoteStore, files FileOperations, skills SkillOperations) *Registry {
	return BuiltinWithTodo(notes, files, skills, nil)
}

// BuiltinWithTodo adds the durable task family.
func BuiltinWithTodo(notes storage.NoteStore, files FileOperations, skills SkillOperations, todos TodoOperations) *Registry {
	return BuiltinWithSearch(notes, files, skills, todos, nil)
}

// BuiltinWithSearch adds the API-backed network search tool. The backend is
// injected so this package remains independent of HTTP and provider details.
func BuiltinWithSearch(notes storage.NoteStore, files FileOperations, skills SkillOperations, todos TodoOperations, search SearchOperations) *Registry {
	return BuiltinWithHTTP(notes, files, skills, todos, search, nil)
}

// BuiltinWithHTTP adds the read-only, policy-backed HTTP tool.
func BuiltinWithHTTP(notes storage.NoteStore, files FileOperations, skills SkillOperations, todos TodoOperations, search SearchOperations, httpOps HTTPOperations) *Registry {
	return BuiltinWithMCP(notes, files, skills, todos, search, httpOps, nil)
}

// BuiltinWithMCP adds the MCP catalog and approval-gated call surface.
func BuiltinWithMCP(notes storage.NoteStore, files FileOperations, skills SkillOperations, todos TodoOperations, search SearchOperations, httpOps HTTPOperations, mcpOps MCPOperations) *Registry {
	return BuiltinWithSequential(notes, files, skills, todos, search, httpOps, mcpOps, nil)
}

// BuiltinWithSequential adds the local reasoning-state tool.
func BuiltinWithSequential(notes storage.NoteStore, files FileOperations, skills SkillOperations, todos TodoOperations, search SearchOperations, httpOps HTTPOperations, mcpOps MCPOperations, sequential SequentialThinkingOperations) *Registry {
	return BuiltinWithCommands(notes, files, skills, todos, search, httpOps, mcpOps, sequential, nil)
}

// BuiltinWithCommands adds both controlled process tool names over one backend.
func BuiltinWithCommands(notes storage.NoteStore, files FileOperations, skills SkillOperations, todos TodoOperations, search SearchOperations, httpOps HTTPOperations, mcpOps MCPOperations, sequential SequentialThinkingOperations, commands CommandOperations) *Registry {
	registered := []Tool{
		NewEchoInfo(), NewWriteNote(notes), NewListNotes(notes), NewReadNote(notes), NewAskUser(),
		NewListDir(files), NewReadFile(files), NewSearchFiles(files), NewWriteFile(files), NewPatch(files),
		NewSkillsList(skills), NewSkillView(skills), NewSkillManage(skills),
		NewTaskCreate(todos), NewTaskGet(todos), NewTaskUpdate(todos), NewTaskList(todos),
	}
	if search != nil {
		registered = append(registered, NewNetworkSearch(search))
	}
	if httpOps != nil {
		registered = append(registered, NewHTTPRequest(httpOps))
	}
	if mcpOps != nil {
		registered = append(registered, NewMCPListTools(mcpOps), NewMCPCall(mcpOps))
	}
	if sequential != nil {
		registered = append(registered, NewSequentialThinking(sequential))
	}
	if commands != nil {
		registered = append(registered, NewExecute(commands), NewCommandline(commands))
	}
	registered = append(registered, NewToolSearch(baseToolsForSearch(notes, files, skills, todos, search, httpOps, mcpOps, sequential, commands)))
	return NewRegistry(registered...)
}

func baseToolsForSearch(notes storage.NoteStore, files FileOperations, skills SkillOperations, todos TodoOperations, search SearchOperations, httpOps HTTPOperations, mcpOps MCPOperations, sequential SequentialThinkingOperations, commands CommandOperations) []Tool {
	registered := []Tool{
		NewEchoInfo(), NewWriteNote(notes), NewListNotes(notes), NewReadNote(notes), NewAskUser(),
		NewListDir(files), NewReadFile(files), NewSearchFiles(files), NewWriteFile(files), NewPatch(files), NewSkillsList(skills), NewSkillView(skills), NewSkillManage(skills),
		NewTaskCreate(todos), NewTaskGet(todos), NewTaskUpdate(todos), NewTaskList(todos),
	}
	if search != nil {
		registered = append(registered, NewNetworkSearch(search))
	}
	if httpOps != nil {
		registered = append(registered, NewHTTPRequest(httpOps))
	}
	if mcpOps != nil {
		registered = append(registered, NewMCPListTools(mcpOps), NewMCPCall(mcpOps))
	}
	if sequential != nil {
		registered = append(registered, NewSequentialThinking(sequential))
	}
	if commands != nil {
		registered = append(registered, NewExecute(commands), NewCommandline(commands))
	}
	return registered
}

// Resolve selects the enabled tools by name, preserving order. An unknown
// name is a startup error (FR-10: config names must resolve).
func (r *Registry) Resolve(enabled []string) ([]Tool, error) {
	out := make([]Tool, 0, len(enabled))
	for _, name := range enabled {
		if IsBrowserUseName(name) {
			return nil, fmt.Errorf("tools: browser automation tool %q is excluded from Vivy", name)
		}
		t, ok := r.byName[name]
		if !ok {
			return nil, fmt.Errorf("tools: unknown tool %q in tools.enabled", name)
		}
		out = append(out, t)
	}
	if search, ok := r.byName[ToolSearchName].(*toolSearchTool); ok {
		search.restrict(enabled)
	}
	return out, nil
}
