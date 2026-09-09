# Vivy Plugin Specification v1

> Status: **Normative**
> Decision date: 2026-09-09
> Applies to public Module source selected by a Vivy Generation Recipe.
> `vivy.plugin/v0`, `Seam`, and the old God `Plugin` interface are unsupported
> and have no migration path.

## 1. Definition

A Vivy plugin is a public T2 Module source that:

1. has a `vivy.module/v1` Descriptor;
2. provides one or more public Ports from `VIVY-PORT-CATALOG.md`;
3. is named explicitly by a Generation Recipe;
4. is validated and compiled into a new immutable Generation;
5. appears with exact version, source hash, Ports, Grants, and dependencies in
   Generation Inspect.

It is not a runtime-loaded shared library, remote URL, Skill document, MCP
Server, standalone executable, arbitrary Eino Graph, or directory discovered
at startup.

Internal and public Modules share Descriptor, dependency graph, lifecycle,
Generation provenance, and Inspect semantics. They do not share authority.
Public Modules cannot provide `core/*` Ports or claim T0/T1 Trust.

## 2. Source layout

The target public Module layout is:

```text
plugins/<module-name>/
  vivy-module.yaml
  go.mod                    when the Go module is independently versioned
  module.go                 Module constructor and typed Providers
  module_test.go
  ui/                       only when providing a UI Port
    package.json
    pnpm-lock.yaml
    src/
```

The directory is a source boundary, not an execution boundary. One directory
has one Module identity but may provide several cohesive public Ports. A module
that mixes unrelated products should be split.

Public Go code imports only versioned public SDK packages. It MUST NOT import:

- `agent-vivy/internal/*`;
- Eino or EinoExt;
- generated Assembly packages;
- raw Journal, Policy, storage, RPC server, or credential packages.

## 3. Descriptor

Example:

```yaml
apiVersion: vivy.module/v1
module:
  id: acme/search
  version: 1.0.0
source:
  ref: git:acme/search@0123456
  sha256: 9f4a000000000000000000000000000000000000000000000000000000000000
provides:
  - port: std/tool@v1
  - port: std/ui-extension@v1
  - port: std/control-action@v1
requires:
  - port: core/tool-host@v1
  - port: core/presentation-host@v1
  - port: core/action-host@v1
requestedGrants:
  - net.client
lifecycle:
  scope: generation
```

Trust does not appear in this document. The Source Catalog identifies the
source; the Recipe selects the source and approves effective Grants.

`Describe` and parsing the Descriptor perform no side effects. Source hashes
are calculated from a canonical file set with normalized paths and explicit
exclusions for build output and local scratch.

## 4. No universal Plugin interface

Each Port has its own typed Provider interface. A Module constructor returns a
typed contribution set; it does not implement methods such as `Seam()`,
`Tools()`, or `Grants()`.

Conceptual target:

```go
type Module interface {
    Descriptor() module.Descriptor
    Construct(context.Context, module.Host) (module.Instance, error)
}

type Contributions struct {
    Tools          []tool.Provider
    ToolWorlds     []toolworld.Provider
    Channels       []channel.Provider
    ContextSources []contextsource.Provider
}
```

The concrete v1 SDK may split these types into focused packages. It MUST NOT
collapse them back into `[]any`, reflection registration, or a single interface
whose methods grow for every new capability.

## 5. Recipe inclusion

Example target Recipe:

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
order:
  std/ui-extension@v1:
    - vivy/default-ui
    - acme/search
```

Rules:

- The Recipe lists every external Module explicitly.
- The Assembly Compiler never scans `plugins/` for candidates.
- Removal means deleting the Module from the Recipe and building a new
  Generation.
- Configuration can activate only code already compiled into the Generation.
- A Recipe cannot grant a capability the Port or Trust level forbids.
- Ordered Port composition is explicit; last-writer-wins is forbidden.

## 6. Public capability rules

### Tools

All Tool Providers use `std/tool@v1` or `std/tool-world@v1`. They enter the
single ToolHost path. Public Modules cannot replace the protected internal Tool
IDs listed in `VIVY-PORT-CATALOG.md`.

### Channels and Face

ChannelHost consumes `std/channel@v1`; a Channel never appears in the model
Tool table. FaceHost consumes one `std/face@v1` for an Interactive Generation;
a Face remains an authenticated control-plane client.

### Provider profiles

A public Module can provide `std/provider-profile@v1` data. It cannot provide
an executable Model Provider. Model execution is an internal Eino/EinoExt
adapter; a missing pinned capability is deferred indefinitely.

### Context, Skills, MCP, and Observers

Sources return typed, bounded data to their internal Hosts. They cannot mutate
the final Prompt, Run, Journal, or Tool execution path. MCP Server instances
are T3 external systems managed by MCPHost, not native public Modules.

### Middleware

Only `std/middleware/pre-tool@v1` may affect execution. It can pass, deny,
require approval, or rewrite arguments. ToolHost revalidates after every
rewrite. Failure is fail-closed.

### Control Actions

Module-specific control operations use `std/control-action@v1` and the single
ActionHost RPC method. Plugins cannot register arbitrary RPC or HTTP routes.

## 7. Full UI access by default

A Module providing `std/ui-extension@v1` or `std/ui-root@v1` receives complete
UI access when selected into the Generation. There is:

- no `ui.full` Grant;
- no UI permission prompt;
- no component or DOM allow-list;
- no CSS isolation requirement;
- no per-change UI audit.

UI code may modify or replace routes, navigation, pages, components, styles,
themes, client state, shortcuts, commands, and the Web Face root. It may use
browser APIs and can observe client-visible state. This is trusted code, not a
sandboxed widget.

The build still enforces provenance:

- UI source and dependency lock hashes enter the Generation inputs;
- the built UI artifact hash enters the Manifest;
- `std/ui-root@v1` remains exclusive;
- extension order and replacement relationships are explicit;
- remote code download and T3 UI injection are forbidden.

Backend authorization is unaffected. The server rechecks identity, schema,
Policy, Grant, approval, and instance state. UI code cannot grant itself
backend authority by hiding, replacing, or forging a view.

## 8. Runtime and lifecycle

Module code follows:

```text
Describe -> Construct -> Start -> Ready -> Frozen -> Stop -> Close
```

Configured instances follow:

```text
Configured -> Activate -> Ready | Unavailable -> Deactivate -> Close
```

Required startup failure aborts the startup and closes earlier owners in
reverse order. Cleanup is idempotent and deadline-bound. Runtime instance
failure is visible as `Unavailable`; it cannot select an undeclared fallback.

Long-running work belongs to a Port whose lifecycle and resource ownership
explicitly permit it. A generic Tool Module cannot start an invisible daemon.

## 9. Verify, pack, and inspect target contract

The v1 command surface remains conceptually:

```text
vivy-sdk verify plugins/<name>
vivy-sdk pack --recipe vivy.generation.yml
vivy-sdk inspect-artifact dist/<generation>
```

These commands are target contracts until their implementation phase reaches
`SUPPORTED`. Documentation and Skills MUST NOT pretend that the current v0 SDK
already implements v1.

`verify` checks Descriptor schema, typed Port declarations, import firewall,
source identity, Grant requests, UI build metadata, and focused conformance.

`pack` compiles the complete Recipe graph, creates typed generated wiring,
builds backend and UI contributions, runs Generation conformance, embeds the
immutable Manifest, and emits no artifact on failure.

`inspect-artifact` displays Module/Port graph, Trust assignment, effective
Grants, source and artifact hashes, lifecycle order, Middleware/UI composition,
default and deferred capability status, and compiler version.

## 10. Failure rules

The following fail before a formal Generation is emitted:

- unsupported `apiVersion`;
- duplicate Module ID or exclusive Provider;
- missing Consumer or required Provider;
- dependency cycle or conflict;
- public Provider for `core/*`;
- reserved protected Tool identity;
- unknown, excessive, or unapproved Grant;
- forbidden import;
- unpinned external source;
- UI root conflict or ambiguous extension order;
- invalid schema or lifecycle scope;
- missing Conformance evidence for a claimed `SUPPORTED` Port;
- Eino-scoped capability with neither a verified upstream adapter nor an
  explicit `DEFERRED-INDEFINITE` result.

Errors MUST name the Module, Port, field or edge, and the violated rule. Error
cause chains are preserved and Secret values are redacted.

## 11. Prohibited legacy and bypasses

New work MUST NOT:

- use `vivy.plugin/v0`;
- implement or preserve `Plugin`, `Seam`, `SeamProvider`, or Seam-specific
  compatibility types;
- add a v0-to-v1 converter or hidden legacy branch;
- install a Module by editing `internal/runtime/engine.go`;
- register a Module during process startup;
- load Go or UI code from an unpinned runtime location;
- bypass ToolHost, ChannelHost, FaceHost, ActionHost, Policy, or Journal;
- import Eino outside `internal/runtime` and `internal/provider`;
- describe same-process T2 code as sandboxed.

## 12. Completion definition

A public plugin feature is complete only when its Port is `SUPPORTED`, the
Module passes its focused Conformance Suite, the complete Recipe passes
Generation conformance, Inspect proves provenance and authority, failure-path
tests pass, `just ci` is green, and the iteration log records human acceptance.

Until the v1 foundation exists, the correct result of a plugin implementation
request is to follow the approved phase plan, not to fall back to v0.
