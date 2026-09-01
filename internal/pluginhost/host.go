// Package pluginhost adapts sdk/plugin tools onto the Vivy tool contract.
// Plugins never import this package.
package pluginhost

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
	"agent-vivy/sdk/plugin"
)

// WorkspaceLookup returns the current run workspace. Missing lookups fail closed.
type WorkspaceLookup func(ctx context.Context) (string, error)

// Adapt turns compiled user plugins into first-class Vivy tools. The
// recorder (optional) routes plugin file writes into the kernel's
// file_versions chain; nil keeps writes unrecorded.
func Adapt(plugins []plugin.Plugin, lookup WorkspaceLookup, recorder tools.FileVersionRecorder) []tools.Tool {
	var out []tools.Tool
	for _, p := range plugins {
		if p == nil {
			continue
		}
		// Channel plugins are consumed by the kernel ChannelHost (C3);
		// they must never become tools.
		if p.Seam() == plugin.SeamChannel {
			continue
		}
		for _, t := range p.Tools() {
			if t == nil || t.Name() == "" {
				continue
			}
			out = append(out, hostedTool{plugin: p, tool: t, lookup: lookup, recorder: recorder})
		}
	}
	return out
}

type hostedTool struct {
	plugin   plugin.Plugin
	tool     plugin.Tool
	lookup   WorkspaceLookup
	recorder tools.FileVersionRecorder
}

func (h hostedTool) Spec() domain.ToolSpec {
	readonly := h.tool.Effect() == plugin.EffectRead
	spec := domain.ToolSpec{
		Name:        h.tool.Name(),
		Description: "User plugin tool " + h.tool.Name(),
		Readonly:    readonly,
		Keywords:    []string{h.plugin.Name(), h.tool.Name()},
		Params:      schemaParams(h.tool.Schema()),
	}
	return spec
}

func (h hostedTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	env := hostedEnv{plugin: h.plugin, lookup: h.lookup, ctx: ctx, recorder: h.recorder}
	return h.tool.Run(ctx, env, args)
}

type hostedEnv struct {
	plugin   plugin.Plugin
	lookup   WorkspaceLookup
	ctx      context.Context
	recorder tools.FileVersionRecorder
}

func (e hostedEnv) Workspace() string {
	root, err := e.workspace()
	if err != nil {
		return ""
	}
	return root
}

func (e hostedEnv) OpenRead(name string) (io.ReadCloser, error) {
	if !e.has(plugin.GrantFSRead) {
		return nil, plugin.ErrDenied
	}
	path, err := e.resolve(name)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (e hostedEnv) OpenWrite(name string) (io.WriteCloser, error) {
	if !e.has(plugin.GrantFSWrite) {
		return nil, plugin.ErrDenied
	}
	path, err := e.resolve(name)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return e.openRecordingWrite(path)
}

// Spawn starts one child process behind GrantProcSpawn (VC-3, D4). The
// command is a bare PATH name or a workspace-relative path; the child runs
// in the workspace and outlives this tool call — WithoutCancel keeps the
// context's values while dropping the run's cancellation, because a
// language server must survive between calls. The plugin owns the process
// until Close.
func (e hostedEnv) Spawn(ctx context.Context, spec plugin.SpawnSpec) (plugin.Proc, error) {
	if !e.has(plugin.GrantProcSpawn) {
		return nil, plugin.ErrDenied
	}
	command, err := e.resolveCommand(spec.Command)
	if err != nil {
		return nil, err
	}
	root, err := e.workspace()
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
	return hostedProc{cmd: cmd, stdin: stdin, stdout: stdout, stderr: stderr}, nil
}

// resolveCommand accepts a bare executable name (PATH lookup) or a
// workspace-relative path with separators. Bare names must not be joined
// onto the workspace, so the separator check precedes resolve.
func (e hostedEnv) resolveCommand(command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", plugin.ErrInvalidArgs
	}
	if !strings.ContainsAny(command, `/\`) && !filepath.IsAbs(command) && !strings.Contains(command, ":") {
		return command, nil
	}
	return e.resolve(command)
}

type hostedProc struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

func (p hostedProc) Stdin() io.WriteCloser { return p.stdin }
func (p hostedProc) Stdout() io.ReadCloser { return p.stdout }
func (p hostedProc) Stderr() io.ReadCloser { return p.stderr }

func (p hostedProc) Wait() error { return p.cmd.Wait() }

// Close kills the child and reaps it. Kill on an already-exited process is
// an error on some platforms, so its result is deliberately ignored; Wait
// reports the real outcome.
func (p hostedProc) Close() error {
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	return p.cmd.Wait()
}

func (e hostedEnv) has(need plugin.Grant) bool {
	for _, grant := range e.plugin.Grants() {
		if grant == need {
			return true
		}
	}
	return false
}

func (e hostedEnv) workspace() (string, error) {
	if e.lookup == nil {
		return "", plugin.ErrDenied
	}
	root, err := e.lookup(e.ctx)
	if err != nil || root == "" {
		if err == nil {
			err = plugin.ErrDenied
		}
		return "", err
	}
	return root, nil
}

func (e hostedEnv) resolve(name string) (string, error) {
	if name == "" || path.IsAbs(name) || filepath.IsAbs(name) || strings.ContainsAny(name, `:\`) || strings.HasPrefix(name, "/") {
		return "", plugin.ErrInvalidArgs
	}
	cleaned := path.Clean(name)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", plugin.ErrInvalidArgs
	}
	root, err := e.workspace()
	if err != nil {
		return "", err
	}
	full := filepath.Join(root, filepath.FromSlash(cleaned))
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", plugin.ErrInvalidArgs
	}
	return full, nil
}

func schemaParams(raw json.RawMessage) map[string]domain.ToolParam {
	var doc struct {
		Properties map[string]struct {
			Type        string `json:"type"`
			Description string `json:"description"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	req := map[string]struct{}{}
	for _, name := range doc.Required {
		req[name] = struct{}{}
	}
	out := make(map[string]domain.ToolParam, len(doc.Properties))
	for name, prop := range doc.Properties {
		_, required := req[name]
		out[name] = domain.ToolParam{Desc: prop.Description, Required: required, Type: prop.Type}
	}
	return out
}
