package mcphost

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
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

func TestDisabledConfiguredInstanceStaysInactiveAndCannotOpen(t *testing.T) {
	enabled := false
	factory := &fakeFactory{sessions: []*fakeSession{{tools: []RemoteTool{{Name: "should_not_run"}}}}}
	host, err := New(Config{Factory: factory, Instances: []InstanceConfig{{
		ID: "disabled", Endpoint: "https://example.invalid/mcp", Enabled: &enabled,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	status := host.Status()
	if len(status) != 1 || status[0].State != StateInactive || status[0].Enabled {
		t.Fatalf("disabled status = %#v", status)
	}
	if _, err := host.DiscoverTools(context.Background(), "disabled"); !errors.Is(err, ErrInstanceUnavailable) {
		t.Fatalf("disabled discovery error = %v, want ErrInstanceUnavailable", err)
	}
	if factory.opens != 0 {
		t.Fatalf("disabled discovery opened %d sessions", factory.opens)
	}
}

func TestDisabledDeferredInstanceProjectsInactiveAndCannotOpen(t *testing.T) {
	enabled := false
	factory := &fakeFactory{sessions: []*fakeSession{{tools: []RemoteTool{{Name: "should_not_run"}}}}}
	host, err := New(Config{Factory: factory, Instances: []InstanceConfig{{
		ID: "disabled", Endpoint: "https://example.invalid/mcp", DeferredReason: "OAuth is not configured", Enabled: &enabled,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	status := host.Status()
	if len(status) != 1 || status[0].State != StateInactive || status[0].Enabled || status[0].DeferredReason == "" {
		t.Fatalf("disabled deferred status = %#v", status)
	}
	if _, err := host.DiscoverTools(context.Background(), "disabled"); !errors.Is(err, ErrInstanceUnavailable) {
		t.Fatalf("disabled deferred discovery error = %v, want ErrInstanceUnavailable", err)
	}
	if _, err := host.ListResources(context.Background(), "disabled"); !errors.Is(err, ErrInstanceUnavailable) {
		t.Fatalf("disabled deferred resource error = %v, want ErrInstanceUnavailable", err)
	}
	if factory.opens != 0 {
		t.Fatalf("disabled deferred instance opened %d sessions", factory.opens)
	}
}

func TestDiscoverLazilyActivatesAndNamespacesTools(t *testing.T) {
	session := &fakeSession{tools: []RemoteTool{{Name: "echo_tool", Description: "echo", Schema: json.RawMessage(`{"type":"object"}`)}}}
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
	if tools[0].ID == "echo_tool" || tools[0].InstanceID != "docs" || tools[0].RemoteName != "echo_tool" || tools[0].SchemaHash == "" {
		t.Fatalf("projected tool = %#v", tools[0])
	}
	if status := host.Status(); len(status) != 1 || status[0].State != StateReady || status[0].ToolCount != 1 {
		t.Fatalf("ready status = %#v", status)
	}
}

func TestProjectedSchemaAndIdentityHashesAreDeterministic(t *testing.T) {
	build := func(schema json.RawMessage) ToolDefinition {
		session := &fakeSession{tools: []RemoteTool{{Name: "echo", Schema: schema}}}
		host, err := New(Config{Factory: &fakeFactory{sessions: []*fakeSession{session}}, Instances: []InstanceConfig{{ID: "docs", Command: "fixture"}}})
		if err != nil {
			t.Fatal(err)
		}
		items, err := host.DiscoverTools(context.Background(), "docs")
		if err != nil {
			t.Fatal(err)
		}
		return items[0]
	}
	first := build(json.RawMessage(`{"type":"object","properties":{"value":{"type":"string"}}}`))
	second := build(json.RawMessage(`{ "properties": { "value": { "type": "string" } }, "type": "object" }`))
	if first.SchemaHash == "" || first.InstanceHash == "" || first.RemoteHash == "" {
		t.Fatalf("projected hashes missing: %#v", first)
	}
	if first.SchemaHash != second.SchemaHash || first.InstanceHash != second.InstanceHash || first.RemoteHash != second.RemoteHash {
		t.Fatalf("hashes are not deterministic: first=%#v second=%#v", first, second)
	}
}

func TestSchemaChangeMarksBindingUnavailable(t *testing.T) {
	session := &fakeSession{tools: []RemoteTool{{Name: "echo", Schema: json.RawMessage(`{"type":"object","properties":{"first":{"type":"string"}}}`)}}}
	factory := &fakeFactory{sessions: []*fakeSession{session}}
	host, err := New(Config{Factory: factory, Instances: []InstanceConfig{{ID: "docs", Command: "fixture"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.DiscoverTools(context.Background(), "docs"); err != nil {
		t.Fatal(err)
	}
	// A second discovery from the same session is the remote capability
	// reconciliation boundary. The implementation must not silently replace
	// a live ToolHost binding with a new schema.
	session.tools[0].Schema = json.RawMessage(`{"type":"object","properties":{"second":{"type":"integer"}}}`)
	if _, err := host.DiscoverTools(context.Background(), "docs"); !errors.Is(err, ErrSchemaChanged) {
		t.Fatalf("schema change error = %v, want ErrSchemaChanged", err)
	}
	if _, ok := host.LookupTool("mcp.docs.echo"); ok {
		t.Fatal("schema-changed binding remained executable")
	}
	status := host.Status()
	if len(status) != 1 || status[0].State != StateUnavailable {
		t.Fatalf("schema-change status = %#v, want unavailable", status)
	}
}

func TestFailedDiscoveryAtomicallyClearsBindingAndSession(t *testing.T) {
	session := &fakeSession{tools: []RemoteTool{{Name: "echo", Schema: json.RawMessage(`{"type":"object"}`)}}}
	factory := &fakeFactory{sessions: []*fakeSession{session, {discoverErr: errors.New("list failed")}}}
	host, err := New(Config{Factory: factory, MaxSafeRetries: 1, Instances: []InstanceConfig{{ID: "docs", Command: "fixture"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.DiscoverTools(context.Background(), "docs"); err != nil {
		t.Fatal(err)
	}

	session.discoverErr = errors.New("list failed")
	if _, err := host.DiscoverTools(context.Background(), "docs"); err == nil {
		t.Fatal("failed discovery unexpectedly succeeded")
	}
	if _, ok := host.LookupTool("mcp.docs.echo"); ok {
		t.Fatal("failed discovery left a stale ToolHost binding")
	}
	if _, err := host.CallTool(context.Background(), "mcp.docs.echo", nil); !errors.Is(err, ErrUnknownTool) {
		t.Fatalf("stale binding call error = %v, want ErrUnknownTool", err)
	}
	status := host.Status()
	if len(status) != 1 || status[0].State != StateUnavailable {
		t.Fatalf("failed discovery status = %#v, want unavailable", status)
	}
	if session.closes != 1 {
		t.Fatalf("failed discovery session closes = %d, want 1", session.closes)
	}
}

func TestFailedProjectionAtomicallyClearsBindingAndSession(t *testing.T) {
	session := &fakeSession{tools: []RemoteTool{{Name: "echo", Schema: json.RawMessage(`{"type":"object"}`)}}}
	factory := &fakeFactory{sessions: []*fakeSession{session}}
	host, err := New(Config{Factory: factory, Instances: []InstanceConfig{{ID: "docs", Command: "fixture"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.DiscoverTools(context.Background(), "docs"); err != nil {
		t.Fatal(err)
	}

	session.tools[0].Schema = json.RawMessage(`{"type":`)
	if _, err := host.DiscoverTools(context.Background(), "docs"); !errors.Is(err, ErrInvalidTool) {
		t.Fatalf("projection error = %v, want ErrInvalidTool", err)
	}
	if _, ok := host.LookupTool("mcp.docs.echo"); ok {
		t.Fatal("failed projection left a stale ToolHost binding")
	}
	if session.closes != 1 {
		t.Fatalf("failed projection session closes = %d, want 1", session.closes)
	}
}

func TestSchemaHashPreservesLargeJSONNumbers(t *testing.T) {
	build := func(schema json.RawMessage) string {
		session := &fakeSession{tools: []RemoteTool{{Name: "echo", Schema: schema}}}
		host, err := New(Config{Factory: &fakeFactory{sessions: []*fakeSession{session}}, Instances: []InstanceConfig{{ID: "docs", Command: "fixture"}}})
		if err != nil {
			t.Fatal(err)
		}
		items, err := host.DiscoverTools(context.Background(), "docs")
		if err != nil {
			t.Fatal(err)
		}
		return items[0].SchemaHash
	}
	first := build(json.RawMessage(`{"type":"number","minimum":9007199254740992}`))
	second := build(json.RawMessage(`{"type":"number","minimum":9007199254740993}`))
	if first == second {
		t.Fatalf("large JSON numbers collapsed to one schema hash: %q", first)
	}
}

func TestDottedNamespacesFailClosed(t *testing.T) {
	if _, err := New(Config{Factory: &fakeFactory{}, Instances: []InstanceConfig{{ID: "docs.v2", Command: "fixture"}}}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("dotted instance id error = %v, want ErrInvalidConfig", err)
	}
	session := &fakeSession{tools: []RemoteTool{{Name: "echo.v2"}}}
	host, err := New(Config{Factory: &fakeFactory{sessions: []*fakeSession{session}}, Instances: []InstanceConfig{{ID: "docs", Command: "fixture"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.DiscoverTools(context.Background(), "docs"); !errors.Is(err, ErrInvalidTool) {
		t.Fatalf("dotted remote name error = %v, want ErrInvalidTool", err)
	}
}

func TestUnsafeNamespacesFailClosedWithoutSanitizing(t *testing.T) {
	for _, instanceID := range []string{"docs.v2", "docs server", " docs", "docs ", "docs/server", "docs$prod"} {
		t.Run("instance/"+instanceID, func(t *testing.T) {
			if _, err := New(Config{Factory: &fakeFactory{}, Instances: []InstanceConfig{{ID: instanceID, Command: "fixture"}}}); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("instance id %q accepted/sanitized: %v", instanceID, err)
			}
		})
	}
	for _, remoteName := range []string{"echo.v2", "echo tool", " echo", "echo ", "echo/tool", "echo$prod", "écho"} {
		t.Run("remote/"+remoteName, func(t *testing.T) {
			session := &fakeSession{tools: []RemoteTool{{Name: remoteName}}}
			host, err := New(Config{Factory: &fakeFactory{sessions: []*fakeSession{session}}, Instances: []InstanceConfig{{ID: "docs", Command: "fixture"}}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := host.DiscoverTools(context.Background(), "docs"); !errors.Is(err, ErrInvalidTool) {
				t.Fatalf("remote name %q accepted/sanitized: %v", remoteName, err)
			}
		})
	}
}

func TestProtectedProjectedIDFailsClosed(t *testing.T) {
	session := &fakeSession{tools: []RemoteTool{{Name: "bash"}}}
	host, err := New(Config{
		Factory:      &fakeFactory{sessions: []*fakeSession{session}},
		ProtectedIDs: []string{"mcp.docs.bash"},
		Instances:    []InstanceConfig{{ID: "docs", Command: "fixture"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.DiscoverTools(context.Background(), "docs"); !errors.Is(err, ErrProtectedToolID) {
		t.Fatalf("protected projected id error = %v, want ErrProtectedToolID", err)
	}
}

func TestConfiguredCircuitThresholdHonored(t *testing.T) {
	factory := &fakeFactory{err: errors.New("open failed")}
	host, err := New(Config{Factory: factory, CircuitFailureThreshold: 2, MaxSafeRetries: 0, Instances: []InstanceConfig{{ID: "docs", Command: "fixture"}}})
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := host.DiscoverTools(context.Background(), "docs"); err == nil {
			t.Fatal("open failure unexpectedly succeeded")
		}
	}
	if got := factory.opens; got != 2 {
		t.Fatalf("factory opens = %d, want configured threshold attempts", got)
	}
	if _, err := host.DiscoverTools(context.Background(), "docs"); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("third open error = %v, want ErrCircuitOpen", err)
	}
	if factory.opens != 2 {
		t.Fatalf("circuit-open discovery reopened session: opens=%d", factory.opens)
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
	first := &fakeSession{block: true}
	second := &fakeSession{block: true}
	factory := &fakeFactory{sessions: []*fakeSession{first, second}}
	host, err := New(Config{Factory: factory, OperationTimeout: time.Millisecond, MaxSafeRetries: 1, Instances: []InstanceConfig{{ID: "slow", Command: "fixture"}}})
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

func TestReplaceInstancesRemovesChangedBindingBeforeReopening(t *testing.T) {
	oldSession := &fakeSession{tools: []RemoteTool{{Name: "old", Schema: json.RawMessage(`{"type":"object"}`)}}}
	newSession := &fakeSession{tools: []RemoteTool{{Name: "new", Schema: json.RawMessage(`{"type":"object"}`)}}}
	factory := &fakeFactory{sessions: []*fakeSession{oldSession, newSession}}
	host, err := New(Config{
		Factory:   factory,
		Instances: []InstanceConfig{{ID: "docs", Endpoint: "https://old.example.invalid/mcp"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	initial, err := host.DiscoverTools(context.Background(), "docs")
	if err != nil {
		t.Fatal(err)
	}
	if len(initial) != 1 || initial[0].ID != "mcp.docs.old" {
		t.Fatalf("initial tools = %#v", initial)
	}

	if err := host.ReplaceInstances([]InstanceConfig{{ID: "docs", Endpoint: "https://new.example.invalid/mcp"}}); err != nil {
		t.Fatalf("replace instances: %v", err)
	}
	if _, ok := host.LookupTool("mcp.docs.old"); ok {
		t.Fatal("changed instance left its old binding executable")
	}
	if _, err := host.CallTool(context.Background(), "mcp.docs.old", nil); !errors.Is(err, ErrUnknownTool) {
		t.Fatalf("old binding call error = %v, want ErrUnknownTool", err)
	}
	if oldSession.closes != 1 {
		t.Fatalf("old session closes = %d, want 1", oldSession.closes)
	}

	replaced, err := host.DiscoverTools(context.Background(), "docs")
	if err != nil {
		t.Fatal(err)
	}
	if len(replaced) != 1 || replaced[0].ID != "mcp.docs.new" {
		t.Fatalf("replacement tools = %#v", replaced)
	}
	if _, ok := host.LookupTool("mcp.docs.old"); ok {
		t.Fatal("old binding reappeared after replacement discovery")
	}
	if _, ok := host.LookupTool("mcp.docs.new"); !ok {
		t.Fatal("new binding missing after replacement discovery")
	}
}

type interleavedDiscoverySession struct {
	started chan struct{}
	release chan struct{}
	tools   []RemoteTool
}

type orderedDiscoverySession struct {
	started   chan int
	responses []chan []RemoteTool
	count     int
	mu        sync.Mutex
}

func (session *orderedDiscoverySession) DiscoverTools(ctx context.Context) ([]RemoteTool, error) {
	session.mu.Lock()
	index := session.count
	session.count++
	session.mu.Unlock()
	session.started <- index
	select {
	case tools := <-session.responses[index]:
		return append([]RemoteTool(nil), tools...), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (*orderedDiscoverySession) CallTool(context.Context, string, json.RawMessage) (ToolResult, error) {
	return ToolResult{}, nil
}
func (*orderedDiscoverySession) ListResources(context.Context) ([]RemoteResource, error) {
	return nil, nil
}
func (*orderedDiscoverySession) ReadResource(context.Context, string) (ResourceContent, error) {
	return ResourceContent{}, nil
}
func (*orderedDiscoverySession) Close() error { return nil }

type orderedDiscoveryFactory struct{ session *orderedDiscoverySession }

func (factory *orderedDiscoveryFactory) Open(context.Context, InstanceConfig) (Session, error) {
	return factory.session, nil
}

func TestConcurrentDiscoveryOlderResponseCannotOverwriteNewerSchema(t *testing.T) {
	session := &orderedDiscoverySession{
		started:   make(chan int, 2),
		responses: []chan []RemoteTool{make(chan []RemoteTool, 1), make(chan []RemoteTool, 1)},
	}
	host, err := New(Config{Factory: &orderedDiscoveryFactory{session: session}, Instances: []InstanceConfig{{ID: "docs", Command: "fixture"}}})
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	go func() {
		_, discoverErr := host.DiscoverTools(context.Background(), "docs")
		results <- discoverErr
	}()
	if index := <-session.started; index != 0 {
		t.Fatalf("first discovery index = %d, want 0", index)
	}
	go func() {
		_, discoverErr := host.DiscoverTools(context.Background(), "docs")
		results <- discoverErr
	}()
	if index := <-session.started; index != 1 {
		t.Fatalf("second discovery index = %d, want 1", index)
	}

	// The newer request completes first. The older response must not publish
	// its older schema after this newer catalog has become authoritative.
	session.responses[1] <- []RemoteTool{{Name: "echo", Schema: json.RawMessage(`{"type":"integer","minimum":2}`)}}
	secondErr := <-results
	session.responses[0] <- []RemoteTool{{Name: "echo", Schema: json.RawMessage(`{"type":"integer","minimum":1}`)}}
	firstErr := <-results
	if secondErr != nil {
		t.Fatalf("newer discovery error = %v", secondErr)
	}
	if !errors.Is(firstErr, ErrStaleDiscovery) {
		t.Fatalf("older discovery error = %v, want ErrStaleDiscovery", firstErr)
	}
	definition, ok := host.LookupTool("mcp.docs.echo")
	if !ok {
		t.Fatal("newer discovery binding disappeared")
	}
	if string(definition.Schema) != `{"type":"integer","minimum":2}` {
		t.Fatalf("binding schema = %s, want newer response", definition.Schema)
	}
}

func (session *interleavedDiscoverySession) DiscoverTools(ctx context.Context) ([]RemoteTool, error) {
	close(session.started)
	select {
	case <-session.release:
		return append([]RemoteTool(nil), session.tools...), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func (*interleavedDiscoverySession) CallTool(context.Context, string, json.RawMessage) (ToolResult, error) {
	return ToolResult{}, nil
}
func (*interleavedDiscoverySession) ListResources(context.Context) ([]RemoteResource, error) {
	return nil, nil
}
func (*interleavedDiscoverySession) ReadResource(context.Context, string) (ResourceContent, error) {
	return ResourceContent{}, nil
}
func (*interleavedDiscoverySession) Close() error { return nil }

func TestRetiredDiscoveryCannotResurrectBinding(t *testing.T) {
	oldSession := &interleavedDiscoverySession{
		started: make(chan struct{}), release: make(chan struct{}),
		tools: []RemoteTool{{Name: "old", Schema: json.RawMessage(`{"type":"object"}`)}},
	}
	host, err := New(Config{
		Factory:   &interleavedFactory{old: oldSession},
		Instances: []InstanceConfig{{ID: "docs", Endpoint: "https://old.example.invalid/mcp"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, discoverErr := host.DiscoverTools(context.Background(), "docs")
		result <- discoverErr
	}()
	<-oldSession.started
	if err := host.ReplaceInstances([]InstanceConfig{{ID: "docs", Endpoint: "https://new.example.invalid/mcp"}}); err != nil {
		t.Fatal(err)
	}
	close(oldSession.release)
	if err := <-result; !errors.Is(err, ErrInstanceUnavailable) {
		t.Fatalf("retired discovery error = %v, want ErrInstanceUnavailable", err)
	}
	if _, ok := host.LookupTool("mcp.docs.old"); ok {
		t.Fatal("retired discovery resurrected an old binding")
	}
}

type interleavedFactory struct {
	mu    sync.Mutex
	old   *interleavedDiscoverySession
	new   *fakeSession
	opens int
}

func (factory *interleavedFactory) Open(_ context.Context, config InstanceConfig) (Session, error) {
	factory.mu.Lock()
	defer factory.mu.Unlock()
	factory.opens++
	if config.Endpoint == "https://old.example.invalid/mcp" {
		return factory.old, nil
	}
	if factory.new == nil {
		factory.new = &fakeSession{tools: []RemoteTool{{Name: "new", Schema: json.RawMessage(`{"type":"object"}`)}}}
	}
	return factory.new, nil
}
