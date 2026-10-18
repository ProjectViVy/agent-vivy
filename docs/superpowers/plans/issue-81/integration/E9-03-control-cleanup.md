# E9-03 Cognitive Control and Composition Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans after implementation authorization. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Scope cognitive cancellation to its current strategy and close an opened owner on every failed VIVY composition.

**Architecture:** Reuse old P2.4 coherent control projection/scoped cancellation and P4.1b local failure cleanup. Main already projects C5 pending/phase/block fields; retain those and remove torn double reads. Existing App owns the bundle after successful transfer.

**Tech Stack:** Go, existing SnapshotStore, Service terminal synchronization and App composition.

**Spec:** #32 C5/C6/C7; old P2.4/P4.1b at `6045c0a...`; architecture.md. Immediate predecessor: E9-01.

## Global Constraints

No DIVA UI/native lifecycle changes, new ownership API or generic cleanup framework. Preserve current main control projection, E5 provenance/readiness, E6 Skill recovery and #80 pending-run cleanup. Sequence shared `service.go`/App edits.

## Review Focus

Foreign foreground/stale strategy IDs; terminal status winning immediately before cancel; two snapshot reads yielding mixed state; composition failing at each point after owner open; successful composition accidentally double-closing on ownership transfer.

## Files and interfaces

Modify `internal/runtime/cognitive_service.go`, `internal/app/{assembly_cognitive,app}.go`; adjust existing cognitive contract/dispatch only if necessary for the existing full fields. Extend `internal/runtime/cognitive_service_test.go`, `internal/app/assembly_cognitive_test.go`, `internal/modules/diva-cognitive/actions_test.go`. Reuse old App failure fixture; proposed dedicated file `internal/app/cognitive_cleanup_test.go` only if no suitable existing fixture file owns it.

Produce old scoped methods:

```go
func (s *Service) CognitiveControlState(ctx context.Context) (cognitivecontract.ControlState, error)
func (s *Service) CancelCognitiveRun(ctx context.Context, runID domain.RunID) (bool, error)
```

`ErrCognitiveRunMismatch` rejects stale/foreign/non-strategy IDs. Keep `ControlPort.Cancel` signature and `App.New/NewWithAssembly` signatures. One snapshot supplies policy/revision/window/phase/block/active ID; immutable bundle SourceID is supplied by existing adapter. Preserve main `CognitiveStatusView` consumers.

## Tasks

### 1. Coherent control and exact cancellation

- [ ] Recover `TestCognitiveControlStateOneSnapshot`, `TestCognitiveCancelScopeMatrix` and actual App/dispatch projection assertions from old P2.4. One-read fixture rejects a second read. Exact current strategy cancel works; foreground, stale, other strategy and completed IDs cannot cancel a run. Barrier test lets terminal settlement win before cancellation and requires no stale cancellation event/effect.
- [ ] Run focused runtime/App tests red. Implement one-snapshot method; App GetState delegates to it. Scoped cancel serializes with admission, validates durable E9-01 intent/run lineage, rechecks under the existing terminal gate, then signals only the validated run. Avoid calling generic cancellation while holding a gate that suspended-run terminal persistence also needs.
- [ ] Run `go test ./internal/runtime ./internal/app ./internal/modules/diva-cognitive -count=1`, targeted race `go test -race ./internal/runtime -run '^TestCognitive(ControlState|CancelScope)' -count=1`. Expected: all control fields coherent and unrelated foreground run stays active.

### 2. Failure cleanup ownership

- [ ] Recover `TestCognitiveBundleClosedOnEveryCompositionFailure`: inject subsequent observer host, primary admission, binding resolution, Service/control attachment and action validation failures. Assert exactly one bundle close and no retained owner handles. Successful construction transfers ownership, has no premature close, and App.Close closes exactly once.
- [ ] Run `go test ./internal/app -run '^TestCognitiveBundleClosedOnEveryCompositionFailure$' -count=1` before patching. Register one local deferred failure close immediately after successful bundle open; disable it only after successful App ownership transfer. Remove now-redundant scattered closes in this scope and preserve other resources' cleanup order.
- [ ] Reconcile E5/E6 current App modifications; do not replace complete App source or resurrect old host logger ownership. Rerun failure matrix, full App tests and `just ci`. Record actual dependency/candidate used and unexercised native handles separately.

### 3. Independent review

- [ ] Reviewer checks cancellation lock ordering and terminal race, one-read projection and all ownership-transfer returns against actual source. Commit explicit paths with human attribution and future summary/verification/acceptance evidence.

**Acceptance:** Cognitive control cannot cancel unrelated runs; status derives from one durable snapshot; every failed composition closes its opened cognitive owner without double close or new lifecycle machinery.

**Stop conditions:** Incompatible intent schema/predecessor; no safe existing terminal gate; shared peer cleanup conflict; or tests expose a broader normal-cancel persistence defect belonging to #80 follow-up. Report exact affected action and keep unrelated tasks separate.
