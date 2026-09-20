# CH-P0-2 Backend Owner Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Story:** CH-P0-2

**Parent:** [#42 — Channel subsystem modularization](https://github.com/ProjectViVy/agent-vivy/issues/42)

**Epic index:** [Channel Modularization Epic Execution Index](channel-modularization/index.md)

**Status:** COMPLETE; see `docs/logs/2026-09-19-channel-p0-2/acceptance.md`

**Immediate predecessor:** accepted CH-P0-1

**Goal:** Make `vivy/channel-host` the real backend owner so app and RPC compose Channel only through the CH-P0-1 contracts while preserving the default Generation's behavior.

**Architecture:** Add `internal/modules/channel` as the canonical Factory/Owned implementation and keep `internal/channelhost` as its private, proven pipeline engine. Generated Assembly supplies the factory; app performs only contract-level late construction/lifecycle orchestration; RPC validates and dispatches generic typed contributions.

**Tech Stack:** Go 1.26, generated Go Assembly, JSON-RPC 2.0, existing `sdk/module` lifecycle and `sdk/port/channel` ABI, YAML-backed app settings, pnpm/Vitest frontend compatibility gates.

**Spec:** `docs/superpowers/specs/2026-09-19-channel-p0-2-backend-owner-design.md`

## Global Constraints

- Work on the existing `feat/channel-p0-1-contracts` linked worktree, as explicitly selected by the user.
- Preserve the default manifest, five first-party Channel Providers, JSON responses, settings schema, grants, Run path, and delivery behavior.
- `internal/app` and `internal/rpc` must not import `internal/channelhost` or `internal/modules/channel` after composition is complete.
- `internal/modules/channel` must not import `internal/app`, `internal/rpc`, generated Assembly, or platform Provider implementations.
- Construction starts no networking; Provider networking starts only from `Owned.Start` after Service recovery.
- Do not add reduced Recipes, conditional Channel implementation imports, physical omission claims, UI Module work, or `ui_extensions` population.
- Do not expand the public `sdk/port/channel` ABI.
- Keep one Service, one dispatcher, one settings document, and one Channel owner. No discovery, reflection, or dynamic loading.
- Use TDD for every behavior change and commit after each independently reviewable task.
- Go commands use `PATH=/workspace/scratch/4f137d903ad1/toolchains/go1.26.4/bin:$PATH` and `GOFLAGS=-buildvcs=false` in this worktree.

## Review Focus

- A duplicate contributed method across two contributions must fail handler construction deterministically; Task 4 pins this.
- A contribution colliding with any built-in switch case, including a compatibility alias, must fail rather than become unreachable; Task 4 pins the complete core vocabulary.
- `WithoutEars` must retain compiled inventory and management inspection without starting a Provider; Tasks 2 and 6 pin this.
- A failed or read-only settings update must neither mutate unrelated settings nor invoke `OnSettingsChanged`; Tasks 3 and 6 pin this.
- Startup failure after Channel construction must stop/close the owned instance once and before storage teardown; Task 6 pins rollback and close ordering.

---

## File map

| File | Responsibility |
|---|---|
| `internal/channelcontract/contract.go` | Add the process-availability selection input required by `WithoutEars`. |
| `internal/rpccontract/protocol.go` | Own common JSON-RPC error codes used by core and contributed handlers. |
| `internal/modules/channel/module.go` | Factory, Owned lifecycle, private Host construction, Inspect, delivery/run-hook delegation. |
| `internal/modules/channel/binding.go` | Provider binding and grant-limited `channel.Host` facade moved out of app. |
| `internal/modules/channel/management.go` | Channel RPC DTOs and three contributed handlers. |
| `internal/app/channel_settings.go` | Focused adapter from shared app settings to `channelcontract.SettingsAccess`. |
| `internal/rpc/contribution_dispatch.go` | Immutable validated contribution index and capability projection. |
| `internal/rpc/control.go` | Consume generic contribution index; remove all Channel-specific code/imports. |
| `internal/modules/defaults/catalog.go` | Bind the canonical Host to its specialized factory constructor. |
| `sdk/internal/assembly/source.go` | Carry the build-owned Channel factory constructor binding. |
| `sdk/internal/assembly/runtime_generate.go` | Emit the factory field and skip the legacy placeholder owner lifecycle. |
| `internal/generated/assembly/zz_default.go` | Regenerated default Assembly containing the real Channel factory. |
| `internal/app/app.go` | Late construct/start/stop/close only through `channelcontract.Owned`. |

### Task 1: Complete the frozen contracts for P0-2 execution

**Files:**
- Modify: `internal/channelcontract/contract.go`
- Modify: `internal/channelcontract/contract_test.go`
- Modify: `internal/rpccontract/protocol.go`
- Modify: `internal/rpccontract/contribution_test.go`
- Modify: `internal/rpc/protocol.go`

**Interfaces:**
- Consumes: existing `channelcontract.Selection`, `rpccontract.Error`, and core RPC constants.
- Produces: `Selection.ProcessAvailable bool`; `channelcontract.ErrInvalidSettings` plus `MarkInvalidSettings(error) error`; shared `rpccontract.ParseError`, `InvalidRequest`, `MethodNotFound`, `InvalidParams`, `InternalError`, `ServerOverload`, `CodeNotFound`, and `CodeConflict` constants; source-compatible aliases in `rpc`.

- [ ] **Step 1: Write failing contract tests**

Add tests that require the process decision and shared error codes:

```go
func TestSelectionCarriesProcessAvailability(t *testing.T) {
	selection := channelcontract.Selection{ProcessAvailable: true}
	if !selection.ProcessAvailable {
		t.Fatal("ProcessAvailable = false, want true")
	}
}

func TestInvalidSettingsMarkerPreservesMessageAndIdentity(t *testing.T) {
	want := errors.New("allow_from wildcard is forbidden")
	got := channelcontract.MarkInvalidSettings(want)
	if got.Error() != want.Error() || !errors.Is(got, channelcontract.ErrInvalidSettings) {
		t.Fatalf("marked error = %v", got)
	}
}

func TestContributionErrorsUseSharedProtocolCodes(t *testing.T) {
	if InvalidParams != -32602 || MethodNotFound != -32601 || CodeNotFound != -32004 || CodeConflict != -32009 {
		t.Fatalf("unexpected shared RPC codes: %d %d %d %d", InvalidParams, MethodNotFound, CodeNotFound, CodeConflict)
	}
}
```

- [ ] **Step 2: Run the focused tests and confirm RED**

Run:

```bash
go test ./internal/channelcontract ./internal/rpccontract -run 'TestSelectionCarriesProcessAvailability|TestContributionErrorsUseSharedProtocolCodes' -count=1
```

Expected: compile failure because the field and shared constants do not exist.

- [ ] **Step 3: Add the minimal contract fields and constants**

Add to `Selection`:

```go
type Selection struct {
	Providers        []channel.ChannelProvider
	Grants           map[string][]module.GrantBinding
	Config           Config
	ProcessAvailable bool
}
```

Add an implementation-free invalid-settings marker whose `Error` method
returns the wrapped cause text unchanged, whose `Unwrap` exposes the cause,
and whose `Is` method matches `ErrInvalidSettings`. `MarkInvalidSettings(nil)`
returns nil. This lets the app adapter classify its validation error without
leaking the app settings package into the Module.

Move the protocol codes into `rpccontract/protocol.go` with their existing
numeric values, including the two control codes needed by Channel management.
Replace definitions in `rpc` with aliases:

```go
const (
	ParseError     = rpccontract.ParseError
	InvalidRequest = rpccontract.InvalidRequest
	MethodNotFound = rpccontract.MethodNotFound
	InvalidParams  = rpccontract.InvalidParams
	InternalError  = rpccontract.InternalError
	ServerOverload = rpccontract.ServerOverload
	CodeNotFound   = rpccontract.CodeNotFound
	CodeConflict   = rpccontract.CodeConflict
)
```

Leave `CodeBadGateway` and `ModuleActionMethod` in `internal/rpc` because they
are not part of the generic contributed-handler contract.

- [ ] **Step 4: Run contract and RPC regression suites**

Run:

```bash
go test ./internal/channelcontract ./internal/rpccontract ./internal/rpc -count=1
```

Expected: PASS with no wire-value changes.

- [ ] **Step 5: Commit the contract correction**

```bash
git add internal/channelcontract internal/rpccontract internal/rpc/protocol.go internal/rpc/control.go
git commit -m "feat(channel): complete backend owner contracts"
```

### Task 2: Implement the canonical Channel Factory and Owned lifecycle

**Files:**
- Create: `internal/modules/channel/module.go`
- Create: `internal/modules/channel/module_test.go`
- Create: `internal/modules/channel/binding.go`
- Create: `internal/modules/channel/binding_test.go`
- Modify: `internal/modules/authority_conformance_test.go`
- Create: `internal/config/opaque.go`
- Create: `internal/config/opaque_test.go`
- Modify: `internal/channelhost/channelenv.go`
- Read/migrate from: `internal/app/channels.go`
- Read/migrate tests from: `internal/app/channels_test.go`

**Interfaces:**
- Consumes: `channelcontract.Factory`, `channelcontract.Dependencies`, `channelcontract.Selection`, existing `channelhost.New`, and the public Channel Provider ABI.
- Produces: `func NewFactory() channelcontract.Factory`; an Owned instance satisfying every embedded contract; package-private `bindProviders` and grant facade; `config.OpaqueYAMLToJSON(yaml.Node) (json.RawMessage, error)` for the composition adapter.

- [ ] **Step 1: Write failing Factory/Owned tests**

Create compile assertions and behavior tests:

```go
var _ channelcontract.Factory = NewFactory()
var _ channelcontract.Owned = (*owned)(nil)

func TestConstructDoesNotStartProviders(t *testing.T) {
	provider := &providerProbe{}
	instance, err := NewFactory().Construct(context.Background(), validDeps(t), channelcontract.Selection{
		Providers: []channel.ChannelProvider{provider},
		Config: channelcontract.Config{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		ProcessAvailable: true,
	})
	if err != nil { t.Fatal(err) }
	if provider.starts != 0 { t.Fatalf("starts = %d, want 0", provider.starts) }
	if err := instance.Close(context.Background()); err != nil { t.Fatal(err) }
}

func TestUnavailableProcessKeepsCompiledInventoryWithoutStart(t *testing.T) {
	provider := &providerProbe{}
	instance, err := NewFactory().Construct(context.Background(), validDeps(t), channelcontract.Selection{
		Providers: []channel.ChannelProvider{provider}, ProcessAvailable: false,
	})
	if err != nil { t.Fatal(err) }
	if err := instance.Start(context.Background()); err != nil { t.Fatal(err) }
	state := instance.Inspect()
	if !state.Compiled || state.ProcessAvailable || len(state.Providers) != 1 || provider.starts != 0 {
		t.Fatalf("state=%+v starts=%d", state, provider.starts)
	}
}
```

Also port the existing duplicate Provider name, unknown configured Provider,
secret grant, network grant, inbound grant, and defensive grant-copy tests
from `internal/app/channels_test.go` into `binding_test.go`.

- [ ] **Step 2: Run the new package tests and confirm RED**

Run:

```bash
go test ./internal/modules/channel -count=1
```

Expected: package or symbols missing.

- [ ] **Step 3: Move Provider binding behind the Module boundary**

Create `binding.go` with these private entry points and move the existing
implementations without changing grant semantics:

```go
func bindProviders(
	providers []channel.ChannelProvider,
	grants map[string][]module.GrantBinding,
	configured channelcontract.Config,
) ([]channel.Channel, error)

type providerChannel struct {
	name string
	provider channel.ChannelProvider
	grants []module.GrantBinding
	instance channel.Instance
	maxRunes int
}

type grantedChannelHost struct {
	channel.Host
	moduleID string
	grants []module.GrantBinding
}
```

The configured-name check must use `channelcontract.Config`; the private Host
conversion occurs only after binding succeeds. Copy every constraints slice
and map exactly as the current app implementation does.

- [ ] **Step 4: Centralize the existing opaque YAML-to-JSON conversion**

Move the exact zero/null/object conversion now implemented by
`channelhost.settingsToJSON` into:

```go
func OpaqueYAMLToJSON(node yaml.Node) (json.RawMessage, error)
```

under `internal/config`. Keep the current tests for absent, null, nested,
sequence, scalar, and non-string-map-key inputs; point the private Host helper
at the shared function. This is serialization only: it must not validate or
interpret Provider keys.

- [ ] **Step 5: Implement Factory and Owned lifecycle**

Use this concrete shape:

```go
type factory struct{}

func NewFactory() channelcontract.Factory { return factory{} }

type owned struct {
	host *channelhost.Host
	config channelcontract.Config
	processAvailable bool
	startOnce sync.Once
	stopOnce sync.Once
	closeOnce sync.Once
	startErr error
	closeErr error
}

func (factory) Construct(ctx context.Context, deps channelcontract.Dependencies, selection channelcontract.Selection) (channelcontract.Owned, error)
func (o *owned) Start(context.Context) error
func (o *owned) Ready(context.Context) error
func (o *owned) Stop(context.Context) error
func (o *owned) Close(context.Context) error
func (o *owned) OnRunEvent(context.Context, domain.RunEvent)
func (o *owned) Deliver(context.Context, string, string, string) error
func (o *owned) Inspect() channelcontract.State
```

`Construct` rejects nil Journal/Messages/Sessions/Run and converts inert config
to a detached `config.Channels`. Convert each non-empty JSON settings value by
`json.Unmarshal` into `any` and `yaml.Node.Encode`; invalid JSON fails
construction before any Provider starts. `Start` skips the private Host when
`ProcessAvailable` is false; otherwise it calls `StartAll`. `Ready` returns the
recorded start error. `Stop` calls `StopAll` once. `Close` calls `Stop` and then
releases Module-owned references once. `OnRunEvent` and `Deliver` delegate to
the private Host.

Update the L0 authority source test so `channelhost.New(` is permitted only in
`internal/modules/channel/module.go` and remains forbidden in every other
internal Module. Keep the existing bans on second Service, policy, ActionHost,
and control-RPC construction unchanged.

Map private Host inspection to the frozen summary:

```go
channelcontract.ProviderState{
	Name: status.Name, Compiled: true, Configured: status.Configured,
	Running: status.Started, Error: status.Note,
}
```

- [ ] **Step 6: Run Module, Host, config serialization, and former app-binding tests**

Run:

```bash
go test ./internal/modules/channel ./internal/channelhost ./internal/config ./internal/app -run 'Test.*Channel|TestOpaqueYAML|TestWithoutEars' -count=1
```

Expected: Module/Host tests PASS. Existing app tests may remain until Task 6,
but no behavior test may be deleted without an equivalent Module test.

- [ ] **Step 7: Commit the real owner core**

```bash
git add internal/modules/channel internal/modules/authority_conformance_test.go internal/channelhost internal/config
git commit -m "feat(channel): implement canonical backend owner"
```

### Task 3: Move Channel management into typed contributions

**Files:**
- Create: `internal/modules/channel/management.go`
- Create: `internal/modules/channel/management_test.go`
- Modify: `internal/modules/channel/module.go`
- Read/migrate from: `internal/rpc/control.go:5011-5224`
- Read/migrate tests from: `internal/rpc/control_test.go` Channel test block

**Interfaces:**
- Consumes: `rpccontract.MethodBinding`, shared codes/Request/Error, `channelcontract.SettingsAccess`, private Host inspection, and Selection config.
- Produces: `func (o *owned) RPCBindings() []rpccontract.MethodBinding` with exactly three immutable bindings.

- [ ] **Step 1: Write failing contribution parity tests**

Port the existing RPC cases into the Module package and invoke bindings by
method name. Required assertions:

```go
func binding(t *testing.T, owner channelcontract.Owned, method string) rpccontract.HandlerFunc {
	t.Helper()
	for _, item := range owner.RPCBindings() {
		if item.Method == method { return item.Handler }
	}
	t.Fatalf("binding %q missing", method)
	return nil
}
```

Test `channel/inspect`, `channel/get`, and `channel/update` for:

- exact capability names (`channel.inspect`, `channel.get`, `channel.update`);
- stable JSON field names and non-nil empty `allow_from` arrays;
- unknown names (`CodeNotFound` for get, `InvalidParams` for update);
- overlay-only configuration;
- read-only and frozen conflicts;
- wildcard and invalid `token_env` validation from the settings adapter;
- no notification after failure; and
- exactly one notification after successful persistence.

- [ ] **Step 2: Confirm RED**

Run:

```bash
go test ./internal/modules/channel -run 'TestManagement|TestRPCBindings' -count=1
```

Expected: `RPCBindings` is missing or empty.

- [ ] **Step 3: Implement the exact contribution surface**

Return a fresh slice so consumers cannot mutate owner state:

```go
func (o *owned) RPCBindings() []rpccontract.MethodBinding {
	return []rpccontract.MethodBinding{
		{Method: "channel/inspect", Capability: "channel.inspect", Handler: o.inspectChannel},
		{Method: "channel/get", Capability: "channel.get", Handler: o.getChannel},
		{Method: "channel/update", Capability: "channel.update", Handler: o.updateChannel},
	}
}
```

Keep the current DTO JSON tags and messages. Decode parameters with
`json.Unmarshal`; malformed params return `InvalidParams` and
`"params must be a valid JSON object"`. Fold saved overlays over startup
config without exposing opaque Provider settings. Call `Settings.Update`
only after name and policy checks; call `OnSettingsChanged` only after a
successful update.

Map an error matching `channelcontract.ErrInvalidSettings` to `InvalidParams`
with its unchanged validation message. Map every other settings backend error
to `InternalError` with the existing generic message so paths or document
contents never cross RPC.

- [ ] **Step 4: Run Module and RPC compatibility tests**

Run:

```bash
go test ./internal/modules/channel ./internal/rpc -run 'TestManagement|TestRPCBindings|TestChannel' -count=1
```

Expected: new Module tests PASS; legacy RPC tests still pass before their
ownership assertions are migrated in Task 6.

- [ ] **Step 5: Commit management ownership**

```bash
git add internal/modules/channel
git commit -m "feat(channel): contribute module-owned management methods"
```

### Task 4: Add generic RPC contribution attachment

**Files:**
- Create: `internal/rpc/contribution_dispatch.go`
- Create: `internal/rpc/contribution_dispatch_test.go`
- Modify: `internal/rpc/control.go`
- Modify: `internal/rpc/control_test.go`

**Interfaces:**
- Consumes: `[]rpccontract.Contribution` on `ControlDeps`.
- Produces: immutable `map[string]rpccontract.HandlerFunc` and sorted unique contributed capabilities inside `controlHandler`.

- [ ] **Step 1: Write failing dispatcher tests**

Add a fake contribution and verify construction, dispatch, context, Peer, and
capabilities:

```go
type testContribution []rpccontract.MethodBinding
func (c testContribution) RPCBindings() []rpccontract.MethodBinding { return append([]rpccontract.MethodBinding(nil), c...) }

func TestControlHandlerDispatchesValidatedContribution(t *testing.T) {
	seenPeer := false
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.Contributions = []rpccontract.Contribution{testContribution{{
			Method: "test/ping", Capability: "test.ping",
			Handler: func(ctx context.Context, peer rpccontract.Peer, request Request) (any, *Error) {
				seenPeer = peer != nil
				return map[string]string{"method": request.Method}, nil
			},
		}}}
	})
	got, rpcErr := callControl(t, env.handler, "test/ping", nil)
	if rpcErr != nil || got.(map[string]string)["method"] != "test/ping" || !seenPeer {
		t.Fatalf("got=%#v err=%v peer=%v", got, rpcErr, seenPeer)
	}
}
```

Also test empty contribution, duplicate contributed method, repeated
capability, collision with `initialize`, collision with compatibility alias
`attachment/resolve`, and deterministic diagnostics.

- [ ] **Step 2: Confirm RED**

Run:

```bash
go test ./internal/rpc -run 'TestControlHandler.*Contribution|TestCoreMethodVocabulary' -count=1
```

Expected: `ControlDeps.Contributions` and dispatch behavior are missing.

- [ ] **Step 3: Build the immutable contribution index**

Implement:

```go
type contributionDispatch struct {
	handlers map[string]rpccontract.HandlerFunc
	capabilities []string
}

func newContributionDispatch(contributions []rpccontract.Contribution) (contributionDispatch, error)
```

Flatten detached binding slices, call
`rpccontract.ValidateMethodBindings(coreMethodNames, bindings)`, build the map,
de-duplicate capabilities, and sort capabilities. Nil contributions and empty
binding slices are valid.

Add to `ControlDeps`:

```go
Contributions []rpccontract.Contribution
```

Construct the index once in `NewControlHandler`; return a wrapped composition
error on failure. Append contributed capabilities to initialize/capabilities.
In the switch default, look up the exact contributed method before the fixed
namespace rejection and standard `MethodNotFound` response.

- [ ] **Step 4: Pin the complete core method vocabulary**

Add `coreMethodNames` containing every literal handled by the core switch,
including aliases. Add a source-AST test that walks `controlHandler.Handle`,
collects every string literal in switch case expressions, resolves the sole
identifier case `ModuleActionMethod` to its constant value, and compares that
set to the map. Reject any other non-literal case expression in the test.
Exclude only the `default` contribution lookup because it has no case literal.

The equality assertion must report sorted `missing` and `extra` lists. This
ensures a future core case cannot accidentally become an accepted but
unreachable contribution collision.

- [ ] **Step 5: Run RPC and protocol suites**

Run:

```bash
go test ./internal/rpc ./internal/rpccontract -count=1
```

Expected: PASS; without contributions, all prior non-Channel behavior and
capability ordering remain unchanged.

- [ ] **Step 6: Commit generic attachment**

```bash
git add internal/rpc/contribution_dispatch.go internal/rpc/contribution_dispatch_test.go internal/rpc/control.go internal/rpc/control_test.go
git commit -m "feat(rpc): attach validated module contributions"
```

### Task 5: Emit the selected Channel Factory from generated Assembly

**Files:**
- Modify: `internal/modules/defaults/catalog.go`
- Modify: `internal/modules/defaults/catalog_test.go`
- Modify: `internal/modules/defaults/constructors.go`
- Modify: `sdk/internal/assembly/source.go`
- Modify: `sdk/internal/assembly/generate.go`
- Modify: `sdk/internal/assembly/minimal_internal_test.go`
- Modify: `sdk/internal/assembly/runtime_generate.go`
- Modify: `sdk/internal/assembly/runtime_generate_test.go`
- Modify: `sdk/internal/cmd/generate-default/main.go`
- Modify: `sdk/internal/frontend_v1.go`
- Modify: `sdk/internal/removal_conformance_test.go`
- Regenerate: `internal/generated/assembly/zz_default.go`

**Interfaces:**
- Consumes: `channelmodule.NewFactory()` and the compiler-selected canonical `core/channel-host@v1` owner.
- Produces: `RuntimeAssembly.ChannelFactory channelcontract.Factory`; build-owned `GoBinding.ChannelFactoryConstructor string`.

- [ ] **Step 1: Write failing catalog and generator tests**

Require the default record to use the transitional build-owned defaults bridge
and specialized factory constructor:

```go
if record.Binding.ImportPath != "agent-vivy/internal/modules/defaults" ||
	record.Binding.Package != "defaults" ||
	record.Binding.ChannelFactoryConstructor != "NewChannelFactory" {
	t.Fatalf("channel host binding = %+v", record.Binding)
}
```

Generate a plan containing a canonical Host binding and assert the output:

```go
mustContain(t, generated, "ChannelFactory channelcontract.Factory")
mustContain(t, generated, "ChannelFactory: defaults.NewChannelFactory()")
mustNotContain(t, generated, "defaults.NewChannelHost().Construct")
```

Also assert a non-Channel Module with an ordinary constructor is still emitted
through the existing lifecycle path. Generate a plan with no Channel Host and
assert the runtime type still contains `ChannelFactory
channelcontract.Factory`, its Build function leaves the field nil, and no
Channel factory call is emitted. The contract-shaped nil seam is required for
app compilation; it is not physical-omission evidence.

- [ ] **Step 2: Confirm RED**

Run:

```bash
go test ./internal/modules/defaults ./sdk/internal/assembly -run 'Test.*Channel.*Factory|TestGenerateRuntimeAssembly' -count=1
```

Expected: missing binding field/factory output.

- [ ] **Step 3: Add the focused build binding**

Add `ChannelFactoryConstructor string` to both internal default Binding and
`assembly.GoBinding`; copy it in `generate-default`, frontend source-record
construction, and minimal internal test setup.

Change the canonical catalog record to a specialized factory binding with no
ordinary lifecycle constructor:

```go
factoryRecord(
	"vivy/channel-host",
	"NewChannelFactory",
	source,
	port("core/channel-host@v1", "vivy.channel-host"),
)
```

`factoryRecord` sets `ChannelFactoryConstructor` and leaves `Constructor`
empty. Update source-binding validation so an empty ordinary Constructor is
legal only when the record is the canonical build-owned Channel Host and has
the specialized factory constructor. Every other selected Module continues to
require its current lifecycle constructor.

Add the transitional bridge in `defaults/constructors.go`:

```go
func NewChannelFactory() channelcontract.Factory {
	return channelmodule.NewFactory()
}
```

This deliberate P0-2 bridge keeps the implementation linked through the
existing defaults package even in other generated shapes, so this slice does
not claim physical omission. P0-3 removes the bridge and makes the concrete
Module import genuinely selection-controlled.

- [ ] **Step 4: Generate the factory field and skip placeholder lifecycle**

When the canonical selected record has `ChannelFactoryConstructor`, import
`internal/channelcontract`, emit the field and initializer through the
defaults bridge, and skip its
ordinary owner construction in `RuntimeAssembly.Start`. Reject either of these
invalid plans:

- factory constructor on a Module that does not provide
  `core/channel-host@v1`; or
- selected canonical Host without the build-owned factory constructor.

Keep the `ChannelFactory` contract field in the generated runtime type. P0-2
does not add a reduced Recipe or prove concrete import omission.

- [ ] **Step 5: Regenerate and verify the checked-in default**

Run:

```bash
go run ./sdk/internal/cmd/generate-default -output internal/generated/assembly/zz_default.go
gofmt -w internal/generated/assembly/zz_default.go
go test ./internal/modules/defaults ./sdk/internal/assembly ./internal/generated/assembly -count=1
```

Expected: one `channelmodule.NewFactory()` initializer and no
`defaults.NewChannelHost().Construct` call.

- [ ] **Step 6: Commit generated factory selection**

```bash
git add internal/modules/defaults sdk/internal/assembly sdk/internal/cmd/generate-default/main.go sdk/internal/frontend_v1.go internal/generated/assembly/zz_default.go
git commit -m "feat(assembly): emit canonical channel factory"
```

### Task 6: Migrate app composition and remove RPC/app Channel ownership

**Files:**
- Create: `internal/app/channel_settings.go`
- Create: `internal/app/channel_settings_test.go`
- Modify: `internal/app/app.go`
- Modify: `internal/app/app_test.go`
- Modify: `internal/app/assembly_authority_test.go`
- Delete or reduce: `internal/app/channels.go`
- Delete or migrate: `internal/app/channels_test.go`
- Modify: `internal/rpc/control.go`
- Modify: `internal/rpc/control_test.go`

**Interfaces:**
- Consumes: generated `ChannelFactory`, `channelcontract.Dependencies`, `Selection`, `Owned`, and generic `ControlDeps.Contributions`.
- Produces: app composition with no concrete Host or Channel management knowledge.

- [ ] **Step 1: Write failing app settings-adapter tests**

Implement tests against a temporary shared settings document:

```go
func TestChannelSettingsAccessPreservesUnrelatedDocumentFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	enabled := true
	_, err := settings.Save(path, settings.Settings{
		Locale: "zh-CN",
		Channels: []settings.ChannelOverlay{{Name: "dormant", Enabled: &enabled}},
	})
	if err != nil { t.Fatal(err) }

	access := newChannelSettingsAccess(path, false)
	allow := []string{"alice"}
	_, err = access.Update(context.Background(), channelcontract.ChannelOverlay{Name: "active", AllowFrom: &allow})
	if err != nil { t.Fatal(err) }
	doc, err := settings.Load(path)
	if err != nil { t.Fatal(err) }
	if doc.Locale != "zh-CN" || len(doc.Channels) != 2 { t.Fatalf("document = %+v", doc) }
}
```

Also test `Writable` for empty/non-empty path, `Frozen`, read-only Read returning
an empty projection, pointer-field preservation, validation errors, and
context cancellation before I/O.

- [ ] **Step 2: Confirm adapter RED**

Run:

```bash
go test ./internal/app -run 'TestChannelSettingsAccess' -count=1
```

Expected: adapter missing.

- [ ] **Step 3: Implement the focused settings adapter**

Use:

```go
type channelSettingsAccess struct {
	path string
	frozen bool
}

func newChannelSettingsAccess(path string, frozen bool) channelcontract.SettingsAccess
func (a channelSettingsAccess) Read(context.Context) (channelcontract.Settings, error)
func (a channelSettingsAccess) Update(context.Context, channelcontract.ChannelOverlay) (channelcontract.Settings, error)
func (a channelSettingsAccess) Writable() bool
func (a channelSettingsAccess) Frozen() bool
func channelSelectionConfig(config.Channels) (channelcontract.Config, error)
```

`Update` must call the existing `settings.Update(path, fn)` and
`Settings.UpsertChannelOverlay`; convert only the Channel projection on return.
Check `ctx.Err()` immediately before Load/Update. Do not add another settings
manager or write a Channel-only file. If `settings.IsValidationError(err)` is
true, return `channelcontract.MarkInvalidSettings(err)`; return I/O errors
unchanged.

`channelSelectionConfig` copies every envelope field, calls
`config.OpaqueYAMLToJSON` for the opaque settings node, and clones every slice
and `json.RawMessage`. A conversion failure aborts app composition with the
Channel name in the wrapped error; no malformed settings silently become an
empty object at this boundary.

- [ ] **Step 4: Write failing composition/lifecycle tests**

Add a fake `channelcontract.Factory`/Owned through a generated Assembly test
fixture and assert:

- Construct receives all Providers/grants/config and the expected
  `ProcessAvailable` value;
- `WithoutEars` still constructs but never starts the owner;
- normal composition starts after recovery and before the app is returned;
- the owner is supplied as Service `RunHook` and `ChannelDeliverer`;
- its RPC contribution is supplied to the control handler;
- a Start failure closes the owner once; and
- App.Close stops/closes it before the backend and generated Assembly.

Use an ordered string probe such as:

```go
type lifecycleProbe struct { events []string }
func (p *lifecycleProbe) add(event string) { p.events = append(p.events, event) }
```

Expected normal tail:

```go
[]string{"channel.stop", "channel.close", "assembly.close", "storage.close"}
```

- [ ] **Step 5: Confirm composition RED**

Run:

```bash
go test ./internal/app -run 'TestChannelOwner|TestWithoutEars|TestAppClose.*Channel' -count=1
```

Expected: app still constructs and owns concrete `channelhost.Host`.

- [ ] **Step 6: Replace concrete composition with Factory/Owned**

In `app.go`:

```go
var channelOwner channelcontract.Owned
if runtimeAssembly.ChannelFactory != nil {
	selectionConfig, selectionErr := channelSelectionConfig(cfg.Channels)
	if selectionErr != nil { return nil, selectionErr }
	channelOwner, err = runtimeAssembly.ChannelFactory.Construct(ctx, channelcontract.Dependencies{
		Journal: backend, Messages: backend, Sessions: backend,
		Credentials: credentialResolver, Settings: newChannelSettingsAccess(liveSettingsPath, resolver.Frozen()),
		Logger: logger,
		Run: func(ctx context.Context, sessionID domain.SessionID, text string, prov *domain.Provenance) (domain.RunID, error) {
			if svc == nil { return "", errors.New("app: runtime service is not wired") }
			return svc.RunWithOptions(ctx, sessionID, text, runtime.RunOptions{Provenance: prov})
		},
		OnSettingsChanged: func() {
			if onSettingsChanged != nil { onSettingsChanged() }
		},
	}, channelcontract.Selection{
		Providers: runtimeAssembly.Channels,
		Grants: runtimeAssembly.ChannelGrants,
		Config: selectionConfig,
		ProcessAvailable: ao.channels,
	})
}
```

Use `resolver.Frozen()` as the single existing frozen-session decision. Declare
`var onSettingsChanged func()` before Channel construction, pass the guarded
closure shown above, then assign the existing `ControlDeps.OnSettingsChanged`
body to `onSettingsChanged` after `svc` exists and reuse that same function in
`ControlDeps`. Do not duplicate the live-reload logic. Append `channelOwner` to
Run hooks, pass it to Service channels, and pass
`[]rpccontract.Contribution{channelOwner}` to `ControlDeps` when non-nil.

After recovery call `channelOwner.Start` followed by `channelOwner.Ready`; if
either fails, call Stop/Close during rollback. Store `channelcontract.Owned` on
`App`. Close it before Service/storage and do not separately call private Host
methods.

- [ ] **Step 7: Delete old ownership and migrate assertions**

Remove from `internal/rpc/control.go`:

- `internal/channelhost`, `internal/config`, and Channel-only settings imports;
- `ControlDeps.Channels` and `ConfigChannels`;
- Channel DTOs, conversion helpers, and three handlers;
- hard-coded `channel.*` capabilities; and
- the three core switch cases.

Remove Provider binding/facade code from `internal/app/channels.go`; delete the
file if no app-owned code remains. Move every still-relevant test to Module,
generic RPC, or settings-adapter suites. Update `WithoutEars` comments to say
the compiled owner remains but process networking is unavailable.

- [ ] **Step 8: Run ownership and behavior suites**

Run:

```bash
go test ./internal/modules/channel ./internal/channelhost ./internal/rpc ./internal/app ./internal/runtime ./internal/generated/assembly -count=1
! rg -n 'internal/(channelhost|modules/channel)' internal/app internal/rpc --glob '*.go'
```

Expected: PASS and empty forbidden-import output.

- [ ] **Step 9: Commit the ownership migration**

```bash
git add internal/app internal/rpc
git commit -m "refactor(channel): compose backend through module owner"
```

### Task 7: Close conformance, documentation, and release gates

**Files:**
- Modify: `docs/architecture/CHANNEL-MODULARIZATION.md`
- Create: `docs/logs/2026-09-19-channel-p0-2/summary.md`
- Create: `docs/logs/2026-09-19-channel-p0-2/verification.md`
- Create: `docs/logs/2026-09-19-channel-p0-2/acceptance.md`
- Modify: `sdk/internal/assembly/conformance_results.json`
- Modify: focused conformance tests if ownership assertions require updates

**Interfaces:**
- Consumes: completed Tasks 1-6.
- Produces: reviewable CH-P0-2 evidence without claiming CH-P0-3 through CH-P0-5.

- [ ] **Step 1: Add explicit scope-fence tests/checks**

Record and run:

```bash
for path in \
  recipes/web-no-channels.vivy.yml \
  recipes/web-channels-no-ui.vivy.yml \
  recipes/web-telegram-only.vivy.yml; do
  test ! -e "$path"
done

test -z "$(git diff 22acb698 -- ui/src/components/settings ui/src/generated/assembly.ts recipes)"
! rg -n 'internal/(channelhost|modules/channel)' internal/app internal/rpc --glob '*.go'
```

Expected: PASS. The generated backend is allowed to import the concrete Module
in P0-2; UI/generated UI and Recipes are not.

- [ ] **Step 2: Run focused Go verification**

Run:

```bash
go test ./internal/channelcontract ./internal/rpccontract ./internal/modules/channel ./internal/channelhost ./internal/rpc ./internal/app/settings ./internal/app ./internal/moduleport ./internal/modules/... ./sdk/internal/assembly -count=1
go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui
```

Expected: PASS.

- [ ] **Step 3: Run UI and i18n compatibility gates**

Run the repository's existing commands recorded in the CH-P0-1 verification:

```bash
pnpm --dir ui typecheck
pnpm --dir ui test
pnpm --dir ui build
```

Run both repository i18n completeness and cross-face scripts. Expected: PASS
with no new Channel UI behavior or copy.

- [ ] **Step 4: Refresh source-bound conformance evidence**

First run the producer suite. If the only failure is the internal source digest,
compute the new digest with:

```bash
go run ./sdk/internal/cmd/source-hash internal ""
```

Mechanically replace only the internal-rooted entries whose executed suites
match, then rerun:

```bash
go test ./sdk/internal/conformance -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1
```

Expected: PASS. Do not change external Provider evidence without an executed
producer result requiring it.

- [ ] **Step 5: Run full repository and independent module gates**

Run:

```bash
packages=$(go list ./... | rg -v '^agent-vivy/internal/workflow($|/)')
go vet $packages
go test -timeout 20m $packages
```

Then, for every independent module under `plugins/*` and `faces/*`, run:

```bash
go vet ./...
go test ./...
```

Expected: PASS. Record the existing `internal/workflow` carve-out and the
environment limitation that literal `just ci` is unavailable; do not claim a
literal `just ci` pass.

- [ ] **Step 6: Update architecture and evidence**

Mark only CH-P0-2 ownership/wiring/management relocation complete. Document:

- final factory and contribution interfaces;
- lifecycle and settings authority evidence;
- before/after forbidden dependency scans;
- exact commands and outputs;
- default-generation compatibility; and
- the explicit P0-3/P0-4/P0-5 exclusions.

No document may claim physical executable or UI asset omission.

- [ ] **Step 7: Run final diff and scope review**

Run:

```bash
git diff --check
git status --short
git diff --stat 22acb698
git diff 22acb698 -- recipes ui/src/components/settings ui/src/generated/assembly.ts
```

Expected: clean whitespace; no scope-fence diff; only planned files changed.

- [ ] **Step 8: Commit CH-P0-2 evidence**

```bash
git add docs/architecture/CHANNEL-MODULARIZATION.md docs/logs/2026-09-19-channel-p0-2 sdk/internal/assembly/conformance_results.json
git commit -m "docs(channel): record p0 backend owner evidence"
```

- [ ] **Step 9: Request whole-branch review**

Review from `22acb6982d1d4abce948acf08a8575fb98d5669f` through HEAD for:

- real ownership rather than a wrapper-only migration;
- default behavior and wire compatibility;
- lifecycle/rollback correctness;
- generic dispatcher safety;
- dependency direction; and
- absence of P0-3 through P0-5 work.

Fix every Critical/Important finding in one bounded pass, rerun affected tests,
and record any deferred Minor finding with its cost.
