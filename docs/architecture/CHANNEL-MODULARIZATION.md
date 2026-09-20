# Channel Modularization Contract

> Status: **Normative architecture; CH-P0-2 backend ownership complete**
> Decision date: 2026-09-19  
> Implementation baseline: `main@9e6db43c81e1d7ce249ee785f1b6efd91a09c5bb`  
> Parent decision: [issue #42](https://github.com/ProjectViVy/agent-vivy/issues/42)

## 1. Outcome and scope

Channel is a removable, build-owned internal subsystem. A later Generation may
omit the Channel Host, every Channel Provider, its management methods, and its
dedicated UI without affecting ordinary chat, tools, or retained historical
data. Backend Channel operation does not require Channel UI.

CH-P0-1 froze the contracts and conformance cases. CH-P0-2 moves backend
ownership behind those contracts while preserving the default Generation.
Physical omission and UI modularization remain later slices.

### Requirements

- **CH-01 — canonical T1 owner:** `vivy/channel-host` is the sole build-owned
  T1 provider of `core/channel-host@v1`, cardinality `0..1`.
- **CH-02 — conditional Consumer:** every selected `std/channel@v1` Provider
  requires the canonical Channel Host. A Provider without it is a compile
  error; a selected Host with zero Providers is valid.
- **CH-03 — one composition seam:** generated Assembly and the app compose
  Channel only through the internal factory/owned-instance contract.
- **CH-04 — unchanged runtime authorities:** admitted inbound work enters the
  existing `Service.RunWithOptions` path. Channel does not create a Service,
  Journal, policy engine, storage authority, or credential authority.
- **CH-05 — typed management contribution:** management methods attach through
  one generic, typed, build-owned RPC contribution. Public Providers never
  receive raw dispatcher registration.
- **CH-06 — honest absence:** without a management contribution, Channel
  methods return the standard `MethodNotFound` result and `channel.*`
  capability names are absent.
- **CH-07 — distinct states:** Inspect distinguishes code compiled into the
  Generation, process availability, configuration, and provider runtime
  state. `WithoutEars()` may disable process availability without falsifying
  compiled inventory.
- **CH-08 — lossless dormant settings:** configuration for an omitted Provider
  is inert, retained, and preserved by unrelated writes. Reintroduction
  validates it before activation.
- **CH-09 — imports are inclusion authority:** generated backend imports and UI
  asset composition, derived from the Recipe, determine physical inclusion.
  A nil value, hidden tab, empty Provider slice, or metadata row is not
  omission evidence.
- **CH-10 — generic UI projection:** `initialize`/`capabilities` may expose a
  secret-free `ui_extensions` projection. It reports generated selection,
  effective configuration, and process requirements; it never selects or
  loads code.
- **CH-11 — lifecycle safety:** startup rolls back in reverse dependency order.
  `Stop` and `Close` are idempotent, bounded, and release registrations and
  workers once.
- **CH-12 — stable default:** the default Generation retains all existing
  first-party Channel Providers and behavior throughout staged delivery.

## 2. Verified current implementation

After CH-P0-2:

- generated Assembly exposes one `channelcontract.Factory` for the selected
  canonical `vivy/channel-host` owner;
- `internal/modules/channel` constructs and owns the private
  `internal/channelhost.Host`, Provider binding, Grant facades, lifecycle,
  inspection, and `channel/inspect|get|update` handlers;
- `internal/app` constructs, starts, and closes only
  `channelcontract.Owned`; it retains the single Run callback and shared
  settings-document adapter but no concrete Channel Host knowledge;
- `internal/rpc` validates and dispatches generic `rpccontract.Contribution`
  bindings and contains no Channel-specific method, DTO, capability, or Host
  import;
- `WithoutEars()` constructs the compiled owner with process availability
  false and does not start Provider networking; and
- the default Generation still carries all five first-party Providers and the
  Web settings shell still imports Channel UI statically.

The last point is deliberate: CH-P0-2 establishes backend ownership but does
not claim generated import, executable, or UI-asset omission.

## 3. Boundary and dependency direction

```text
app + generated Assembly -> channel composition contracts <- Channel module
Channel module -> existing runtime/storage/credential/RPC contracts
RPC dispatcher -> implementation-free RPC protocol/contribution contracts
Channel Provider -> std/channel@v1 -> canonical Channel Host
```

The internal composition contract contains only inert configuration/state,
focused dependencies, a factory, and an owned instance. It must not import
`internal/channelhost`, a platform Provider, or a platform SDK. The owned
instance implements the existing `module.Instance`, `runtime.RunHook`, and
`runtime.ChannelDeliverer` contracts and contributes its management handlers.

The generic RPC contribution binds an exact method name and capability name to
an implementation-free handler contract in `internal/rpccontract`. Its
`Request` and `Error` are also the dispatcher protocol types; its typed `Peer`
view preserves the existing connection operations while authenticated caller
and identity remain server-attested in `context.Context`. `internal/rpc`
re-exports the contribution names and compile-checks that `*rpc.Peer` satisfies
that view. Validation rejects empty bindings, duplicate methods, and
core-method replacement. It is not a runtime-discovered or public registry.

The Channel construction dependencies include a focused settings authority:
read the current Channel overlay projection, atomically upsert one overlay,
report writable/frozen policy, and notify the app after a successful write.
It does not expose the settings path or the rest of the shared settings
document. This is sufficient for `channel/get` and `channel/update` to retain
their present policy and persistence semantics when ownership moves.

## 4. Ownership and lifecycle

The selected canonical Module owns construction, Provider binding,
Grant admission, session mapping, inbound dispatch, terminal delivery,
Channel-specific configuration interpretation, management handlers, and
lifecycle. Platform Providers keep their present source boundaries and public
`std/channel@v1` ABI.

Construction performs no networking. The app constructs the selected owner
before Service composition, supplies it as the run hook and delivery seam,
then starts and readies it after durable recovery and before the public control
plane is returned. Shutdown closes admission and network ingress before
dependent runtime/storage teardown. Partial startup invokes contract-level
Stop/Close rollback in reverse dependency order.

The Host-selected/zero-Provider state is legal and starts no connection.
Missing configuration or credentials leaves a Provider inactive. Existing
per-Provider failure behavior is preserved during ownership migration.

## 5. Configuration and absence behavior

Recipe selection is the only build authority. Runtime configuration can
activate only compiled capabilities. Existing `channels.<platform>` keys,
token environment references, allowlists, opaque settings, and overlay
precedence remain wire/storage compatible; no global Channel enable flag is
added.

An enabled configuration for an unavailable Provider fails startup with a
clear diagnostic. A disabled entry for an unavailable Provider may remain
inert with a diagnostic. An unrelated settings update cannot delete omitted
Provider entries. Unknown or malformed active configuration never activates
silently.

With no Host there is no adapter construction, listener, worker, inbound hook,
delivery implementation, or management contribution. Historical Channel
messages remain generic Journal data and stay readable.

## 6. UI projection contract

The generic projection has this wire shape:

```json
{"ui_extensions":[{"id":"vivy/channel-ui","enabled":true}]}
```

It contains no credentials, Provider settings, or authorization decisions.
Omission remains backward compatible. CH-P0-1 defines and tests the shape but
does not populate it, negotiate it, move Channel UI, or add settings-section
composition. Those changes belong to CH-P0-4.

## 7. Failure model and rollback

- missing conditional Host, duplicate Host, or a public `core/*` Provider:
  Assembly compile failure;
- duplicate or core-colliding RPC method contribution: composition failure;
- absent management contribution: normal JSON-RPC `MethodNotFound` and no
  advertised Channel capability;
- enabled unavailable Provider: explicit startup failure;
- partial start: reverse-order cleanup without double stop/close; and
- failed staged release: restore one coherent previous backend/UI Generation
  while retaining configuration and historical data.

No destructive schema migration is required.

## 8. Performance and economy

The design adds no request-time discovery, reflection, dynamic code loading,
second dispatcher, or second runtime. Generated wiring and a small validated
binding slice are sufficient. Normal Channel traffic continues through the
existing Host and Service paths. CH-P0-2 changes composition ownership only;
performance measurements remain deferred until executable or asset
composition changes.

## 9. Verification seams

CH-P0-1 and CH-P0-2 evidence consists of:

1. compile-time satisfaction of the factory/owned-instance contract;
2. contract package dependency closure without Channel implementation,
   dispatcher implementation, or platform SDKs;
3. RPC contribution validation for empty, invalid, duplicate, and core
   collisions;
4. Assembly cases for zero Providers, Provider-without-Host, duplicate Host,
   and public core Provider;
5. lossless settings preservation for an omitted Provider; and
6. backend/frontend serialization of the optional UI projection;
7. canonical factory generation with no placeholder Channel owner lifecycle;
8. Module-owned Provider/Grant, management-wire, settings, lifecycle, and
   rollback suites;
9. generic contribution dispatch, capability, collision, and core-vocabulary
   conformance; and
10. dependency scans proving app/RPC no longer import the Channel Host or
    implementation Module.

Physical executable and asset omission is not claimed until CH-P0-3 through
CH-P0-5 produce source-bound artifact evidence.

## 10. Slice fence

- **CH-P0-1 — complete:** contracts, normative text, conformance cases,
  baseline evidence.
- **CH-P0-2 — complete:** real backend owner, binding/config/management
  relocation, generated factory, generic contribution dispatch, and app/RPC
  concrete ownership removal.
- **CH-P0-3:** conditional generated imports, reduced Recipes, backend physical
  omission and Inspect truth.
- **CH-P0-4:** Channel UI Module, settings contribution, visibility and asset
  composition.
- **CH-P0-5:** complete release matrix, packaged artifacts, browser/network
  evidence, rollback record.

Each completed slice must not perform work assigned to a later slice.

## 11. Eino decision

This change is composition and ownership only. It preserves the existing
callback into `Service.RunWithOptions` and adds no model, prompt, streaming,
context, tool, or orchestration capability. No Eino/EinoExt adapter is needed.
