package mcphost

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"agent-vivy/internal/contexthost"
	"agent-vivy/internal/toolhost"
	"agent-vivy/sdk/port/contextsource"
	"agent-vivy/sdk/port/pretool"
	"agent-vivy/sdk/port/toolworld"
)

type bridgeWorldHost struct{}

func (bridgeWorldHost) ModuleID() string                       { return "vivy/mcp-host" }
func (bridgeWorldHost) Workspace() string                      { return "" }
func (bridgeWorldHost) OpenRead(string) (io.ReadCloser, error) { return nil, toolworld.ErrDenied }
func (bridgeWorldHost) OpenWrite(string) (io.WriteCloser, error) {
	return nil, toolworld.ErrDenied
}
func (bridgeWorldHost) Spawn(context.Context, toolworld.SpawnSpec) (toolworld.Proc, error) {
	return nil, toolworld.ErrDenied
}

type bridgeMiddleware struct{ calls int }

func (*bridgeMiddleware) ID() string { return "fixture/middleware" }
func (middleware *bridgeMiddleware) Evaluate(_ context.Context, request pretool.Request) (pretool.Decision, error) {
	middleware.calls++
	if !strings.HasPrefix(request.ToolID, "mcp.") {
		return pretool.Decision{Kind: pretool.Deny, ReasonCode: "namespace", SafeMessage: "not mcp"}, nil
	}
	return pretool.Decision{Kind: pretool.Pass}, nil
}

func TestMCPToolBridgeEntersSoleToolHost(t *testing.T) {
	session := &fakeSession{tools: []RemoteTool{{Name: "bash", Description: "remote bash name", Schema: json.RawMessage(`{"type":"object"}`)}}}
	factory := &fakeFactory{sessions: []*fakeSession{session}}
	mcpHost, err := New(Config{Factory: factory, Instances: []InstanceConfig{{ID: "docs", Command: "fixture"}}})
	if err != nil {
		t.Fatal(err)
	}
	middleware := &bridgeMiddleware{}
	governed, err := toolhost.New(toolhost.Config{
		Worlds:       []toolhost.WorldBinding{{OwnerID: "vivy/mcp-host", Provider: NewToolWorld(mcpHost), Host: bridgeWorldHost{}}},
		ProtectedIDs: []string{"bash"},
		Middleware:   []pretool.Provider{middleware},
	})
	if err != nil {
		t.Fatal(err)
	}
	definitions, err := governed.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(definitions) != 1 || definitions[0].ID == "bash" || !strings.HasPrefix(definitions[0].ID, "mcp.docs.") {
		t.Fatalf("MCP definition escaped namespace/protection: %#v", definitions)
	}
	entry, ok := governed.Lookup(definitions[0].ID)
	if !ok || !entry.Dynamic || entry.WorldID != "mcp" {
		t.Fatalf("MCP Tool is not a dynamic ToolHost entry: %#v, ok=%v", entry, ok)
	}
	if _, err := governed.ApplyMiddleware(context.Background(), toolhost.Request{ID: definitions[0].ID, Args: json.RawMessage(`{}`)}, nil); err != nil {
		t.Fatal(err)
	}
	if middleware.calls != 1 {
		t.Fatalf("middleware calls = %d, want 1", middleware.calls)
	}
	result, err := governed.Invoke(context.Background(), toolhost.Request{ID: definitions[0].ID, Args: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "ok" || session.calls != 1 {
		t.Fatalf("ToolHost invoke result=%q calls=%d", result.Text, session.calls)
	}
}

func TestMCPResourceRequiresExplicitContextBridge(t *testing.T) {
	withoutSession := &fakeSession{
		resources: []RemoteResource{{URI: "docs://guide", Name: "guide", MediaType: "text/plain"}},
		read:      ResourceContent{URI: "docs://guide", MediaType: "text/plain", Text: "guide body"},
	}
	withoutFactory := &fakeFactory{sessions: []*fakeSession{withoutSession}}
	without, err := New(Config{Factory: withoutFactory, Instances: []InstanceConfig{{ID: "docs", Command: "fixture", ResourceBridge: false}}})
	if err != nil {
		t.Fatal(err)
	}
	withoutContext, err := contexthost.New(contexthost.Config{Sources: []contextsource.Provider{NewResourceSource(without)}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := withoutContext.Query(context.Background(), contexthost.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 0 || withoutFactory.opens != 0 {
		t.Fatalf("disabled resource bridge candidates=%#v opens=%d", result.Candidates, withoutFactory.opens)
	}

	withSession := &fakeSession{
		resources: []RemoteResource{{URI: "docs://guide", Name: "guide", MediaType: "text/plain"}},
		read:      ResourceContent{URI: "docs://guide", MediaType: "text/plain", Text: "guide body"},
	}
	withFactory := &fakeFactory{sessions: []*fakeSession{withSession}}
	with, err := New(Config{Factory: withFactory, Instances: []InstanceConfig{{ID: "docs", Command: "fixture", ResourceBridge: true}}})
	if err != nil {
		t.Fatal(err)
	}
	withContext, err := contexthost.New(contexthost.Config{Sources: []contextsource.Provider{NewResourceSource(with)}})
	if err != nil {
		t.Fatal(err)
	}
	result, err = withContext.Query(context.Background(), contexthost.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Content != "guide body" || withFactory.opens != 1 {
		t.Fatalf("enabled resource bridge result=%#v opens=%d", result, withFactory.opens)
	}
}

func TestMCPBinaryResourceDoesNotBecomeContext(t *testing.T) {
	session := &fakeSession{
		resources: []RemoteResource{{URI: "blob://image", Name: "image", MediaType: "image/png"}},
		read:      ResourceContent{URI: "blob://image", MediaType: "image/png", Blob: []byte("base64-data")},
	}
	factory := &fakeFactory{sessions: []*fakeSession{session}}
	host, err := New(Config{Factory: factory, Instances: []InstanceConfig{{ID: "media", Command: "fixture", ResourceBridge: true}}})
	if err != nil {
		t.Fatal(err)
	}
	source := NewResourceSource(host)
	page, err := source.Query(context.Background(), contextsource.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Candidates) != 0 {
		t.Fatalf("binary MCP resource leaked into text ContextSource: %#v", page.Candidates)
	}
}

func TestMCPPromptAutomaticSkillConversionHasNoBridge(t *testing.T) {
	source := NewResourceSource(&Host{})
	if _, ok := any(source).(interface{ ListPrompts(context.Context) error }); ok {
		t.Fatal("MCP resource bridge unexpectedly exposes prompts")
	}
}
