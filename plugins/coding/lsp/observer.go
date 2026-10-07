package lsp

import (
	"context"
	"fmt"
	"time"

	plugin "agent-vivy/sdk/port/toolworld"
)

// backfillWait bounds the per-file diagnostics wait during backfill; it is
// deliberately shorter than the interactive lsp_diagnostics default.
const backfillWait = 2 * time.Second

// maxBackfillLines caps what one file contributes to a mutation result.
const maxBackfillLines = 30

// ObserveWrite implements plugin.DiagnosticObserver (VC-3 backfill): the
// kernel calls it after write/patch/multiedit changed files, so lint/type
// errors ride along in the mutation result instead of costing a separate
// lsp_diagnostics call. Files without a configured language server and
// servers that stay quiet are skipped — silence means nothing to report,
// never success.
func (p *Plugin) ObserveWrite(ctx context.Context, env plugin.Host, paths []string) []string {
	root := env.Workspace()
	if root == "" {
		return nil
	}
	var out []string
	for _, rel := range paths {
		lang, ok := languageFor(rel)
		if !ok {
			continue
		}
		srv, err := p.mgr.get(ctx, env, lang, root)
		if err != nil {
			continue
		}
		body, err := readFile(env, rel)
		if err != nil {
			continue
		}
		uri := pathToURI(root, rel)
		base := srv.diagGeneration(uri)
		if err := srv.openText(ctx, lang.Name, uri, string(body)); err != nil {
			continue
		}
		_, _ = srv.waitForDiagnostics(ctx, uri, base, backfillWait)
		out = append(out, formatDiagnosticLines(rel, srv.diagnosticsFor(uri))...)
	}
	return out
}

// formatDiagnosticLines renders one file's diagnostics as backfill lines;
// a quiet file contributes nothing.
func formatDiagnosticLines(rel string, diags []diagnostic) []string {
	if len(diags) == 0 {
		return nil
	}
	shown := diags
	if len(shown) > maxBackfillLines {
		shown = shown[:maxBackfillLines]
	}
	out := make([]string, 0, len(shown)+1)
	for _, d := range shown {
		name := severityNames[d.Severity]
		if name == "" {
			name = fmt.Sprintf("severity(%d)", d.Severity)
		}
		line := fmt.Sprintf("%s:%d:%d: %s: %s", rel, d.Range.Start.Line+1, d.Range.Start.Character+1, name, d.Message)
		if d.Source != "" {
			line += " [" + d.Source + "]"
		}
		out = append(out, line)
	}
	if extra := len(diags) - len(shown); extra > 0 {
		out = append(out, fmt.Sprintf("... %d more", extra))
	}
	return out
}
