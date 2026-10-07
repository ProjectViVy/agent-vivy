# ACP-01 Compatibility and Contract Freeze Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Track the ordered steps with checkboxes; delegation is optional and requires the applicable authorization.

**Goal:** Obtain an executable dependency verdict and one reviewed G0 contract without shipping a functional ACP Face.
**Architecture:** Use an isolated SDK probe, then resolve the existing draft's concrete decisions. Keep the candidate dependency outside the product until accepted.
**Tech Stack:** Go 1.26.4; ACP wire 1 / schema-v1.21.0; existing FaceHost and Control; candidate github.com/eino-contrib/acp v0.0.4, not yet accepted.
**Spec:** [Reconciled detailed design](../../specs/2026-10-07-acp-stdio-face-design.md), executable baseline dd78fcf142f384d47ce5cfefb43738fdb9a7346d.
**State / dependencies:** [Single index](index.md#story-status-and-dependencies). No predecessor. G0 execution is not started by this documentation request.

## Global Constraints

Follow the [shared constraints](index.md#global-constraints), proposed limits in design section 10, and repository plugin/kernel workflows. This conditional plan does not approve its public interfaces or schedule G1. ACP-01 freezes them and revises downstream steps before execution. No product data access or generated-assembly hand edits.

## Review Focus

Exercise wire fields and form variants; prompt-before-cancel admission; real blocked pipes; SDK errors/logging; and code omission versus retained assets.

## Files and interfaces

Proposed: sdk/internal/testdata/acp-compat/go.mod, go.sum, agent_test.go, transport_test.go, wire_test.go, testdata/wire-cases.json; docs/research/acp-sdk-compatibility.md.
Read: candidate agent_gen.go, conn/agent.go, conn/agent_outbound_gen.go, transport/stdio/stdio.go, errors.go, logger.go and internal/jsonrpc/connection.go. Reading upstream internals is research; importing them is forbidden.

Consumes public APIs already observed:

- acp.BaseAgent; typed Initialize, NewSession, Prompt and SessionCancel methods.
- acpconn.NewAgentConnectionFromTransport; Start(context.Context) error; Close() error; Done() <-chan struct{}; Err() error.
- stdio.NewTransport(io.Reader, io.Writer, ...stdio.Option); stdio.WithMaxMessageSize(int).
- RequestPermission, SessionUpdate and UnstableCreateElicitation typed outbound methods.
- acp.SetLogger and acp.LevelDisabled.

Produces a report containing exact SDK and schema commits/checksums, toolchain/OS, public options available, fixture results, durations, observed queues/allocations, selected-client compatibility and an accept/reject decision. Unknown APIs remain explicit gaps, not invented option names.

## Contract adoption files and outputs

Existing to review/update when this Story executes:

- docs/superpowers/specs/2026-10-07-acp-stdio-face-design.md
- docs/architecture/ACP-REMOTE-CONTROL-PROPOSAL.md
- docs/architecture/VIVY-FACE-PACK.md
- docs/architecture/VIVY-PORT-CATALOG.md
- docs/architecture/VIVY-ASSEMBLY.md
- docs/TODO.md; all links in this plan directory.

Proposed canonical destination: docs/architecture/ACP-STDIO-FACE.md, produced by moving the design, not by keeping two full versions.

Consumes the probe tasks' exact pin, public API inventory, schema/pipe/race/error results and verdict.
Produces: approved SDK ref and public construction calls; reviewed presentation mechanism and Options.In; accepted path/privacy policy and limits; selected real-client version/OS; explicit G0 result and a separately recorded G1 scheduling state. These outputs are documentation decisions, not claims of implementation.

## Task 1: Compile and exercise the pinned wire surface

- [ ] Create the isolated module with require github.com/eino-contrib/acp v0.0.4; download once and record go.sum. Do not edit root go.mod.
- [ ] Write TestWireRoundTrip and TestUnknownElicitationActionFailsClosed. Embed acp.BaseAgent; override only the four pilot handlers. Use SDK types for method bodies and a test-only standard-library JSON peer for assertions. Pin these observations:

~~~text
initialize result.protocolVersion == 1
session/new accepts [] and returns the test-owned sessionId
session/update is written before the session/prompt result
permission selected optionId is preserved exactly
elicitation/create uses mode=form; accept/decline/cancel decode distinctly
unknown form action never increments answered or approved counters
~~~

- [ ] In the isolated module run go test -run 'Test(WireRoundTrip|UnknownElicitationActionFailsClosed)$' -count=1 -v. Expected first failure identifies missing probe behavior or a real schema mismatch.
- [ ] Implement only probe wiring and schema comparisons; rerun the same command. A typed/wire mismatch is a candidate rejection, never a reason to hand-copy schema types.

## Task 2: Prove resource, cancellation and leakage behavior

- [ ] Write TestPromptCancelAdmissionOrder with a barrier at the beginning of Prompt before its active slot is installed. Send prompt then cancel through the pipe; let cancel run while Prompt is paused. Assert the earlier prompt is eventually cancelled and a later prompt is unaffected. Repeat with four occupied prompt handlers. This tests worker admission, not just adapter latching after run ID.
- [ ] Write TestTransportBoundaries subtests for exactly 1 MiB and 1 MiB+1 inbound frames, EOF, blocked stdout, broken pipe, flood and shutdown. Parent-process deadlines must kill and reap a stuck test child; an in-memory writer is insufficient.
- [ ] Inventory the public pending-frame and timeout setters. Record whether 64 pending frames can be enforced before handlers and whether the whole launch can terminate within the proposed 10 seconds. v0.0.4's observed internal setters do not count as a public solution.
- [ ] Write TestSDKWireErrorsAndLogging using fixed synthetic secret/path canaries in decode failure, handler error and panic. Disable SDK logging before Start; assert no canary, originError or stack reaches protocol output or SDK diagnostics. Run the whole fixture group:

~~~text
go test -run 'Test(PromptCancelAdmissionOrder|TransportBoundaries|SDKWireErrorsAndLogging)$' -count=1 -v
go test -race -run TestPromptCancelAdmissionOrder -count=20 -timeout=2m
go list -deps ./...
~~~

Expected: all acceptance assertions pass for an accepted candidate, no SDK HTTP/WS/proxy/Hertz/Gin imports in the probe closure. A failure or unsupported public control is retained in the verdict. The repetition targets one known scheduling race, not broad optional stress testing.

## Task 3: Record the decision and stop at the dependency boundary

- [ ] Record each design section 12.8 row as PASS / FAIL / BLOCKED with command and observation. Record queue sizing separately from measured allocation/RSS; do not present 64 MiB as a memory measurement.
- [ ] For a failure, identify the smallest public upstream change: option exposure, admission ordering and/or safe internal error path. Specify affected upstream files and an acceptance fixture. No invented future version or automatic SDK switch.
- [ ] If a patched candidate is needed, keep G0 closure blocked until an exact reviewed ref and rerun evidence exist. Local experiments may be recorded after execution is authorized; publishing to upstream requires its own explicit authorization.
- [ ] Add iteration evidence and record the probe result in the index without marking ACP-01 Done. Commit explicit probe/report/log paths with: test(acp): characterize candidate SDK compatibility.

## Task 4: Resolve the concrete remaining decisions

- [ ] Verify every required compatibility row passed on the proposed exact ref. A baseline rejection or absent public queue/ordering/error fix keeps this Story blocked.
- [ ] Present the owner one concrete review set in the existing design: D2 accepts canonical session cwd but suppresses outbound private/workspace roots; the proposed TextPresentationHost is optional for old Faces and required by the current ACP draft; line buffering can defer text until model completion. Review that API and presentation tradeoff explicitly.
- [ ] Select exactly one build route from design section 12.1. Evaluate Candidate A's existing executable/overlay first, using current source/import inspection and, when needed, an isolated fake-Face build probe. Identify exact Web/TUI implementation packages and the selection/overlay symbols. If it cannot satisfy code omission, present Candidate B's extra public Recipe/manifest cost for owner review. Record the chosen build/asset contract, remove the unused implementation route from ACP-02/05 and do not implement a functional ACP Face in this probe.
- [ ] Include the proposed bounds, OS support evidence and exact real ACP client/version used for later smoke. Require a real client capable of the positive permission and form-answer paths. If unavailable, record the product gate as blocked; unavailable-form cancellation is an additional negative test, never a replacement for a successful answer.
- [ ] Confirm the Host text boundary is implementable from the actual run-workspace lookup and existing logging.Redact rules: document how native/slash/JSON-escaped roots and split model chunks are covered, where unsafe units are suppressed, and which executable assertions ACP-02/ACP-03 must still prove. No streaming-token redaction guarantee is implied.
- [ ] Record the owner's decisions already supplied in the session and ask only for unresolved public-contract or material-scope decisions. Restricted mcpServers=[] scope does not need reapproval.
- [ ] If changes are needed, update the design once and revise the affected Story signatures/tests. Do not mark G1 Ready while any constructor/options signature still depends on an unselected SDK.

## Task 5: Adopt the reviewed contract without stale competing statements

- [ ] Move the accepted spec to docs/architecture/ACP-STDIO-FACE.md and update every relative plan link. Leave only a short redirect at the old path so historical delivery links keep working; do not retain a second full specification. Preserve its source baseline and decision history.
- [ ] Amend only the conflicting sections in ACP-REMOTE-CONTROL-PROPOSAL.md and VIVY-FACE-PACK.md: local stdio restricted pilot is distinct from deferred remote ACP. Update VIVY-PORT-CATALOG.md for the proposed additive input/facet contract and VIVY-ASSEMBLY.md only for the accepted packaging semantics, marking their implementation status accurately.
- [ ] Record still-open SDK/implementation work under docs/TODO.md section 0.1 with links to this index. Do not close Issue #1, silently reschedule unrelated platform phases or edit AGENTS.md.
- [ ] Check links and the diff, then run the required repository gate for canonical product-contract changes:

~~~text
git diff --check
just ci
~~~

Expected: no whitespace errors; every required CI target passes. These docs do not need implementation-shaped tests. If the supported environment is unavailable, record the gate as pending and do not claim Story completion.

## Task 6: Release only evidence-backed work

- [ ] Check all five design G0 closure conditions against concrete evidence. Record the accepted pin/options and owner decisions in the canonical document and the SDK report rather than a new decision registry.
- [ ] Mark ACP-01 Done only when the accepted SDK evidence, owner contract review and required gate have passed. Record G1 scheduling separately. ACP-02 stays Blocked until the owner explicitly schedules G1; approving G0 or consolidating docs alone does not release it.
- [ ] Add iteration evidence and commit the explicit contract/plan/log paths with: docs(acp): accept restricted pilot contract and G0 evidence.

**Acceptance:** The exact dependency passes the accepted wire/resource/privacy requirements; the owner reviews one contract; required canonical-contract gates pass. A reproducible rejection report completes the probe task only. G0 may be accepted while G1 remains unscheduled.

**Handoff:** Accepted SDK constructor/options, bounded limits, one packaging route, privacy mechanism, client/version and evidence. Remove rejected alternatives and revise every affected Story before it is marked Ready.
