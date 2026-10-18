# Architecture boundaries and decision record — PG-1

## Reused authority

The simplest adequate change keeps UI behavior in `vivy/notebook` and `vivy/workflow-ui`, browser plumbing in repository scripts/config, and Toolkit verification beside existing owners. The alternative of independent report/workflow services or a new generic launcher framework adds ownership and maintenance without serving these gaps. No new runtime, Port, scheduler, Journal, permission model, storage owner or plugin loader is needed. No runtime performance gain is claimed; UI control reuse and focused deterministic fixtures avoid repeated inference and unnecessary I/O.

Normative references: [Module standard](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/docs/architecture/VIVY-MODULE-STANDARD.md), [Port catalog](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/docs/architecture/VIVY-PORT-CATALOG.md), [Plugin spec](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/docs/architecture/VIVY-PLUGIN-SPEC.md), [Assembly](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/docs/architecture/VIVY-ASSEMBLY.md). Existing reports, workflow and Toolkit documents linked in source-evidence remain their product contract owners.

| Decision | Concrete rule | Failure/stop behavior |
| --- | --- | --- |
| PG-AD1 Report UI reuse | Mount existing NotebookReports with entry=null when no selection, through existing generation/settings actions; reuse generated-result refresh | No direct backend writes, report seeding or Notebook redesign; absent reports capability removes controls |
| PG-AD2 Binding home | Read/write `Node.inputs?: Record<string, Binding>`; task/tool_names remain in config | Invalid JSON shape never mutates artifact; unknown config fields remain backend-rejected, no permissive decoder |
| PG-AD3 Launch authority | Eligible parent has active session equal to parent.session_id and status=active, with backend revalidation | Idle, terminal/history, missing/foreign parent are unavailable; no old-run fallback or fabricated parent |
| PG-AD4 Test plumbing | One `just toolkits-e2e` owns preparation and two existing suites, supports explicit isolated ports and native executable suffix | Occupied/invalid ports fail; never steal processes or reuse an unrelated server; no assert suppression |
| PG-AD5 TK-13 delta | Reuse existing A01–A28/B1–B14 requirements and proof, extend only uncovered scenarios | Missing evidence remains blocked/unverified; no second matrix redefining semantics |
| PG-AD6 Status ownership | D1 queues exact corrections for centralized landing, preserves historical evidence | Do not edit public indices/status or claim feature completion in this branch |

## Interface/ownership inventory

- `vivy/notebook` existing T2 UI Module provides `std/ui-extension@v1` to PresentationHost; reports actions run through existing ActionHost / `std/control-action@v1` providers and trusted report runtime/storage. Selected UI needs no additional UI Grant.
- `vivy/workflow-ui` existing T2 UI Module remains optional and session-bound; `FaceBridge.call<T>(method, params)` is the existing transport seam. `WorkflowClient.startRun({workflow,revision?,draft_etag?,input?})` consumes canonical artifacts and existing product RPC.
- `Service.INOFYStartRun(ctx,sessionID,INOFYStartRunParams)` verifies session ownership, then `startINOFYWorkflow` calls `currentChildAuthorizer`. The latter requires active status, WorkerParentAuthority and live tool ceiling. Browser checks improve feedback; they cannot authorize a run.
- Toolkit content remains data interpreted by Skill/Mask/Rules/Hook/MCP owners; no new content publishing or runtime executable loading. Core Storage remains the only schema/migration authority. No schema change is planned.
- Test setup may exercise real authority with disposable fixtures; it cannot establish user privileges through browser assertions or hand-insert a fake production parent Run.

## Eino capability check before any code

Inspected pin: `github.com/cloudwego/eino v0.9.13`, EinoExt MCP `v0.0.9`, OpenAI `v0.1.13`, Claude `v0.1.25`; INOFY `71e2c9bbe47d`. These are the inspected main pins, not the unmerged #80 candidate.

`internal/runtime/engine.go` already uses `adk.NewChatModelAgent`, `adk.NewRunner` and `compose.ToolsNodeConfig`. `internal/runtime/mcpadapter.go:132` uses `eino-ext/components/tool/mcp.GetTools(ctx,&Config{Cli:...})`. Report execution and INOFY child tasks already enter that runtime/provider seam. R1/W1/W2 add no agent loop, MCP conversion or checkpoint implementation; T3 tests the existing adapter. Eino compose graph availability does not authorize replacing INOFY, the sole workflow engine.

At implementation kickoff inspect those pinned actual packages again before any orchestration/adapter modification. If testing reveals a missing upstream adapter capability, mark that separate capability DEFERRED-INDEFINITE and stop its implementation; do not add custom transport/model substitutes. Eino imports stay only in internal/runtime and internal/provider.

## Scope and change invalidation

A public admission change, weaker acceptance, new tool/Port or storage semantics change invalidates the affected plan and requires a concrete owner decision. Local assertion/fixture placement and UI composition may be revised within these boundaries with an evidence note. A schema, authority, or executable-consent defect found by T1–T3 is reported with a minimal regression; production remediation is a separately authorized follow-up, not hidden inside a verification Story.
