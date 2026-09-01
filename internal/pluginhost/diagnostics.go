package pluginhost

import (
	"context"

	"agent-vivy/internal/tools"
	"agent-vivy/sdk/plugin"
)

var _ tools.WriteDiagnosticsSource = (*DiagnosticBridge)(nil)

// DiagnosticBridge fans post-write diagnostics out to registered
// tool-world plugins that implement plugin.DiagnosticObserver (VC-3
// backfill). It implements tools.WriteDiagnosticsSource and is wired to
// the filesystem backend by the composition root; with no observers it
// reports nothing.
type DiagnosticBridge struct {
	plugins []plugin.Plugin
	lookup  WorkspaceLookup
}

func NewDiagnosticBridge(plugins []plugin.Plugin, lookup WorkspaceLookup) *DiagnosticBridge {
	return &DiagnosticBridge{plugins: plugins, lookup: lookup}
}

// WriteDiagnostics collects lines from every tool-world observer, in
// registration order. Channel-seam plugins and non-observers are skipped;
// each observer receives its own granted Env.
func (b *DiagnosticBridge) WriteDiagnostics(ctx context.Context, paths []string) []string {
	if b == nil || len(paths) == 0 {
		return nil
	}
	var out []string
	for _, p := range b.plugins {
		if p == nil || p.Seam() != plugin.SeamToolWorld {
			continue
		}
		observer, ok := p.(plugin.DiagnosticObserver)
		if !ok {
			continue
		}
		env := hostedEnv{plugin: p, lookup: b.lookup, ctx: ctx}
		for _, line := range observer.ObserveWrite(ctx, env, paths) {
			if line != "" {
				out = append(out, line)
			}
		}
	}
	return out
}
