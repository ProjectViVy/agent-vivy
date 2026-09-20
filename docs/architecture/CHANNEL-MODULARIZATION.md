# Channel Modularization Contract

> Status: **Normative architecture; CH-P0-1 contract foundation**  
> Decision date: 2026-09-19  
> Implementation baseline: `main@9e6db43c81e1d7ce249ee785f1b6efd91a09c5bb`  
> Parent decision: [issue #42](https://github.com/ProjectViVy/agent-vivy/issues/42)

## 1. Outcome and scope

Channel is a removable, build-owned internal subsystem. A later Generation may
omit the Channel Host, every Channel Provider, its management methods, and its
dedicated UI without affecting ordinary chat, tools, or retained historical
data. Backend Channel operation does not require Channel UI.

CH-P0-1 freezes the contracts and conformance cases needed to implement that
outcome. It does not move ownership or make the subsystem removable yet.

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

At the baseline revision:

- `internal/app/app.go` calls `bindChannels`, constructs
  `*channelhost.Host`, injects its Run hook and delivery interface, passes it to
  RPC, and owns startup/shutdown;
- `internal/app/channels.go` owns Provider construction and Grant facades;
- `internal/modules/defaults/constructors.go:NewChannelHost` constructs only a
  no-op lifecycle owner, not the real Host instance;
- `internal/rpc/control.go` imports `internal/channelhost`, owns Channel DTO
  conversion and handlers, and advertises `channel.*` capabilities
  unconditionally;
- Channel configuration validation and overlay behavior span
  `internal/config`, `internal/app`, and `internal/app/settings`; and
- the Web settings shell imports Channel UI statically.

These are baseline facts, not completed modularization.

## 3. Boundary and dependency direction

```text
app + generated Assembly -> channel composition contracts <- Channel module
Channel module -> existing runtime/storage/credential/RPC contracts
RPC dispatcher -> generic validated contributions
Channel Provider -> std/channel@v1 -> canonical Channel Host
```

The internal composition contract contains only inert configuration/state,
focused dependencies, a factory, and an owned instance. It must not import
`internal/channelhost`, a platform Provider, or a platform SDK. The owned
instance implements the existing `module.Instance`, `runtime.RunHook`, and
`runtime.ChannelDeliverer` contracts and contributes its management handlers.

The generic RPC contribution binds an exact method name and capability name to
the existing `HandlerFunc`. Validation rejects empty bindings, duplicate
methods, and core-method replacement. It preserves authenticated peer context,
method policy, and existing error mapping. It is not a runtime-discovered or
public registry.

## 4. Ownership and lifecycle

The selected canonical Module eventually owns construction, Provider binding,
Grant admission, session mapping, inbound dispatch, terminal delivery,
Channel-specific configuration interpretation, management handlers, and
lifecycle. Platform Providers keep their present source boundaries and public
`std/channel@v1` ABI.

Construction performs no networking. Assembly constructs and registers the
Channel run-hook/delivery seams before a Run can begin, binds the one Run
callback after Service creation, then starts Channel after Service is usable
and before the public control plane is ready. Shutdown closes admission and
network ingress before dependent runtime/storage teardown. Partial startup
unwinds in reverse order.

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
existing Host and Service paths. CH-P0-1 adds only contracts, validation, and
tests; performance measurements are deferred until executable ownership or
artifact composition changes.

## 9. Verification seams

CH-P0-1 evidence consists of:

1. compile-time satisfaction of the factory/owned-instance contract;
2. contract package dependency closure without Channel implementation or
   platform SDKs;
3. RPC contribution validation for empty, invalid, duplicate, and core
   collisions;
4. Assembly cases for zero Providers, Provider-without-Host, duplicate Host,
   and public core Provider;
5. lossless settings preservation for an omitted Provider; and
6. backend/frontend serialization of the optional UI projection.

Physical executable and asset omission is not claimed until CH-P0-3 through
CH-P0-5 produce source-bound artifact evidence.

## 10. Slice fence

- **CH-P0-1:** contracts, normative text, conformance cases, baseline evidence.
- **CH-P0-2:** real backend owner, binding/config/management relocation, manual
  app ownership removal.
- **CH-P0-3:** conditional generated imports, reduced Recipes, backend physical
  omission and Inspect truth.
- **CH-P0-4:** Channel UI Module, settings contribution, visibility and asset
  composition.
- **CH-P0-5:** complete release matrix, packaged artifacts, browser/network
  evidence, rollback record.

CH-P0-1 must not perform work assigned to a later slice.

## 11. Eino decision

This change is composition and ownership only. It preserves the existing
callback into `Service.RunWithOptions` and adds no model, prompt, streaming,
context, tool, or orchestration capability. No Eino/EinoExt adapter is needed.

