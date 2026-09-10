# Vivy Module Standard v1

> Status: **Normative**
> Decision date: 2026-09-09
> Applies to: Vivy kernel composition, internal organs, first-party modules,
> public plugins, generated assemblies, and Generation inspection.
> Companion contracts: `VIVY-ASSEMBLY.md`, `VIVY-PORT-CATALOG.md`,
> `VIVY-PLUGIN-SPEC.md`.

## 1. Normative language

`MUST`, `MUST NOT`, `SHOULD`, and `MAY` are requirements in descending
strength. A lower-level implementation document cannot override this standard.

Vivy uses one composition model:

```text
Generation Recipe
  -> Assembly Compiler
  -> typed generated wiring
  -> immutable Kernel + internal Modules + public Modules
  -> frozen Runtime Assembly
  -> one Service.Run / Journal / Policy path
  -> Eino adapters inside the import quarantine
```

At architecture level, every product capability is a Module. At product and
UI level it keeps its real name: Tool, Channel, Face, Skill Source, Context
Source, Observer, UI Module, ModelHost, or Storage Engine. `Plugin` means only
a public, pluggable Module source; it is not a universal runtime interface.

## 2. Four layers

### L0 — Immutable Kernel

L0 owns the physical laws of a Vivy species:

- Module and Port catalogs;
- Assembly Compiler and Generation identity;
- domain identifiers, event schemas, and run state machine;
- the single `Service.Run` / `RunWithOptions` entry;
- Journal authority and durability-before-visibility;
- Policy, approval, Grant, identity, and RPC authorization;
- credential redaction authority;
- ChannelHost and FaceHost authority;
- Eino import quarantine.

L0 cannot be supplied or replaced by a public plugin. A Module Descriptor
cannot claim L0 authority.

### L1 — Required Internal

Every Agent Generation contains exactly one compatible implementation of its
required internal capabilities: LoopDriver, ModelHost, Storage Engine,
Checkpoint Store, Credential Resolver, Sandbox Backend, and the default
ToolHost. Implementations may evolve behind internal Ports; authority remains
with L0.

### L2 — Optional Internal Organs

ContextHost, SkillHost, MCPHost, ObserverHost, StatusHost, PresentationHost,
ActionHost, compaction, worker supervision, and similar organs are internal.
The default Vivy Generation includes the established first-party organs. A
minimal Recipe may omit one only when no selected Provider requires it.

### L3 — Public Pluggable Alliance

Public Modules may provide only Ports named in `VIVY-PORT-CATALOG.md`. They may
contribute Tools, ToolWorlds, Channels, a Face, Provider Profiles, Context or
Skill Sources, pre-tool Middleware, Observers, Status Sources, UI code, and
typed Control Actions. They never acquire L0 authority.

## 3. Module Descriptor

Every Module MUST have a pure-data Descriptor. The canonical shape is:

```yaml
apiVersion: vivy.module/v1
module:
  id: example/search-tools
  version: 1.2.3
source:
  ref: git:example/search-tools@0123456
  sha256: <64 lowercase hex characters>
provides:
  - port: std/tool@v1
requires:
  - port: core/tool-host@v1
optional:
  - port: std/observer/diagnostic@v1
conflicts:
  - module: example/legacy-search
requestedGrants:
  - net.client
i18n:
  catalog: i18n/catalog.json
  default_locale: en
  locales: [en, zh, ja]
lifecycle:
  scope: generation
```

The Descriptor MUST contain only identity, dependency, capability, conflict,
Grant request, and lifecycle facts. It MUST NOT execute initialization, read
environment variables, resolve Secrets, open files, start processes, access
the network, or mutate a registry. A backend-only Module with no
human-readable keys MAY omit `i18n`; a Module that provides UI content or
refers to a `label_key` MUST declare exactly one catalog.

The catalog path is relative to and confined within the selected Module
source. It MUST NOT be absolute, remote, runtime-discovered, or escape through
traversal or a symlink. `default_locale` MUST be `en` in v1. `locales` is a
normalized, duplicate-free packaged set containing `en`; future locales may
be packaged even though only `en` and `zh` are currently active. Public plugin
development requires English and Chinese as the baseline, while an
English-complete third-party catalog with missing or partial Chinese may
compile with build-owned `INCOMPLETE_LOCALE` evidence.

Core owns `vivy.*`. A Module with ID `<module-id>` owns the literal namespace
`plugin.<module-id>.*`; slash-to-dot or other lossy rewriting is forbidden.
Catalog parsing, validation, and hashing have no runtime side effects.

### 3.1 Identity

- Module IDs are lowercase, namespace-qualified, and stable.
- One Generation contains exactly one version and source hash for a Module ID.
- Version is semantic versioning; the source hash, not the version label, is
  the artifact identity.
- Trust is assigned by Source Catalog and Recipe lane. A Descriptor cannot
  declare itself `internal`, `trusted`, or `kernel`.
- All provided and required Ports include an explicit major version.

### 3.2 Dependencies and conflicts

- `requires` is required by default.
- Optional dependencies appear only in `optional`.
- Missing Provider, duplicate exclusive Provider, dependency cycle, or
  undeclared conflict is an Assembly compile error.
- A Provider with no Consumer is an error; silent discard is forbidden.
- A Recipe may explicitly prune the Provider and its dependent Modules. It may
  not use `allowUnused` as a general escape hatch.
- Default conflict resolution is failure. Loading order and
  last-writer-wins are forbidden.

## 4. Typed Port rules

A Port is a versioned, typed contract with one authority owner. Standard Ports
come from the canonical catalog. An extension Port is legal only when its
contract package is shared by Provider and Consumer and is also registered in
the catalog before use.

A Port Definition MUST state:

1. stable name and version;
2. Provider cardinality;
3. sole Host Consumer;
4. input and output types;
5. authority retained by the Kernel or Host;
6. lifecycle scope;
7. failure and timeout behavior;
8. permitted Trust levels and Grants;
9. conformance requirements;
10. Inspect projection.

`map[string]any`, reflection-based discovery, untyped event buses, and a God
`Plugin` interface are not Port contracts.

## 5. Trust model

| Level | Subject | Meaning |
|---|---|---|
| T0 | Immutable Kernel | Owns final authority and composition rules. |
| T1 | Internal Module | Repository-maintained code; still uses least authority and the single Host path. |
| T2 | Generation-trusted Plugin | Explicitly selected, source-pinned code compiled into the Generation. The Generation owner accepts process-level trust. |
| T3 | External-untrusted Instance | MCP or Sidecar process reached only through a governed transport. It is never linked into the Vivy process. |

Go code compiled into one process is not made into an operating-system sandbox
by a Manifest. T2 Grants are governance and conformance contracts. Code that
requires a hard trust boundary MUST run as T3. Documentation and Inspect MUST
not describe T2 as sandboxed.

## 6. Grants

The formal Grant vocabulary is:

```text
fs.read
fs.write
channel.poll
channel.webhook
channel.listen
channel.a2a
secret.read
proc.spawn
tty
argv
rpc.client
net.client
```

UI code is deliberately not represented by a Grant. A selected UI Module has
complete UI access by default; see `VIVY-PLUGIN-SPEC.md`.

Effective Grants are the deterministic intersection:

```text
requested by Module
  intersect Port allow-list
  intersect Trust-level ceiling
  intersect Recipe approval
```

Rules:

- Default-open Ports do not imply default Grants.
- A Manifest requests; the Assembly Compiler decides.
- Unknown or excessive Grants fail the build.
- Effective Grants freeze with the Generation and cannot expand at runtime.
- Host facades are scoped by workspace, instance, identity, and declared
  resource. Public Modules receive no raw database, Journal, Policy, credential
  store, or Eino object.
- `secret.read` names approved Secret references and cannot enumerate Secrets.
- `fs.read` and `fs.write` are rooted and path-contained.
- `proc.spawn` and `tty` are T1 by default; a T2 use requires an explicit
  Recipe decision and conformance evidence.
- `rpc.client` means authenticated Vivy Control RPC, not arbitrary networking.
- `net.client` carries Recipe egress constraints such as host, scheme, port,
  and TLS policy. For T2 it remains a compliance contract, not a hard sandbox.
- Secret values never enter Descriptor, Recipe, Manifest, error, log, or
  Journal payload.

## 7. Two lifecycles

### 7.1 Code Module lifecycle

```text
Describe -> Construct -> Start -> Ready -> Frozen -> Stop -> Close
```

- `Describe` and `Construct` MUST have no external side effects.
- `Start` may allocate only resources permitted by the effective Grants.
- `Ready` means required dependencies and owned resources are usable.
- `Frozen` prohibits changes to the Module graph or effective Grants.
- Startup failure rolls back all already-started owners in reverse order.
- `Stop` and `Close` are idempotent, deadline-bound, and preserve error causes.
- A required Module failure aborts the Generation start.

### 7.2 Runtime Instance lifecycle

```text
Configured -> Activate -> Ready | Unavailable -> Deactivate -> Close
```

Code can exist in a Generation while an instance is unconfigured or inactive.
Configuration never links new code. `Unconfigured` is not a missing Module;
`Unavailable` means an enabled instance failed. Neither state permits a silent
fallback to a different Provider.

## 8. Default-open policy

All public v1 Port Consumers and Hosts are compiled and enabled in the default
Vivy Generation. Established first-party implementations are included as the
zero-behavior baseline.

Default-open means the capability contract is present. It does not mean:

- scanning directories for unknown Modules;
- linking an external Module not named by the Recipe;
- activating a network connection without configuration and credentials;
- choosing a Provider by last registration;
- granting requested authority automatically.

External Modules enter only through an explicit Recipe. A minimal Recipe may
omit optional organs and Providers, and Inspect MUST report the resulting
absence.

## 9. Capability completion standard

A capability becomes `SUPPORTED` only when all seven artifacts exist:

1. Port Definition;
2. SDK Contract;
3. Host Consumer;
4. at least one real Provider;
5. Failure Model;
6. Conformance Suite;
7. Inspect Projection.

States are:

| State | Meaning |
|---|---|
| `RESERVED` | Name is protected but cannot be selected. |
| `SPECIFIED` | Normative design exists; implementation is absent. |
| `CANDIDATE` | Seven artifacts exist and default-Generation validation is running. |
| `SUPPORTED` | All required gates pass. |
| `DEFERRED-INDEFINITE` | Not scheduled and no placeholder implementation is permitted. |

Descriptor text, README claims, empty interfaces, and stubs do not establish
support. The Assembly Compiler and conformance evidence are the authority.

Every Conformance Suite MUST include registration, missing and duplicate
Provider, incompatible version, dependency cycle, denied Grant, timeout,
cancellation, startup rollback, unavailable instance, idempotent cleanup,
Secret redaction, Inspect provenance, default behavior, and at least one
representative real failure path.

## 10. Eino decision boundary

For model execution, agent orchestration, middleware, context/RAG, checkpoint,
MCP adaptation, OAuth, callbacks, and multi-agent behavior:

1. inspect the repository-pinned Eino/EinoExt version and cite the concrete
   package and API;
2. if a suitable capability exists, adapt it behind `internal/runtime` or
   `internal/provider`;
3. if it does not exist, mark that capability `DEFERRED-INDEFINITE`.

The plugin platform itself remains Vivy-owned because Eino does not provide
Module Descriptor, Trust, Grant, Generation provenance, Port catalog, or
Assembly Compiler semantics. Eino types MUST NOT enter public SDK, product,
storage, RPC, policy, plugin, or UI contracts.

## 11. Clean break

Vivy has no deployed plugin consumer base. v1 therefore has no compatibility
track:

- `vivy.plugin/v0`, `Seam`, and the God `Plugin` interface are rejected;
- there is no Adapter, alias, deprecation period, or conversion command;
- an old `apiVersion` fails as unsupported;
- new documentation, tests, examples, and Skills describe only v1;
- existing first-party behavior is directly re-expressed as v1 Modules and
  Ports, without preserving the old ABI.

## 12. Non-negotiable review questions

Every Module or Port change must answer:

1. Who provides it?
2. Who consumes it?
3. Which layer owns final authority?
4. What is its exact cardinality?
5. What happens when it is missing, duplicated, slow, or unavailable?
6. Which Grants can it request, and who approves them?
7. How does it appear in Inspect?
8. Which conformance test proves it is real?
9. Does it preserve the single `Service.Run` / Journal / Policy path?
10. Which pinned Eino/EinoExt API was checked when the work touches its scope?
11. If the Module exposes UI or a `label_key`, is its catalog present,
    source-confined, and owned by the literal `plugin.<module-id>.*` namespace?

An unanswered question is a specification failure, not an implementation
detail.
