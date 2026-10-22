package view

import (
	encodinghex "encoding/hex"
	"strings"
	"testing"

	"agent-vivy/sdk/tui/surface"
)

func TestD14ChatAssemblyObservesSameLengthNestedEdits(t *testing.T) {
	message := surface.Message{ID: "tool", Role: surface.RoleTool, Tool: &surface.ToolCard{ToolName: "bash", ToolCallID: "call", Status: "done", Result: "before"}}
	driver := &testDriver{sessions: []surface.Session{{ID: "session"}}, active: "session", messages: map[string][]surface.Message{"session": {message}}}
	model := New(driver)
	before := strings.Join(model.chatLines(100, model.palette), "\n")
	if !strings.Contains(before, "before") {
		t.Fatalf("missing initial tool output: %q", before)
	}
	// ActiveMessages returns value copies but the nested card is shared.
	// A cached snapshot must own the prior card, including same-size edits.
	driver.messages["session"][0].Tool.Result = "after!"
	after := strings.Join(model.chatLines(100, model.palette), "\n")
	if !strings.Contains(after, "after!") || strings.Contains(after, "before") {
		t.Fatalf("same-length tool edit stayed stale: %q", after)
	}
	driver.messages["session"][0] = surface.Message{ID: "user", Role: surface.RoleUser, Attachments: []surface.Attachment{{Name: "a.png", MimeType: "image/png"}}, FileContexts: []surface.FileContext{{Path: "a.go", Name: "a.go"}}}
	_ = model.chatLines(100, model.palette)
	driver.messages["session"][0].Attachments[0].Name = "b.png"
	driver.messages["session"][0].FileContexts[0].Name = "b.go"
	driver.messages["session"][0].FileContexts[0].Path = "b.go"
	changed := strings.Join(model.chatLines(100, model.palette), "\n")
	if !strings.Contains(changed, "b.png") || !strings.Contains(changed, "b.go") || strings.Contains(changed, "a.png") || strings.Contains(changed, "a.go") {
		t.Fatalf("nested slice edits stayed stale: %q", changed)
	}
}

func TestD14TUIAcceptsBoundedReversibleMCPCommandName(t *testing.T) {
	name := "mcp-" + encodinghex.EncodeToString([]byte(strings.Repeat("S", 256))) + "-" + encodinghex.EncodeToString([]byte(strings.Repeat("P", 256)))
	driver := &dynamicCommandDriver{testDriver: &testDriver{}, commands: []surface.DynamicCommand{{ID: "mcp:existing-owner-id", Kind: "mcp_prompt", Name: name, Usage: "/" + name}}}
	model := New(driver)
	registry, dynamic := model.effectiveCommandRegistry()
	if _, ok := registry.Lookup(name); !ok {
		t.Fatal("maximal reversible MCP command was hidden by the TUI registry")
	}
	if dynamic[name].ID != "mcp:existing-owner-id" {
		t.Fatal("TUI lost the authoritative command ID")
	}
}
