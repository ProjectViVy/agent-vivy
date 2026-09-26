# SCX Architecture Design v0.1

Date: 2026-09-12
Project: ProjectViVy/agent-vivy
Status: architecture direction recorded from owner decisions; interface details remain under review; implementation and release are not scheduled by this document

## 1. Purpose and provenance

SCX must accommodate changing context pipelines: conversation continuity, retrieval, files, multimodal resources, subagent analysis, external memory systems, personality, emotion, and future embodied observations. Its architecture must remain economical to implement and operate.

This document consolidates the current conversation. It does not reconstruct the missing original SCX specification or invent an expansion of the SCX acronym. The original stage plan has not been recovered. The current authority mapping and evidence ledger are in [SCX-PLUGIN-INTEGRATION.md](SCX-PLUGIN-INTEGRATION.md). The original plan’s real-device adaptation and testing sequence remains authoritative when recovered; this document neither replaces it nor invents stage identifiers.

The user reports P3 and P7 complete. This statement is planning input, not a fresh verification of merged commits or gate evidence. No code, external integration, hardware test, or automated test was executed for this document.

### Agreed direction

- Preserve a broad context architecture; memory and continuity are use cases, not the entire scope.
- Keep summarization, original-fragment retrieval, and hybrid approaches replaceable.
- Provide architectural cooperation with laputa-garden without assuming its current API or internals. Do not model the design after memU.
- Address importance, personality/emotion state, feedback timing, and lifecycle hooks.
- Reuse existing Vivy authority, persistence, and task mechanisms.
- Use small local fixtures and fake providers for initial contract validation. No real-device environment is currently available. Do not build an extra hardware or simulation test platform.

Concrete names, schemas, interface versions, and the proposed initial hook surface below remain review proposals.

### Repository integration constraints

This review draft is subordinate to [the Module Standard](VIVY-MODULE-STANDARD.md),
[the Port Catalog](VIVY-PORT-CATALOG.md), and [the P8 plan](../plans/plugin-platform/PLG-P8-scx-integration-gates.md).
It does not register new public Ports or replace the separate P8 integration evidence document.
Public contributions must use cataloged Ports and their sole Hosts. Event delivery
must be reconciled with ObserverHost, management with ActionHost, and preparation
with ContextHost. No public raw-Journal subscription is authorized.

The frozen Generation graph remains immutable. References to unloading mean
instance deactivation or Generation shutdown/replacement, not runtime code loading
or changing Module registrations inside a frozen Generation.

**Eino capability check:** this consolidation adds no implementation. Before code
or an implementation plan for model execution, context/RAG, callbacks or subagents,
inspect the repository-pinned Eino/EinoExt APIs and record the concrete reuse mapping.
Reuse existing Vivy adapters and Hosts; missing upstream capabilities follow the
normative `DEFERRED-INDEFINITE` rule rather than authorizing custom substitutes.
That capability assessment has not been completed by this draft. The proposed
internal contracts are responsibilities to map, not approved parallel frameworks.

## 2. Central model

**Resources preserve evidence; a Context View selects what one model call receives; committed events drive subsequent updates.**

The stable model has three objects:

| Object | Responsibility | Exclusions |
| --- | --- | --- |
| Resource reference | Identify accessible content, its version, scope, representation, and location | Does not grant access or imply local storage |
| Context View | Record the effective selection for one model invocation | Does not store all history or schedule arbitrary work |
| Committed event | Record a confirmed occurrence with identity and causal linkage | Does not imply successful delivery to an external subscriber |

A candidate is a temporary proposal referring to a resource, optionally with bounded inline content. A receipt records the disposition of an operation or delivery. Neither requires another general-purpose framework.

### Ownership

| Owner | Responsibility |
| --- | --- |
| Existing Kernel/storage contracts | Durable Vivy facts, identities, authorization records, and lifecycle truth |
| ContextHost | Validate and select candidates; enforce scope, retention rules, freshness, and budgets |
| Runtime/model adapter | Convert selected representations into supported model input; finalize the effective view |
| ToolHost | Govern model-initiated retrieval and effectful tool actions |
| Existing task mechanism | Own explicit expensive Vivy processing, cancellation, and execution outcomes |
| External system | Own its storage, indexing, algorithms, and internal jobs |
| UI/Inspect | Explain selections and states; never create durable truth through rendering |

An external memory system may own long-term knowledge. It cannot redefine Vivy's executed actions, current authorization, or terminal run outcomes. Sources cannot write the final prompt or create a second execution path.

## 3. Scope map

| Domain | Architectural accommodation | Initial validation |
| --- | --- | --- |
| Continuity | Recent history, summaries, original fragments, task state | Fixed records; no algorithm selection |
| Memory | Query/read plus authorized experience and management exchange | Fake receiver; no live laputa-garden assumption |
| RAG | Replaceable retrieval, chunking, and ranking behind candidate contracts | Predetermined matches over plain text |
| Files | Resource versions, snapshots, scoped reads, changes | Small local fixture files |
| Multimodal | Typed representations, external references, capability-aware adaptation | Contract examples; no broad media engine |
| Subagents | Task-produced analysis as a derived resource | A prepared result example; no new scheduler |
| Personality/emotion | Scoped versioned state, retention treatment, expiry | Plain-text and structured fixtures |
| Embodied data | Time, validity and typed spatial extensions | Schema examples only; original device plan governs real testing |
| Skills/MCP/tools | Existing governed Hosts and explicit provenance | Preserve existing integration boundaries |

The initial delivery does not include a generic DAG editor, a new agent runtime, a vector database requirement, arbitrary hook scripts, live sensor integration, learned ranking, or an emotion-generation algorithm.

## 4. Resource and candidate semantics

Minimum resource semantics are identity, owning source, version semantics, access scope, representation, and provenance. Fragment locators can describe text lines, document pages, image regions, or time intervals. A source may return bounded inline text or a reference resolved only after selection.

Different representations of one resource must remain distinguishable: an original image, OCR text, thumbnail, and generated description are not interchangeable evidence. Derived resources identify their inputs and processing version when available. Summaries and model interpretations are labeled as derived, not upgraded into direct observations.

Version support is a declared capability. A versioned reference must resolve to that version or fail explicitly. A source that cannot provide stable versions advertises best-effort reads. It must not silently return current content while claiming an old version.

References are not bearer permissions. Access is checked during discovery and actual resolution. If authorization is revoked before use, the selected item is rejected. Export permission is separate from permission to read locally.

### Freshness and domain extensions

The core supports observation time, validity/expiry where applicable, and a versioned extension mechanism. Embodied extensions may carry coordinate frame, units, calibration version, synchronization window, and missing-sample information. These are extension examples, not a claim of device support.

Resource providers retain ownership of their bytes. Large payloads need not enter the Journal. The view records whether evidence is replayable by version, backed by an authorized snapshot, or no longer retrievable. A hash alone does not make a request reproducible.

## 5. Per-call Context View lifecycle

1. Establish task intent, identity/scope, permitted sources, model capabilities, budget, and deadline.
2. Query selected sources for bounded candidates and availability states.
3. Validate scope, provenance, freshness, representation, and version claims.
4. Apply retention rules and deterministic selection; remove demonstrable duplicates.
5. Resolve selected resources and adapt representations to the model.
6. Recheck actual request cost. Remove optional entries in deterministic order within a bounded retry policy; reject when necessary input cannot fit.
7. Finalize an immutable view for that model-call attempt and associate it with request outcome.
8. Commit relevant runtime results and expose authorized events to subscribers.

A Run can contain several model calls and therefore several views. A retry reuses the view only if its inputs and route remain valid; otherwise a new attempt records a new view. Preparation cancellation is not recorded as a model request that was sent. The view distinguishes selected, omitted, adapted, prepared, and sent states where relevant; none implies model understanding or adoption.

State arriving during an in-flight model call normally takes effect at the next call boundary. Urgent interruption uses existing cancellation/steering. It does not mutate the recorded input of an earlier call.

### Cost and determinism

Use a shared request deadline and model-call budget. Candidate estimates are provisional; the adapter owns final capability and cost checks. Media may require limits on count, bytes, duration or other supported units, not only text tokens.

Do not add an LLM ranking call to the default path. Start with explicit quotas, source-local ranking and stable tie breaks. Cache derived work only when input version, processor/config version, scope and freshness permit reuse. Time-dependent state cannot be validated by content hash alone.

## 6. Importance, authority, and state

Keep instruction authority, retention treatment, ordering, relevance, confidence, and freshness separate. Providers may suggest retention; the Host applies configured permissions and policy.

| Treatment | Meaning | Budget failure |
| --- | --- | --- |
| Required | This invocation depends on the content | Reject or explicitly revise the request |
| Reserved allocation | Allocate space to a configured category such as core personality | Use an approved compact representation; otherwise report insufficiency under configured policy |
| Competitive | Select within remaining budget | Omit with an explicit reason |

Ordering is not authority. Scores from different retrievers are not presumed calibrated. Initial deduplication uses resource identity/version and exact fragment overlap, not speculative semantic equivalence.

Core personality is explicitly selected, scoped and versioned. Agent emotion is a time-dependent state. Inferred user emotion is uncertain evidence, not a permanent user attribute. Neither memory nor emotion grants tool permission. A recalled authorization statement cannot replace a valid current authorization record.

Explicit corrections supersede affected derived state where the relationship is known. Ambiguous conflicts retain their evidence rather than silently selecting a winner. Provider state updates are consumed at the next eligible view boundary.

## 7. Two extension channels and hooks

### Request channel

Used for candidate queries, resource reads, state preparation, checks, and explicit management operations. Calls have typed outcomes, scope, deadline and cost bounds. The owning Host decides how proposals affect execution. A source query may perform bounded retrieval but cannot silently launch unbounded model/agent work.

### Event channel

Used for committed experiences, terminal outcomes, invalidation and receipts. Observation listeners cannot short-circuit execution. Reliable external delivery reuses `std/observer/run@v1` and ObserverHost: stable event IDs, ordered redacted committed projections, a Host-managed persistent cursor, and receiver deduplication. Remote completion receipts are a separate integration concern; do not create a second event bus or expose the raw Journal. Process-local notification alone is not reliable delivery.

### Proposed initial lifecycle phases

| Phase | Use | Semantics |
| --- | --- | --- |
| Before model-call preparation completes | Prepare candidates/state and check necessary conditions | Bounded request; Host-controlled application |
| After Run terminal outcome commits | Deliver authorized completion/failure/cancellation evidence | Event consumption; reliability declared per subscriber |

Preparation stays internal until a cataloged extension is reviewed; terminal observation reuses `std/observer/run@v1`. These phases do not add public Ports.

Other stages discussed previously—input commit, tool-result commit, view finalization, correction/deletion and feedback—remain lifecycle design considerations. They are not all new public hooks in the first implementation. Management commands and their receipts still require correct semantics even when no generic public hook is exposed.

Notification and interception must have different signatures. Ordinary observers receive immutable facts and cannot swallow the continuation. Checks can deny only within their authorized role; no hook can grant new execution authority. Any future transformation hook returns an explicit proposal instead of mutating an unrestricted shared Context.

Dependencies and ordering must be deterministic where required; unrelated bounded queries may execute concurrently. Aggregate deadlines, cancellation propagation, recursion limits and causal IDs prevent runaway chains. Unloading stops new registrations and follows existing ownership rules for accepted work; exact drain behavior must be verified against the runtime.

## 8. Bidirectional external-system cooperation

laputa-garden is an intended integration partner, not a presumed protocol. Integration maps its actual capabilities to these contracts.

| Direction | Exchange |
| --- | --- |
| Into Vivy | Personality, memory candidates, emotion snapshots, evidence references and availability |
| Out of Vivy | Authorized committed experiences, terminal results and explicit user feedback |
| Management | Remember/correct/forget requests with operation identity and completion receipts |

Subscription configuration determines what each recipient can receive. Local read access does not authorize export. Payloads use the minimum necessary data; raw text/media is included only when permitted.

Use at-least-once delivery with receiver deduplication where supported. A timeout after sending may mean an unknown outcome; query a receipt or retry under an idempotency contract. If the receiver lacks that contract, automatic retry of side effects is restricted. Maintain state per capability or operation: query availability, delivery backlog, indexing progress and deletion completion can differ.

Delivery failure does not retroactively fail a completed user task. If external confirmation is genuinely a completion prerequisite, declare it before execution and model it as part of that task.

Receipts distinguish accepted, pending, completed and failed. Accepted deletion is not completed deletion. Prevent locally serving a known-deleted or revoked item while external cleanup remains pending, according to applicable retention policy.

Feedback carries causal lineage. An assistant response influenced by an emotion snapshot is not independent evidence of that emotion. Silence is not positive feedback. No hidden reasoning is needed or exported for these fixtures.

## 9. Expensive work and availability

OCR, indexing, subagent analysis and model-generated summaries are explicit work owned by the existing task mechanism or by the external service. Their outputs become resources. Context queries return ready, stale, pending, unavailable or unsupported outcomes as appropriate.

Waiting, using permitted stale data, requesting processing or failing is a Host/task policy decision, not a silent provider fallback. Derived caches can be invalidated or rebuilt without changing original event truth. Resource changes and deletion must invalidate affected local derivations when dependencies are known; external systems report their own completion state.

## 10. Failure behavior

| Condition | Required behavior |
| --- | --- |
| Optional source timeout | Record unavailable/omitted; proceed if invocation requirements remain satisfied |
| Required source timeout | Fail preparation with a stable reason |
| Resource changes during resolution | Reject mismatched version; bounded requery only if policy permits |
| Unsupported modality | Apply an explicitly allowed representation or report unsupported; never claim the model saw omitted media |
| Required content exceeds budget | Fail preparation; do not silently truncate requirements |
| External delivery fails | Preserve task outcome and separately track delivery |
| Duplicate event | Idempotent receiver produces one logical update |
| Cancelled preparation | Cancel owned work; do not report request sent |
| New emotion during generation | Apply at next model-call boundary unless existing steering interrupts |
| Generation rollback | Restore compatible runtime composition; do not pretend external writes were undone |

## 11. Plaintext reference fixtures

These examples are specifications for future local tests. They are not executed tests, production schemas, personality policy, or laputa-garden API examples. All identifiers and timestamps below are synthetic. A controllable clock avoids timing-dependent tests.

### A. Personality and emotion

```yaml
fixture: personality-emotion
clock: 2026-09-12T12:00:00Z
scope: {workspace: demo, session: s1}
personality:
  id: persona-1
  version: p1
  text: "Speak calmly. Admit uncertainty. Keep explanations concise."
  host_retention: reserved
emotion:
  id: emotion-1
  version: e1
  text: "Agent state: concerned; prefer a measured tone."
  observed_at: 2026-09-12T11:59:50Z
  valid_until: 2026-09-12T12:00:20Z
  evidence_kind: external_state
user_input: "Explain the next task. Do not modify files."
```

Expected reference behavior:

1. View V1 includes p1 and valid e1 under the Host's budget rules. Neither changes the user's no-write instruction.
2. e2 arrives after V1 is sent: V1 stays unchanged; the next view may use e2.
3. With the clock past e1 expiry and no replacement, e1 is omitted as expired.
4. A candidate requesting highest authority does not obtain it through its priority metadata.
5. The record identifies selected state versions, not a claim that the model experienced or adopted an emotion.

### B. File retrieval and version mismatch

```text
Resource: project://demo/plan.txt
Version: f1
1: Current scope: discuss SCX architecture.
2: Implementation is not scheduled by this fixture.
3: Keep summary and retrieval strategies replaceable.

Resource: project://demo/notes.txt
Version: n1
1: Old proposal: always use summary compression.
2: This proposal was superseded by plan.txt version f1.

Query: What context strategy is decided?
Preset hit: plan.txt@f1, lines 1-3
```

Expected answer evidence: strategy replaceability is the current decision; no specific algorithm is selected. This validates fixture routing and provenance, not semantic search quality.

Mutation variant: replace plan.txt with f2 between query and read. Exact-version resolution returns f1 if retained; otherwise it reports version unavailable. Returning f2 while labeling it f1 fails the contract.

Budget variant: add a long optional passage. The Host omits it deterministically. If the required passage itself cannot fit, preparation fails rather than reporting successful inclusion.

### C. Memory return and deduplication

The frozen G0 capability profile for this seam is `VIVY-MEMORY-PROFILE.md`.

```yaml
event:
  id: evt-001
  kind: run.terminal
  run: run-001
  session: s1
  workspace: demo
  outcome: completed
  view: V1
  payload:
    result: "SCX architecture discussed; no files modified."
  causal_parent: user-001
subscription:
  recipient: fake-memory
  allowed_fields: [outcome, result, view]
  raw_file_export: false
```

Expected behavior:

1. No terminal delivery before the outcome commits.
2. Fake receiver accepts evt-001 once; duplicate delivery returns its prior receipt without a second logical update.
3. A transient outage leaves the delivery pending and the completed Run completed.
4. Reconnection resumes delivery from durable progress. An ambiguous acknowledgement is resolved with idempotency/receipt semantics.
5. Raw fixture files are absent from the exported payload.
6. A cancelled variant sends outcome cancelled, never completed.

### D. Non-executing extension examples

- A subagent analysis resource cites its source file versions and task outcome. Querying that result does not spawn another agent.
- An image resource offers an original-image reference and an OCR derivative. The selected representation is recorded explicitly.
- A synthetic observation extension carries timestamp, expiry, coordinate-frame identifier and units. This proves only expressibility, not sensor synchronization or robot safety.

These examples do not add real-device acceptance tests to this cycle.

## 12. Validation and gate relationship

The bounded local validation scope is fixtures A-C. Example D is a schema/contract review. No new simulator, hardware harness, live emotion engine, embedding service or memory deployment is required.

| Gate | Relationship to this design | Evidence status |
| --- | --- | --- |
| A | Map the accepted contract to existing Ports/Hosts, verify fake providers and architecture boundaries, pin evidence and interface versions | Not passed by this document |
| B | Exercise selected real application composition with local fake backends; preserve default/minimal behavior and failure semantics | Not executed |
| C | Apply selected release conformance and actual rollback requirements | Not executed |

Do not weaken existing P8 gates by treating plaintext examples as executable evidence. Conversely, do not expand them into speculative hardware testing. Map these concerns to actual original SCX stage IDs only after recovering the plan.

The existing public Context Source has a text-oriented candidate payload; the inspected generic Runtime projection is textual while image attachments have a separate path. Interface changes therefore need a compatibility decision, not an assumption that MediaType alone provides generic multimodality. Prefer additive internal adapters where sufficient; freeze a public revision only after representative consumers prove it necessary.

## 13. Decisions to resolve before implementation

| Decision | Recommended starting point | Required evidence |
| --- | --- | --- |
| Public Port revision vs internal adaptation | Reuse current Ports wherever their semantics are sufficient | Inspect current SDK/Host code and consumer compatibility |
| View-record persistence location | Reuse existing storage contracts, avoid a second Journal | Confirm append/recovery and request-attempt association |
| Reliable subscriber progress | Reuse the ObserverHost-managed persistent cursor; track remote receipts separately | Verify adapter receipt wiring; no parallel delivery framework |
| Initial hook surface | Two public phases; typed request and event channels | Check actual Runtime call boundaries and cancellation behavior |
| Original SCX stage mapping | Preserve original plan | Recover authoritative stage document; do not invent IDs |
| laputa-garden adapter | Capability mapping | Inspect its actual contract when integration is scheduled |

The existing Port Catalog already specifies reliable Run Observers and Host-managed cursors. Inspect that implementation before proposing any persistence addition. These are specific implementation inputs, not reasons to request a new round of product direction. The design can be reviewed now; detailed code planning should resolve the repository-dependent choices.

## 14. Reference rationale

The design is Vivy-owned. External references inform bounded decisions rather than define its architecture.

- [Vivy P8 integration gates](https://github.com/ProjectViVy/agent-vivy/blob/main/docs/plans/plugin-platform/PLG-P8-scx-integration-gates.md): authority boundaries and evidence gates; not the recovered SCX feature plan.
- [Vivy Context Source](https://github.com/ProjectViVy/agent-vivy/blob/main/sdk/port/contextsource/contextsource.go) and [Runtime projection](https://github.com/ProjectViVy/agent-vivy/blob/main/internal/runtime/contextadapter.go): baseline inspected during discussion; main links may evolve.
- [Codex Memory v2 read instructions](https://github.com/openai/codex/blob/c4017a87aacc7558002b7cb510025e967c1d765e/codex-rs/ext/memories/templates/memories/read_path_v2.md): selective evidence retrieval is a useful candidate strategy, not proof that summarization is obsolete.
- [DSH event modes](https://github.com/deepseek-ai/deepseek-harness/blob/c291e7961a515f6d7af9304e7fd1d257929aef26/docs/cordis-tutorial/04-events.md): lifecycle dispatch semantics motivate separating observers from interceptors.
- [DSH prompt assembly](https://github.com/deepseek-ai/deepseek-harness/blob/c291e7961a515f6d7af9304e7fd1d257929aef26/packages/core/system-prompt/README.md): scoped contributions and ordering are useful; SCX separately models retention and authority.
- [DSH compaction](https://github.com/deepseek-ai/deepseek-harness/blob/c291e7961a515f6d7af9304e7fd1d257929aef26/packages/compaction/compaction/README.md): history and derived visibility can remain separate. SCX's upper-level contract is broader than condensation.

## 15. Review outcome sought

The ownership model, two extension channels, per-call Context View, and bounded local fixture scope form the recorded architecture direction. Exact interfaces and implementation fit remain under review. This record does not schedule all SCX work, change public interfaces, pass gates, or authorize release.
