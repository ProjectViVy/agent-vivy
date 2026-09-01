// Package lsp is the VC-3 language-server plugin (D4) and the first
// standalone Vivy plugin module. It spawns language servers through
// Env.Spawn (granted proc.spawn) and exposes their intelligence as
// model tools. The only import window is agent-vivy/sdk/plugin.
package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"time"

	"agent-vivy/sdk/plugin"
)

// Plugin owns the manager for the process lifetime; connections persist
// across tool calls and are reaped by idle timeout.
type Plugin struct {
	mgr *manager
}

func New() plugin.Plugin {
	p := &Plugin{mgr: newManager()}
	p.mgr.startReaper()
	return p
}

func (p *Plugin) Name() string      { return "lsp" }
func (p *Plugin) Seam() plugin.Seam { return plugin.SeamToolWorld }

func (p *Plugin) Grants() []plugin.Grant {
	return []plugin.Grant{plugin.GrantFSRead, plugin.GrantFSWrite, plugin.GrantProcSpawn}
}

func (p *Plugin) Tools() []plugin.Tool {
	return []plugin.Tool{
		diagnosticsTool{mgr: p.mgr},
		definitionTool{mgr: p.mgr},
		referencesTool{mgr: p.mgr},
		symbolsTool{mgr: p.mgr},
		renameTool{mgr: p.mgr},
	}
}

type diagnosticsTool struct {
	mgr *manager
}

func (diagnosticsTool) Name() string { return "lsp_diagnostics" }

func (diagnosticsTool) Effect() plugin.Effect { return plugin.EffectRead }

func (diagnosticsTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Workspace-relative path of the file to check"},"wait_ms":{"type":"integer","description":"Milliseconds to wait for the language server to publish diagnostics (default 3000, max 15000)"}},"required":["path"]}`)
}

// Run opens the file from disk (saved content only), syncs it into the
// language server for its language, and waits for a fresh
// publishDiagnostics round.
func (t diagnosticsTool) Run(ctx context.Context, env plugin.Env, args json.RawMessage) (string, error) {
	var in struct {
		Path   string `json:"path"`
		WaitMS int    `json:"wait_ms"`
	}
	if err := json.Unmarshal(args, &in); err != nil || !workspaceRel(in.Path) {
		return "", plugin.ErrInvalidArgs
	}
	lang, ok := languageFor(in.Path)
	if !ok {
		return "", fmt.Errorf("lsp: no language server configured for %s", in.Path)
	}
	root := env.Workspace()
	if root == "" {
		return "", plugin.ErrDenied
	}
	wait := time.Duration(in.WaitMS) * time.Millisecond
	if wait <= 0 {
		wait = 3 * time.Second
	}
	if wait > 15*time.Second {
		wait = 15 * time.Second
	}
	srv, err := t.mgr.get(ctx, env, lang, root)
	if err != nil {
		return "", err
	}
	body, err := readFile(env, in.Path)
	if err != nil {
		return "", fmt.Errorf("lsp: read %s: %w", in.Path, err)
	}
	uri := pathToURI(root, in.Path)
	if err := srv.openText(ctx, lang.Name, uri, string(body)); err != nil {
		return "", err
	}
	timedOut, err := srv.waitForDiagnostics(ctx, uri, wait)
	if err != nil {
		return "", err
	}
	return formatDiagnostics(root, uri, srv.diagnosticsFor(uri), timedOut), nil
}

func readFile(env plugin.Env, rel string) ([]byte, error) {
	rc, err := env.OpenRead(rel)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(rc)
}

// workspaceRel accepts workspace-relative paths only: no absolute forms,
// no drive letters, no escapes.
func workspaceRel(p string) bool {
	if p == "" || path.IsAbs(p) || filepath.IsAbs(p) || strings.ContainsAny(p, `:\`) || strings.HasPrefix(p, "/") {
		return false
	}
	cleaned := path.Clean(p)
	return cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}

var severityNames = map[int]string{
	1: "error",
	2: "warning",
	3: "info",
	4: "hint",
}

const maxReportedDiagnostics = 200

func formatDiagnostics(root, uri string, diags []diagnostic, timedOut bool) string {
	var b strings.Builder
	if len(diags) == 0 {
		b.WriteString("no diagnostics")
	} else {
		shown := diags
		if len(shown) > maxReportedDiagnostics {
			shown = shown[:maxReportedDiagnostics]
		}
		for _, d := range shown {
			where, _ := uriToRel(root, uri)
			name := severityNames[d.Severity]
			if name == "" {
				name = fmt.Sprintf("severity(%d)", d.Severity)
			}
			b.WriteString(fmt.Sprintf("%s:%d:%d: %s: %s", where, d.Range.Start.Line+1, d.Range.Start.Character+1, name, d.Message))
			if d.Source != "" {
				b.WriteString(" [" + d.Source + "]")
			}
			b.WriteString("\n")
		}
		if extra := len(diags) - len(shown); extra > 0 {
			b.WriteString(fmt.Sprintf("... %d more\n", extra))
		}
	}
	if timedOut {
		b.WriteString("(wait_ms elapsed without a diagnostics publish; results may be stale or pending)\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
