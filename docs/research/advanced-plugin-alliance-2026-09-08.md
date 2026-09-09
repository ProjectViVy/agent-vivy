# Vivy Advanced Plugin Alliance Research: Compile-Time Free Assembly and Kernel Boundaries

> Date: 2026-09-08
> Status: **research archive**. The standards-level decisions of 2026-09-09 supersede the proposals in this document; implementation follows only
> `docs/architecture/VIVY-MODULE-STANDARD.md`, `VIVY-PORT-CATALOG.md`,
> `VIVY-PLUGIN-SPEC.md` and `VIVY-ASSEMBLY.md`. This document retains only investigation evidence and historical
> options; in particular, it must not be used to restore v0 compatibility, executable public Providers, or the constrained UI proposal.
> Scope: Vivy species kernel; excludes Vivy Studio shell implementation
> Goal: evaluate “everything is a plugin,” unify the assembly semantics of `internal` and `pluggable`, and preserve the single `Service.Run` / Journal / policy path.

## 0. Executive Summary

### Conclusion

The direction is feasible, but the original proposition must be made precise:

> **Every product capability is a declarable, dependency-bearing, replaceable assembly unit; not every kernel invariant can be taken over by an ordinary plugin.**

Recommended: **a unified metamodel, tiered ports, generative assembly, and runtime freeze**:

1. `internal` and `pluggable` share the Module Descriptor, dependency graph, lifecycle, Generation provenance, and inspect;
2. They do not share permissions. `internal` may implement privileged ports; `pluggable` may implement only public SDK ports through Grant/Host;
3. Capability implementations can be selected freely before compilation; `vivy-sdk pack` resolves dependencies and generates strongly typed wiring, then Go compiles and links it;
4. The plugin graph freezes after process startup; there is no Go dynamic loading, hot-unload, or second runtime;
5. Eino is a controlled orchestration kernel, not a plugin manager. Eino components are adapted only in `internal/runtime` and `internal/provider`;
6. `Service.Run`, Journal authority, policy/approval, the domain event schema, the composition compiler, and similar elements must be coupled to the kernel; their backends may be internal providers, but authoritative semantics cannot be exported.

### Recommended Decisions

| Decision | Recommendation |
|---|---|
| Overall model | `Module + Port + Generated Assembly` |
| Assembly timing | Resolve before pack/Go build; at startup initialize only modules already compiled in |
| Runtime dynamic plugins | Do not implement |
| Public plugin execution | Compile trusted source into the same EXE; route untrusted capabilities through MCP/sidecar, without pretending they are Go plugins |
| API shape | Unify metadata, separate capability interfaces; reject one universal `Plugin` interface |
| Conflict semantics | Fail-closed by default; prohibit last-writer-wins |
| Eino | Internal loop/provider adapter; reuse ADK/Compose/Middleware; do not leak into the SDK |
| First migration batch | Bring Tool / ToolWorld / Channel / Face into the alliance while preserving the existing ABI |
| Second-batch ports | Provider, pre-tool middleware, run observer, context source, skill source |
| UI | Keep Face as the whole face; create a separate fine-grained `ui/slot`, not the same category |

## 1. Current Vivy: It Has a Plugin Skeleton, but Not Yet an “Alliance”

### 1.1 Established Facts

Current plugins are “source-governance units + compile-time pack,” not runtime dynamic libraries:

- `vivy-plugin.json` is verified by the SDK;
- `vivy-sdk pack` generates a registration file and compiles a new EXE;
- source not included in the recipe does not belong to that Generation;
- plugins cannot import `internal` or Eino;
- a crash in an in-process plugin affects the entire EXE.

Evidence: `docs/architecture/VIVY-PLUGIN-SPEC.md:16-28,165-177,227-264`.

The existing source already supports five seams: `tool`, `tool-world`, `provider`, `channel`, and `face`, plus 11 Grants; the architecture documentation still has “four seam types” and the old Grant vocabulary, so documentation drift has occurred. Evidence: `sdk/plugin/plugin.go:12-35,37-88` compared with `docs/architecture/VIVY-PLUGIN-SPEC.md:77-87,98-117`.

The public capabilities that actually have consumers are:

| Capability | Current consumer | Status |
|---|---|---|
| Tool / ToolWorld | `internal/pluginhost.Adapt` → `internal/tools.Tool` | Established |
| Channel | `ChannelHost` | Established |
| Face | `FaceHost` / control-plane client | Established |
| Diagnostic observer | file mutation diagnostic bridge | Established narrow capability |
| LSP status provider | control-plane status source | Established narrow capability |
| Provider seam | No dedicated consumer | Only an enumeration/catalog concept; not yet established |

`pluginhost.Adapt` explicitly skips only Channel, then adapts the other `plugin.Plugin.Tools()` into tools; this proves that `SeamProvider` currently has no independent runtime semantics. Evidence: `internal/pluginhost/host.go:23-45`.

### 1.2 Current Assembly Limitations

1. **The unified interface is too narrow**: `Plugin` returns only `Tools()`; Channel/Face require side-channel ABIs, and continuing to add capabilities would turn the interface into a God object.
2. **Classification and trust are mixed together**: `seam` expresses the capability type, but cannot express internal/pluggable trust level, dependencies, conflicts, cardinality, or lifecycle.
3. **The generated registry covers only user plugins**: the factory loop/world/provider/tool are still wired manually by `internal/app`; true compile-time free composition is impossible.
4. **There is no dependency graph**: no unified rules for Definition/Provider/Consumer, missing dependencies, dependency cycles, or choosing among multiple implementations.
5. **There is no owner-scoped lifecycle**: tools are values, while Channel/Face have their own start/stop; background resources, cleanup, and failure rollback have no unified semantics.
6. **The default species body and narrow Generation have different semantics**: the committed `internal/generated/plugins/zz_register.go` manually compiles all first-party Channels, and pack replaces it with a narrow registry. Evidence: `internal/generated/plugins/zz_register.go:1-28`.
7. **Runtime still has dynamic capability switches**: builtin tools can rebuild the Eino engine through settings, while plugin tools are always appended; MCP servers can also be replaced at runtime. These are “activation/configuration of compiled-in capabilities,” not loading new code. Evidence: `internal/app/app.go:335-358,394-471,614-619`.

### 1.3 Relationship to the Older Architecture Documentation

`VIVY-ASSEMBLY.md` currently specifies that “only user-defined items are called plugins,” while factory capabilities are named loop/world/provider/tool. Evidence: `docs/architecture/VIVY-ASSEMBLY.md:30-38,42-74,140-157`.

This report proposes a higher-level **unification of the internal metamodel**:

- The product/UI should still call them “tools, model exits, worlds, channels, and faces”;
- the architecture and pack layers should treat them uniformly as Modules;
- “internal/pluggable” is a source and trust classification; the product interface does not need to display everything as a “plugin.”

Therefore, this is not a simple documentation supplement, but a directional extension of the existing naming contract; the contract can be changed only after an architecture decision.

## 2. Conclusions from Reference Projects

## 2.1 Eino: Reuse Orchestration, Do Not Recreate the Plugin System

Vivy is pinned to Eino `v0.9.13`; the local `.workspace/eino` is already a higher alpha version, so API judgments use `v0.9.13` in the Go module cache as the reference.[1]

Eino v0.9.13 provides the following component interfaces and adapters:

- Component interfaces: ChatModel, Tool, Retriever, Embedding, Loader, Transformer, Indexer;
- Compose: Graph, Chain, Workflow, Parallel, Branch, Lambda, and `Compile`;
- ADK: Agent, Runner, Interrupt, Resume, Checkpoint;
- Agent middleware, Tool middleware, and callbacks;
- Concrete OpenAI, Claude, MCP, and other adapters in EinoExt.

Local evidence: `go.mod:13-19`, plus `C:/Users/Administrator/go/pkg/mod/github.com/cloudwego/eino@v0.9.13/components/types.go:17-86`, `compose/chain.go:157-536`, `compose/graph.go:296-467`, `adk/handler.go:139-265`, and `adk/runner.go:50-149`.

Eino has no general-purpose plugin discovery, versioning, trust, Grant, or Factory registry. `schema.Register` registers serialization types; callbacks do not guarantee global ordering across Handlers and are not a durable event bus. Therefore, Eino cannot be treated as Vivy's plugin manager.[1]

Vivy correctly reuses Eino's ADK, dynamic tool search, Skill, AgentsMD, reduction, and summarization; tools enter Eino only through `toolAdapter`, where argument validation, policy, hooks, approval, and result boundaries are enforced uniformly. Evidence: `internal/runtime/engine.go:9-15,118-220`, `internal/runtime/tooladapter.go:24-35,79-180`.

Conclusion: build a Vivy-owned assembly compiler on top of Eino for the new plugin system; do not rewrite Eino's existing Compose/ADK/Middleware, and do not expose Eino types to the SDK.

## 2.2 Hermes: Learn the Facade/Provider Profile, Reject Multiple Registries

Hermes is currently not one plugin system, but a collection of coexisting extension mechanisms for generic Python plugins, model providers, memory, context engine, MCP, skills, dashboard, gateway hooks, and more.[2]

Worth adopting:

- provider profiles declare only endpoint/auth/request differences; the core owns the client, credential rotation, and streaming;
- `PluginContext` acts as a Host facade and does not hand out kernel objects directly;
- deferred tools are only a model-visibility layer; actual calls still pass through the tool registry, hooks, and approval;
- profiles isolate config, session, skill, and subprocess home;
- plugin/MCP failure can be locally unavailable.

Local evidence: `C:/Users/Administrator/Desktop/morediva/.workspace/hermes-agent/providers/base.py:1-10`, `hermes_cli/plugins.py:286-354`, `tools/tool_search.py:150-209`, `hermes_cli/profiles.py:37-52`.

Do not copy:

- multiple discovery loaders;
- a global registry;
- a mixed conflict strategy of last-writer-wins / first-writer-wins / silent ignore;
- non-transactional `register(ctx)`;
- treating skills, MCP, code plugins, and UI plugins as the same trust level.

Hermes's core lesson can be summarized as: plugins implement capabilities; the kernel decides when a capability is available, who can call it, whether approval is required, and how the result enters session state.[2]

## 2.3 DeepSeek Harness / Cordis: Learn the Semantics, Do Not Copy Hot Loading

Cordis's minimal base consists of a root Context, Reflect service registration, Registry, Fiber/effect lifecycle, Events, and Loader; product capabilities enter the system through Definition, Provider, Consumer, `inject`, and owner-scoped effects.[3]

Local evidence:

- `.../.workspace/deepseek-harness/deepseek-harness/vendor/cordis/src/context.ts:9-84`
- `vendor/cordis/src/registry.ts:91-145,189-337`
- `vendor/cordis/src/fiber.ts:139-154,212-332,402-441,641-695`
- `vendor/cordis/src/events.ts:24-32,125-301`
- `docs/user/develop/practice/index.md:5-49`

Semantics most worth migrating to the Go compile-time model:

| Cordis | Vivy equivalent |
|---|---|
| Service Definition / Provider / Consumer | typed Port contract / Provider factory / generated consumer wiring |
| `inject` | Dependency DAG at pack time |
| Fiber owner | Module owner + Cleanup stack |
| effect/disposer | Register after successful `Start`, reverse-order `Close` |
| PENDING | Fail directly on missing compile-time dependencies; allow absent only when explicitly optional |
| profile/bundle/patch | generation recipe layering |
| event dispatch mode | Typed middleware/event phase |
| rollback | Revoke all contributions from this Module when startup partially fails |

Do not migrate runtime dynamic import, reflective `ctx.foo`, hot reload on dependency changes, Node VM, or a generic event bus without a payload contract.

DSH also reveals an important rule: a Provider without a Consumer is not a complete seam. Vivy's current `SeamProvider` is exactly in this state.

## 2.4 Other Corroborating Evidence

OpenFang still has many static modules and `match` dispatch for Channel/Provider/Tool, but its WASM fuel, epoch timeout, and capability-checked host functions are worth referencing for a future untrusted execution layer.[4]

ZeroClaw uses feature flags and macros to maintain a single catalog of provider slots, and generates configuration, traversal, and factory dispatch together; the Go equivalent should be generation codegen, not multiple handwritten switches. Its WASM Channel still has a placeholder in the current snapshot, showing that “manifest-declared capability” cannot replace conformance tests.[5]

OpenClaw's manifest-first design, metadata snapshot, owner-tagged registry, and rollback are mature, but native plugins share a process and permissions with the Gateway, so it is unsuitable as Vivy's default third-party trust model.[6]

Pi's small typed lifecycle, per-extension error collection, and stale-context invalidation are useful references; its extensions have host-process permissions by default and require an external sandbox to form a security boundary.[7]

Combined conclusion:

- ZeroClaw: borrow the compile-time single source of truth;
- Cordis: borrow dependency handling and owner lifecycle;
- Hermes: borrow the capability facade and provider profile;
- OpenClaw: borrow the manifest snapshot and atomic rollback;
- OpenFang: borrow resource metering only for a future sidecar/WASM layer;
- Pi: borrow the simple lifecycle, not the permission model.

## 3. Recommended Architecture: Unified Module, Tiered Port

## 3.1 Four-Layer Model

```text
Generation Recipe
      │
      ▼
Assembly Compiler / Verifier           ← kernel, sole authority
      │  resolve DAG + trust + grants + conflicts
      ▼
Generated Typed Wiring                 ← Go source, do not edit by hand
      │
      ├── internal modules             ← privileged implementations
      └── pluggable modules            ← public SDK + Host grants
      │
      ▼
Frozen Runtime Assembly
      │
      ▼
Single Service.Run / Journal / Policy  ← sole authoritative path
      │
      ▼
Eino adapter / providers               ← only internal/runtime + internal/provider
```

Key point: what is unified is the **governance and assembly metamodel**, not every capability being forced into one Go interface.

## 3.2 `internal` and `pluggable`

| Dimension | internal | pluggable |
|---|---|---|
| Source | Vivy repository / controlled first-party modules | User or ecosystem source modules |
| Selection | generation recipe | generation recipe |
| Description | Same Module Descriptor | Same Module Descriptor |
| Lifecycle | Same owner/cleanup model | Same owner/cleanup model |
| Implementable ports | public + privileged core-provider ports | public ports only |
| Import | Internal package firewall; Eino still restricted to runtime/provider | `sdk/plugin`, public port contract, own dependencies; internal/Eino prohibited |
| World access | Still prefer narrow interfaces | Grant-filtered Host facade only |
| Trust | Trusted at build time | Reviewed in-process source; not a security sandbox |
| Failure | Required modules fail startup by default | Manifest explicitly marks required/optional; implicit degradation prohibited |

Modules cannot declare themselves `internal` through the manifest. The assembly compiler assigns the trust level from the source catalog / recipe lane and writes it into the Generation; otherwise a user plugin could self-escalate.

## 3.3 Module Descriptor

Upgrade from a single `seam` to a multi-contribution description:

```yaml
apiVersion: vivy.module/v1
id: builtin/eino-loop
version: 1.0.0
source: internal
provides:
  - port: core/loop-driver@v1
    implementation: eino
requires:
  - port: core/chat-model@v1
  - port: std/tool-catalog@v1
optional:
  - port: std/context-source@v1
conflicts: []
grants: []
lifecycle: process
```

Rules:

- Valid values for `source/trust` are assigned by pack; do not trust a module's self-report;
- one Module may provide multiple related Ports, but each Port still has an independent typed contract;
- `requires` is required by default; optional must be explicit;
- the Port catalog defines cardinality, scope, permitted trust, and failure policy;
- the Descriptor is pure data, with no initialization side effects;
- v0 `seam` may be converted into one standard Port during the transition.

## 3.4 Port Tiers

### A. kernel-closed ports

Only the kernel or internal modules may provide/consume these:

- `core/loop-driver@v1`
- `core/chat-model@v1`
- `core/storage-engine@v1`
- `core/checkpoint-store@v1`
- `core/credential-resolver@v1`
- `core/sandbox-backend@v1`

“closed” does not mean the implementation is hard-coded; it means it can be replaced only by a controlled internal module and must satisfy kernel conformance.

### B. public standard ports

May be implemented by internal or pluggable modules:

- `std/tool@v1`
- `std/tool-world@v1`
- `std/channel@v1`
- `std/face@v1`
- `std/provider-profile@v1`
- `std/context-source@v1`
- `std/skill-source@v1`
- `std/middleware/pre-tool@v1`
- `std/observer/run@v1`
- `std/observer/diagnostic@v1`
- `std/ui/slot@v1` (future)

### C. extension ports

Use `x/<author>/<port>@vN`, but do not provide a `map[string]any` service locator:

- producer and consumer must share a verifiable Go contract package;
- codegen generates static imports and type assignments, letting the Go compiler perform the final type check;
- the kernel records only ID, version, owner, and provenance; it does not automatically expose an extension port to the model, RPC, or Journal;
- an extension provider without a consumer errors at pack time instead of existing silently.

## 3.5 Why Not a Universal `Plugin` Interface

An interface containing Tool, Channel, Provider, Storage, UI, Hook, and Lifecycle would produce:

- many meaningless no-op methods;
- contamination between different lifecycles;
- a public API widened by internal privileged capabilities;
- every new capability breaking all implementations;
- no way to express one module providing multiple independent contributions.

Recommendation: unify Descriptor/owner, keep Port interfaces separate, and have the generated binder place each module's typed contributions into the Assembly.

## 4. Assembly Compiler

## 4.1 Three Kinds of “Compilation” Must Be Distinguished

1. **Assembly compile**: `vivy-sdk pack` parses the recipe, manifest, dependencies, and permissions;
2. **Go compile/link**: generate static imports/wiring and build the EXE;
3. **Eino Compile**: at process startup, compile the runtime graph for the already constructed Graph/Chain/Workflow.

Eino Compile is not Go plugin loading and cannot replace the first two steps.

## 4.2 Pipeline

```text
generation.yml + module manifests + source catalog
  → normalize v0 seam to v1 ports
  → validate identity/version/provenance/hash
  → assign trust class
  → resolve provides/requires/optional/conflicts
  → enforce port cardinality and trust policy
  → detect missing dependency / duplicate / cycle
  → validate grants and import quarantine
  → generate typed zz_assembly.go
  → go build / conformance tests
  → embed immutable Generation manifest
  → startup initialize and freeze
```

Must preserve:

- Do not scan directories for automatic inclusion; a module exists only when named by the recipe;
- default conflict = error; do not use last-writer-wins;
- multiple implementations of a single port may be selected only explicitly by the recipe;
- a missing optional dependency must produce an inspectable state;
- Generated files cannot be edited manually;
- the artifact can list module, port, implementation, version, trust, grants, source hash, and dependency edges.

## 4.3 Startup Lifecycle

Recommended phases:

```text
Describe → Construct → Start → Ready → Frozen → Stop → Close
```

Semantics:

- `Describe` must be a pure function;
- `Construct` creates typed contributions in DAG order without starting background work;
- only `Start` may acquire resources; register owner cleanup immediately at every step;
- required-module failure: close already-started modules in reverse order and abort startup;
- optional-module failure: continue only when the manifest and Port policy explicitly allow unavailable;
- after `Frozen`, adding or removing code modules is prohibited;
- `Stop/Close` runs in reverse DAG order, is idempotent, and has a deadline;
- the kernel creates child owners for Run/session-scope resources, but no separate runtime registry may be created.

Do not implement Cordis hot updates, but retain its most important owner-scoped cleanup semantics.

## 4.4 Failure and Conflict Matrix

| Situation | Result |
|---|---|
| Situation | Result |
|---|---|
| Required port missing | pack fail |
| Multiple providers for a single port without explicit selection | pack fail |
| Dependency cycle | pack fail, and print the cycle |
| pluggable requests a closed port | verify fail |
| Grant exceeds the Port's permitted maximum | verify fail |
| Eino import appears outside runtime/provider | import gate fail |
| Module construction fails | startup fail + rollback |
| Optional module startup fails | Mark unavailable; continue only when the contract allows it |
| Tool/schema name conflict | pack fail |
| Extension port has no consumer | pack fail or explicit `allowUnused`; fail by default |
| Module panic | The current in-process model can damage the EXE; document and expose this in inspect |
| Untrusted third-party code | Do not compile it in; use MCP/sidecar instead |

## 5. Map of Vivy's Existing Capabilities in the “Plugin Alliance”

## 5.1 First Tier: Include Directly, Preserve the Existing ABI

| Alliance member | Current state | Action |
|---|---|---|
| Tool | builtin + pluginhost already have a unified domain Tool | Wrap it in a Module Descriptor; do not change the Eino adapter |
| ToolWorld | LSP and file/process world capabilities already have Grant Env | Include it as `std/tool-world`; split out optional observer ports |
| Channel | Channel ABI + ChannelHost are established | Channel becomes a module contribution; keep Host in the kernel |
| Face | Whole Face ABI + FaceHost are established | Face becomes an exclusive public port; keep Host/RPC in the kernel |
| Diagnostic observer | Narrow optional interface already exists | Promote it to `std/observer/diagnostic` |
| LSP status | Narrow provider interface already exists | Promote it to a read-only status port |

This step should be a compatibility migration: existing plugins need not be rewritten all at once; pack translates the v0 manifest.

## 5.2 Second Tier: Suitable to Add, but Consumer Must Be Added First

| Candidate | Pluggable implementation | Host control that must remain |
|---|---|---|
| Model provider | provider profile, request quirks, model-catalog enricher | credential resolver, route freeze, streaming accounting, raw model ID rules |
| Loop driver | Eino ADK/Compose loop or a future replacement | `Service.Run`, Journal mapping, budget, cancel, terminal |
| World backend | sandbox/local/fs/exec/http/fetch/download | workspace identity, Grant, network policy, audit |
| Context source | AGENTS.md, always skill, retrieval context | token budget, session/log boundary, redaction |
| Compaction strategy | Eino reduction/summarization parameters and strategy | durable compaction event, checkpoint compatibility, token accounting |
| Tool middleware | pre/post-tool typed contribution | phase ordering, policy re-evaluation, approval cannot be bypassed |
| Run observer | audit, telemetry, channel projection | Journal ordering, redaction, backpressure; observer must not become authority |
| Skill source | filesystem/marketplace/custom source | trust scan, install governance, session-mount truth |
| Title generator | provider/model strategy chain | session identity, durable update, usage attribution |
| Search/media backend | network search, image/audio/video provider | routing, secret, timeout, result caps |
| MCP adapter | server-to-tool/resource/prompt adapter | transport lifecycle, approval, schema/size, remote side effects |
| UI slot | Declarative card/tab/action contribution | RPC auth, state-mutation policy, renderer isolation |

Note: the MCP endpoint/config itself remains an external dependency and is not a local code plugin; what can be made pluggable is “projecting MCP capabilities into Vivy's adapter/provider.”

## 5.3 Third Tier: internal-only Providers

| Candidate | Replaceable part | Semantics that cannot be replaced |
|---|---|---|
| Storage backend | SQLite / Postgres engine implementation | Journal append contract, transaction boundaries, terminal uniqueness |
| Checkpoint store | blob backend / encoding implementation | Vivy envelope, engine version, checksum, fail-closed recovery |
| Credential backend | env/OS vault/future secret store | Secrets are not persisted, recorded, or exposed beyond the minimum |
| Sandbox implementation | OS-specific executor | policy, workspace scope, deny rules, audit |
| Worker transport | process transport / future remote worker adapter | parent authority, budget, approval route, event validation |
| Memory/index backend | retrieval/index provider | namespace, authorization, durability, provenance |

These may become internal Modules, but cannot become ordinary pluggable ports.

## 5.4 Not Yet Entering the Alliance

- Arbitrary event-bus listeners; define typed event/phase first;
- Arbitrary RPC routes; define the UI slot/action contract first;
- Arbitrary Journal readers/writers; expose only a narrow query or append-intent API;
- Arbitrary policy-evaluator replacement; policy-rule contributions may be exposed, but the final arbiter remains in the kernel;
- Arbitrary long-lived background services; only with an explicit Port, owner, deadline, cleanup, and resource budget;
- Arbitrary Eino Graph/Lambda; only internal recipes may assemble them, and they cannot become a public SDK ABI.

## 6. Content That Must Be Coupled

“Must be coupled” means that semantics and authority must be owned uniformly by the kernel; it does not mean every backend implementation must be hard-coded.

| Must-couple item | Reason | Replaceable boundary |
|---|---|---|
| Module/Port catalog and Assembly compiler | If ordinary plugins could replace these too, the system could not define whether a plugin is valid | None; kernel bootstrap foundation |
| Generation identity/provenance/hash | Artifacts must be reproducible and inspectable | Hash implementation may be maintained internally, not exposed |
| Domain IDs, event schema, run state machine | All modules must share one language | Version evolution only |
| Single `Service.Run` / `RunWithOptions` | Prevent a second agent runtime and multiple termination semantics | LoopDriver replaceable; Service is not |
| Journal authority | Durability-before-visibility, ordering, exactly-one-terminal | Backend may be replaced internally |
| policy / approval / Grant enforcement | Plugins cannot self-grant or bypass security decisions | Rules may be contributed; final arbiter is not replaceable |
| tool dispatch envelope | Schema, argument safety, policy, hook rewrite, and approval must use the same path | Tool implementation replaceable |
| checkpoint envelope/recovery protocol | Version, checksum, migration, and resume must remain consistent | Store backend replaceable |
| workspace/session/tenant identity | Files, memory, channels, and model context all depend on the same scope | Adapter replaceable; identity authority is not |
| credential resolution/redaction | Secrets must not enter the manifest, Journal, or errors | Secret backend internal-only |
| ChannelHost / FaceHost | Transport/UI may enter run/RPC only through the host | Channel/Face adapter replaceable |
| RPC protocol and mutation authorization | UI/plugins must not bypass the control plane | Renderer/slot replaceable |
| budget/cancellation/worker supervision | Must be measured uniformly across models, tools, and child runs | Transport may be replaced internally |
| Eino quarantine | Avoid framework types contaminating product contracts | Eino adapter/version replaceable |

The existing `Service` explicitly guarantees “persist before publish” and maintains active/pending/recovery/budget/terminal states. Evidence: `internal/runtime/service.go:144-205,2542-2612`. None of these may be delegated to a Loop plugin or Eino callback.

Tool policy must also retain a single path: argument validation → policy → pre-hook → revalidation/repolicy after rewrite → approval → execution. Evidence: `internal/runtime/tooladapter.go:79-180`.

## 7. Eino Capability Check

This design touches the agent loop, model/tool orchestration, middleware, streaming, checkpoint, MCP, and context, so the reusable Eino surface must be listed first.

| Requirement | Eino/EinoExt capability | Vivy decision |
|---|---|---|
| Agent loop | `adk.NewChatModelAgent`, `adk.Runner` | Reuse the Eino loop as an internal module |
| Tool orchestration | `components/tool`, `compose.ToolsNode` | Reuse; Domain Tool goes through the existing adapter |
| Dynamic tool visibility | `adk/middlewares/dynamictool/toolsearch` | Already reused; not a plugin registry |
| Skill injection | `adk/middlewares/skill` | Already reused; Skill source can become a Vivy port |
| Project instructions | agentsmd middleware | Already reused; source can be assembled, boundary remains with Vivy |
| Compaction | reduction + summarization middleware | Already reused; strategy can be assembled, durability remains with Vivy |
| Middleware | `TypedChatModelAgentMiddleware`, `ToolMiddleware` | Compose directly internally; public exposure through a Vivy typed port |
| Graph composition | Graph/Chain/Workflow/Branch/Parallel | Internal recipe target; do not expose Eino ABI |
| Callbacks | callbacks Handler | Observation only; not Journal authority |
| Checkpoint/resume | `CheckPointStore`, Interrupt, Resume | Reuse Eino mechanisms; retain Vivy envelope/store authority |
| MCP tools | EinoExt `mcp.GetTools` | Schema/tool adapter only; transport/governance remains with Vivy |
| RAG | Retriever/Embedding/Loader/Transformer/Indexer interfaces | Adapt these interfaces first in the future; do not rewrite equivalent orchestration |

The legitimate gap for a custom Vivy Assembly is that Eino has no module discovery, dependency graph, version/trust/grant, Generation provenance, public SDK firewall, or kernel authority contract. The migration boundary is also clear: if Eino later provides a stable component registry, the factory adapter inside `internal/runtime` may be replaced, but Vivy's manifest, trust, Service.Run, and Journal may not.

## 8. Phased Delivery

### P0: Decide the Contract, Write No Features

1. Accept or reject “everything is a Module at the architecture layer, displayed by its real name at the product layer”;
2. Freeze the definitions of `internal` / `pluggable`;
3. Freeze Port naming, cardinality, scope, and failure policy;
4. Define the v0 seam → v1 port compatibility strategy;
5. Update the five-seam/Grant drift in `VIVY-ASSEMBLY.md` and `VIVY-PLUGIN-SPEC.md`.

Acceptance: no code migration; the contract can answer who provides, who consumes, who has authority, and what happens on failure.

### P1: Minimal Closed Loop for the Assembly Compiler

- Module Descriptor v1;
- source catalog and trust assignment;
- provides/requires/conflicts DAG;
- duplicate/missing/cycle diagnostics;
- generated typed wiring;
- Generation inspect;
- owner-scoped startup cleanup;
- v0 manifest compatibility adapter.

Initially carry only Tool/ToolWorld/Channel/Face, without changing behavior.

### P2: First-Party internal Assembly

- Map the current app wiring to internal modules;
- describe first; do not rush to physically move directories;
- extract Eino LoopDriver, World, Provider, and builtin Tool factories;
- migrate one Port at a time, proving that it still passes through the single Service.Run.

### P3: Advanced Public Ports

In order of increasing risk:

1. observer/diagnostic, run observer;
2. context source, skill source;
3. provider profile / chat model consumer;
4. pre/post-tool middleware;
5. UI slot.

Every Port must ship with Definition, Provider, Consumer, failure model, and conformance suite together; adding only an enumeration is prohibited.

### P4: internal-only Backends

- storage engine;
- checkpoint store;
- sandbox backend;
- credential backend;
- memory/index backend;
- worker transport.

Contract tests must prove that replacement implementations do not change kernel authority.

### P5: Untrusted Ecosystem (As Needed)

Do this only when there is clear market demand:

- sidecar JSON-RPC/MCP; or
- WASM/WIT + fuel/timeout/capability host.

Do not combine this with the first version's compile-time Go plugin project.

## 9. Acceptance Matrix

| Acceptance item | Required result |
|---|---|
| Free assembly | After the recipe removes a Module, the artifact contains none of its imports/implementation |
| Strong typing | Port type errors are exposed through generated code/Go compilation failure |
| Dependencies | missing/duplicate/cycle fail deterministically at pack time |
| Permissions | pluggable cannot provide a closed port or claim to be internal |
| Architectural unification | Every user/Channel/worker entry eventually reaches the same Service.Run |
| Eino | Only runtime/provider imports Eino; the public SDK has no Eino types |
| durability | Journal still precedes event visibility |
| tool governance | builtin/plugin/MCP tools pass through the same policy/approval pipeline |
| Lifecycle | Partial startup failure can clean up by owner in reverse order |
| provenance | inspect displays Module, Port, version, trust, Grant, hash, and dependencies |
| runtime freeze | No new Go module can be loaded at runtime; configuration can only activate compiled-in capabilities |
| Compatibility | v0 Tool/Channel/Face plugins can continue to pack through the adapter |

## 10. Main Risks

| Risk | Control |
|---|---|
| “Everything is a plugin” evolves into a second runtime | Kernel-owned Service/Journal/Policy are explicitly non-replaceable |
| Excessive DI / `map[string]any` | Standard Ports are typed; extension Ports share a contract package; generated wiring |
| Public SDK bloat | Each Port has an independent version; internal Ports do not enter the public SDK |
| Manifest self-escalation | Trust assigned by source catalog/recipe |
| In-process plugin mistaken for a sandbox | State it in documentation and inspect; route untrusted code through sidecar/WASM |
| Eino type leakage | Retain import quarantine and adapter |
| Compile-time and runtime switches confused | Generation determines “existence”; settings determine “activation” of compiled-in capabilities |
| One large migration | Metadata/wiring first, then migrate Port by Port; move directories last |
| Old plugins break | v0 compatibility adapter + conformance tests |
| Documentation drifts again | Port catalog generates the manifest schema, inspect schema, and documentation tables |

## 11. Today's Convergence: Four-Layer Completion Model

The `internal/pluggable` distinction above is a trust classification; to answer “what can be missing while it is still Vivy/Agent,” product completion also needs a four-layer necessity model.

### L0: Immutable Kernel

The Kernel defines only physical laws; it does not carry a specific vendor or optional product capability:

- Domain ID, event schema, run state machine;
- single `Service.Run` / `RunWithOptions`;
- Journal authority, durability-before-visibility, exactly-one-terminal;
- final policy / approval / Grant decisions;
- session/workspace identity;
- budget, cancel, recovery, worker authority;
- credential redaction;
- Module/Port catalog, Assembly compiler, Generation provenance;
- ChannelHost, FaceHost, RPC mutation authorization;
- Eino import quarantine.

L0 loss is not “one fewer feature”; it means the system can no longer prove who started a run, who persisted it, who decided, or how it terminated.

### L1: Required Internal Modules

L1 implementations may be replaced, but every valid Generation must select exactly one implementation for each required single Port; missing or ambiguous selections fail at pack time.

| Required Port | Default implementation | Replacement direction |
|---|---|---|
| `core/loop-driver` | Eino ADK | Other controlled loop driver |
| `core/chat-model` | OpenAI-compatible bootstrap | Anthropic, Gemini, DeepSeek, Ollama, etc. through the same Port |
| `core/storage-engine` | SQLite | Postgres or a later controlled backend |
| `core/checkpoint-store` | Vivy versioned blob bridge | Internal backend compatible with the same envelope |
| `core/sandbox-backend` | Current workspace/sandbox implementation | OS-specific internal implementation |
| `core/credential-resolver` | env/config reference | OS vault or controlled credential backend |
| `core/face` | Explicit selection of Web / TUI / Headless in the recipe | Other implementation satisfying the Face contract |

Therefore, “the core is missing, so even the loop cannot run” should be implemented as: L0 always exists, and L1 must satisfy cardinality in the recipe; it does not mean permanently welding the specific Eino, OpenAI, or SQLite code into the Kernel.

### L2: Optional Internal Organs

After L2 is removed, the minimal model/tool loop still runs, but the default product will clearly no longer resemble a complete Agent. Enable these in the default species; minimal/embedded recipes may remove them:

- Skill middleware and Skill source host;
- MCP runtime/transport adapter;
- context compaction;
- ToolSearch / deferred tools;
- AGENTS.md/project instruction loader;
- filesystem/execute/HTTP/fetch/download world;
- cron scheduler;
- memory/retrieval;
- title generator;
- LSP/diagnostics;
- child-agent/worker capability;
- audit/telemetry observer;
- context source pipeline.

Keep the boundaries explicit:

- The MCP adapter is optional internal; an MCP endpoint/server is configuration, not a local plugin;
- Skill middleware/source host is optional internal; a specific `SKILL.md` is a data asset, not a code plugin;
- the compaction strategy is optional, but durable compaction truth, token accounting, and checkpoint compatibility still follow Kernel standards.

### L3: Pluggable Alliance Products

L3 consists of ecosystem products that users select as needed and that can be developed and released independently:

- Anthropic, Gemini, DeepSeek, Ollama/local model providers;
- OpenAI official OAuth, OpenAI Codex OAuth, Azure OpenAI;
- image generation/editing, video generation, TTS/STT;
- Web search, browser, RAG/vector providers;
- third-party Channel, Memory, Skill source, IDE/LSP integration;
- A2UI protocol/renderer;
- Web/TUI panels, cards, actions;
- notification/delivery providers.

L3 can use the same Module/Port/Generation standard as L2, but cannot obtain L0 authority or declare itself internal through the manifest.

## 12. OpenAI, Anthropic, and OAuth Boundaries

Do not define the entire OpenAI provider as an immutable Kernel. The correct split is:

1. The Kernel fixes the standards: ChatModel Port, streaming/tool-call contract, route identity, provider-native model ID, usage accounting, credential handle, retry/cancel/error classification;
2. Required Internal in the default Generation: `builtin/openai-compatible`, ensuring it works out of the box;
3. Plugin Alliance: Anthropic, OpenAI official OAuth, OpenAI Codex OAuth, Azure OpenAI, and other provider/auth products.

OAuth plugins may provide `AuthFlow`, `TokenRefresh`, `CredentialSource`, and `ProviderProfile`, but cannot own secret authority. Token storage, scope, refresh serialization, redaction, and route freeze remain controlled by the Kernel/Required Internal.

Therefore the more accurate product rule is:

> OpenAI-compatible is the default indispensable baseline implementation; what is immutable is the Provider standard and governance, not any particular vendor's implementation.

Anthropic should migrate into the Plugin Alliance under the same ChatModel/Provider Port, rather than remaining a special case in the app composition root.

## 13. Reserved Standards for UI, TUI, and A2UI

The current Face solves “replacing the whole face,” but does not yet solve plugins contributing local UI to an existing Web/TUI Face. Two levels of extension must be reserved before full completion.

### 13.1 Cross-Face Presentation Port

First define declarative, platform-neutral contributions:

- card, list, table, status;
- form, action, notification, progress;
- detail view, artifact/media preview.

Plugins output a typed ViewModel; Web/TUI render it independently. Thus image generation, MCP status, Provider login, and plugin settings need not carry arbitrary React and TUI code at the same time.

### 13.2 Platform-Specific Slots

When declarative capabilities are insufficient, open `ui/web-slot` and `ui/tui-slot`. Reserve:

- navigation;
- session-sidebar / run-sidebar;
- settings/provider / settings/plugin;
- message-attachment / tool-result;
- status-bar / command-palette;
- inspector / modal.

FaceHost must own slot key, cardinality, owner, lifecycle, action→RPC mapping, authorization, session scope, cleanup, and renderer failure isolation. Plugins must not arbitrarily register RPC routes, directly change Journal/session state, inject host DOM, or seize the global TUI keyboard.

### 13.3 A2UI

A2UI is an independent alliance product, not the Vivy UI Kernel:

- `a2ui-protocol`: schema/decoder;
- `a2ui-web-renderer`: Web renderer contribution;
- `a2ui-tui-fallback`: downgrade complex components to table/form/text/action.

Image generation provides media capabilities and artifacts; A2UI provides dynamic declarative interaction. They can be combined, but neither may become a required dependency of the other.

## 14. Vivy Plugin Standard Deliverables

For plugins to become a new Vivy standard, the documentation specification and machine-executable specification must be delivered together.

| Specification | Constraints |
|---|---|
| `VIVY-MODULE-STANDARD.md` | identity, internal/pluggable, provides/requires, conflict, Generation, runtime freeze |
| `VIVY-PORT-STANDARD.md` | Definition/Provider/Consumer, naming, cardinality, scope, version, public/internal/closed |
| `VIVY-PLUGIN-LIFECYCLE.md` | Describe/Construct/Start/Ready/Stop/Close, owner cleanup, rollback |
| `VIVY-PLUGIN-SECURITY.md` | trust assignment, Grant, secret/fs/process/network, in-process/sidecar/WASM |
| `VIVY-UI-EXTENSION-STANDARD.md` | Face, Presentation, Web/TUI slot, action, state, renderer, A2UI adapter |
| `VIVY-PLUGIN-DEVELOPER-GUIDE.md` | create, verify, test, pack, inspect, publish, upgrade, diagnose |

The machine-executable portion must include at least:

- `vivy.module/v1` JSON Schema;
- generated Port catalog;
- manifest/import/grant validator;
- conformance test kit;
- reference plugins;
- compatibility matrix;
- inspect output contract.

Without a conformance suite, a “standard” is only a recommendation document and cannot form an ecosystem.

## 15. Difficulty and Effort Estimate

### 15.1 Baseline and Assumptions

Local census on 2026-09-08: the relevant scope contains about 393 Go files / 65,127 lines of Go and 372 TypeScript/TSX files / 40,952 lines of TypeScript/TSX, for a total of 832 related source/configuration files, including 275 test files. Estimates are in person-days for one senior engineer familiar with Vivy, excluding major midstream requirement changes, external marketplace/cloud services, and long-term ecosystem operations.

### 15.2 Itemized Estimate

| Work package | Person-days |
|---|---:|
| Standard freeze | 6–10 |
| Assembly compiler, typed wiring, v0 compatibility | 12–18 |
| Optional Internal migration | 15–25 |
| Required Internal porting | 20–30 |
| Provider Alliance and OAuth baseline | 10–16 |
| UI/TUI plugin reservations and A2UI foundation | 20–35 |
| Reference products such as image capabilities and ecosystem release | 18–30 |
| Conformance, security, documentation, and release cross-cutting work | 12–20 |
| **Total for full completion** | **113–184** |

### 15.3 Deliverable Definitions

| Definition | Scope | Estimate |
|---|---|---:|
| Specification proposal | Documentation, Port catalog, manifest v1, migration matrix, UI/TUI reservations | 6–10 person-days |
| Plugin foundation v1 | The above specifications + Assembly compiler + behavior-preserving migration of existing Tool/ToolWorld/Channel/Face | 26–40 person-days |
| Publishable Plugin Standard v1 | Also includes Optional/Required Internal, Provider Alliance baseline, and major conformance | 69–109 person-days |
| Full PLUGINS completion | Also includes UI/TUI, A2UI, OAuth, image and other reference products | 113–184 person-days |

One senior engineer completing the full scope would take about 5.7–9.2 months. With Songbai supervision and 2–3 isolated Codex lanes, accounting for the fact that the composition root, Port contracts, and migration waves cannot be fully parallelized, the realistic calendar estimate is about 4–6 months.

Difficulty assessment: the specification itself is medium difficulty; the plugin foundation and existing-seam migration are already refactoring-grade; full completion is a large architectural project spanning runtime, SDK, provider, storage, build, Web UI, and TUI. It is not a rewrite from scratch because the existing verify/pack/Generation/SDK/ChannelHost/FaceHost provide a foundation; it also cannot be implemented as a big bang, and should migrate Port by Port through P0–P5.

The figures above are range estimates, not scheduling commitments. Before formally starting, freeze the standard, define the first-phase acceptance boundary, and then schedule according to the dependency graph.

## 16. Final Judgment

Vivy can achieve an advanced version of “everything is a plugin,” but the correct form is neither Cordis's runtime hot tree, Hermes's multiple registries, nor forcing every capability into one `Plugin` interface.

The correct form is:

> **The Kernel is the non-replaceable physical law; internal/pluggable are organs that follow the same assembly protocol with different permissions; Eino is the orchestration engine inside the loop organ; Generation is the single source of composition truth.**

The first step should not refactor `engine.go`, but define the Module/Port/Assembly Compiler contract; then use the existing Tool/ToolWorld/Channel/Face for a behavior-preserving migration to prove the metamodel, and only then open Provider, middleware, context, and UI slot ports.

This round completes only the research and architecture proposal; full PLUGINS completion is not authorized or formally scheduled, and no source or test changes have been implemented.

## Sources

[1] https://github.com/cloudwego/eino/tree/v0.9.13 — cloudwego/eino v0.9.13
[2] https://github.com/NousResearch/hermes-agent/tree/cf328723d43d101a99fa27b9f358d0f336f4e17f — NousResearch/hermes-agent cf328723
[3] https://github.com/deepseek-ai/deepseek-harness/tree/cd5ef8148158c3a752a658978873241fdf8e2bbc — deepseek-ai/deepseek-harness cd5ef814
[4] https://github.com/RightNow-AI/openfang/tree/acf2587e46be174c10200489c9a2d23a39a98aeb — RightNow-AI/openfang acf2587e
[5] https://github.com/zeroclaw-labs/zeroclaw/tree/d91e08eaefc4750fbd59a4d98f0adcfe0e785b41 — zeroclaw-labs/zeroclaw d91e08ea
[6] https://github.com/openclaw/openclaw — openclaw/openclaw
[7] https://github.com/earendil-works/pi/tree/6564d9471702727141e20b305d17679e06373e57 — earendil-works/pi 6564d947
