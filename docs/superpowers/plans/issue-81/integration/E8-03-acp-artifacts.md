# E8-03 ACP Packed Artifact and Client Acceptance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans after implementation authorization. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Seal the completed optional ACP Face and prove selected/omitted artifacts plus a genuine protocol client turn.

**Architecture:** Reuse existing ACP-05 Candidate A: cmd/vivy selected-generation overlay, exact Recipe pin, one selected Face and core authority. A scripted protocol client launches the actual packed executable over pipes with a local deterministic model.

**Tech Stack:** Existing vivy-sdk verify/pack/Inspect/conformance; Go; Playwright-free stdio client; existing accepted `@agentclientprotocol/typescript-sdk` route or equivalent scripted wire peer that proves all required reverse calls.

**Spec:** Existing ACP `issue1-acp-face/ACP-05.md` and canonical AC-01..AC-12 matrix, reconciled for D2. Immediate predecessor: E8-02.

## Global Constraints

No force push, automatic PR/merge/release, live paid provider, IDE installation or second packaging framework. Root `go test ./...` excludes nested ACP/probe modules. Generated source and evidence must come from actual tooling. A pack success or Inspect identity alone does not certify conformance.

## Review Focus

Optional adapter still linked when omitted; Web/TUI implementation pulled into ACP by private launch reuse; stale source hash/evidence; client permission works but positive question answer missing; valid unit tests while actual stdout has startup logs or cleanup hangs.

## Files and interfaces

Proposed `recipes/acp.vivy.yml`, `plugins/acp/README.md`, `plugins/acp/conformance_test.go`, `sdk/internal/acp_artifact_test.go`; reuse branch `sdk/internal/pack_face_overlay_test.go`. Modify actual source pin references, `sdk/internal/conformance/reproduction_test.go` release-suite selection and generated `sdk/internal/assembly/conformance_results.json` only through executed evidence tooling. Keep current main packer/removal changes.

Consumes E8-02 complete adapter and exact fork pin. Produces selected ACP artifact, a separately packed default/minimal artifact, manifest/Inspect/dependency inventory, client transcript and a per-AC acceptance table with SHA/hash/Generation/binary identity. No new runtime API.

## Tasks

### 1. Explicit Recipe and source closure

- [ ] Add `TestACPRecipeSelection`, `TestACPPackedArtifactScriptedPeer` in proposed SDK artifact test. Derive Recipe from supported coding core closure, replace the TUI Face with `projectvivy/acp` and `rpc.client`, exclude active UI roots/contributions, preserve protected tool semantics. Use actual supported source declarations; no new Recipe field.
- [ ] Run `go test ./sdk/internal -run '^TestACP(RecipeSelection|PackedArtifactScriptedPeer)$' -count=1` red before adding the Recipe. Generate selected Assembly/overlay using the existing compiler. Retain default Web/TUI/gateway behavior tests.
- [ ] Read the actual current declared source hash and run `go run ./sdk/internal/cmd/source-hash plugins/acp <actual-current-hash>` with its literal value. Update only applicable declarations/pins. Recompute after README/test changes in the hashed source. Run `go run ./sdk verify plugins/acp`.
- [ ] Pack into fresh nonexisting output directories: `go run ./sdk pack --recipe recipes/acp.vivy.yml --output <fresh-acp-output>` and a supported existing default/minimal Recipe into `<fresh-control-output>`. If external source is selected, supply its exact `--source`; do not infer filesystem discovery.
- [ ] Run `go run ./sdk inspect-artifact <fresh-acp-output>` and control equivalent. Assert sole ACP Face, correct source/Grant/dependency identity; control physically omits `plugins/acp`; ACP omits Web/TUI Face implementation. Inspect actual linked package/build inventory as well as manifest. Report any inert asset retention per accepted Candidate A contract rather than claiming stronger omission than observed.

### 2. Packed subprocess and real protocol client

- [ ] Use existing supported scripted client dependency if present; otherwise a test-owned stdlib NDJSON peer must implement positive reverse permission/form responses, not mock adapter internals. Do not install unknown software. Launch packed binary with isolated home/Journal/workspaces and local scripted model; stdout is captured separately from stderr.
- [ ] Demonstrate initialize/new/prompt, a real tool-bearing turn, allow-once and reject-once, successful form answer plus cancel/expiry/unsupported form, and two sessions with one cancelled before run ID. Record actual session/run/tool/review identity and committed sequence order. Unsupported form is a negative case, not a replacement for a successful answer.
- [ ] Demonstrate process root A/session root B; early/reordered/duplicate event projection; per-model digest integrity; broken/blocked stdout and EOF cleanup; seeded safe SDK handler/panic wire errors. D2 has no generic outbound text-redaction guarantee. No secret or production source is used in canaries.
- [ ] Run artifact tests green and `go test ./sdk/internal/conformance -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1`, selected Host/Port suites, plugin/probe nested suites and `just ci`. Advance conformance only from matching executed sources; no edited pass flags or hidden skips. If a specific native pipe behavior cannot run, its platform row remains blocked.

### 3. Final review and acceptance

- [ ] Record every canonical AC-01..AC-12 row PASS/FAIL/BLOCKED with exact commands/client observations. ACP-05 product proof is bounded pilot evidence, not full ACP v1 conformance/TCK certification.
- [ ] Independent reviewer checks source closure, omitted implementation, one runtime, positive reverse calls and cleanup against the actual artifact. Capture generation, binary checksum, toolchain/platform and SDK source in `docs/logs/<delivery-date>-issue81-acp-artifact/{summary,verification,acceptance}.md` (future paths).
- [ ] Commit explicit source/docs/evidence paths under human attribution. Parent handles approval/acceptance separately; rollback means selecting/repacking a prior authorized artifact, not deleting state. Do not open a PR or close #1 automatically.

**Acceptance:** Optional ACP implementation is physically selected/omitted as promised; actual packed subprocess completes prompt/tool/permission/question/cancel flows with clean NDJSON and truthful teardown. Every unresolved platform/SDK/CI row is explicit.

**Stop conditions:** Missing E8-02 acceptance, unavailable supported client tooling, physical omission failure, source-bound evidence mismatch or unresolved final CI. Preserve artifacts/transcript and name only affected rows; never fabricate release readiness.
