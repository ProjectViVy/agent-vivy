# SCX Plugin Integration Map

Date: 2026-09-12
Status: initial design mapping; implementation unscheduled; no Gate passed

[Architecture direction](SCX-ARCHITECTURE-DESIGN.md) defines the design.
[Module Standard](VIVY-MODULE-STANDARD.md), [Port Catalog](VIVY-PORT-CATALOG.md)
and [P8](../plans/plugin-platform/PLG-P8-scx-integration-gates.md) retain authority.
Names below map responsibilities; no new selectable Port or public API is created.

## Capability and authority mapping

| Capability | Provider / contract | Sole consumer and authority | Scope and lifecycle | Failure and conformance owner | Gates |
| --- | --- | --- | --- | --- | --- |
| Context candidates: history, RAG, personality, emotion | Selected `std/context-source@v1` Provider; richer reference shape under review | ContextHost validates and selects; Runtime alone adapts model input | Workspace/session identity plus authorized user/tenant scope; per call; state version/expiry | Required input fails preparation; optional source reports unavailable; ContextHost contract suite | A/B |
| Resource resolution and file/media representations | Existing internal resource adapters or approved scoped source facade; no new public Port | Owning internal resource authority; ContextHost selection and Runtime adaptation | Versioned resource/fragment, scoped read, bounded payload | Version mismatch or unsupported modality is explicit; resource and Runtime suites | A/B/C |
| Context View and retention policy | T1 internal policy; not a public Provider | ContextHost selects, Runtime finalizes; Kernel owns durable association | One immutable effective view per model-call attempt | Required budget overflow rejects; optional omission recorded; Host/Runtime suites | A/B |
| Model-visible history/file retrieval | Namespaced `std/tool@v1`, or existing protected T1 Tool | ToolHost owns dispatch, policy and bounds | Request scope, deadline and result limits | No direct execution bypass; ToolHost suite | A/B |
| Memory experience return | `std/observer/run@v1` | ObserverHost supplies ordered redacted committed projections and Host-managed cursor | Explicit recipient scope; at-least-once; receiver deduplication | Backlog does not rewrite Run outcome; ObserverHost plus adapter receipt tests | A/B/C |
| Ephemeral diagnostics | `std/observer/diagnostic@v1` | ObserverHost | Bounded process-local observation | Best effort is not reliable experience delivery; ObserverHost suite | A/B |
| Remember/correct/forget and configuration | `std/control-action@v1` | ActionHost; Kernel authorization remains authoritative | Operation identity, target scope, accepted/pending/completed receipt | No false completed deletion; ActionHost plus adapter tests | A/B/C |
| State/coverage/receipt inspection | `std/status-source@v1`; existing UI presentation if needed | StatusHost; UI has no durable authority | Per capability/operation, redacted | Distinguish query health, backlog and deletion progress; StatusHost suite | A/B/C |
| Expensive derivation or subagent analysis | Existing T1 task/worker mechanism; published result resource | Existing Service/worker ownership; Eino adapter quarantine | Explicit task lifecycle, cancellation and source lineage | Queries cannot secretly spawn unbounded work; task/Runtime suite | A/B |
| MCP context/tools | Existing T3 instance through MCPHost and cataloged Sources/ToolWorld | MCPHost -> ContextHost or ToolHost | Configured instance, no automatic network activation | Existing transport and Host failures; MCP conformance | A/B |
| Skills | `std/skill-source@v1` | SkillHost | Scoped versioned skill projection | No implicit tool authority; SkillHost suite | A/B |
| Future embodied observations | Typed resource extension; no new device Port selected | Future adapter owner must be mapped before implementation | Observation time, expiry, frame/units, data gaps | Contract examples only now; real-device acceptance follows original plan | A now; B/C only if selected later |

Public Modules receive no raw Journal, storage, credentials or Eino objects.
External memory may own its own database and jobs; its Vivy connector follows
compiled Module/Host and instance rules. Personality importance never grants
instruction authority. Local read permission never implies permission to export.

## Hook boundary

Two proposed initial phases: bounded preparation before each model call, and
committed Run-terminal event consumption. Preparation remains internal to the
ContextHost/Runtime path until a cataloged extension is reviewed. Terminal
observation reuses Run Observers. The only existing public execution-changing
Middleware remains `std/middleware/pre-tool@v1`; do not overload it for model hooks.
No mutable global Context, dynamic plugin loading, or parallel event bus is added.
Module removal requires a new Generation; instance deactivation does not mutate
the frozen graph. Notifications cannot veto; checks cannot grant new authority.

## Evidence ledger

| Transition | Required evidence | Current record |
| --- | --- | --- |
| Original SCX stage -> Gate mapping | Recovered owner-maintained stage IDs | Unresolved; no invented IDs |
| Gate A | Exact P1-P4 evidence commits and versions; fake-provider compile and forbidden-dependency checks | Not assembled by this documentation update |
| Gate B | P2-P7 evidence including required P5; actual selected SCX path, default/minimal behavior and failure cases | Not executed; P3/P7 completion is owner-reported, not reverified here |
| Gate C | P9 artifacts, supported Port evidence, deterministic rebuild and exercised rollback | Not executed |
| Eino capability decision | Repository-pinned API inspection for each scoped implementation | Required before implementation planning/code; missing capabilities defer under the normative rule |
| External systems and hardware | Adapter/device evidence for selected scope | No live environment used; no support claim |

## Bounded local validation

Use architecture section 11 fixtures A-C: versioned personality/emotion with a
controlled clock; predetermined text-file retrieval with a version race and budget
case; committed memory return with deduplication, outage and export filtering.
Fixtures are unexecuted specifications. They do not prove model quality, real
laputa-garden compatibility, media processing or device behavior. Multimodal,
subagent and embodied examples only check representation at this stage.

## Open implementation inputs

Tracked in [the living board](../TODO.md): original stage recovery, current SDK
representation fit, Eino reuse mapping, and ObserverHost/ActionHost receipt wiring.
No separate scheduler, storage framework or universal hook engine is authorized.
