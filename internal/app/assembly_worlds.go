package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
	"agent-vivy/sdk/module"
	toolworldport "agent-vivy/sdk/port/toolworld"
)

type worldWorkspaceLookup func(context.Context) (string, error)

type generatedWriteDiagnostics struct {
	observers []toolworldport.DiagnosticObserver
	worldIDs  []string
	grants    map[string][]module.GrantBinding
	lookup    worldWorkspaceLookup
	recorder  tools.FileVersionRecorder
}

func (source generatedWriteDiagnostics) WriteDiagnostics(ctx context.Context, paths []string) []string {
	var out []string
	for index, observer := range source.observers {
		if observer == nil || index >= len(source.worldIDs) {
			continue
		}
		worldID := source.worldIDs[index]
		allowed := make(map[module.Grant][]string)
		for _, grant := range source.grants[worldID] {
			allowed[grant.Name] = append([]string(nil), grant.Constraints[constraintKey(grant.Name)]...)
		}
		host := generatedWorldHost{moduleID: worldID, grants: allowed, lookup: source.lookup, recorder: source.recorder, ctx: ctx}
		out = append(out, observer.ObserveWrite(ctx, host, paths)...)
	}
	return out
}

func assemblyHasToolWorld(providers []toolworldport.Provider, id string) bool {
	for _, provider := range providers {
		if provider != nil && provider.Definition().ID == id {
			return true
		}
	}
	return false
}

type generatedWorldTool struct {
	provider toolworldport.Provider
	def      toolworldport.ToolDefinition
	grants   map[module.Grant][]string
	lookup   worldWorkspaceLookup
	recorder tools.FileVersionRecorder
}

func (tool generatedWorldTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: tool.def.ID, Description: tool.def.Description,
		Readonly: tool.def.Effect == toolworldport.EffectRead,
		Keywords: []string{tool.provider.Definition().ID, tool.def.ID}, Params: schemaParams(tool.def.Schema)}
}

func (tool generatedWorldTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	host := generatedWorldHost{moduleID: tool.provider.Definition().ID, grants: tool.grants, lookup: tool.lookup, recorder: tool.recorder, ctx: ctx}
	result, err := tool.provider.Invoke(ctx, host, tool.def.ID, args)
	return result.Text, err
}

func bindToolWorlds(ctx context.Context, providers []toolworldport.Provider, grants map[string][]module.GrantBinding, lookup worldWorkspaceLookup, recorder tools.FileVersionRecorder) ([]tools.Tool, error) {
	var out []tools.Tool
	seen := make(map[string]bool)
	for _, provider := range providers {
		if provider == nil || provider.Definition().ID == "" {
			return nil, fmt.Errorf("app: generated ToolWorld provider has no identity")
		}
		if provider.Definition().ID == "mcp" {
			continue
		}
		allowed := worldGrantMap(grants[provider.Definition().ID])
		host := generatedWorldHost{moduleID: provider.Definition().ID, grants: allowed, lookup: lookup, recorder: recorder, ctx: ctx}
		definitions, err := provider.Discover(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("app: discover ToolWorld %s: %w", provider.Definition().ID, err)
		}
		for _, definition := range definitions {
			if definition.ID == "" || seen[definition.ID] {
				return nil, fmt.Errorf("app: duplicate or empty ToolWorld tool id %q", definition.ID)
			}
			seen[definition.ID] = true
			out = append(out, generatedWorldTool{provider: provider, def: definition, grants: allowed, lookup: lookup, recorder: recorder})
		}
	}
	return out, nil
}

// BindToolWorlds exposes the internal ToolHost dynamic binding boundary to
// the SDK conformance suite. Product composition uses the same implementation.
func BindToolWorlds(ctx context.Context, providers []toolworldport.Provider, grants map[string][]module.GrantBinding, lookup func(context.Context) (string, error), recorder tools.FileVersionRecorder) ([]tools.Tool, error) {
	return bindToolWorlds(ctx, providers, grants, lookup, recorder)
}

func worldGrantMap(bindings []module.GrantBinding) map[module.Grant][]string {
	allowed := make(map[module.Grant][]string, len(bindings))
	for _, grant := range bindings {
		allowed[grant.Name] = append([]string(nil), grant.Constraints[constraintKey(grant.Name)]...)
	}
	return allowed
}

func closeToolWorlds(ctx context.Context, providers []toolworldport.Provider) error {
	var failures []error
	for index := len(providers) - 1; index >= 0; index-- {
		if providers[index] != nil {
			failures = append(failures, providers[index].Close(ctx))
		}
	}
	return errors.Join(failures...)
}

type generatedWorldHost struct {
	moduleID string
	grants   map[module.Grant][]string
	lookup   worldWorkspaceLookup
	recorder tools.FileVersionRecorder
	ctx      context.Context
}

func (host generatedWorldHost) ModuleID() string { return host.moduleID }
func (host generatedWorldHost) Workspace() string {
	root, _ := host.workspace()
	return root
}
func (host generatedWorldHost) OpenRead(name string) (io.ReadCloser, error) {
	if _, ok := host.grants[module.GrantFSRead]; !ok {
		return nil, toolworldport.ErrDenied
	}
	resolved, err := host.resolveGranted(name, module.GrantFSRead)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(resolved)
	if err != nil {
		return nil, err
	}
	if host.recorder != nil {
		if info, statErr := file.Stat(); statErr == nil {
			host.recorder.TrackAccess(host.ctx, tools.SessionIDFromContext(host.ctx), path.Clean(name), info.ModTime().UnixMilli())
		}
	}
	return file, nil
}
func (host generatedWorldHost) OpenWrite(name string) (io.WriteCloser, error) {
	if _, ok := host.grants[module.GrantFSWrite]; !ok {
		return nil, toolworldport.ErrDenied
	}
	resolved, err := host.resolveGranted(name, module.GrantFSWrite)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o700); err != nil {
		return nil, err
	}
	var old []byte
	if existing, readErr := os.ReadFile(resolved); readErr == nil {
		old = existing
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return nil, readErr
	}
	if host.recorder != nil {
		sessionID := tools.SessionIDFromContext(host.ctx)
		if at, ok, accessErr := host.recorder.LastAccess(host.ctx, sessionID, path.Clean(name)); accessErr == nil && ok {
			if info, statErr := os.Stat(resolved); statErr == nil && info.ModTime().UnixMilli() > at {
				return nil, fmt.Errorf("app: %s changed on disk after the last read", path.Clean(name))
			}
		}
	}
	return &generatedWorldWriter{ctx: host.ctx, path: resolved, display: path.Clean(name), old: old, recorder: host.recorder}, nil
}
func (host generatedWorldHost) Spawn(ctx context.Context, spec toolworldport.SpawnSpec) (toolworldport.Proc, error) {
	commands, ok := host.grants[module.GrantProcSpawn]
	if !ok {
		return nil, toolworldport.ErrDenied
	}
	command := strings.TrimSpace(spec.Command)
	if command == "" {
		return nil, toolworldport.ErrInvalidArgs
	}
	if len(commands) > 0 && !containsConstraint(commands, command) && !containsConstraint(commands, filepath.Base(command)) {
		return nil, toolworldport.ErrDenied
	}
	if strings.ContainsAny(command, `/\`) || filepath.IsAbs(command) || strings.Contains(command, ":") {
		resolved, err := host.resolve(command)
		if err != nil {
			return nil, err
		}
		command = resolved
	}
	root, err := host.workspace()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(context.WithoutCancel(ctx), command, spec.Args...)
	cmd.Dir = root
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, err
	}
	return generatedWorldProc{cmd: cmd, stdin: stdin, stdout: stdout, stderr: stderr}, nil
}
func (host generatedWorldHost) workspace() (string, error) {
	if host.lookup == nil {
		return "", toolworldport.ErrDenied
	}
	root, err := host.lookup(host.ctx)
	if err != nil || root == "" {
		if err == nil {
			err = toolworldport.ErrDenied
		}
		return "", err
	}
	return root, nil
}
func (host generatedWorldHost) resolve(name string) (string, error) {
	if name == "" || path.IsAbs(name) || filepath.IsAbs(name) || strings.ContainsAny(name, `:\`) {
		return "", toolworldport.ErrInvalidArgs
	}
	cleaned := path.Clean(name)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", toolworldport.ErrInvalidArgs
	}
	root, err := host.workspace()
	if err != nil {
		return "", err
	}
	full := filepath.Join(root, filepath.FromSlash(cleaned))
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", toolworldport.ErrInvalidArgs
	}
	return full, nil
}

func (host generatedWorldHost) resolveGranted(name string, grant module.Grant) (string, error) {
	full, err := host.resolve(name)
	if err != nil {
		return "", err
	}
	roots := host.grants[grant]
	if len(roots) == 0 {
		return full, nil
	}
	cleaned := path.Clean(name)
	for _, root := range roots {
		allowed := path.Clean(root)
		if allowed != "." && allowed != ".." && !path.IsAbs(allowed) && (cleaned == allowed || strings.HasPrefix(cleaned, allowed+"/")) {
			return full, nil
		}
	}
	return "", toolworldport.ErrDenied
}

func constraintKey(grant module.Grant) string {
	switch grant {
	case module.GrantFSRead, module.GrantFSWrite:
		return "roots"
	case module.GrantProcSpawn:
		return "commands"
	case module.GrantSecretRead:
		return "names"
	default:
		return ""
	}
}

func containsConstraint(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

type generatedWorldWriter struct {
	ctx      context.Context
	path     string
	display  string
	old      []byte
	recorder tools.FileVersionRecorder
	buffer   bytes.Buffer
	closed   bool
}

func (writer *generatedWorldWriter) Write(content []byte) (int, error) {
	if writer.closed {
		return 0, os.ErrClosed
	}
	if writer.buffer.Len()+len(content) > storage.FileVersionMaxBytes {
		return 0, fmt.Errorf("app: ToolWorld write exceeds %d bytes", storage.FileVersionMaxBytes)
	}
	return writer.buffer.Write(content)
}

func (writer *generatedWorldWriter) Close() error {
	if writer.closed {
		return os.ErrClosed
	}
	writer.closed = true
	content := writer.buffer.Bytes()
	if bytes.Equal(writer.old, content) {
		return nil
	}
	mode := os.FileMode(0o600)
	if info, err := os.Stat(writer.path); err == nil {
		mode = info.Mode().Perm()
	}
	temporary, err := os.CreateTemp(filepath.Dir(writer.path), ".vivy-toolworld-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, writer.path); err != nil {
		return err
	}
	if writer.recorder != nil {
		sessionID := tools.SessionIDFromContext(writer.ctx)
		writer.recorder.RecordMutation(writer.ctx, sessionID, tools.RunIDFromContext(writer.ctx), writer.display, writer.old, append([]byte(nil), content...))
		writer.recorder.TrackAccess(writer.ctx, sessionID, writer.display, time.Now().UnixMilli())
	}
	return nil
}

type generatedWorldProc struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

func (proc generatedWorldProc) Stdin() io.WriteCloser { return proc.stdin }
func (proc generatedWorldProc) Stdout() io.ReadCloser { return proc.stdout }
func (proc generatedWorldProc) Stderr() io.ReadCloser { return proc.stderr }
func (proc generatedWorldProc) Wait() error           { return proc.cmd.Wait() }
func (proc generatedWorldProc) Close() error {
	if proc.cmd.Process != nil {
		_ = proc.cmd.Process.Kill()
	}
	return proc.cmd.Wait()
}

func schemaParams(raw json.RawMessage) map[string]domain.ToolParam {
	var schema struct {
		Properties map[string]struct {
			Type        string `json:"type"`
			Description string `json:"description"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if json.Unmarshal(raw, &schema) != nil {
		return nil
	}
	required := make(map[string]bool, len(schema.Required))
	for _, name := range schema.Required {
		required[name] = true
	}
	params := make(map[string]domain.ToolParam, len(schema.Properties))
	for name, property := range schema.Properties {
		params[name] = domain.ToolParam{Type: property.Type, Desc: property.Description, Required: required[name]}
	}
	return params
}
