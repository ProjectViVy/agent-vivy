package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"agent-vivy/sdk/plugin"
)

// filePos is the shared argument shape of the position-based tools. Line
// and column are 1-based (model-facing); LSP wants 0-based.
type filePos struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

// syncOpen validates position args, syncs the file from disk into the
// server, and returns the connection plus the LSP position.
func syncOpen(ctx context.Context, env plugin.Env, t *manager, raw json.RawMessage) (*server, string, filePos, error) {
	var in filePos
	if err := json.Unmarshal(raw, &in); err != nil || !workspaceRel(in.Path) {
		return nil, "", in, plugin.ErrInvalidArgs
	}
	if in.Line < 1 || in.Column < 1 {
		return nil, "", in, fmt.Errorf("lsp: line and column are 1-based")
	}
	lang, ok := languageFor(in.Path)
	if !ok {
		return nil, "", in, fmt.Errorf("lsp: no language server configured for %s", in.Path)
	}
	root := env.Workspace()
	if root == "" {
		return nil, "", in, plugin.ErrDenied
	}
	srv, err := t.get(ctx, env, lang, root)
	if err != nil {
		return nil, "", in, err
	}
	if err := syncFile(ctx, srv, env, lang, root, in.Path); err != nil {
		return nil, "", in, err
	}
	return srv, pathToURI(root, in.Path), in, nil
}

// syncFile reads the saved file and pushes it into the server buffer.
func syncFile(ctx context.Context, srv *server, env plugin.Env, lang language, root, rel string) error {
	rc, err := env.OpenRead(rel)
	if err != nil {
		return fmt.Errorf("lsp: read %s: %w", rel, err)
	}
	defer func() { _ = rc.Close() }()
	body, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("lsp: read %s: %w", rel, err)
	}
	return srv.openText(ctx, lang.Name, pathToURI(root, rel), string(body))
}

type definitionTool struct {
	mgr *manager
}

func (definitionTool) Name() string { return "lsp_definition" }

func (definitionTool) Effect() plugin.Effect { return plugin.EffectRead }

func (definitionTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Workspace-relative path of the file"},"line":{"type":"integer","description":"1-based line of the symbol"},"column":{"type":"integer","description":"1-based column of the symbol"}},"required":["path","line","column"]}`)
}

func (t definitionTool) Run(ctx context.Context, env plugin.Env, args json.RawMessage) (string, error) {
	srv, uri, pos, err := syncOpen(ctx, env, t.mgr, args)
	if err != nil {
		return "", err
	}
	result, err := srv.call(ctx, "textDocument/definition", definitionParams{
		TextDocument: textDocumentIdentifier{URI: uri},
		Position:     position{Line: pos.Line - 1, Character: pos.Column - 1},
	})
	if err != nil {
		return "", err
	}
	return formatLocations(env.Workspace(), result)
}

type referencesTool struct {
	mgr *manager
}

func (referencesTool) Name() string { return "lsp_references" }

func (referencesTool) Effect() plugin.Effect { return plugin.EffectRead }

func (referencesTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Workspace-relative path of the file"},"line":{"type":"integer","description":"1-based line of the symbol"},"column":{"type":"integer","description":"1-based column of the symbol"},"include_declaration":{"type":"boolean","description":"Also report the declaration itself (default false)"}},"required":["path","line","column"]}`)
}

func (t referencesTool) Run(ctx context.Context, env plugin.Env, args json.RawMessage) (string, error) {
	// syncOpen ignores the extra include_declaration key; decode it
	// separately.
	var extra struct {
		IncludeDeclaration bool `json:"include_declaration"`
	}
	_ = json.Unmarshal(args, &extra)
	srv, uri, pos, err := syncOpen(ctx, env, t.mgr, args)
	if err != nil {
		return "", err
	}
	result, err := srv.call(ctx, "textDocument/references", referenceParams{
		TextDocument: textDocumentIdentifier{URI: uri},
		Position:     position{Line: pos.Line - 1, Character: pos.Column - 1},
		Context:      referenceContext{IncludeDeclaration: extra.IncludeDeclaration},
	})
	if err != nil {
		return "", err
	}
	return formatLocations(env.Workspace(), result)
}

type symbolsTool struct {
	mgr *manager
}

func (symbolsTool) Name() string { return "lsp_symbols" }

func (symbolsTool) Effect() plugin.Effect { return plugin.EffectRead }

func (symbolsTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Workspace-relative path of the file"}},"required":["path"]}`)
}

func (t symbolsTool) Run(ctx context.Context, env plugin.Env, args json.RawMessage) (string, error) {
	var in struct {
		Path string `json:"path"`
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
	srv, err := t.mgr.get(ctx, env, lang, root)
	if err != nil {
		return "", err
	}
	if err := syncFile(ctx, srv, env, lang, root, in.Path); err != nil {
		return "", err
	}
	result, err := srv.call(ctx, "textDocument/documentSymbol", definitionParams{
		TextDocument: textDocumentIdentifier{URI: pathToURI(root, in.Path)},
	})
	if err != nil {
		return "", err
	}
	tree, flat, err := parseSymbols(result)
	if err != nil {
		return "", fmt.Errorf("lsp: decode symbols: %w", err)
	}
	return formatSymbols(root, tree, flat), nil
}

// formatLocations renders a raw definition/references result: Location,
// Location[] or null.
func formatLocations(root string, raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "no matches", nil
	}
	var single location
	if err := json.Unmarshal(raw, &single); err == nil && single.URI != "" {
		return renderLocation(root, single), nil
	}
	var many []location
	if err := json.Unmarshal(raw, &many); err != nil || len(many) == 0 {
		return "no matches", nil
	}
	var b strings.Builder
	for _, loc := range many {
		b.WriteString(renderLocation(root, loc))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func renderLocation(root string, loc location) string {
	rel, _ := uriToRel(root, loc.URI)
	return fmt.Sprintf("%s:%d:%d", rel, loc.Range.Start.Line+1, loc.Range.Start.Character+1)
}

var symbolKinds = map[int]string{
	1: "file", 2: "module", 3: "namespace", 4: "package", 5: "class",
	6: "method", 7: "property", 8: "field", 9: "constructor", 10: "enum",
	11: "interface", 12: "function", 13: "variable", 14: "constant",
	15: "string", 16: "number", 17: "boolean", 18: "array", 19: "object",
	20: "key", 21: "null", 22: "enum-member", 23: "struct", 24: "event",
	25: "operator", 26: "type-parameter",
}

func kindName(kind int) string {
	if name, ok := symbolKinds[kind]; ok {
		return name
	}
	return fmt.Sprintf("kind(%d)", kind)
}

func formatSymbols(root string, tree []documentSymbol, flat []symbolInformation) string {
	if len(flat) > 0 {
		var b strings.Builder
		for _, s := range flat {
			rel, _ := uriToRel(root, s.Location.URI)
			fmt.Fprintf(&b, "%s %s %s:%d:%d\n", kindName(s.Kind), s.Name,
				rel, s.Location.Range.Start.Line+1, s.Location.Range.Start.Character+1)
		}
		return strings.TrimRight(b.String(), "\n")
	}
	if len(tree) == 0 {
		return "no symbols"
	}
	var b strings.Builder
	walkSymbols(&b, tree, 0)
	return strings.TrimRight(b.String(), "\n")
}

func walkSymbols(b *strings.Builder, nodes []documentSymbol, depth int) {
	for _, s := range nodes {
		b.WriteString(strings.Repeat("  ", depth))
		fmt.Fprintf(b, "%s %s :%d:%d\n", kindName(s.Kind), s.Name,
			s.Range.Start.Line+1, s.Range.Start.Character+1)
		walkSymbols(b, s.Children, depth+1)
	}
}
