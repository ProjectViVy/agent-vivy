package app

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"testing"

	"agent-vivy/internal/channelcontract"
	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/rpccontract"
	"agent-vivy/internal/runtime"
)

func TestChannelOwnerImportsNoConcreteChannelImplementation(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	forbidden := map[string]bool{
		path.Join("agent-vivy", "internal", "channelhost"):        true,
		path.Join("agent-vivy", "internal", "modules", "channel"): true,
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imported := range file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatalf("parse import in %s: %v", name, err)
			}
			if forbidden[path] {
				t.Fatalf("%s imports forbidden concrete Channel owner %q", name, path)
			}
		}
	}
}

type ownerProbeFactory struct {
	mu        sync.Mutex
	construct int
	deps      channelcontract.Dependencies
	selection channelcontract.Selection
	owner     *ownerProbe
}

func (factory *ownerProbeFactory) Construct(_ context.Context, deps channelcontract.Dependencies, selection channelcontract.Selection) (channelcontract.Owned, error) {
	factory.mu.Lock()
	defer factory.mu.Unlock()
	factory.construct++
	factory.deps = deps
	factory.selection = selection
	if factory.owner == nil {
		factory.owner = &ownerProbe{}
	}
	return factory.owner, nil
}

type ownerProbe struct {
	mu       sync.Mutex
	events   []string
	startErr error
	stopErr  error
	closeErr error
}

func (owner *ownerProbe) add(event string) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	owner.events = append(owner.events, event)
}
func (owner *ownerProbe) Start(context.Context) error { owner.add("start"); return owner.startErr }
func (owner *ownerProbe) Ready(context.Context) error { owner.add("ready"); return nil }
func (owner *ownerProbe) Stop(context.Context) error  { owner.add("stop"); return owner.stopErr }
func (owner *ownerProbe) Close(context.Context) error { owner.add("close"); return owner.closeErr }
func (owner *ownerProbe) OnRunEvent(context.Context, domain.RunEvent) {
	owner.add("run-event")
}
func (owner *ownerProbe) Deliver(context.Context, string, string, string) error { return nil }
func (owner *ownerProbe) Inspect() channelcontract.State {
	return channelcontract.State{Compiled: true}
}
func (owner *ownerProbe) RPCBindings() []rpccontract.MethodBinding {
	return []rpccontract.MethodBinding{{
		Method: "test/channel-owner", Capability: "test.channel-owner",
		Handler: func(context.Context, rpccontract.Peer, rpccontract.Request) (any, *rpccontract.Error) {
			return "owned", nil
		},
	}}
}
func (owner *ownerProbe) snapshot() []string {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return append([]string(nil), owner.events...)
}

func channelOwnerAssembly(factory channelcontract.Factory) genassembly.RuntimeAssembly {
	assembly := genassembly.BuildDefault()
	assembly.ChannelFactory = factory
	return assembly
}

func TestWithoutEarsConstructsChannelOwnerWithoutStarting(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	factory := &ownerProbeFactory{owner: &ownerProbe{}}
	app, err := NewWithAssembly(context.Background(), newDeepSeekTestConfig(t), channelOwnerAssembly(factory), WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Close() })
	if factory.construct != 1 || factory.selection.ProcessAvailable {
		t.Fatalf("construct=%d process_available=%v", factory.construct, factory.selection.ProcessAvailable)
	}
	if events := factory.owner.snapshot(); len(events) != 0 {
		t.Fatalf("WithoutEars owner events=%v before close", events)
	}
	if len(factory.selection.Providers) == 0 || factory.deps.Settings == nil || factory.deps.Run == nil {
		t.Fatalf("selection/dependencies were not supplied: selection=%+v deps=%+v", factory.selection, factory.deps)
	}
}

func TestChannelOwnerStartsAndContributesToControl(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	factory := &ownerProbeFactory{owner: &ownerProbe{}}
	app, err := NewWithAssembly(context.Background(), newDeepSeekTestConfig(t), channelOwnerAssembly(factory), WithoutGateway())
	if err != nil {
		t.Fatal(err)
	}
	if factory.construct != 1 || !factory.selection.ProcessAvailable {
		t.Fatalf("construct=%d process_available=%v", factory.construct, factory.selection.ProcessAvailable)
	}
	if events := factory.owner.snapshot(); len(events) != 2 || events[0] != "start" || events[1] != "ready" {
		t.Fatalf("startup events=%v", events)
	}
	got, rpcErr := app.control.Handle(context.Background(), nil, rpccontract.Request{Method: "test/channel-owner"})
	if rpcErr != nil || got != "owned" {
		t.Fatalf("contributed dispatch got=%v err=%v", got, rpcErr)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	events := factory.owner.snapshot()
	if len(events) != 4 || events[2] != "stop" || events[3] != "close" {
		t.Fatalf("lifecycle events=%v", events)
	}
}

func TestChannelOwnerStartFailureClosesOwner(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	factory := &ownerProbeFactory{owner: &ownerProbe{startErr: errors.New("start failed")}}
	app, err := NewWithAssembly(context.Background(), newDeepSeekTestConfig(t), channelOwnerAssembly(factory), WithoutGateway())
	if app != nil || err == nil {
		t.Fatalf("app=%v err=%v, want startup failure", app, err)
	}
	events := factory.owner.snapshot()
	if len(events) != 3 || events[0] != "start" || events[1] != "stop" || events[2] != "close" {
		t.Fatalf("rollback events=%v", events)
	}
}

func TestChannelOwnerRollbackJoinsCleanupErrors(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	stopErr := errors.New("channel stop cleanup")
	closeErr := errors.New("channel close cleanup")
	factory := &ownerProbeFactory{owner: &ownerProbe{stopErr: stopErr, closeErr: closeErr}}
	cfg := newDeepSeekTestConfig(t)
	cfg.Logging.Format = "invalid"
	app, err := NewWithAssembly(context.Background(), cfg, channelOwnerAssembly(factory), WithoutGateway())
	if app != nil || err == nil {
		t.Fatalf("app=%v err=%v, want composition failure", app, err)
	}
	if !errors.Is(err, stopErr) || !errors.Is(err, closeErr) {
		t.Fatalf("composition error does not retain cleanup failures: %v", err)
	}
	events := factory.owner.snapshot()
	if len(events) != 2 || events[0] != "stop" || events[1] != "close" {
		t.Fatalf("rollback events=%v", events)
	}
}
