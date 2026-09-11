package mcphost

import (
	"context"
	"testing"

	"agent-vivy/internal/statushost"
	statusport "agent-vivy/sdk/port/status"
)

func TestMCPStatusReadDoesNotConnectOrRevive(t *testing.T) {
	factory := &fakeFactory{sessions: []*fakeSession{{}}}
	host, err := New(Config{Factory: factory, Instances: []InstanceConfig{
		{ID: "inactive", Command: "fixture"},
		{ID: "unconfigured"},
		{ID: "oauth", Endpoint: "https://example.invalid/mcp", DeferredReason: "oauth_deferred"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	statusHost, err := statushost.New(statushost.Config{Providers: []statusport.Provider{NewStatusSource(host)}})
	if err != nil {
		t.Fatal(err)
	}
	result := statusHost.Read(context.Background(), statusport.Request{})
	if factory.opens != 0 {
		t.Fatalf("status read activated %d MCP sessions", factory.opens)
	}
	if len(result) != 1 || len(result[0].Snapshot.Items) != 3 {
		t.Fatalf("status result = %#v", result)
	}
	states := map[string]string{}
	for _, item := range result[0].Snapshot.Items {
		states[item.ID] = item.State
	}
	if states["mcp/inactive"] != string(StateInactive) || states["mcp/unconfigured"] != string(StateUnconfigured) || states["mcp/oauth"] != string(StateDeferred) {
		t.Fatalf("projected states = %#v", states)
	}
}
