# SCX Plugin Integration Map

Date: 2026-09-13
Status: Gate A passed; Gate B blocked on the unrecovered SCX stage map and selected-slice contract fit; Gate C not passed

[Architecture direction](SCX-ARCHITECTURE-DESIGN.md) defines the design.
[Module Standard](VIVY-MODULE-STANDARD.md), [Port Catalog](VIVY-PORT-CATALOG.md)
and [P8](../plans/plugin-platform/PLG-P8-scx-integration-gates.md) retain authority.
Names below map responsibilities; no new selectable Port or public API is created.

## Capability and authority mapping

`Provider` below always means the typed provider selected by a frozen Recipe;
it never means an unconstrained “SCX service” or a runtime-discovered plugin.
T2 Modules receive only the effective intersection of requested, Port-allowed,
and Recipe-approved Grants. T1 Modules remain on the same Host path.

| Capability | Module / Provider and Port | Sole consumer and authority | Cardinality, Trust, and Grants | Data scope and lifecycle | Failure and conformance owner | Gates |
| --- | --- | --- | --- | --- | --- | --- |
| Context candidates: continuity, RAG, personality, emotion | T1 `vivy/context-source` or a selected T2 `contextsource.Provider`; `std/context-source@v1` | T1 `vivy/context-host`; ContextHost validates/selects and Runtime alone creates model input | `0..n`; T1/T2; Port allows scoped `fs.read`, `secret.read`, `rpc.client`, and `net.client` | Authorized workspace/session/request; bounded query per model-call preparation; version/freshness travel with candidates | Optional failure is visible and may be omitted; required semantics and expiry enforcement are Gate B work; `internal/contexthost` suite | A/B |
| Resource resolution and file/media representations | Gate A supports only inline plaintext in `contextsource.Candidate`; there is no typed resource reference or resolver Port | ContextHost validates inline candidates; `internal/runtime` alone adapts supported content into model input | Inherits `std/context-source@v1`; no bearer permission or extra Grant is implied | `SourceID`, `ContentID`, and `Version` label inline content; exact-version byte resolution, access scope, and replay are not implemented | Gate B/C is contract-blocked if this slice is selected until a typed resolution seam and Host-level version-race, scope, bounds, and replay tests exist | A classification; B/C only after contract fit |
| Per-call Context View and retention policy | T1 `vivy/context-host`; closed `core/context-host@v1`; no public Provider | ContextHost owns selection; Runtime owns final Eino projection; Kernel storage/Journal owns any durable association | `0..1`, required when a Context Source exists; T1 only; no public Grant | Immutable effective selection per model-call attempt, scoped to workspace/session/request and budget | Required overflow rejects; optional omission is deterministic; ContextHost/Runtime suites own Gate B proof | A/B |
| Durable memory/index authority | Any future SCX memory/index backend remains closed internal code behind T1 `vivy/storage`; `core/storage-engine@v1`; no public memory/index Port is selected | Kernel storage/Journal contracts retain identity, provenance, transaction, retention, and migration authority | Exactly one T1 Storage Engine per Generation; no public Grant or direct Provider access | Any selected slice must propagate workspace/session/tenant identity and define retention, deletion, and replay scope | Existing storage conformance owns durable invariants; a selected SCX backend requires its own migration, isolation, failure, and recovery tests | A classification; B only if selected |
| Model-visible history/file retrieval | Existing protected T1 Tool or selected T2 `tool.ToolProvider`; `std/tool@v1` | T1 `vivy/tool-host`, then the one Runtime Policy/approval/Journal path | `0..n`; T1/T2; scoped Tool Grants only; protected Tool IDs stay T1-reserved | One governed invocation with workspace/run/session identity, deadline, schema and result bounds | Any direct execution path fails architecture checks; ToolHost whole-envelope conformance | A/B |
| Dynamic retrieval catalog | Selected T2/T3-backed `toolworld.Provider`; `std/tool-world@v1` | T1 `vivy/tool-host` owns discovery, schema identity, visibility and dispatch | `0..n`; T2 in-process adapter or T3 external instance; scoped Tool Grants | Frozen Provider binding; bounded runtime discovery for an already configured instance | Schema drift/unavailable discovery fails closed; ToolWorld/ToolHost conformance | A/B |
| Memory experience return | A future selected T2 `observer.RunProvider`; `std/observer/run@v1` is only a candidate seam | T1 `vivy/observer-host` currently supplies committed, ordered, redacted events and owns cursor state, but fans every event to every selected Run Provider | `0..n`; T1/T2; Port allows scoped `fs.write`, `rpc.client`, and `net.client` | Current ABI has no recipient/event subscription or allowed-field projection; at-least-once delivery and stable event IDs exist | Gate B/C is contract-blocked if export is selected until Host-owned subscription/filter policy, field projection, deduplication, outage, and receipt tests exist | A classification; B/C only after contract fit |
| Ephemeral diagnostics | Selected `observer.DiagnosticProvider`; `std/observer/diagnostic@v1` | T1 `vivy/observer-host` | `0..n`; T1/T2; same Port Grant ceiling as run observers | Bounded process-local diagnostic lifetime; not a durable experience stream | Best effort with visible drops; ObserverHost conformance | A/B |
| Remember/correct/forget and configuration | Selected T2 `controlaction.Provider`; `std/control-action@v1` | T1 `vivy/action-host`; Kernel authentication, Policy, and audit remain final | `0..n`; T1/T2; current Port permits `rpc.client`; any external effect also needs a separately authorized Host capability | Typed operation identity and target scope; accepted/pending/completed/failed receipt lifecycle | Invalid schema/authority or unavailable instance fails closed; ActionHost and selected-adapter receipt suites | A/B/C |
| State/coverage/receipt inspection | Selected `status.Provider`; `std/status-source@v1`; optional existing UI presentation | T1 `vivy/status-host`; UI has no durable authority | `0..n`; T1/T2; read-only `rpc.client` ceiling | Redacted bounded snapshot per capability/operation; read performs no activation or probe | Query health, delivery backlog, and deletion progress remain distinct; StatusHost suite | A/B/C |
| Expensive derivation or subagent analysis | Existing T1 Service/task/worker mechanism; output may later be exposed by a Context Source | Existing Service/worker authority; model conversion remains inside Eino quarantine | Closed internal ownership; no selectable Port or new Grant | Explicit task/run identity, cancellation, bounds, and source lineage; never hidden inside a query | Query cannot spawn unbounded work; task, worker, and Runtime suites | A/B |
| MCP context/tools | T1 `vivy/mcp-host` (`core/mcp-host@v1`) owns T3 instances and provides reserved `mcp` `std/tool-world@v1`; explicit resource bridge produces Context Source data | MCPHost -> ToolHost for Tools; MCPHost -> ContextHost for opted-in Resources | Host `0..1`, ToolWorld `0..n`; T1/T3; instance transport/credential bounds; no automatic network activation | Configured instance/session lifecycle; unconfigured is inactive; remote process remains outside the Generation | Unavailable transport/schema is explicit and cannot bypass either Host; MCPHost, ToolHost, and ContextHost suites | A/B |
| Skills | T1 `vivy/skill-source` or selected T2 `skillsource.Provider`; `std/skill-source@v1` | T1 `vivy/skill-host` | `0..n`; T1/T2; scoped `fs.read`, `rpc.client`, or `net.client` | Versioned skill content and activation scope within the frozen Generation | Loading grants no Tool authority; SkillHost conformance | A/B |
| Future embodied observations | Future T1/T2 Context Source using reviewed typed resource metadata; no device Port selected | Future adapter -> ContextHost -> Runtime; device safety remains outside this Gate | Inherits `std/context-source@v1` only after contract-fit proof; no implied device Grant | Observation time, expiry, frame/units and missing-sample semantics; real-device lifecycle follows the recovered SCX plan | Representation review only in Gate A; real-device conformance remains unclaimed until selected | A now; B/C only if selected later |

Public Modules receive no raw Journal, storage, credentials or Eino objects.
External memory may own its own database and jobs; its Vivy connector follows
compiled Module/Host and instance rules. Personality importance never grants
instruction authority. Local read permission never implies permission to export.

## Hook boundary

Two proposed initial phases: bounded preparation before each model call, and
committed Run-terminal event consumption. Preparation remains internal to the
ContextHost/Runtime path until a cataloged extension is reviewed. A future
terminal export may reuse Run Observers only after a Host-owned subscription
and allowed-field projection contract is defined. The only existing public execution-changing
Middleware remains `std/middleware/pre-tool@v1`; do not overload it for model hooks.
No mutable global Context, dynamic plugin loading, or parallel event bus is added.
Module removal requires a new Generation; instance deactivation does not mutate
the frozen graph. Notifications cannot veto; checks cannot grant new authority.

## Evidence ledger

| Transition | Required evidence | Current record |
| --- | --- | --- |
| Original SCX stage -> Gate mapping | Recovered owner-maintained stage IDs | Unresolved after repository history and branch search; no IDs invented; blocks Gate B slice selection, not Gate A evidence assembly |
| Gate A | Exact P1-P4 evidence commits and versions; fake-provider compile and forbidden-dependency checks | **PASSED 2026-09-13**; initial tests in `271d7c367cc19966bc67d01724d36365e8c07dc8`, with sole-consumer/compiler and source-firewall hardening in `9d2adff0df61bc13147f6eee73e3909a3f557617` |
| Gate B | P2-P7 evidence including required P5; actual selected SCX path, default/minimal behavior and failure cases | Not executed; waits for the recovered stage mapping and contract fit for the selected slice |
| Gate C | P9 artifacts, supported Port evidence, deterministic rebuild and exercised rollback | Not executed |
| Eino capability decision | Repository-pinned API inspection for each scoped implementation | Pinned v0.9.13 projection and Retriever APIs are classified below; any selected adapter is reassessed before Gate B code |
| External systems and hardware | Adapter/device evidence for selected scope | No live environment used; no support claim |

### Exact Gate A platform evidence

All listed contract and Port versions are the frozen versions consumed by Gate A.
The delivery SHA identifies the implementation evidence; the merge SHA proves
the same evidence landed on the program baseline.

| Phase | Delivery evidence | Merge evidence | Frozen Gate A contracts and Ports | Executable evidence |
| --- | --- | --- | --- | --- |
| PLG-P1 | `0033d115c740f2fd80e22094b2bf274959faaeb2` | `659ec1499c52137baa1d86354bf6d8d863c528d6` | Descriptor `vivy.module/v1`; Recipe `vivy.generation/v1`; compiler and public Port catalog | `sdk/internal/assembly/compiler_test.go`, `sdk/internal/assembly/evidence_test.go`, `sdk/module/descriptor_test.go`, `sdk/port/catalog_test.go` |
| PLG-P2 | `0033d115c740f2fd80e22094b2bf274959faaeb2` | `659ec1499c52137baa1d86354bf6d8d863c528d6` | `std/tool@v1`, `std/tool-world@v1`; generated default/minimal Generation parity | `sdk/internal/frontend_v1_test.go`, `sdk/internal/assembly/runtime_generate_test.go`, `internal/app/default_generation_test.go` |
| PLG-P3 | `fd3cb50e5a50e8c3d4a2b7c1bf75395886c9163b`; approval-resume correction `133743db87bd69030d957c4327011c920a4df4a3` | `a6e80885fdc5a0fd754f137c5c8dc7c42868faf5` | `core/tool-host@v1`, `std/tool@v1`, `std/tool-world@v1`, `std/middleware/pre-tool@v1`; protected Tool IDs and one governed execution envelope | `internal/app/assembly_governance_e2e_test.go#TestToolEnvelopeConformanceAcrossAllSourceClasses` and the focused ToolHost/Runtime/Assembly suites recorded in `docs/logs/2026-09-12-plugin-v1-p3-closure/` |
| PLG-P4 | `ae17cddb637f9f73a3486febf4dc66fa178c77d6`; CI closure `a4f8022a12dcea8196dffb4f1c28c27447a7d803` | `c331467d85e2c93bf2fcc9b4e920646402d2bc10` | `std/context-source@v1`, `std/skill-source@v1`, `core/context-host@v1`, `core/skill-host@v1`, `core/mcp-host@v1`; MCP supplies reserved `mcp` on `std/tool-world@v1` | `sdk/internal/assembly/p4_conformance_test.go`, `internal/contexthost/conformance_test.go`, `internal/skillhost/conformance_test.go`, `internal/mcphost/conformance_test.go` |

### Representation and pinned Eino fit

`contextsource.Candidate` carries stable `SourceID`/`ContentID`, media type,
inline content/size, confidence, a version label, update time, and metadata.
That is sufficient only for the Gate A inline-plaintext fake. It has no typed
resource reference, access scope, replay contract, or exact-version resolver,
so it cannot prove a predetermined file-version race or generic multimodal
loading. Required/expiry semantics, durable Context View persistence, and
external memory receipt are likewise unproved. A selected Gate B slice must
first close the applicable contract gap and then add Host-level tests.

| Need | Pinned Eino v0.9.13 evidence | Decision for Gate A |
| --- | --- | --- |
| Final model input | `schema.Message`, `schema.MessageInputPart`, and `schema.UserMessage` in `schema/message.go` | **ADAPT existing path**: `internal/runtime/contextadapter.go` remains the sole ContextHost-to-Eino projection |
| Retrieval component | `components/retriever.Retriever.Retrieve`; `compose.Chain.AppendRetriever` and graph Retriever nodes | **AVAILABLE, NOT SELECTED**: Gate A adds no Eino graph and no public Eino type; reassess only for the named Gate B slice |
| Public SCX Provider ABI | No Eino type is required | **KEEP VIVY CONTRACT**: compile-time fakes implement only `contextsource`, `skillsource`, `tool`, and `toolworld`; the compiler rejects public-Port consumers other than the cataloged build-owned T1 Host, and the source firewall rejects direct Runtime, Eino, and concrete local Module imports |

## Bounded local validation

Gate A executes a versioned plaintext Context candidate plus fake public
Providers, governed graph compilation, Trust/Grant/lifecycle checks, protected
Tool rejection, public-Port sole-consumer enforcement, and direct Runtime/Eino/
concrete-Module import rejection. Architecture section 11 fixtures A-C remain
Gate B specifications: personality/emotion with a controlled clock;
predetermined text-file retrieval with a version race and budget case; committed
memory return with deduplication, outage, and export filtering. The latter two
are contract-fit inputs, not capabilities supported by the current ABI. Gate A
does not prove model quality, real laputa-garden compatibility, media processing,
or device behavior.

## Open implementation inputs

Tracked in [the living board](../TODO.md): original stage recovery and contract
fit for any selected typed-resource/exact-version or observer-export slice,
including ObserverHost/ActionHost receipt wiring. Gate A freezes and proves the
existing v1 boundaries without selecting a product slice. Gate B must not
implement an unnamed stage or claim an unfit current contract. No separate
scheduler, storage framework, or universal hook engine is authorized.
