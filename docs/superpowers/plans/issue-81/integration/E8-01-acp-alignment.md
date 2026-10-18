# E8-01 Existing ACP Branch Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans after implementation authorization. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Align already-implemented ACP launch/session/projection code with current main and verify the exact accepted SDK fork.

**Architecture:** Recover ACP-02/03 from `21c5b8bb49edd1502a033b0ca557737e102b9551`. Preserve selected Face -> FaceHost -> existing Control -> Service/Journal. Reuse current main's Face input and code launch behavior.

**Tech Stack:** Go 1.26.4; ACP wire v1/schema-v1.21.0; `eino-contrib/acp v0.0.4` replaced by `mastwet/acp v0.0.0-20261007071707-780fb4dd4b98`; existing SDK compiler/packer.

**Spec:** Existing ACP branch `docs/architecture/ACP-STDIO-FACE.md`, `docs/superpowers/plans/issue1-acp-face/{ACP-01,ACP-02,ACP-03}.md`; architecture.md D-ACP. Status: package.md.

## Global Constraints

Use the canonical owner-approved restricted pilot, not older all-ACP-is-remote wording. No new protocol dispatcher, public Port, runtime, remote listener, multi-Face selection or SDK dependency switch. Public plugin imports no `internal/*` or Eino. Main already provides `Options.In`; do not duplicate it. Source alignment and planning do not themselves authorize ACP G1 execution.

## Review Focus

Protocol stdout contamination; process cwd A/session root B; reordered/duplicate/early-terminal events; cancel arriving before prompt admission or after prior completion; SDK panic/handler errors and blocked pipes. Existing branch tests cover many of these; retain and rerun rather than rewrite.

## Files and interfaces

Branch-only: `internal/faceprocess/{launch,private}.go` and tests; `plugins/acp/{module,transport,agent,control,input,prompt,projection,reduce}.go` and tests; `sdk/internal/testdata/acp-compat` nested module; `sdk/internal/pack_face_overlay_test.go`; ACP architecture/research/Story docs. Reconcile modifications to `cmd/vivy/main.go`, `internal/{app/facehost,codeface/launch,rpc/control,rpc/protocol,config/config}.go`, Face tests and `sdk/internal/frontend_v1.go`. Preserve later main changes and removed embedded-host architecture.

Consumes `face.Options{In,Out,Err}`, `face.Host.Call/OnEvent`, current session canonical workspace and committed event schemas. Produces existing branch `newConnection(a *agent, in io.Reader, out io.Writer) (*acpconn.AgentConnection, error)`, `agent.Prompt`, `agent.SessionCancel`, `promptScope`, owned `promptState` and reducer handoff needed by E8-02. Do not rename these privately coupled interfaces during transplant without revising the dependent plan.

## Tasks

### 1. Reconcile source and old documentation

- [ ] Compare ACP branch patch with selected candidate using exact trees. Recover code by concern, not a whole branch merge. Keep current main codeface/work-control/Provider behavior. Maintain one private per-launch home/Journal and generic selected-Face dispatch before stdout logging.
- [ ] Recover the active five-Story ACP package and canonical contract. Correct stale G1/implementation status from actual source without claiming acceptance. Remove only contradictory sanitizer/presentation references under the accepted D2 ruling; preserve SDK-safe errors and original-byte integrity. Keep superseded eight-Story files as redirects.
- [ ] Inspect current main RPC schemas and events consumed by the branch. Bind session/project context to the canonical accepted session root. Preserve repository and sandbox containment, no ACP-originated fetch of ResourceLink, and nonempty mcpServers rejection before effects.

### 2. Verify the existing SDK pin, not a new selection

- [ ] Resolve full fork SHA `780fb4dd4b98b3936fe572f61ae83b3abc2f918d`, module sum and go.mod replacement; read its actual option/sanitization implementations. Existing transport calls `WithAgentMaxPendingDispatch(64)`, `WithAgentShutdownTimeout(10s)`, `WithAgentSanitizedErrors()`, `WithAgentConnectionLabel("projectvivy.acp")`, and stdio `WithMaxMessageSize(1<<20)`.
- [ ] Adapt the existing isolated `sdk/internal/testdata/acp-compat/go.mod` to this same exact fork for candidate probes; preserve historical v0.0.4 report. Run `go test ./... -count=1 -v` and `go test -race ./... -count=1` **inside that nested module**. Exercise option reachability, queue overflow, cooperative/blocked shutdown, safe generic errors/panic, disabled raw-frame logging and stdio write failure. Expected: required corrected rows pass with explicit bounds; a historical rejection report alone is insufficient.
- [ ] In `plugins/acp`, run `go list -deps ./...`; verify the ACP SDK import closure excludes its HTTP/WS/proxy/Hertz/Gin transports. Existing core/model networking is not prohibited by this SDK-specific condition. Do not install a different SDK or create a fallback codec if a required probe fails; block E8-01's pin gate with the exact failing row.

### 3. Validate recovered launch/session/projection

- [ ] Run existing plugin `go test ./... -count=1` and `go test -race ./... -count=1` in `plugins/acp`. Preserve branch tests for digest windows, canonical root and active-vs-idle cancel latch. Record cases actually run, not only package exit.
- [ ] Add only uncovered current-main compatibility regressions: proposed `TestACPCurrentMainControlContract` in `plugins/acp/control_test.go` verifies real current event shapes/capabilities; `TestSelectedFaceDefaultRegression` in `cmd/vivy/selected_dispatch_test.go` verifies gateway/TUI/run are unchanged and ACP startup emits only NDJSON. The branch's existing root/session tests must assert process A/session B and reject unsafe cross-root file references before effects.
- [ ] Observe any missing regression red before modifying the corresponding seam. For prompt order: seq=3 before 2 buffers; duplicate 2 projects once; early terminal cannot hang; a digest mismatch cannot produce end_turn; model.request/tool.requested resets the current digest window. For cancellation: targeted active cancel does not latch onto the next prompt; idle cancel applies only within accepted 5s TTL; two sessions do not affect one another.
- [ ] Run `go test ./internal/faceprocess ./internal/app ./internal/rpc ./internal/codeface ./cmd/vivy -count=1`, plugin suites and `just ci`. Real launch smoke uses an isolated home and stdio subprocess with deterministic local model. ACP-04 missing interaction coverage remains open and visible for E8-02.
- [ ] Independent reviewer checks main compatibility, SDK correction proof, default-product regression and no obsolete host import. Commit explicit paths with human attribution; supply E8-02 exact source/SDK SHAs and private interface inventory.

**Acceptance:** Recovered existing ACP launch and prompt paths compose with main, corrected SDK is pinned/probed, supported event/cancel/root contracts pass, and incomplete interactions remain explicitly unaccepted.

**Stop conditions:** Required SDK fork proof fails; current public Face/Control seam cannot express the accepted contract; unexpected version/limits/privacy decision; or recovery demands removed sdk/host/v1. No silent dependency-route or product-contract change.
