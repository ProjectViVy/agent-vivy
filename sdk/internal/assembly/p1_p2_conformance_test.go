package assembly_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	apphost "agent-vivy/internal/app"
	"agent-vivy/internal/config"
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/tools"
	assemblyv1 "agent-vivy/sdk/internal/assembly"
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port"
	channelport "agent-vivy/sdk/port/channel"
	faceport "agent-vivy/sdk/port/face"
	toolport "agent-vivy/sdk/port/tool"
	worldport "agent-vivy/sdk/port/toolworld"
)

var errConformanceUnavailable = errors.New("conformance instance unavailable")

// TestP1P2PortConformanceSuite is the executable support gate for the four
// public Ports promoted by P1/P2. Graph cases use the real Assembly Compiler;
// runtime cases cross each production Host consumer with a typed Provider.
func TestP1P2PortConformanceSuite(t *testing.T) {
	for _, portName := range []string{"std/tool@v1", "std/tool-world@v1", "std/channel@v1", "std/face@v1"} {
		portName := portName
		t.Run(portName, func(t *testing.T) {
			definition, ok := port.PublicCatalog().Lookup(module.PortRef{Port: portName})
			if !ok {
				t.Fatal("promoted Port is absent from the closed catalog")
			}
			t.Run("registration through sole Host", func(t *testing.T) {
				plan := compilePortFixture(t, definition, portFixture(t, definition))
				hostID := conformanceHostID(definition)
				found := false
				for _, edge := range plan.PortEdges {
					if edge.Port.Port == portName && edge.Provider == "fixture/provider" && edge.Consumer == hostID {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing %s Provider -> sole Host edge in %#v", portName, plan.PortEdges)
				}
				if err := exercisePort(t, portName, "success", context.Background()); err != nil {
					t.Fatalf("typed Provider/Host path: %v", err)
				}
			})

			t.Run("missing Provider", func(t *testing.T) {
				consumer := conformanceDescriptor("fixture/consumer")
				consumer.Requires = []module.Requirement{{PortRef: module.PortRef{Port: portName}, Provider: "fixture/missing"}}
				assertPortCompileError(t, definition, []module.Descriptor{consumer}, assemblyv1.Recipe{APIVersion: assemblyv1.RecipeAPIVersionV1, Modules: []string{consumer.Module.ID}}, "missing provider")
			})

			t.Run("duplicate Provider", func(t *testing.T) {
				fixture := portFixture(t, definition)
				second := fixture[0]
				second.Module.ID = "fixture/other"
				second.Source.Ref = "test:fixture/other"
				fixture = append(fixture, second)
				assertPortCompileError(t, definition, fixture, conformanceRecipe(definition, fixture), "duplicate provider id")
			})

			t.Run("incompatible version", func(t *testing.T) {
				fixture := portFixture(t, definition)
				fixture[0].Provides[0].Port = strings.TrimSuffix(portName, "@v1") + "@v2"
				assertPortCompileError(t, definition, fixture, conformanceRecipe(definition, fixture), "unknown Port")
			})

			t.Run("dependency cycle", func(t *testing.T) {
				fixture := portFixture(t, definition)
				fixture[0].Lifecycle.After = []string{fixture[1].Module.ID}
				fixture[1].Lifecycle.After = []string{fixture[0].Module.ID}
				assertPortCompileError(t, definition, fixture, conformanceRecipe(definition, fixture), "dependency cycle")
			})

			t.Run("denied Grant", func(t *testing.T) {
				fixture := portFixture(t, definition)
				fixture[0].RequestedGrants = []module.Grant{definition.AllowedGrants[0]}
				assertPortCompileError(t, definition, fixture, conformanceRecipe(definition, fixture), "is not approved")
			})

			t.Run("timeout", func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
				defer cancel()
				if err := exercisePort(t, portName, "wait", ctx); !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("runtime error = %v, want deadline exceeded", err)
				}
			})

			t.Run("cancellation", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				if err := exercisePort(t, portName, "wait", ctx); !errors.Is(err, context.Canceled) {
					t.Fatalf("runtime error = %v, want canceled", err)
				}
			})

			t.Run("unavailable instance", func(t *testing.T) {
				if err := exercisePort(t, portName, "unavailable", context.Background()); !errors.Is(err, errConformanceUnavailable) {
					t.Fatalf("runtime error = %v, want unavailable", err)
				}
			})

			t.Run("Secret redaction", func(t *testing.T) {
				secret := "conformance-secret-canary"
				fixture := portFixture(t, definition)
				grant := definition.AllowedGrants[0]
				fixture[0].RequestedGrants = []module.Grant{grant}
				recipe := conformanceRecipe(definition, fixture)
				recipe.GrantApprovals = []assemblyv1.GrantApproval{{Module: fixture[0].Module.ID, Name: grant, Constraints: map[string][]string{"token": {secret}}}}
				_, err := conformanceCompiler(t, fixture).Compile(context.Background(), recipe)
				if err == nil || strings.Contains(err.Error(), secret) {
					t.Fatalf("redacted compiler failure = %v", err)
				}
			})

			t.Run("Inspect provenance", func(t *testing.T) {
				fixture := portFixture(t, definition)
				plan := compilePortFixture(t, definition, fixture)
				canonical, err := assemblyv1.CanonicalRecipe(conformanceRecipe(definition, fixture))
				if err != nil {
					t.Fatal(err)
				}
				manifest, raw, err := assemblyv1.SealManifest(plan, assemblyv1.SealInputs{SpecificationVersion: "test", CompilerVersion: "test", SDKVersion: "test", CanonicalRecipe: canonical, CapabilityStates: map[string]assemblyv1.CapabilityState{portName: assemblyv1.CapabilityUnavailable}})
				if err != nil {
					t.Fatal(err)
				}
				inspected, err := assemblyv1.InspectManifest(raw)
				if err != nil || inspected.GenerationID != manifest.GenerationID || inspected.CapabilityStates[portName] != assemblyv1.CapabilityUnavailable {
					t.Fatalf("InspectManifest() = %#v, %v", inspected, err)
				}
			})

			t.Run("default behavior", func(t *testing.T) {
				exerciseDefaultPort(t, portName)
			})

			t.Run("representative real failure", func(t *testing.T) {
				if err := exerciseHostFailure(t, portName); err == nil {
					t.Fatal("production Host accepted an ungranted or unwired operation")
				}
			})
		})
	}

	t.Run("startup rollback", func(t *testing.T) {
		instance := &conformanceChannelInstance{mode: "unavailable"}
		channels, err := apphost.BindChannels([]channelport.ChannelProvider{conformanceChannelProvider{instance: instance}}, nil, config.Channels{})
		if err != nil {
			t.Fatal(err)
		}
		if err := channels[0].Start(context.Background(), &conformanceChannelHost{}); !errors.Is(err, errConformanceUnavailable) {
			t.Fatalf("Start() error = %v", err)
		}
		if instance.stops.Load() != 1 {
			t.Fatalf("failed instance Stop calls = %d, want 1", instance.stops.Load())
		}
	})

	t.Run("idempotent cleanup", func(t *testing.T) {
		instance := &conformanceChannelInstance{}
		channels, err := apphost.BindChannels([]channelport.ChannelProvider{conformanceChannelProvider{instance: instance}}, nil, config.Channels{})
		if err != nil {
			t.Fatal(err)
		}
		if err := channels[0].Start(context.Background(), &conformanceChannelHost{}); err != nil {
			t.Fatal(err)
		}
		if err := channels[0].Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := channels[0].Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
		if instance.stops.Load() != 1 {
			t.Fatalf("Stop calls = %d, want 1", instance.stops.Load())
		}
	})
}

func conformanceHostID(definition port.Definition) string {
	switch definition.Consumer.Port {
	case "core/tool-host@v1":
		return "vivy/tool-host"
	case "core/channel-host@v1":
		return "vivy/channel-host"
	case "core/face-host@v1":
		return "vivy/face-host"
	default:
		panic("unknown conformance Host: " + definition.Consumer.Port)
	}
}

func portFixture(t *testing.T, definition port.Definition) []module.Descriptor {
	t.Helper()
	hostID := conformanceHostID(definition)
	provider := conformanceDescriptor("fixture/provider", module.PortRef{Port: definition.Ref.Port, ID: "fixture.provider"})
	provider.Requires = []module.Requirement{{PortRef: definition.Consumer, Provider: hostID}}
	host := conformanceDescriptor(hostID, module.PortRef{Port: definition.Consumer.Port, ID: strings.ReplaceAll(hostID, "/", ".")})
	return []module.Descriptor{provider, host}
}

func conformanceDescriptor(id string, provides ...module.PortRef) module.Descriptor {
	return module.Descriptor{APIVersion: module.APIVersionV1, Module: module.Identity{ID: id, Version: "1.0.0"}, Source: module.Source{Ref: "test:" + id, SHA256: strings.Repeat("a", 64)}, Provides: provides, Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration}}
}

func conformanceRecipe(definition port.Definition, descriptors []module.Descriptor) assemblyv1.Recipe {
	recipe := assemblyv1.Recipe{APIVersion: assemblyv1.RecipeAPIVersionV1}
	for _, descriptor := range descriptors {
		recipe.Modules = append(recipe.Modules, descriptor.Module.ID)
	}
	if definition.Cardinality == port.CardinalityExclusive {
		recipe.Exclusive = map[string]string{definition.Ref.Port: "fixture/provider"}
	}
	return recipe
}

func conformanceCompiler(t *testing.T, descriptors []module.Descriptor) assemblyv1.Compiler {
	t.Helper()
	records := make([]assemblyv1.SourceRecord, 0, len(descriptors))
	for _, descriptor := range descriptors {
		records = append(records, assemblyv1.SourceRecord{Descriptor: descriptor, Trust: assemblyv1.TrustT1})
	}
	catalog, err := assemblyv1.NewSourceCatalog(records)
	if err != nil {
		t.Fatal(err)
	}
	return assemblyv1.Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: assemblyv1.P1P2PortEvidence()}
}

func compilePortFixture(t *testing.T, definition port.Definition, descriptors []module.Descriptor) assemblyv1.AssemblyPlan {
	t.Helper()
	plan, err := conformanceCompiler(t, descriptors).Compile(context.Background(), conformanceRecipe(definition, descriptors))
	if err != nil {
		t.Fatalf("Compile(%s): %v", definition.Ref.Port, err)
	}
	return plan
}

func assertPortCompileError(t *testing.T, definition port.Definition, descriptors []module.Descriptor, recipe assemblyv1.Recipe, want string) {
	t.Helper()
	_, err := conformanceCompiler(t, descriptors).Compile(context.Background(), recipe)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("Compile(%s) error = %v, want %q", definition.Ref.Port, err, want)
	}
}

func conformanceResult(ctx context.Context, mode string) error {
	switch mode {
	case "success":
		return nil
	case "unavailable":
		return errConformanceUnavailable
	case "wait":
		<-ctx.Done()
		return ctx.Err()
	default:
		return fmt.Errorf("unknown conformance mode %q", mode)
	}
}

type conformanceToolProvider struct{ mode string }

func (conformanceToolProvider) Definition() toolport.Definition {
	return toolport.Definition{ID: "conformance_tool", Description: "P1/P2 conformance", Effect: toolport.EffectRead, Schema: json.RawMessage(`{"type":"object"}`)}
}
func (provider conformanceToolProvider) Invoke(ctx context.Context, host toolport.Host, _ json.RawMessage) (toolport.Result, error) {
	if provider.mode == "host-failure" {
		_, err := host.InvokeTool(ctx, "another_tool", nil)
		return toolport.Result{}, err
	}
	return toolport.Result{Text: "ok"}, conformanceResult(ctx, provider.mode)
}

type conformanceWorldProvider struct{ mode string }

func (conformanceWorldProvider) Definition() worldport.Definition {
	return worldport.Definition{ID: "conformance_world", Description: "P1/P2 conformance"}
}
func (conformanceWorldProvider) Discover(context.Context, worldport.Host) ([]worldport.ToolDefinition, error) {
	return []worldport.ToolDefinition{{ID: "conformance_world_tool", Description: "P1/P2 conformance", Effect: worldport.EffectRead, Schema: json.RawMessage(`{"type":"object"}`)}}, nil
}

func (provider conformanceWorldProvider) Invoke(ctx context.Context, host worldport.Host, _ string, _ json.RawMessage) (worldport.Result, error) {
	if provider.mode == "host-failure" {
		_, err := host.OpenRead("denied.txt")
		return worldport.Result{}, err
	}
	return worldport.Result{Text: "ok"}, conformanceResult(ctx, provider.mode)
}
func (conformanceWorldProvider) Close(context.Context) error { return nil }

type conformanceChannelProvider struct{ instance *conformanceChannelInstance }

func (conformanceChannelProvider) Definition() channelport.Definition {
	return channelport.Definition{ID: "vivy.conformance", MaxMessageRunes: 1024}
}

func (provider conformanceChannelProvider) Construct(_ context.Context, host channelport.Host) (channelport.Instance, error) {
	provider.instance.host = host
	return provider.instance, nil
}

type conformanceChannelInstance struct {
	mode  string
	host  channelport.Host
	stops atomic.Int32
}

func (instance *conformanceChannelInstance) Start(ctx context.Context) error {
	if instance.mode == "host-failure" {
		_, err := instance.host.Secret("TOKEN")
		return err
	}
	if instance.mode == "unavailable" {
		return errConformanceUnavailable
	}
	return nil
}
func (instance *conformanceChannelInstance) Stop(context.Context) error {
	instance.stops.Add(1)
	return nil
}
func (instance *conformanceChannelInstance) Send(ctx context.Context, _ channelport.OutboundMessage) ([]string, error) {
	return []string{"ok"}, conformanceResult(ctx, instance.mode)
}

type conformanceFaceProvider struct{ mode string }

func (conformanceFaceProvider) Definition() faceport.Definition {
	return faceport.Definition{ID: "vivy.conformance", Kind: "conformance"}
}
func (provider conformanceFaceProvider) Construct(_ context.Context, host faceport.Host) (faceport.Instance, error) {
	if provider.mode == "unavailable" {
		return nil, errConformanceUnavailable
	}
	return conformanceFaceInstance{mode: provider.mode, host: host}, nil
}

type conformanceFaceInstance struct {
	mode string
	host faceport.Host
}

func (instance conformanceFaceInstance) Run(ctx context.Context, _ faceport.Options) (faceport.Result, error) {
	if instance.mode == "host-failure" {
		_, err := instance.host.Call(ctx, "initialize", nil)
		return faceport.Result{}, err
	}
	return faceport.Result{Status: "ok"}, conformanceResult(ctx, instance.mode)
}

type conformanceFaceHost struct{ fail bool }

func (conformanceFaceHost) ModuleID() string { return "vivy/face-host" }
func (host conformanceFaceHost) Call(context.Context, string, any) (json.RawMessage, error) {
	if host.fail {
		return nil, errors.New("control plane unavailable")
	}
	return json.RawMessage(`{}`), nil
}
func (conformanceFaceHost) OnEvent(func(string, json.RawMessage)) {}

type conformanceChannelHost struct{}

func (*conformanceChannelHost) ModuleID() string              { return "vivy/channel-host" }
func (*conformanceChannelHost) Secret(string) (string, error) { return "secret", nil }
func (*conformanceChannelHost) HTTP() *http.Client            { return &http.Client{} }
func (*conformanceChannelHost) DialTLS(context.Context, string, string) (net.Conn, error) {
	return nil, errors.New("not connected")
}
func (*conformanceChannelHost) Settings() json.RawMessage { return json.RawMessage(`{}`) }
func (*conformanceChannelHost) PublishInbound(context.Context, channelport.InboundMessage) error {
	return nil
}
func (*conformanceChannelHost) Media() channelport.MediaStore { return nil }
func (*conformanceChannelHost) Logger() *slog.Logger          { return slog.Default() }

type conformanceModuleHost string

func (host conformanceModuleHost) ModuleID() string { return string(host) }

type conformanceHosts struct{}

func (conformanceHosts) ForModule(id string) module.Host { return conformanceModuleHost(id) }

func exercisePort(t *testing.T, portName, mode string, ctx context.Context) error {
	t.Helper()
	switch portName {
	case "std/tool@v1":
		registry, err := apphost.BindGeneratedTools([]toolport.ToolProvider{conformanceToolProvider{mode: mode}}, tools.NewRegistry())
		if err != nil {
			return err
		}
		bound, ok := registry.Lookup("conformance_tool")
		if !ok {
			return errors.New("conformance Tool was not registered")
		}
		_, err = bound.InvokableRun(ctx, json.RawMessage(`{}`))
		return err
	case "std/tool-world@v1":
		bound, err := apphost.BindToolWorlds(ctx, []worldport.Provider{conformanceWorldProvider{mode: mode}}, map[string][]module.GrantBinding{"conformance_world": {}}, func(context.Context) (string, error) { return t.TempDir(), nil }, nil)
		if err != nil {
			return err
		}
		if len(bound) != 1 {
			return fmt.Errorf("registered ToolWorld tools = %d", len(bound))
		}
		_, err = bound[0].InvokableRun(ctx, json.RawMessage(`{}`))
		return err
	case "std/channel@v1":
		instance := &conformanceChannelInstance{mode: mode}
		channels, err := apphost.BindChannels([]channelport.ChannelProvider{conformanceChannelProvider{instance: instance}}, nil, config.Channels{})
		if err != nil {
			return err
		}
		if err := channels[0].Start(ctx, &conformanceChannelHost{}); err != nil {
			return err
		}
		defer channels[0].Stop(context.Background())
		_, err = channels[0].Send(ctx, channelport.OutboundMessage{Parts: []channelport.Part{{Kind: channelport.PartText, Text: "ping"}}})
		return err
	case "std/face@v1":
		_, err := apphost.RunFaceProvider(ctx, conformanceFaceProvider{mode: mode}, conformanceFaceHost{fail: mode == "host-failure"}, faceport.Options{Prompt: "ping", Out: io.Discard, Err: io.Discard})
		return err
	default:
		return fmt.Errorf("unknown Port %s", portName)
	}
}

func exerciseDefaultPort(t *testing.T, portName string) {
	t.Helper()
	assembly := genassembly.BuildDefault()
	if err := assembly.Start(context.Background(), conformanceHosts{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = assembly.Close(context.Background()) })
	switch portName {
	case "std/tool@v1":
		registry, err := apphost.BindGeneratedTools(assembly.Tools, tools.Builtin(nil))
		_, registered := registry.Lookup("ask_user")
		if err != nil || !registered || len(assembly.Manifest.Tools) != len(assembly.Tools) {
			t.Fatalf("default Tool bind: registered=%v providers=%d manifest=%d err=%v", registered, len(assembly.Tools), len(assembly.Manifest.Tools), err)
		}
	case "std/tool-world@v1":
		bound, err := apphost.BindToolWorlds(context.Background(), assembly.Worlds, assembly.ToolWorldGrants, nil, nil)
		if err != nil || len(bound) != 0 || len(assembly.Manifest.ToolWorlds) != 1 || assembly.Manifest.ToolWorlds[0] != "mcp" {
			t.Fatalf("default ToolWorld inactive behavior: bound=%d manifest=%v err=%v", len(bound), assembly.Manifest.ToolWorlds, err)
		}
	case "std/channel@v1":
		bound, err := apphost.BindChannels(assembly.Channels, assembly.ChannelGrants, config.Channels{})
		if err != nil || len(bound) != len(assembly.Manifest.Channels) {
			t.Fatalf("default Channel bind: bound=%d manifest=%d err=%v", len(bound), len(assembly.Manifest.Channels), err)
		}
		for _, channel := range bound {
			if _, err := channel.Send(context.Background(), channelport.OutboundMessage{}); err == nil {
				t.Fatalf("unconfigured default Channel %s became active", channel.Name())
			}
		}
	case "std/face@v1":
		if assembly.Face != nil || assembly.Manifest.Face != "kernel-headless" {
			t.Fatalf("default headless Face = %#v, manifest=%q", assembly.Face, assembly.Manifest.Face)
		}
	}
}

func exerciseHostFailure(t *testing.T, portName string) error {
	t.Helper()
	return exercisePort(t, portName, "host-failure", context.Background())
}
