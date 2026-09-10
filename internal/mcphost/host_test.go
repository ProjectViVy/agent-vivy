package mcphost

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"agent-vivy/sdk/module"
)

type fakeFactory struct {
	opens    int
	sessions []*fakeSession
	err      error
}

func (factory *fakeFactory) Open(context.Context, InstanceConfig) (Session, error) {
	factory.opens++
	if factory.err != nil {
		return nil, factory.err
	}
	if len(factory.sessions) == 0 {
		return nil, errors.New("no fake session")
	}
	session := factory.sessions[0]
	factory.sessions = factory.sessions[1:]
	return session, nil
}

type fakeSession struct {
	discoveries int
	calls       int
	closes      int
	tools       []RemoteTool
	resources   []RemoteResource
	read        ResourceContent
	discoverErr error
	callErr     error
	block       bool
}

func (session *fakeSession) DiscoverTools(ctx context.Context) ([]RemoteTool, error) {
	session.discoveries++
	if session.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return append([]RemoteTool(nil), session.tools...), session.discoverErr
}
func (session *fakeSession) CallTool(context.Context, string, json.RawMessage) (ToolResult, error) {
	session.calls++
	return ToolResult{Text: "ok"}, session.callErr
}
func (session *fakeSession) ListResources(context.Context) ([]RemoteResource, error) {
	return append([]RemoteResource(nil), session.resources...), nil
}
func (session *fakeSession) ReadResource(context.Context, string) (ResourceContent, error) {
	return session.read, nil
}
func (session *fakeSession) Close() error {
	session.closes++
	return nil
}

func TestMCPServerNeverBecomesNativeModule(t *testing.T) {
	host, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := any(host).(module.Module); ok {
		t.Fatal("T3 MCP server host must never satisfy native module.Module")
	}
	typeOfConfig := reflect.TypeOf(InstanceConfig{})
	for _, forbidden := range []string{"Module", "Descriptor", "Provides", "Requires", "Grants"} {
		if _, ok := typeOfConfig.FieldByName(forbidden); ok {
			t.Fatalf("MCP instance config exposes native Module field %s", forbidden)
		}
	}
}

func TestUnconfiguredHostDoesNotConnect(t *testing.T) {
	factory := &fakeFactory{}
	host, err := New(Config{Factory: factory})
	if err != nil {
		t.Fatal(err)
	}
	if got := host.Status(); len(got) != 0 || factory.opens != 0 {
		t.Fatalf("status=%#v opens=%d", got, factory.opens)
	}
}

func TestConfiguredStatusDoesNotActivateInstance(t *testing.T) {
	factory := &fakeFactory{sessions: []*fakeSession{{}}}
	host, err := New(Config{Factory: factory, Instances: []InstanceConfig{{ID: "docs", Endpoint: "https://example.invalid/mcp"}}})
	if err != nil {
		t.Fatal(err)
	}
	status := host.Status()
	if len(status) != 1 || status[0].State != StateInactive || factory.opens != 0 {
		t.Fatalf("status=%#v opens=%d", status, factory.opens)
	}
}

func TestDiscoverLazilyActivatesAndNamespacesTools(t *testing.T) {
	session := &fakeSession{tools: []RemoteTool{{Name: "echo tool", Description: "echo", Schema: json.RawMessage(`{"type":"object"}`)}}}
	factory := &fakeFactory{sessions: []*fakeSession{session}}
	host, err := New(Config{Factory: factory, Instances: []InstanceConfig{{ID: "docs", Command: "fixture"}}})
	if err != nil {
		t.Fatal(err)
	}
	tools, err := host.DiscoverTools(context.Background(), "docs")
	if err != nil {
		t.Fatal(err)
	}
	if factory.opens != 1 || session.discoveries != 1 || len(tools) != 1 {
		t.Fatalf("opens=%d discoveries=%d tools=%#v", factory.opens, session.discoveries, tools)
	}
	if tools[0].ID == "echo tool" || tools[0].InstanceID != "docs" || tools[0].RemoteName != "echo tool" || tools[0].SchemaHash == "" {
		t.Fatalf("projected tool = %#v", tools[0])
	}
	if status := host.Status(); len(status) != 1 || status[0].State != StateReady || status[0].ToolCount != 1 {
		t.Fatalf("ready status = %#v", status)
	}
}

func TestDiscoverRetriesSafeOperationWithinBound(t *testing.T) {
	dead := &fakeSession{discoverErr: errors.New("transport died")}
	ready := &fakeSession{tools: []RemoteTool{{Name: "echo", Schema: json.RawMessage(`{"type":"object"}`)}}}
	factory := &fakeFactory{sessions: []*fakeSession{dead, ready}}
	host, err := New(Config{Factory: factory, MaxSafeRetries: 1, Instances: []InstanceConfig{{ID: "docs", Command: "fixture"}}})
	if err != nil {
		t.Fatal(err)
	}
	items, err := host.DiscoverTools(context.Background(), "docs")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || factory.opens != 2 || dead.closes != 1 {
		t.Fatalf("items=%#v opens=%d dead closes=%d", items, factory.opens, dead.closes)
	}
}

func TestCallToolNeverRetriesUnknownSideEffect(t *testing.T) {
	dead := &fakeSession{tools: []RemoteTool{{Name: "write", Schema: json.RawMessage(`{"type":"object"}`)}}, callErr: errors.New("connection reset")}
	second := &fakeSession{}
	factory := &fakeFactory{sessions: []*fakeSession{dead, second}}
	host, err := New(Config{Factory: factory, MaxSafeRetries: 3, Instances: []InstanceConfig{{ID: "ops", Command: "fixture"}}})
	if err != nil {
		t.Fatal(err)
	}
	items, err := host.DiscoverTools(context.Background(), "ops")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.CallTool(context.Background(), items[0].ID, nil); err == nil {
		t.Fatal("effectful MCP call should surface transport failure")
	}
	if dead.calls != 1 || second.calls != 0 || factory.opens != 1 {
		t.Fatalf("dead calls=%d second calls=%d opens=%d", dead.calls, second.calls, factory.opens)
	}
}

func TestDuplicateRemoteNameFailsClosed(t *testing.T) {
	session := &fakeSession{tools: []RemoteTool{{Name: "echo"}, {Name: "echo"}}}
	factory := &fakeFactory{sessions: []*fakeSession{session}}
	host, err := New(Config{Factory: factory, Instances: []InstanceConfig{{ID: "docs", Command: "fixture"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.DiscoverTools(context.Background(), "docs"); !errors.Is(err, ErrDuplicateRemoteTool) {
		t.Fatalf("duplicate error=%v, want ErrDuplicateRemoteTool", err)
	}
}

func TestDiscoveryTimeoutMarksUnavailable(t *testing.T) {
	session := &fakeSession{block: true}
	factory := &fakeFactory{sessions: []*fakeSession{session}}
	host, err := New(Config{Factory: factory, OperationTimeout: time.Millisecond, Instances: []InstanceConfig{{ID: "slow", Command: "fixture"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.DiscoverTools(context.Background(), "slow"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout error=%v", err)
	}
	status := host.Status()
	if len(status) != 1 || status[0].State != StateUnavailable {
		t.Fatalf("timeout status=%#v", status)
	}
}

func TestCloseCleansEveryActivatedSession(t *testing.T) {
	first := &fakeSession{tools: []RemoteTool{{Name: "one"}}}
	second := &fakeSession{tools: []RemoteTool{{Name: "two"}}}
	factory := &fakeFactory{sessions: []*fakeSession{first, second}}
	host, err := New(Config{Factory: factory, Instances: []InstanceConfig{{ID: "one", Command: "fixture"}, {ID: "two", Command: "fixture"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.DiscoverTools(context.Background(), "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := host.DiscoverTools(context.Background(), "two"); err != nil {
		t.Fatal(err)
	}
	if err := host.Close(); err != nil {
		t.Fatal(err)
	}
	if first.closes != 1 || second.closes != 1 {
		t.Fatalf("session closes=%d/%d", first.closes, second.closes)
	}
}
