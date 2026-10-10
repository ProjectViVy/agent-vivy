package view

import (
	"strings"

	"agent-vivy/sdk/tui/surface"
)

// Tool renderer registry (VCP-G3): a per-tool-name hook that replaces the
// default tool-card body. It is a view-layer seam — deliberately NOT a
// plugin Port (VCP decision: keeps the 14-Port catalog closed). A renderer
// receives the raw ToolCard (Preview for pending, Result for terminal) and
// returns the fully rendered lines, or nil to fall back to the default card.
//
// Register a renderer for structured output (parsed args/diffs); leave the
// tool unregistered or return nil for the raw default rendering. The
// debugToolOutput toggle bypasses nothing here — renderers see the same
// card either way and decide what "expanded" means for their tool.
type ToolRenderer func(tool *surface.ToolCard, width int, p Palette) []string

var toolRenderers = map[string]ToolRenderer{}

// RegisterToolRenderer installs (or, with nil, removes) a renderer for a tool
// name. Names are case-insensitive and trimmed; registering for "" is a no-op.
func RegisterToolRenderer(name string, fn ToolRenderer) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return
	}
	if fn == nil {
		delete(toolRenderers, key)
		return
	}
	toolRenderers[key] = fn
}

func lookupToolRenderer(name string) ToolRenderer {
	return toolRenderers[strings.ToLower(strings.TrimSpace(name))]
}
