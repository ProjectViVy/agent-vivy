# Vivy Assembly and Generation Contract v1

> Status: **Normative**
> Decision date: 2026-09-09
> Companion contracts: `VIVY-MODULE-STANDARD.md`,
> `VIVY-PORT-CATALOG.md`, `VIVY-PLUGIN-SPEC.md`.

## 1. One composition root

Vivy is assembled once, before the product artifact runs:

```text
Generation Recipe + Source Catalog + Module Descriptors
  -> normalize canonical inputs
  -> validate Trust, Ports, dependencies, conflicts, Grants, and hashes
  -> generate typed wiring
  -> compile backend and UI
  -> run conformance
  -> embed immutable Generation Manifest
  -> emit one Generation artifact
```

The running process consumes that frozen Assembly. It does not discover,
compile, import, or replace Module code. Runtime configuration controls only
instances of capabilities already compiled into the Generation.

## 2. Architecture versus product language

At the architecture layer all selectable capabilities are Modules. At product
surfaces they retain their real names: Loop, Tool, Channel, Face, Context
Source, Skill Source, UI Module, Storage Engine, or Provider Profile.

`Plugin` describes a public Module source. It is not a Port, base interface,
runtime registry item, or synonym for every internal organ.

## 3. Assembly inputs

### Generation Recipe

The Recipe states which Module sources enter a Generation, approves scoped
Grants, selects exclusive Providers, and defines order for ordered Ports. It
does not contain Secret values or executable scripts.

### Source Catalog

The Source Catalog resolves a Module ID to an authoritative source class and
assigns T1 or T2 Trust. A Module Descriptor cannot assign its own Trust.

### Module Descriptor

The Descriptor provides pure-data identity, version, source hash, provided and
required Ports, optional dependencies, conflicts, requested Grants, and
lifecycle scope. When localization is required, it also selects one
source-confined `vivy.i18n/v1` catalog with explicit default and packaged
locales.

### Port Catalog

The normative Port Catalog fixes public/internal status, cardinality, sole
Consumer, authority, and conformance state. An unknown Port is not resolved by
late binding; it fails compilation.

## 4. Default Generation

The default Vivy Generation contains:

- all L0 Kernel authorities;
- every required L1 internal Module;
- established L2 internal Hosts and organs;
- Consumers for all public v1 Ports;
- all established first-party feature Providers;
- the protected internal Tool set;
- one product-appropriate Face for an Interactive recipe;
- the default first-party UI root and extensions for the Web Face.

Network-capable runtime instances remain unconfigured and inactive until
configuration and credentials exist. Default inclusion never means silent
network activation.

### Minimal Generation

A deliberate minimal Recipe MAY omit an optional Host and its Providers. The
Assembly Compiler MUST reject a Provider whose Consumer was removed. Inspect
MUST display absence, not claim an inactive capability that was never compiled.

A protected Tool may be explicitly omitted from a minimal VIVY CODE Recipe.
Its reserved identity remains unavailable to public Providers.

## 5. Recipe shape

The target canonical form is:

```yaml
apiVersion: vivy.generation/v1
profile: default
modules:
  - id: vivy/default-body
  - id: acme/search
    source: git:acme/search@0123456
    sha256: 9f4a000000000000000000000000000000000000000000000000000000000000
    grants:
      net.client:
        schemes: [https]
        hosts: [search.example.com]
        ports: [443]
exclusive:
  std/face@v1: vivy/web-face
  std/ui-root@v1: vivy/default-ui
order:
  std/middleware/pre-tool@v1:
    - vivy/policy-guard
    - acme/search-guard
  std/ui-extension@v1:
    - vivy/default-settings
    - acme/search
```

The final implementation may encode the same typed information differently,
but it MUST preserve these facts and deterministic semantics.

## 6. Assembly Compiler gates

### G0 — Parse and canonicalize

- accept only `vivy.generation/v1` and `vivy.module/v1`;
- normalize paths, identifiers, ordering, and structured Grant constraints;
- reject duplicate keys, unsupported fields that change semantics, and all v0
  inputs;
- strictly parse selected catalogs, reject duplicate JSON keys, normalize
  locale tags, enforce resource limits, and serialize canonical catalog JSON;
- produce the same canonical bytes for semantically identical input.

### G1 — Resolve identity and Trust

- resolve every source through the Source Catalog;
- verify exact source and tree hashes;
- confine catalog paths to the resolved Module source and enforce literal
  `plugin.<module-id>.*` ownership;
- assign T1 or T2 independently of the Descriptor;
- reject missing, ambiguous, floating, or self-promoted sources.

### G2 — Compile the Port graph

- resolve every Provider to its sole Consumer;
- validate required and optional edges;
- enforce cardinality and exclusive selection;
- detect cycles and conflicts;
- reject unused Providers and missing conditional Hosts;
- freeze deterministic lifecycle and ordered-Port sequences.

### G3 — Enforce authority

- reject public Providers for `core/*` Ports;
- calculate effective Grants;
- protect reserved Tool and UI identities;
- enforce SDK and import firewalls;
- verify Eino imports exist only in `internal/runtime` and
  `internal/provider`;
- reject any path around Service, Journal, Policy, ToolHost, ChannelHost,
  FaceHost, or ActionHost.

### G4 — Generate and prove

- write typed Assembly wiring to generated files;
- compile selected backend Modules only;
- build selected UI roots and extensions only;
- prove English completeness, per-locale and per-form placeholder parity, and
  deterministic Web/TUI projection of the same selected catalog units;
- run focused Port and complete Generation conformance;
- verify startup rollback and cleanup order;
- emit no formal artifact when any proof fails.

Generated files carry a generated-code marker and MUST NOT be hand edited.

### G5 — Seal

- calculate all component and artifact hashes;
- calculate `SHA-256(canonical_catalog_json)` for every selected catalog;
- calculate the content-addressed Generation ID;
- embed the immutable Manifest and Inspect schema;
- emit the executable/UI artifact and a machine-readable build report;
- verify that reading the embedded Manifest reproduces the sealed identity.

## 7. Generation identity

Generation ID is derived from canonical, content-addressed inputs:

```text
specification version
+ canonical Recipe
+ Module IDs, versions, source refs, and source hashes
+ Port contract versions
+ SDK version
+ backend and frontend dependency lock results
+ UI artifact hashes
+ catalog schema, canonical digest, and packaged locale set
+ Assembly Compiler version
```

Runtime settings, Secret values, instance activation, health, and user data do
not enter the Generation ID.

One Generation ID MUST never identify different code. A changed input produces
a different identity even when a human-readable version string is unchanged.

## 8. Embedded Manifest and Inspect

Every artifact embeds an immutable Manifest containing:

- Generation ID and compiler version;
- canonical Recipe digest;
- Module ID, version, source ref, hash, and assigned Trust;
- provided and required Port edges;
- effective Grants and structured constraints;
- lifecycle start/stop order;
- pre-tool Middleware order;
- UI root, extension order, replacement relationships, and asset hashes;
- catalog schema version, confined path, canonical digest, default and
  packaged locales, per-locale completeness, and evidence identifiers;
- default, inactive, unavailable, unsupported, and deferred capability facts;
- Eino/EinoExt packages and APIs used by scoped internal adapters;
- focused and Generation conformance results.

Inspect is read-only. It MUST distinguish:

- not compiled;
- compiled but unconfigured;
- configured but inactive;
- ready;
- unavailable;
- specified but not implemented;
- deferred indefinitely.

It MUST NOT reveal Secret values, raw Journal blobs, or private configuration.

## 9. Runtime freeze and instance activation

After Assembly startup reaches `Frozen`:

- no Module or Port edge can be added, removed, or reordered;
- no effective Grant can expand;
- no Go package, shared object, UI bundle, or arbitrary remote code can load;
- configuration can create, activate, deactivate, and close only declared
  runtime instances;
- a remote MCP Server may change availability, but remains behind the compiled
  MCPHost and ToolWorld Port;
- a UI refresh cannot install or replace a Module.

## 10. Startup and shutdown transaction

Start Modules in the compiler-produced topological order. A required Module
that fails or misses Ready aborts startup. The Host then:

1. cancels the startup context;
2. stops started Modules in exact reverse owner order;
3. closes constructed instances in reverse order;
4. preserves the initiating error and attaches cleanup failures;
5. records a bounded, redacted diagnostic result;
6. does not publish the Generation as ready.

Stop and Close are idempotent and deadline-bound. Optional runtime instances
can become unavailable only where their Port failure model permits it; the
Module graph itself remains frozen.

## 11. UI Assembly

UI Modules are full code and receive complete UI access by default. The
Assembly Compiler still governs their presence and deterministic composition:

- `std/ui-root@v1` is exclusive;
- `std/ui-extension@v1` follows Recipe order;
- `before`, `after`, and `replaces` references must resolve;
- source, lockfile, and build output hashes enter provenance;
- selected catalog units feed one PresentationHost localization projection for
  Web and TUI with no independent fallback or persisted locale state;
- no runtime remote-code download is part of the Module system.

There is no UI Grant or UI authorization step. Backend RPC, Policy, approval,
ToolHost, and Journal authority remain server-side.

## 12. Rebuild, removal, and rollback

Adding, updating, removing, or reordering a Module always creates a new
Generation. Removal is proven when:

- the Recipe no longer names the Module;
- generated wiring has no import or constructor for it;
- backend and UI artifacts contain no selected source or asset;
- the Manifest has no Module, Port edge, Grant, catalog, generated projection,
  asset, or localization evidence record for it;
- default and focused conformance pass.

Rollback switches to a previously sealed Generation artifact. It does not
modify the current artifact in place and does not perform runtime code
downgrade. Runtime data compatibility remains governed by its own Kernel
contracts and is not weakened by plugin composition.

## 13. Eino boundary

Eino is an internal implementation framework, not the Assembly system.
Assembly may select an internal adapter Module, but public contracts never
contain Eino types.

For an Eino-scoped capability, the implementation phase records the exact
pinned package/API inspected. A suitable upstream primitive is adapted. A
missing primitive produces `DEFERRED-INDEFINITE`; it does not authorize a
parallel Vivy orchestration, Provider, OAuth, RAG, or MCP implementation.

Vivy-owned Assembly remains justified because Eino has no authority contract
for Module identity, dependency graph, Trust, Grants, public SDK firewall,
Generation provenance, or the single Journal and Policy path.

## 14. Clean-break rule

The compiler has no v0 normalization gate. `vivy.plugin/v0`,
`vivy.generation/v0`, `Seam`, and the God `Plugin` interface fail before graph
construction. There is no migration Adapter, warning period, alias, or reverse
conversion.

Existing first-party product behavior is registered directly as v1 Modules.
Behavior parity is mandatory; preserving the old public API is forbidden.

## 15. Completion

The Assembly platform is complete only when:

- all six compiler gates have executable proof;
- the default Generation preserves established product behavior;
- a minimal Generation proves real code removal;
- graph, Trust, Grant, UI, localization, lifecycle, and provenance failures are
  deterministic;
- Inspect reports the sealed truth;
- rollback uses whole Generation artifacts;
- no runtime discovery or v0 path remains;
- SCX Gates A, B, and C in the project plan are satisfied.
