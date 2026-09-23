package mcphost

// ND-1 (docs/plans/nudge/ND-1.md) ToolWorld assertions: a remote
// tool-result error (IsError) crosses the internal error channel as the
// typed ToolExecutionError carrying the untrusted text, while
// JSON-RPC/transport failures keep their untyped fatal path.

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func toolworldFixture(t *testing.T, session *fakeSession) (*ToolWorld, string) {
	t.Helper()
	factory := &fakeFactory{sessions: []*fakeSession{session}}
	host, err := New(Config{Factory: factory, Instances: []InstanceConfig{{ID: "ops", Command: "fixture"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.Close() })
	items, err := host.DiscoverTools(context.Background(), "ops")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("discovered %d tools, want 1", len(items))
	}
	return NewToolWorld(host), items[0].ID
}

func TestToolWorldIsErrorReturnsTypedRemoteError(t *testing.T) {
	session := &fakeSession{
		tools:      []RemoteTool{{Name: "flaky"}},
		callResult: &ToolResult{Text: "remote exploded", IsError: true},
	}
	world, id := toolworldFixture(t, session)

	_, err := world.Invoke(context.Background(), nil, id, nil)
	var execErr *ToolExecutionError
	if !errors.As(err, &execErr) {
		t.Fatalf("IsError result error = %T %v, want *ToolExecutionError", err, err)
	}
	if !strings.Contains(execErr.Text, "remote exploded") || !strings.Contains(execErr.Text, "untrusted") {
		t.Fatalf("remote error text lost the untrusted payload: %q", execErr.Text)
	}
}

func TestToolWorldTransportErrorStaysUntyped(t *testing.T) {
	session := &fakeSession{
		tools:   []RemoteTool{{Name: "flaky"}},
		callErr: errors.New("connection reset"),
	}
	world, id := toolworldFixture(t, session)

	_, err := world.Invoke(context.Background(), nil, id, nil)
	if err == nil {
		t.Fatal("transport failure returned nil error")
	}
	var execErr *ToolExecutionError
	if errors.As(err, &execErr) {
		t.Fatalf("transport failure classified as tool-result error: %v", err)
	}
}

func TestToolWorldSuccessReturnsResult(t *testing.T) {
	session := &fakeSession{tools: []RemoteTool{{Name: "echo"}}}
	world, id := toolworldFixture(t, session)

	result, err := world.Invoke(context.Background(), nil, id, nil)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Text != "ok" {
		t.Fatalf("result text = %q, want ok", result.Text)
	}
}
