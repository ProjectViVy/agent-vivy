# S02: Steering capability, settings/UI/events parity and recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Keep the RPC/UI work in one lane with the runtime capability change; do not split it across parallel writers.

**Goal:** Selected species have full steering capability and settings/UI/event parity. Unselected species expose an explicit `NOT_COMPILED` capability, render no steer controls, and surface already-persisted steer turns as recovery-required without applying or dropping them. Historical `turn.*` events render passively and never instantiate the feature.

**Architecture:** Capability truth is the compiled factory, not a config list. `internal/app/app.go` sets `Service.SteeringAvailable()` from S00's generated accessor; RPC `initialize` advertises `queue.steer` only when available; `turn/steer` and steer-track `queue/mode` fail closed on an absent controller; `queue/state` gains a separate recovery projection while preserving every existing field; both faces (host UI and VIVY CODE TUI) derive controls and command behavior from the negotiated capability.

**Tech Stack:** Go 1.26.4; existing RPC/control plane; TypeScript/React host UI and `@vivy/ui-sdk` Face contract; Go TUI in `sdk/tui`; existing i18n (`en`/`zh`); no new storage backend, config key or schema migration.

**Spec:** [Complete deletion design §§10.1a C11, 10.3, 11.5](../../../plans/2026-10-19-complete-deletion-plan.md). [S01](S01-policy-extraction.md) provides the selected controller and the unselected sentinel. [Index](index.md) owns status and selection matrix.

**Story:** S02 · **Epic:** E-STEERING · **Requirements:** SR1, SR2, SR4, SR5.

**Immediate predecessors:** S01 accepted. External G2 accepted. Baseline A `202aac38`.

**Readiness gate:** S01's `ErrSteeringNotCompiled`, controller injection and Host cleanup are accepted; generated `HasSteeringFactory` is deterministic; the selected and unselected compositions both build. If S01 kept the feature lane visible when unselected, stop and fix S01 rather than layering a UI-only hide.

## Global Constraints

- Existing queue state keys and event payloads remain exactly: `steering`, `follow_up`, `steer_mode`, `follow_up_mode`, `pending`, `last_admitted_run_id`, `admitted_run_id`; events `turn.queued`, `turn.dequeued`, `turn.steered`. New fields are additive and optional.
- Recovery is explicit: never auto-apply a persisted steer turn, never rewrite it to follow-up, never drop it. Generic clear/remove/cancel must still return its text to the user.
- Follow-up, user cancel, approval/question recovery and the generic queue are unchanged and remain available when steering is absent.
- No new config key, settings page, database column or schema migration. The only steering "setting" is the existing RPC `queue/mode` steer track; selection is compile-time.
- UI truth derives from the negotiated capability. No hardcoded species list, no build-time environment flag, no duplicate capability table.
- No Vivy-owned hash; no new event field carrying a digest/checksum. Historical events are read-only input.
- Do not edit issue-91, sibling experiment epics, or the research re-anchor file. Do not run go/just/CI in this planning turn.

## Review Focus

- Lying capability: a client can call `turn/steer` successfully in an unselected build, or an unselected build still registers a control.
- Idle fallback: `turn/steer` on an idle session in an unselected build silently starting a fresh Run instead of reporting NOT_COMPILED (selected builds keep the baseline fresh-run fallback).
- Recovery data loss: `queue/state` hides, counts as pending, auto-admits or drops persisted steer items; clear/remove no longer returns their text.
- Event resurrection: replaying `turn.steered`/`turn.queued` instantiates the controller, shows a steer control or re-arms a cancel.
- Contract drift: `FaceQueueState`/TUI typing changed incompatibly, or the SDK compat test weakened to pass.
- UI split behavior: Enter sends a plain turn instead of the generic follow-up when steering is absent, or the recovery affordance clears unrelated follow-ups.

## Baseline A mapping

| Baseline fact | S02 action |
| --- | --- |
| `internal/rpc/control.go:1136-1215` `initialize` capability list; conditional entries follow `h.deps.Service != nil`. | Append `queue.steer`, `queue.steer.mode` only when `Service.SteeringAvailable()`; leave every existing string untouched. |
| `internal/rpc/control.go:1412-1424` dispatch; `:3683-3730` steer/fallback; `:3829-3845` queue mode. | `steerTurn` returns `MethodNotFound` with `steering is NOT_COMPILED in this species` before any `queueStartFallback`; `setQueueMode` returns the same for track `steer`; follow-up handlers unchanged. |
| `internal/rpc/control.go:3731-3766` queue state; `:2197-2206` session/get queue snapshot. | Add `steering_available`, `steering_unavailable_reason`, `steering_recovery` to both projections; preserve keys/types; recovery does not count in `pending`. |
| `internal/runtime/queue.go:45-67` `QueueState`; `:256-274` clear; `:512-541` lane moves; `:418-510` rebuild/materialize. | Add a recovery lane projection read only by the absent-controller path; clear/remove still drain it; rebuild keeps it; no auto-demotion without a controller. |
| `internal/app/assembly_steering.go` / shared loop-exp integration point (S00/S01). | Publish `SteeringAvailable()` from the generated accessor; one assignment, no config read. |
| `sdk/ui/src/module.ts:2080-2101` Face queue types; `:1963` `getQueueState`. | Add optional `steering_available`, `steering_unavailable_reason`, `steering_recovery`; do not change existing required fields. |
| `ui/src/lib/rpc.ts:25-29,172-180` capability snapshot; `ui/src/lib/store.ts:635` code-mode projection. | Add `steeringAvailable` derived from `queue.steer`; no separate fetch. |
| `ui/src/components/chat/ChatInput.tsx:190-198,198-227,404-441` lane pills, Enter/Shift+Alt+Enter, running controls. | Gate the steer lane pill, sidebar steer button and Enter-steer on the capability; running Enter without steering sends the generic follow-up; recovery banner uses the existing queue-remove API. |
| `ui/src/components/chat/ChatView.tsx:83-90,172` steer/follow-up wiring. | Pass `steeringAvailable`/recovery data from the store; no duplicate capability logic. |
| `ui/src/lib/store.ts:450-459` `turn.*` handling; `:972` `steerMessage`. | Keep passive event handling; guard `steerMessage` with an explicit unavailable error; refresh the recovery projection with the queue. |
| `ui/src/i18n/en.ts`/`zh.ts` queue notices. | Add steering-unavailable and recovery strings in both locales; preserve existing keys. |
| `sdk/tui/live/controller.go:1415+` running Enter behavior; `sdk/tui/live/rpc.go:701-702`; `sdk/tui/face/rpc.go:329-333,439-450,705` `set_steering_mode`. | Gate running-Enter steer on the negotiated capability, send follow-up when absent, return explicit unavailable for `set_steering_mode`, and include the optional recovery fields. |

## Files and ownership

- Modify: `internal/runtime/{queue,service}.go`, `internal/rpc/control.go`, `internal/app/app.go`, `sdk/ui/src/module.ts`, `ui/src/lib/{rpc,store,api}.ts`, `ui/src/components/chat/{ChatInput,ChatView}.tsx`, `ui/src/i18n/{en,zh}.ts`, `sdk/tui/live/{controller,rpc}.go`, `sdk/tui/face/rpc.go`.
- Tests: `internal/rpc/control_test.go`, `internal/runtime/queue_test.go` (recovery), `internal/app/codeface_rpc_test.go`, `ui/src/components/chat/ChatInput.queue.test.tsx`, `ui/src/lib/store.test.ts`, `ui/src/lib/ui-sdk-face-compat.test.ts`, `sdk/tui/live/controller_test.go`, `sdk/tui/face/*_test.go`.
- Do not touch: `sdk/port/loopexp` (S00), generated Assembly (S00/S03), domain event types or payloads, `docs/issue-91`/sibling epics.
- Do not create a new RPC method for recovery: `queue/state` projection + existing `queue/remove`/`queue/clear` are sufficient and avoid a second source of truth.

### Consumed and produced contracts

- Consumes: S01 `Service.SteeringAvailable()`/`ErrSteeringNotCompiled`; S00 `HasSteeringFactory`; existing `FaceQueueState`, `QueueState`, RPC envelope and event schema.
- Produces: capability strings `queue.steer`, `queue.steer.mode`; optional queue-state fields `steering_available`, `steering_unavailable_reason`, `steering_recovery`; the recovery UX and the selected/unselected parity matrix.

## Tasks

### Task 1: Publish capability truth

- [ ] `internal/app/assembly_steering.go` (or the shared loop-exp integration point) records one boolean at construction from the generated steering accessor and passes it to `runtime.NewService` (or the narrow option S01 added). Do not derive it from selected module names, environment or a config file.
- [ ] `Service.SteeringAvailable() bool` returns that value; `ErrSteeringNotCompiled` (S01) is the only error for a missing capability. A selected build with a failed factory fails construction, never returns false.
- [ ] RPC `initialize` adds `queue.steer`, `queue.steer.mode` only when available; update `internal/rpc/control_test.go` for both builds. Existing capability strings and ordering are not otherwise changed.

### Task 2: Fail closed on the RPC surface

- [ ] `steerTurn`: if unavailable, return `MethodNotFound` with message `steering is NOT_COMPILED in this species` before inspecting run state or calling `queueStartFallback`. If available, keep the baseline queue-or-fresh-run behavior byte-for-byte, including the `ErrQueueUnavailable` fallback.
- [ ] `setQueueMode`: track `steer` unavailable -> `MethodNotFound` with the same explicit message; track `follow_up` unchanged; invalid track/mode errors unchanged.
- [ ] `queue/state` and `session/get` queue projection: add `steering_available` (bool), `steering_unavailable_reason` (`not_compiled` only when unavailable, empty when available), and `steering_recovery` (array, empty unless unavailable and persisted steer items exist). Keep `steering: []` when unavailable so existing clients stay array-safe; `pending` is `len(Steering)+len(FollowUps)` and excludes recovery items; `steer_mode` remains the persisted mode string.
- [ ] `queue/clear`: still drains recovery items, returns their texts and journals `turn.dequeued{reason:"cleared"}`. Update the RPC handler to append `state.SteeringRecovery` texts (the core `ClearQueue` already drains `q.steer`). `queue/remove`: still removes a recovery item by ID and returns its text; no capability check on these generic recovery affordances.
- [ ] Add RPC tests: unselected steer (active and idle) -> NOT_COMPILED, no run created; unselected follow-up -> success; unselected clear/remove recovery -> text returned; selected steer/mode -> success; selected queue state contains the legacy keys and types.

### Task 3: Add the recovery projection in the core queue

- [ ] Extend the internal `QueueState` with `SteeringRecovery []domain.QueuedTurn` (JSON omitted when empty) and keep `Steering` feature-only. When the controller is absent, `QueueState` returns `Steering=nil` and `SteeringRecovery=copy(q.steer)`; when present, returns `Steering=copy(q.steer)` and no recovery.
- [ ] `rebuildQueue` continues to load persisted steer items into `q.steer`; when no controller is bound, no path (`materializeQueueTail`, `popFollowUps`, follow-up admission) may demote, admit, re-journal or delete them. Audit every steer-lane move and gate it on controller presence.
- [ ] Preserve the same RunID and `turn.steered` semantics for the selected path; recovery has no new event type and no synthetic `turn.steered`. Historical events remain valid input for replay but never construct a controller.
- [ ] Add runtime tests: restart with a persisted steer item and no controller -> recovery projection, no mutation after a new run starts; restart with controller -> normal selected queue state; clear/remove recovery -> text + marker; pending count unchanged.

### Task 4: Gate the host UI on the negotiated capability

- [ ] `ui/src/lib/rpc.ts` `RpcCapabilities` keeps `capabilities: string[]`; `ui/src/lib/store.ts` derives `steeringAvailable = capabilities.includes('queue.steer')` during initialization and stores it in runtime state. No second capability source.
- [ ] `ui/src/components/chat/ChatView.tsx` passes `steeringAvailable`, `steeringRecovery` and a restore callback to `ChatInput`. `ChatInput` hides the steer lane pill and the sidebar steer button when unavailable; running Enter -> `send('follow_up')`; Shift+Alt+Enter remains follow-up; idle Enter remains a normal send. Do not render a disabled steer button that still implies capability.
- [ ] `ui/src/lib/store.ts::steerMessage` throws an explicit localized unavailable error when the capability is absent; the composer never calls it in that state. Follow-up and normal sends are untouched.
- [ ] Recovery banner: when `steering_recovery` is non-empty, render the items with a restore action that calls the existing queue-remove API for that item and puts its text back in the composer. Do not call `queue/clear` from this affordance (it would also clear unrelated follow-ups). The banner survives session refresh because it comes from `queue/state`.
- [ ] Add i18n keys in `ui/src/i18n/en.ts` and `zh.ts`: unavailable notice, recovery title, recovery hint, restore action. Keep existing `chatInput.steer`/`sendAsFollowUp` semantics for selected builds.
- [ ] Update `ui/src/lib/api.ts` `QueueState` mapping to default `steering_recovery` to `[]`; keep the existing null-to-array normalization for `steering`/`follow_up`.
- [ ] Tests: `ChatInput.queue.test.tsx` unselected -> no steer button/pill, Enter queues follow-up; selected -> unchanged; recovery banner renders and restore calls remove; `store.test.ts` capability derivation and `turn.*` passivity; `ui-sdk-face-compat.test.ts` still passes with the optional fields (do not loosen the type assertions).

### Task 5: Gate the TUI face and its settings command

- [ ] `sdk/tui/live/controller.go`: read the negotiated capability; while a run is active, Enter uses steer only when available, otherwise the generic follow-up; idle Enter is a normal prompt. Keep the TUI's existing key help truthful (do not print a steer binding that is absent).
- [ ] `sdk/tui/face/rpc.go`: `set_steering_mode` returns the explicit NOT_COMPILED message when unavailable and does not call `queue/mode`; `set_follow_up_mode` is unchanged. `queue/state` decoding accepts the optional recovery fields and does not treat them as pending.
- [ ] TUI recovery: surface the recovery-required state in the queue view/status with a restore/remove affordance; no auto-admission. Add tests for both capability states and the command response.
- [ ] Run the existing TUI compatibility tests; if a snapshot/help string changes, update it for both capability states rather than deleting the assertion.

### Task 6: Prove the recovery matrix

- [ ] Add a named test matrix (runtime + RPC where the boundary is RPC): selected run persists a steer item -> selected restart drains it normally; selected run persists a steer item -> unselected species opens the session -> recovery projection, no apply/drop, generic cancel and clear work; unselected species starts a follow-up run -> recovery item untouched; reopen in a selected species -> normal drain resumes. Use one shared database fixture per case.
- [ ] Assert no second Run, no `turn.steered` in the unselected cases, and one marker per drained item on the selected resume (the single-item case has exactly one).
- [ ] Assert `queue/state` never returns a recovery item under `steering` when unavailable, and never returns it under `steering_recovery` when available.

## Verification commands and expected result

Future gates, not executed by this planning turn:

```text
go test ./internal/runtime ./internal/rpc ./internal/app -run 'TestSteering|TestQueue|TestTurnSteer|TestCapabilities' -count=1
go test ./sdk/tui/... -run 'TestSteering|TestQueue|TestFace' -count=1
cd ui && pnpm vitest run src/components/chat/ChatInput.queue.test.tsx src/lib/store.test.ts src/lib/ui-sdk-face-compat.test.ts
just ci
```

Expected: unselected builds advertise no steering capability, reject steer/mode explicitly, keep follow-up/cancel/recovery, and never apply persisted steer items; selected builds keep all baseline queue behavior and UI controls; recovery text is restorable; Face/TUI types stay compatible; `just ci` green. The planning turn does not run these.

## Acceptance, rollback and handoff

S02 is accepted when capability truth comes from the compiled factory, the unselected RPC/UI/TUI surfaces are explicit and control-free, the selected surfaces retain parity, the four recovery cases pass with no data loss or hidden application, and no new config/schema/event path was added.

Rollback: revert S02's commit. S01 remains behaviorally correct but the UI/RPC capability gate and recovery projection disappear; do not revert S01 or S00 to "fix" a UI-only problem. Handoff must include the capability strings, the exact new optional queue-state fields, the recovery matrix results, the selected/unselected RPC responses, the Face-type compatibility evidence, and any locale or TUI help-string change.
