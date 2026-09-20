# CH-P0-2 Backend Owner Design

> Status: proposed implementation design
> Parent decision: issue #42 and `docs/architecture/CHANNEL-MODULARIZATION.md`
> Depends on: CH-P0-1 at `22acb6982d1d4abce948acf08a8575fb98d5669f`

## 1. Outcome

CH-P0-2 makes `vivy/channel-host` the real owner of Channel backend behavior.
The application and RPC dispatcher stop importing or holding the concrete
`internal/channelhost.Host`; they compose Channel only through the
`internal/channelcontract` factory and owned-instance interfaces frozen by
CH-P0-1.

The default Generation must retain its current Provider set, settings wire
shape, RPC results, startup decisions, run path, delivery behavior, and
shutdown order.

## 2. Scope

### Included

- a canonical `internal/modules/channel` implementation of
  `channelcontract.Factory` and `channelcontract.Owned`;
- Provider construction, grant facades, Channel configuration adaptation,
  process inspection, and Channel management handlers owned by that Module;
- generic validated RPC contribution attachment in `internal/rpc`;
- generated Assembly exposure of the selected Channel factory;
- application composition through Factory/Owned only; and
- lifecycle, rollback, compatibility, and ownership conformance tests.

### Excluded

- conditional generated imports or a Generation without the Host;
- reduced Recipes or executable/source physical-omission evidence;
- Channel UI relocation, UI settings contributions, visibility rules, or
  `ui_extensions` population;
- Provider ABI changes, dynamic discovery, runtime loading, a second RPC
  dispatcher, or a second runtime Service; and
- deletion or wholesale relocation of the stable `internal/channelhost`
  pipeline implementation.

Those exclusions remain CH-P0-3 through CH-P0-5.

## 3. Chosen approach

Use a contract factory with an internal Host engine.

`internal/modules/channel` becomes the only composition owner. It constructs
and contains the existing `internal/channelhost.Host`, but that concrete type
is an implementation detail. The app, runtime, generated Assembly, and RPC
dispatcher receive only contract interfaces.

This is preferred over moving every Host source file because physical file
movement would add review surface without improving the ownership boundary.
It is preferred over a thin wrapper because Provider binding and management
handlers must actually leave app/RPC ownership in this slice.

```text
generated Assembly ──> channelcontract.Factory <── internal/modules/channel
        app ─────────> channelcontract.Owned
                            │
                            ├── runtime.RunHook / ChannelDeliverer
                            ├── rpccontract.Contribution
                            └── internal/channelhost (private engine)
```

## 4. Components and responsibilities

### 4.1 `internal/modules/channel`

The package owns:

- `NewFactory`, whose concrete value is selected by generated Assembly;
- `Factory.Construct`, which validates selection and creates no network work;
- `Owned`, which implements `module.Instance`, `runtime.RunHook`,
  `runtime.ChannelDeliverer`, `rpccontract.Contribution`, and `Inspect`;
- the current Provider-to-`channel.Channel` binding and grant-limited Host
  facade now located in `internal/app/channels.go`;
- conversion of inert `channelcontract.Config` into the private Host engine's
  effective configuration;
- conversion of private Host status into `channelcontract.State`; and
- `channel/inspect`, `channel/get`, and `channel/update` handlers and their
  wire DTOs.

The Module may import `internal/channelhost`, `internal/config` validation
helpers, and `internal/rpccontract`. It must not import `internal/app`,
`internal/rpc`, generated Assembly, or a platform Provider implementation.

### 4.2 Generated Assembly

`RuntimeAssembly` exposes one `channelcontract.Factory` selected from the
canonical `core/channel-host@v1` owner. In CH-P0-2 the generated type and the
default generated file still carry that contract field. A transitional
build-owned defaults constructor supplies the concrete Channel Module and
keeps it linked unconditionally, so this slice does not claim physical
omission. P0-3 removes that bridge, may leave the field nil, and makes the
concrete Module import Recipe-controlled without changing app composition.

The internal Go binding gains one focused Channel factory-constructor field.
The generator uses it only for the canonical `core/channel-host@v1` owner and
does not also emit that owner's legacy generic lifecycle constructor. Other
Modules keep the current `module.Module` generation path unchanged.

The late-bound Channel instance is not constructed by `RuntimeAssembly.Start`:
its storage, credentials, settings access, Providers, and Run callback do not
exist at that early lifecycle point. The app constructs it later through the
generated factory. The returned `Owned` instance still follows the standard
`module.Instance` lifecycle.

The existing no-op `defaults.NewChannelHost` owner is removed from the default
owner lifecycle once the real factory is emitted; there must never be two
owners for the canonical Host.

### 4.3 Application composition

The app performs only contract-level orchestration:

1. derive an inert `channelcontract.Selection` from generated Providers,
   grant bindings, effective Channel envelopes, and the process-availability
   decision;
2. create focused `Dependencies`, including storage, credentials, logger, a
   Run callback, and a settings adapter;
3. call the generated factory's `Construct` while the Run callback safely
   references the not-yet-bound Service;
4. attach `Owned` as the runtime Run hook and delivery interface;
5. build the Service and control plane;
6. recover durable runs;
7. call `Owned.Start` before the public server listens; and
8. stop and close `Owned` before dependent runtime/storage teardown.

`WithoutEars()` still creates the Module instance so compiled inventory stays
truthful, but sets an additive `Selection.ProcessAvailable` field false and
does not start Provider networking. Normal compositions set it true. This is
the sole planned additive correction to the P0-1 contract and preserves the
distinction between compiled and running.

The focused settings adapter maps between `internal/app/settings` and
contract-owned overlays. Its update uses the existing atomic shared-document
update, preserving unrelated fields, then invokes the existing change
notification exactly once after success.

### 4.4 Generic RPC attachment

`ControlDeps` accepts a slice of `rpccontract.Contribution` or prevalidated
bindings. `NewControlHandler` flattens and validates them once:

- invalid, duplicate, or core-colliding methods fail composition;
- bindings are stored in an immutable method map;
- capability names are de-duplicated and appended only when their binding is
  present; and
- the default branch dispatches an exact contributed method before returning
  standard `MethodNotFound`.

The core method vocabulary is centralized in `internal/rpc` and tested against
the dispatcher switch so collision validation cannot silently drift. The
dispatcher passes its real `*rpc.Peer`, request context, shared Request, and
shared Error directly to the contributed handler.

Channel names and logic never enter the generic dispatcher after migration.

## 5. Data and control flow

### Inbound message

Provider instance -> grant-limited Channel Host facade -> private Host engine
-> existing injected Run callback -> `Service.RunWithOptions`.

No alternate Service, policy engine, Journal, or session authority is created.

### Outbound message

Runtime `ChannelDeliverer` -> `channelcontract.Owned` -> private Host engine ->
the already-started generated Provider instance.

### Management request

RPC dispatcher -> immutable contributed binding -> Channel-owned handler ->
`Owned.Inspect` or focused settings access. A successful update persists one
overlay atomically, preserves the rest of the document, notifies the app once,
and still requires restart before process state changes.

## 6. Lifecycle and failures

- `Construct` validates duplicate/unknown Providers and required dependencies;
  it starts no goroutine, listener, or Provider.
- `Start` is idempotent. With process availability disabled it is a no-op;
  otherwise it delegates to the current fail-closed per-Provider startup.
- a Host-level wiring failure aborts app construction and triggers generic
  `Stop`/`Close` rollback;
- Provider-specific start failures retain their current recorded-state and
  continue-other-Providers behavior;
- `Stop` closes admission and stops started Providers in reverse order;
- `Close` is idempotent and releases remaining owned state once; and
- settings errors retain the current JSON-RPC codes and sanitized messages.

The app must not separately invoke concrete Host lifecycle methods.

## 7. Compatibility requirements

- The default manifest still contains `vivy/channel-host` and all five current
  first-party Channel Providers.
- Existing `channels.<name>` configuration, settings overlay, grant behavior,
  token environment handling, allowlists, session mapping, event provenance,
  delivery chunking, and RPC JSON remain byte/semantic compatible.
- `channel.*` capabilities appear only from the selected contribution.
- A handler without the contribution returns standard `MethodNotFound`.
- Headless/`WithoutEars` builds do not start Channel networking but do not lie
  about compiled inventory.

## 8. Verification

TDD work must provide:

1. Factory/Owned compile assertions and construction-without-network tests;
2. Provider binding/grant tests moved from app ownership to Module ownership;
3. Inspect and management-handler parity tests, including unknown Provider,
   read-only, frozen, validation failure, overlay-only add, and notification;
4. generic RPC contribution tests for attachment, capabilities, context/Peer
   preservation, duplicate/core collision, and absence;
5. generated Assembly tests proving exactly one canonical factory and no
   placeholder owner lifecycle;
6. app tests proving only contract imports, unchanged Run/delivery wiring,
   `WithoutEars`, startup rollback, and shutdown ordering;
7. dependency scans showing `internal/app` and `internal/rpc` no longer import
   `internal/channelhost` or Channel implementation code;
8. default-generation, headless, UI, i18n, full Go, and independent
   plugin/face conformance gates; and
9. an updated source-bound conformance digest after the producer suite passes.

CH-P0-2 does not claim physical omission. No reduced Recipe or binary/asset
absence check is an acceptance condition here.

## 9. Expected file boundary

Primary changes:

- create `internal/modules/channel/*`;
- modify `internal/channelcontract/*` only for a proven additive correction;
- modify `internal/rpc/control.go` and focused RPC tests;
- remove Channel-owned code from `internal/app/channels.go` and Channel-specific
  composition fields from app structs;
- add the focused app settings adapter;
- modify `sdk/internal/assembly/runtime_generate.go` and regenerate
  `internal/generated/assembly/zz_default.go`;
- replace `defaults.NewChannelHost` placeholder ownership; and
- update architecture/evidence documents and conformance digests.

Explicitly forbidden in this slice:

- new reduced Recipe files;
- conditional Channel imports or manifest omission;
- changes under `ui/src/components/settings` or generated UI assembly;
- platform Provider implementation changes except test-only compatibility
  fixes; and
- public `sdk/port/channel` ABI expansion.
