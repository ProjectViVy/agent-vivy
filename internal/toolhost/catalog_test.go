package toolhost

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	porttool "agent-vivy/sdk/port/tool"
	"agent-vivy/sdk/port/toolworld"
)

type testWorldHost struct{ id string }

func (h testWorldHost) ModuleID() string                         { return h.id }
func (h testWorldHost) Workspace() string                        { return "" }
func (h testWorldHost) OpenRead(string) (io.ReadCloser, error)   { return nil, toolworld.ErrDenied }
func (h testWorldHost) OpenWrite(string) (io.WriteCloser, error) { return nil, toolworld.ErrDenied }
func (h testWorldHost) Spawn(context.Context, toolworld.SpawnSpec) (toolworld.Proc, error) {
	return nil, toolworld.ErrDenied
}

type testWorldProvider struct {
	id            string
	discoveries   [][]toolworld.ToolDefinition
	discoveriesN  int
	discoverErr   error
	waitForCancel bool
	invoked       string
}

func (p *testWorldProvider) Definition() toolworld.Definition { return toolworld.Definition{ID: p.id} }
func (p *testWorldProvider) Discover(ctx context.Context, _ toolworld.Host) ([]toolworld.ToolDefinition, error) {
	if p.waitForCancel {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if p.discoverErr != nil {
		return nil, p.discoverErr
	}
	if len(p.discoveries) == 0 {
		return nil, nil
	}
	index := p.discoveriesN
	if index >= len(p.discoveries) {
		index = len(p.discoveries) - 1
	}
	p.discoveriesN++
	return p.discoveries[index], nil
}
func (p *testWorldProvider) Invoke(_ context.Context, _ toolworld.Host, id string, _ json.RawMessage) (toolworld.Result, error) {
	p.invoked = id
	return toolworld.Result{Text: "dynamic-ok"}, nil
}
func (*testWorldProvider) Close(context.Context) error { return nil }

func TestHostRejectsStaticDynamicToolIDCollision(t *testing.T) {
	world := &testWorldProvider{id: "fixture.world", discoveries: [][]toolworld.ToolDefinition{{{ID: "shared.id"}}}}
	host, err := New(Config{
		Static: []StaticBinding{{OwnerID: "static", Provider: testToolProvider{def: porttool.Definition{ID: "shared.id"}}, Host: testModuleHost{id: "static"}}},
		Worlds: []WorldBinding{{OwnerID: "fixture.world", Provider: world, Host: testWorldHost{id: "fixture.world"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.Discover(context.Background()); !errors.Is(err, ErrDuplicateToolID) {
		t.Fatalf("Discover collision error = %v, want ErrDuplicateToolID", err)
	}
}

func TestHostDiscoveryHonorsTimeout(t *testing.T) {
	world := &testWorldProvider{id: "fixture.slow", waitForCancel: true}
	host, err := New(Config{
		DiscoveryTimeout: time.Millisecond,
		Worlds:           []WorldBinding{{OwnerID: "fixture.slow", Provider: world, Host: testWorldHost{id: "fixture.slow"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.Discover(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Discover timeout error = %v, want deadline exceeded", err)
	}
}

func TestHostInvalidatesDynamicSchemaHashOnRediscovery(t *testing.T) {
	world := &testWorldProvider{id: "fixture.world", discoveries: [][]toolworld.ToolDefinition{
		{{ID: "remote.echo", Schema: json.RawMessage(`{"type":"object","properties":{"a":{"type":"string"}}}`)}},
		{{ID: "remote.echo", Schema: json.RawMessage(`{"type":"object","properties":{"b":{"type":"string"}}}`)}},
	}}
	host, err := New(Config{Worlds: []WorldBinding{{OwnerID: "fixture.world", Provider: world, Host: testWorldHost{id: "fixture.world"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}
	first, ok := host.Lookup("remote.echo")
	if !ok {
		t.Fatal("remote.echo missing after first discovery")
	}
	if _, err := host.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}
	second, ok := host.Lookup("remote.echo")
	if !ok {
		t.Fatal("remote.echo missing after rediscovery")
	}
	if first.SchemaHash == second.SchemaHash {
		t.Fatalf("schema hash did not change: %q", first.SchemaHash)
	}
}

func TestHostFailedDiscoveryClearsPreviousDynamicBindings(t *testing.T) {
	world := &testWorldProvider{
		id:          "fixture.world",
		discoveries: [][]toolworld.ToolDefinition{{{ID: "remote.echo"}}},
	}
	host, err := New(Config{Worlds: []WorldBinding{{OwnerID: "fixture.world", Provider: world, Host: testWorldHost{id: "fixture.world"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := host.Lookup("remote.echo"); !ok {
		t.Fatal("remote.echo missing after initial discovery")
	}

	world.discoverErr = errors.New("remote discovery failed")
	if _, err := host.Discover(context.Background()); !errors.Is(err, world.discoverErr) {
		t.Fatalf("failed discovery error = %v, want %v", err, world.discoverErr)
	}
	if _, ok := host.Lookup("remote.echo"); ok {
		t.Fatal("failed discovery left stale dynamic binding")
	}
	if _, err := host.Invoke(context.Background(), Request{ID: "remote.echo"}); !errors.Is(err, ErrUnknownToolID) {
		t.Fatalf("stale dynamic invoke error = %v, want ErrUnknownToolID", err)
	}
}

func TestToolSchemaHashPreservesLargeJSONNumbers(t *testing.T) {
	first := hashDefinition(porttool.Definition{
		ID:     "large-number",
		Schema: json.RawMessage(`{"type":"number","minimum":9007199254740992}`),
	})
	second := hashDefinition(porttool.Definition{
		ID:     "large-number",
		Schema: json.RawMessage(`{"type":"number","minimum":9007199254740993}`),
	})
	if first == second {
		t.Fatalf("large JSON numbers collapsed to one ToolHost schema hash: %q", first)
	}
}

func TestEveryToolPathUsesHostInvoke(t *testing.T) {
	world := &testWorldProvider{id: "fixture.world", discoveries: [][]toolworld.ToolDefinition{{{ID: "remote.echo"}}}}
	host, err := New(Config{
		Static: []StaticBinding{{OwnerID: "static", Provider: testToolProvider{def: porttool.Definition{ID: "static.echo"}, result: "static-ok"}, Host: testModuleHost{id: "static"}}},
		Worlds: []WorldBinding{{OwnerID: "fixture.world", Provider: world, Host: testWorldHost{id: "fixture.world"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}
	staticResult, err := host.Invoke(context.Background(), Request{ID: "static.echo"})
	if err != nil || staticResult.Text != "static-ok" {
		t.Fatalf("static Invoke = %#v, %v", staticResult, err)
	}
	dynamicResult, err := host.Invoke(context.Background(), Request{ID: "remote.echo"})
	if err != nil || dynamicResult.Text != "dynamic-ok" || world.invoked != "remote.echo" {
		t.Fatalf("dynamic Invoke = %#v, %v; invoked=%q", dynamicResult, err, world.invoked)
	}
}
