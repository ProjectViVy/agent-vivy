package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"

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

// InvocationClassifier optionally narrows one call's risk below the
// tool-level Readonly flag. The runtime approval gate consults it on the
// final arguments, after policy and hooks: a denied call never runs on any
// profile, and a safe call may skip the approval interrupt under the 'auto'
// approval policy.
type InvocationClassifier interface {
	ClassifyInvocation(args json.RawMessage) (InvocationClass, []string, error)
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
	if len(bytes.TrimSpace(spec.Schema)) > 0 {
		return validateJSONSchema(spec.Schema, args)
	}
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

// ValidateSchema compiles one provider JSON Schema without validating an
// instance. Callers use it at registration/discovery boundaries so malformed
// remote contracts fail closed before they enter the model-visible catalog.
func ValidateSchema(rawSchema json.RawMessage) error {
	_, err := compileJSONSchema(rawSchema)
	return err
}

func validateJSONSchema(rawSchema, rawInstance json.RawMessage) error {
	compiled, err := compileJSONSchema(rawSchema)
	if err != nil {
		return &ArgError{Field: "schema", Reason: "is invalid JSON Schema"}
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(bytes.TrimSpace(rawInstance)))
	if err != nil {
		return &ArgError{Field: "args", Reason: fmt.Sprintf("must be valid JSON: %v", err)}
	}
	if err := compiled.Validate(instance); err != nil {
		return &ArgError{Field: "args", Reason: fmt.Sprintf("does not satisfy the JSON Schema: %v", err)}
	}
	return nil
}

func compileJSONSchema(rawSchema json.RawMessage) (*jsonschema.Schema, error) {
	schemaDocument, err := jsonschema.UnmarshalJSON(bytes.NewReader(rawSchema))
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("vivy-tool-schema.json", schemaDocument); err != nil {
		return nil, err
	}
	return compiler.Compile("vivy-tool-schema.json")
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

var assemblyControlledToolNames = []string{"ask_user", "list_dir", "read_file", "search_files", "write_file", "patch", "multiedit", "execute", "bash", "skills_list", "skill_view"}

// AssemblyControlledToolNames returns the built-in identities whose presence
// is authoritative in the generated std/tool@v1 Provider inventory.
func AssemblyControlledToolNames() []string {
	return append([]string(nil), assemblyControlledToolNames...)
}

func IsAssemblyControlledTool(name string) bool {
	for _, controlled := range assemblyControlledToolNames {
		if name == controlled {
			return true
		}
	}
	return false
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

// Lookup returns one registered implementation without changing registry
// order. The app composition root uses it to bind generated Tool providers.
func (r *Registry) Lookup(name string) (Tool, bool) {
	tool, ok := r.byName[name]
	return tool, ok
}

// WithOverrides returns a registry with selected implementations replaced
// while preserving the original deterministic order.
func (r *Registry) WithOverrides(overrides map[string]Tool) *Registry {
	all := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		if replacement := overrides[name]; replacement != nil {
			all = append(all, replacement)
		} else {
			all = append(all, r.byName[name])
		}
	}
	return NewRegistry(all...)
}

func (r *Registry) WithAdditional(additional ...Tool) *Registry {
	all := make([]Tool, 0, len(r.order)+len(additional))
	for _, name := range r.order {
		all = append(all, r.byName[name])
	}
	all = append(all, additional...)
	return NewRegistry(all...)
}

// Without returns a registry with the named identities removed while
// preserving the relative order of every remaining tool.
func (r *Registry) Without(names ...string) *Registry {
	omit := make(map[string]bool, len(names))
	for _, name := range names {
		omit[name] = true
	}
	all := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		if !omit[name] {
			all = append(all, r.byName[name])
		}
	}
	return NewRegistry(all...)
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

// BuiltinWithMCP adds the MCP catalog listing surface.
func BuiltinWithMCP(notes storage.NoteStore, files FileOperations, skills SkillOperations, todos TodoOperations, search SearchOperations, httpOps HTTPOperations, mcpOps MCPOperations) *Registry {
	return BuiltinWithSequential(notes, files, skills, todos, search, httpOps, mcpOps, nil)
}

// BuiltinWithSequential adds the local reasoning-state tool.
func BuiltinWithSequential(notes storage.NoteStore, files FileOperations, skills SkillOperations, todos TodoOperations, search SearchOperations, httpOps HTTPOperations, mcpOps MCPOperations, sequential SequentialThinkingOperations) *Registry {
	return BuiltinWithCommands(notes, files, skills, todos, search, httpOps, mcpOps, sequential, nil)
}

// BuiltinWithCommands adds both controlled process tool names over one backend.
func BuiltinWithCommands(notes storage.NoteStore, files FileOperations, skills SkillOperations, todos TodoOperations, search SearchOperations, httpOps HTTPOperations, mcpOps MCPOperations, sequential SequentialThinkingOperations, commands CommandOperations) *Registry {
	return builtinWithWeb(notes, files, skills, todos, search, httpOps, mcpOps, sequential, commands, nil, nil, nil)
}

// BuiltinWithWeb adds the public-internet surface: a readonly page fetcher
// and an approval-gated workspace download.
func BuiltinWithWeb(notes storage.NoteStore, files FileOperations, skills SkillOperations, todos TodoOperations, search SearchOperations, httpOps HTTPOperations, mcpOps MCPOperations, sequential SequentialThinkingOperations, commands CommandOperations, fetch WebFetchOperations, downloads DownloadOperations) *Registry {
	return builtinWithWeb(notes, files, skills, todos, search, httpOps, mcpOps, sequential, commands, fetch, downloads, nil)
}

// BuiltinWithAgent adds the sub-agent delegation tool over the app-owned
// child-run machinery.
func BuiltinWithAgent(notes storage.NoteStore, files FileOperations, skills SkillOperations, todos TodoOperations, search SearchOperations, httpOps HTTPOperations, mcpOps MCPOperations, sequential SequentialThinkingOperations, commands CommandOperations, fetch WebFetchOperations, downloads DownloadOperations, agentOps AgentOperations) *Registry {
	return builtinWithWeb(notes, files, skills, todos, search, httpOps, mcpOps, sequential, commands, fetch, downloads, agentOps)
}

func builtinWithWeb(notes storage.NoteStore, files FileOperations, skills SkillOperations, todos TodoOperations, search SearchOperations, httpOps HTTPOperations, mcpOps MCPOperations, sequential SequentialThinkingOperations, commands CommandOperations, fetch WebFetchOperations, downloads DownloadOperations, agentOps AgentOperations) *Registry {
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
	if fetch != nil {
		registered = append(registered, NewWebFetch(fetch))
	}
	if downloads != nil {
		registered = append(registered, NewDownload(downloads))
	}
	if agentOps != nil {
		registered = append(registered, NewAgent(agentOps))
	}
	if mcpOps != nil {
		// MCP tools are projected through the generated MCP ToolWorld and the
		// sole ToolHost. Keep only the catalog listing for the control plane.
		registered = append(registered, NewMCPListTools(mcpOps))
	}
	if sequential != nil {
		registered = append(registered, NewSequentialThinking(sequential))
	}
	if commands != nil {
		registered = append(registered, NewExecute(commands), NewCommandline(commands), NewBash(commands))
		if jobs, ok := commands.(JobOperations); ok {
			registered = append(registered, NewJobOutput(jobs), NewJobKill(jobs))
		}
	}
	if searchOps, ok := files.(GrepOperations); ok {
		registered = append(registered, NewGrep(searchOps), NewGlob(searchOps))
	}
	if multi, ok := files.(MultiPatchOperations); ok {
		registered = append(registered, NewMultiEdit(multi))
	}
	return NewRegistry(registered...)
}

// Except returns the registered tools whose names are absent from enabled,
// in registration order — the hidden complement of Resolve.
func (r *Registry) Except(enabled []string) []Tool {
	enabledSet := make(map[string]struct{}, len(enabled))
	for _, name := range enabled {
		enabledSet[name] = struct{}{}
	}
	out := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		if _, ok := enabledSet[name]; !ok {
			out = append(out, r.byName[name])
		}
	}
	return out
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
	return out, nil
}
