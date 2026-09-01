package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"agent-vivy/sdk/plugin"
)

func TestJSONRPCRoundTrip(t *testing.T) {
	var buf strings.Builder
	id := int64(7)
	if err := writeMessage(&buf, rpcMessage{ID: &id, Method: "initialize", Params: json.RawMessage(`{"a":1}`)}); err != nil {
		t.Fatal(err)
	}
	msg, err := readMessage(bufio.NewReader(strings.NewReader(buf.String())))
	if err != nil {
		t.Fatal(err)
	}
	if msg.ID == nil || *msg.ID != 7 || msg.Method != "initialize" || string(msg.Params) != `{"a":1}` {
		t.Fatalf("decoded = %+v", msg)
	}
}

func TestJSONRPCSkipsContentTypeHeader(t *testing.T) {
	wire := "Content-Length: 2\r\nContent-Type: application/vscode-jsonrpc; charset=utf-8\r\n\r\n{}"
	if _, err := readMessage(bufio.NewReader(strings.NewReader(wire))); err != nil {
		t.Fatalf("decode with extra header: %v", err)
	}
}

func TestLanguageFor(t *testing.T) {
	cases := map[string]string{
		"a.go": "gopls",
		"B.TS": "typescript-language-server",
		"c.py": "pyright-langserver",
		"d.rs": "rust-analyzer",
	}
	for path, want := range cases {
		lang, ok := languageFor(path)
		if !ok || lang.Command != want {
			t.Fatalf("languageFor(%q) = %+v, %v", path, lang, ok)
		}
	}
	if _, ok := languageFor("notes.txt"); ok {
		t.Fatal("notes.txt should have no server")
	}
}

func TestWorkspaceRel(t *testing.T) {
	accept := []string{"main.go", "src/x.ts", "./a.go", "a/b/c.py"}
	reject := []string{"", "../x.go", "/abs.go", `C:\x.go`, "C:/x.go", "a/../../b.go"}
	for _, p := range accept {
		if !workspaceRel(p) {
			t.Fatalf("workspaceRel(%q) = false", p)
		}
	}
	for _, p := range reject {
		if workspaceRel(p) {
			t.Fatalf("workspaceRel(%q) = true", p)
		}
	}
}

func TestPathToURIRel(t *testing.T) {
	root := t.TempDir()
	uri := pathToURI(root, "src/main.go")
	if !strings.HasPrefix(uri, "file:///") {
		t.Fatalf("uri = %q", uri)
	}
	rel, ok := uriToRel(root, uri)
	if !ok || rel != "src/main.go" {
		t.Fatalf("uriToRel = %q, %v", rel, ok)
	}
	// Drive-letter case differences on Windows still map to the same file.
	alt := strings.ToUpper(uri[:8]) + uri[8:]
	if rel2, ok := uriToRel(root, alt); !ok || rel2 != "src/main.go" {
		t.Fatalf("case-shifted uri mapped to %q, %v", rel2, ok)
	}
	// A URI outside the workspace is reported verbatim, not relativized.
	foreign, ok := uriToRel(root, "file:///C:/elsewhere/x.go")
	if ok || foreign != "file:///C:/elsewhere/x.go" {
		t.Fatalf("foreign uri = %q, %v", foreign, ok)
	}
}

func TestFormatDiagnostics(t *testing.T) {
	root := t.TempDir()
	uri := pathToURI(root, "main.go")
	got := formatDiagnostics(root, uri, []diagnostic{
		{Range: span{Start: position{Line: 0, Character: 0}}, Severity: 1, Message: "boom", Source: "compiler"},
	}, false)
	if got != "main.go:1:1: error: boom [compiler]" {
		t.Fatalf("got %q", got)
	}
	if got := formatDiagnostics(root, uri, nil, false); got != "no diagnostics" {
		t.Fatalf("empty = %q", got)
	}
	if got := formatDiagnostics(root, uri, nil, true); !strings.Contains(got, "wait_ms elapsed") {
		t.Fatalf("timeout = %q", got)
	}
	many := make([]diagnostic, maxReportedDiagnostics+5)
	out := formatDiagnostics(root, uri, many, false)
	if !strings.Contains(out, "... 5 more") {
		t.Fatalf("truncation missing: %q", out[len(out)-40:])
	}
}

// fakeEnv is a full plugin.Env whose Spawn runs an in-memory fake language
// server over io.Pipes, so the whole client path is exercised without a
// real server binary.
type fakeEnv struct {
	root       string
	files      map[string]string
	written    map[string]string
	spawnCount int
	commands   []string
	// caps overrides the fake server's initialize result (empty means the
	// default `{"capabilities":{}}`).
	caps string
}

func (f *fakeEnv) Workspace() string { return f.root }

func (f *fakeEnv) OpenRead(p string) (io.ReadCloser, error) {
	body, ok := f.files[p]
	if !ok {
		return nil, fmt.Errorf("no such file: %s", p)
	}
	return io.NopCloser(strings.NewReader(body)), nil
}

func (f *fakeEnv) OpenWrite(p string) (io.WriteCloser, error) {
	if f.written == nil {
		f.written = map[string]string{}
	}
	return &memWriter{env: f, key: p}, nil
}

type memWriter struct {
	env *fakeEnv
	key string
}

func (w *memWriter) Write(p []byte) (int, error) {
	w.env.written[w.key] += string(p)
	return len(p), nil
}

func (w *memWriter) Close() error { return nil }

func (f *fakeEnv) Spawn(_ context.Context, spec plugin.SpawnSpec) (plugin.Proc, error) {
	f.spawnCount++
	f.commands = append(f.commands, spec.Command)
	toolInR, toolInW := io.Pipe()
	toolOutR, toolOutW := io.Pipe()
	go serveFakeLSP(toolInR, toolOutW, f.root, f.caps)
	return fakeProc{stdin: toolInW, stdout: toolOutR}, nil
}

type fakeProc struct {
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func (p fakeProc) Stdin() io.WriteCloser { return p.stdin }
func (p fakeProc) Stdout() io.ReadCloser { return p.stdout }
func (p fakeProc) Stderr() io.ReadCloser { return io.NopCloser(strings.NewReader("")) }
func (p fakeProc) Wait() error           { return nil }
func (p fakeProc) Close() error          { return nil }

// serveFakeLSP answers initialize, publishes one diagnostic on didOpen,
// and a clean publish on didChange. root enables handlers that build
// workspace URIs (rename emits a second file's edit). caps overrides the
// initialize result (empty = `{"capabilities":{}}`).
func serveFakeLSP(r io.Reader, w io.Writer, root, caps string) {
	if caps == "" {
		caps = `{"capabilities":{}}`
	}
	br := bufio.NewReader(r)
	publish := func(uri string, diags []diagnostic) {
		raw, _ := json.Marshal(publishDiagnosticsParams{URI: uri, Diagnostics: diags})
		_ = writeMessage(w, rpcMessage{Method: "textDocument/publishDiagnostics", Params: raw})
	}
	for {
		msg, err := readMessage(br)
		if err != nil {
			return
		}
		switch msg.Method {
		case "initialize":
			_ = writeMessage(w, rpcMessage{ID: msg.ID, Result: json.RawMessage(caps)})
		case "initialized":
		case "textDocument/didOpen":
			var p didOpenParams
			if json.Unmarshal(msg.Params, &p) != nil {
				return
			}
			publish(p.TextDocument.URI, []diagnostic{{
				Range:    span{Start: position{Line: 0, Character: 0}},
				Severity: 1,
				Message:  "boom",
				Source:   "test",
			}})
		case "textDocument/didChange":
			var p didChangeParams
			if json.Unmarshal(msg.Params, &p) != nil {
				return
			}
			publish(p.TextDocument.URI, []diagnostic{})
		case "textDocument/definition":
			var p definitionParams
			if json.Unmarshal(msg.Params, &p) != nil {
				return
			}
			raw, _ := json.Marshal([]location{{
				URI:   p.TextDocument.URI,
				Range: span{Start: position{Line: 5, Character: 2}},
			}})
			_ = writeMessage(w, rpcMessage{ID: msg.ID, Result: raw})
		case "textDocument/references":
			var p referenceParams
			if json.Unmarshal(msg.Params, &p) != nil {
				return
			}
			raw, _ := json.Marshal([]location{
				{URI: p.TextDocument.URI, Range: span{Start: position{Line: 9, Character: 0}}},
				{URI: p.TextDocument.URI, Range: span{Start: position{Line: 14, Character: 4}}},
			})
			_ = writeMessage(w, rpcMessage{ID: msg.ID, Result: raw})
		case "textDocument/documentSymbol":
			tree := []documentSymbol{{
				Name: "main", Kind: 12,
				Range:    span{Start: position{Line: 0, Character: 0}},
				Children: []documentSymbol{{Name: "helper", Kind: 12, Range: span{Start: position{Line: 4, Character: 0}}}},
			}}
			raw, _ := json.Marshal(tree)
			_ = writeMessage(w, rpcMessage{ID: msg.ID, Result: raw})
		case "textDocument/rename":
			var p renameParams
			if json.Unmarshal(msg.Params, &p) != nil {
				return
			}
			var we workspaceEdit
			if p.NewName == "ESCAPE" {
				we.Changes = map[string][]textEdit{
					"file:///C:/outside/x.go": {{NewText: "x"}},
				}
			} else {
				we.Changes = map[string][]textEdit{
					p.TextDocument.URI: {{
						Range:   span{Start: position{Line: 0, Character: 0}, End: position{Line: 0, Character: 7}},
						NewText: p.NewName,
					}},
				}
				if root != "" {
					we.Changes[pathToURI(root, "util.go")] = []textEdit{{
						Range:   span{Start: position{Line: 2, Character: 0}, End: position{Line: 2, Character: 0}},
						NewText: "// renamed\n",
					}}
				}
			}
			raw, _ := json.Marshal(we)
			_ = writeMessage(w, rpcMessage{ID: msg.ID, Result: raw})
		}
	}
}

func TestDiagnosticsToolEndToEnd(t *testing.T) {
	env := &fakeEnv{root: t.TempDir(), files: map[string]string{"main.go": "package main\n"}}
	tool := diagnosticsTool{mgr: newManager()}

	first, err := tool.Run(context.Background(), env, json.RawMessage(`{"path":"main.go"}`))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(first, "main.go:1:1: error: boom [test]") {
		t.Fatalf("first = %q", first)
	}
	if env.spawnCount != 1 || env.commands[0] != "gopls" {
		t.Fatalf("spawn = %d %v", env.spawnCount, env.commands)
	}

	env.files["main.go"] = "package main // fixed\n"
	second, err := tool.Run(context.Background(), env, json.RawMessage(`{"path":"main.go"}`))
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	if second != "no diagnostics" {
		t.Fatalf("second = %q", second)
	}
	if env.spawnCount != 1 {
		t.Fatalf("server not reused: spawns = %d", env.spawnCount)
	}
}

func TestDiagnosticsToolArgValidation(t *testing.T) {
	env := &fakeEnv{root: t.TempDir()}
	tool := diagnosticsTool{mgr: newManager()}
	if _, err := tool.Run(context.Background(), env, json.RawMessage(`{"path":"../out.go"}`)); err != plugin.ErrInvalidArgs {
		t.Fatalf("escape err = %v", err)
	}
	if _, err := tool.Run(context.Background(), env, json.RawMessage(`{"path":"notes.txt"}`)); err == nil || !strings.Contains(err.Error(), "no language server") {
		t.Fatalf("unknown ext err = %v", err)
	}
	if env.spawnCount != 0 {
		t.Fatalf("validation failures must not spawn: %d", env.spawnCount)
	}
}

func TestDefinitionAndReferencesTools(t *testing.T) {
	env := &fakeEnv{root: t.TempDir(), files: map[string]string{"main.go": "package main\n"}}
	def := definitionTool{mgr: newManager()}
	got, err := def.Run(context.Background(), env, json.RawMessage(`{"path":"main.go","line":6,"column":3}`))
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	if got != "main.go:6:3" {
		t.Fatalf("definition = %q", got)
	}

	ref := referencesTool{mgr: newManager()}
	got, err = ref.Run(context.Background(), env, json.RawMessage(`{"path":"main.go","line":1,"column":1,"include_declaration":true}`))
	if err != nil {
		t.Fatalf("references: %v", err)
	}
	want := "main.go:10:1\nmain.go:15:5"
	if got != want {
		t.Fatalf("references = %q, want %q", got, want)
	}

	if _, err := ref.Run(context.Background(), env, json.RawMessage(`{"path":"main.go","line":0,"column":1}`)); err == nil || !strings.Contains(err.Error(), "1-based") {
		t.Fatalf("zero line err = %v", err)
	}
}

func TestSymbolsTool(t *testing.T) {
	env := &fakeEnv{root: t.TempDir(), files: map[string]string{"main.go": "package main\n"}}
	tool := symbolsTool{mgr: newManager()}
	got, err := tool.Run(context.Background(), env, json.RawMessage(`{"path":"main.go"}`))
	if err != nil {
		t.Fatalf("symbols: %v", err)
	}
	want := "function main :1:1\n  function helper :5:1"
	if got != want {
		t.Fatalf("symbols = %q, want %q", got, want)
	}
}

func TestParseSymbolsFlatShape(t *testing.T) {
	flatJSON := json.RawMessage(`[{"name":"main","kind":12,"location":{"uri":"file:///w/main.go","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":4}}}}]`)
	tree, flat, err := parseSymbols(flatJSON)
	if err != nil || len(tree) != 0 || len(flat) != 1 || flat[0].Name != "main" {
		t.Fatalf("parseSymbols flat = %v %v %v", tree, flat, err)
	}
}

func TestFormatLocationsNullAndSingle(t *testing.T) {
	root := t.TempDir()
	if got, _ := formatLocations(root, json.RawMessage("null")); got != "no matches" {
		t.Fatalf("null = %q", got)
	}
	single := json.RawMessage(`{"uri":"` + pathToURI(root, "a.go") + `","range":{"start":{"line":2,"character":0},"end":{"line":2,"character":3}}}`)
	if got, _ := formatLocations(root, single); got != "a.go:3:1" {
		t.Fatalf("single = %q", got)
	}
}

func TestRenameToolAppliesWorkspaceEdit(t *testing.T) {
	env := &fakeEnv{root: t.TempDir(), files: map[string]string{
		"main.go": "package main\n\nfunc main() {}\n",
		"util.go": "package main\n\n// helper\n",
	}}
	tool := renameTool{mgr: newManager()}
	got, err := tool.Run(context.Background(), env, json.RawMessage(`{"path":"main.go","line":3,"column":6,"new_name":"renamed"}`))
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if got != "main.go (1 edits)\nutil.go (1 edits)" {
		t.Fatalf("summary = %q", got)
	}
	if env.written["main.go"] != "renamed main\n\nfunc main() {}\n" {
		t.Fatalf("main.go = %q", env.written["main.go"])
	}
	if env.written["util.go"] != "package main\n\n// renamed\n// helper\n" {
		t.Fatalf("util.go = %q", env.written["util.go"])
	}
	if _, err := tool.Run(context.Background(), env, json.RawMessage(`{"path":"main.go","line":3,"column":6,"new_name":"ESCAPE"}`)); err == nil || !strings.Contains(err.Error(), "outside the workspace") {
		t.Fatalf("escape err = %v", err)
	}
	if _, err := tool.Run(context.Background(), env, json.RawMessage(`{"path":"main.go","line":3,"column":6,"new_name":"  "}`)); err == nil || !strings.Contains(err.Error(), "new_name") {
		t.Fatalf("blank name err = %v", err)
	}
}

func TestApplyEditsEncodings(t *testing.T) {
	content := "a := \"👍b\"\nnext\n"
	want := "a := \"👍B\"\nnext\n"
	// utf-16: the emoji is 2 units, so b sits at character 8.
	got, err := applyEdits(content, []textEdit{{
		Range:   span{Start: position{Line: 0, Character: 8}, End: position{Line: 0, Character: 9}},
		NewText: "B",
	}}, posEncUTF16)
	if err != nil || got != want {
		t.Fatalf("utf-16 got %q, %v", got, err)
	}
	// utf-8: the emoji is 4 bytes, so b sits at character 10.
	got, err = applyEdits(content, []textEdit{{
		Range:   span{Start: position{Line: 0, Character: 10}, End: position{Line: 0, Character: 11}},
		NewText: "B",
	}}, posEncUTF8)
	if err != nil || got != want {
		t.Fatalf("utf-8 got %q, %v", got, err)
	}
	// utf-32: the emoji is 1 unit, so b sits at character 7.
	got, err = applyEdits(content, []textEdit{{
		Range:   span{Start: position{Line: 0, Character: 7}, End: position{Line: 0, Character: 8}},
		NewText: "B",
	}}, posEncUTF32)
	if err != nil || got != want {
		t.Fatalf("utf-32 got %q, %v", got, err)
	}
	// A naive utf-16 count on a utf-8 position corrupts the emoji — the
	// encoding must actually flow through.
	if got, _ := applyEdits(content, []textEdit{{
		Range:   span{Start: position{Line: 0, Character: 10}, End: position{Line: 0, Character: 11}},
		NewText: "B",
	}}, posEncUTF16); got == want {
		t.Fatal("utf-16 must not treat byte offset 10 as inside the line")
	}
	if strings.Count(got, "👍") != 1 {
		t.Fatalf("emoji corrupted: %q", got)
	}
	// Insertion at end-of-line beyond the last character clamps.
	got, err = applyEdits("ab\n", []textEdit{{
		Range:   span{Start: position{Line: 0, Character: 9}, End: position{Line: 0, Character: 9}},
		NewText: "c",
	}}, posEncUTF16)
	if err != nil || got != "abc\n" {
		t.Fatalf("clamp got %q, %v", got, err)
	}
	// A later edit must not shift an earlier one.
	got, err = applyEdits("abcdef\n", []textEdit{
		{Range: span{Start: position{Line: 0, Character: 0}, End: position{Line: 0, Character: 1}}, NewText: "X"},
		{Range: span{Start: position{Line: 0, Character: 5}, End: position{Line: 0, Character: 6}}, NewText: "Y"},
	}, posEncUTF16)
	if err != nil || got != "XbcdeY\n" {
		t.Fatalf("multi got %q, %v", got, err)
	}
}

func TestNegotiatedEncoding(t *testing.T) {
	cases := map[string]positionEncoding{
		`{"capabilities":{"positionEncoding":"utf-8"}}`:  posEncUTF8,
		`{"capabilities":{"positionEncoding":"utf-32"}}`: posEncUTF32,
		`{"capabilities":{}}`:                            posEncUTF16,
		`{}`:                                             posEncUTF16,
		`{"capabilities":{"positionEncoding":"utf-7"}}`:  posEncUTF16,
		`not json`:                                       posEncUTF16,
	}
	for raw, want := range cases {
		if got := negotiatedEncoding(json.RawMessage(raw)); got != want {
			t.Fatalf("negotiatedEncoding(%s) = %q, want %q", raw, got, want)
		}
	}
}

func TestInitializeWireOffersEncodings(t *testing.T) {
	var params initializeParams
	params.Capabilities.General.PositionEncodings = offeredPositionEncodings
	wire, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wire), `"positionEncodings":["utf-16","utf-8","utf-32"]`) {
		t.Fatalf("initialize wire = %s", wire)
	}
}

func TestInitializeNegotiatesEncoding(t *testing.T) {
	// The default fake server answers an empty capabilities object, which
	// means the LSP default utf-16.
	env16 := &fakeEnv{root: t.TempDir(), files: map[string]string{"main.go": "package main\n"}}
	srv, err := negotiatedServer(t, env16)
	if err != nil {
		t.Fatal(err)
	}
	if srv.enc != posEncUTF16 {
		t.Fatalf("default enc = %q", srv.enc)
	}

	// A server that declares utf-8 gets honored.
	env8 := &fakeEnv{root: t.TempDir(), files: map[string]string{"main.go": "package main\n"},
		caps: `{"capabilities":{"positionEncoding":"utf-8"}}`}
	srv8, err := negotiatedServer(t, env8)
	if err != nil {
		t.Fatal(err)
	}
	if srv8.enc != posEncUTF8 {
		t.Fatalf("negotiated enc = %q", srv8.enc)
	}
}

func negotiatedServer(t *testing.T, env *fakeEnv) (*server, error) {
	t.Helper()
	mgr := newManager()
	lang, _ := languageFor("main.go")
	return mgr.get(context.Background(), env, lang, env.root)
}

func TestObserveWriteBackfillsDiagnostics(t *testing.T) {
	env := &fakeEnv{root: t.TempDir(), files: map[string]string{
		"main.go":   "package main\n",
		"notes.txt": "hello\n",
	}}
	p := &Plugin{mgr: newManager()}

	got := p.ObserveWrite(context.Background(), env, []string{"main.go", "notes.txt"})
	want := []string{"main.go:1:1: error: boom [test]"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("lines = %v, want %v", got, want)
	}
	if env.spawnCount != 1 {
		t.Fatalf("only the go file must spawn a server: %d %v", env.spawnCount, env.commands)
	}

	// Server reuse on the next mutation; the fake reports a clean publish
	// on didChange, so silence means nothing to report.
	got = p.ObserveWrite(context.Background(), env, []string{"main.go"})
	if len(got) != 0 {
		t.Fatalf("second pass = %v, want none", got)
	}
	if env.spawnCount != 1 {
		t.Fatalf("server not reused: %d", env.spawnCount)
	}
}

func TestFormatDiagnosticLinesCapsAndCounts(t *testing.T) {
	many := make([]diagnostic, maxBackfillLines+3)
	lines := formatDiagnosticLines("a.go", many)
	if len(lines) != maxBackfillLines+1 {
		t.Fatalf("lines = %d", len(lines))
	}
	if lines[maxBackfillLines] != "... 3 more" {
		t.Fatalf("cap note = %q", lines[maxBackfillLines])
	}
	if formatDiagnosticLines("a.go", nil) != nil {
		t.Fatal("empty diagnostics must contribute nothing")
	}
}
