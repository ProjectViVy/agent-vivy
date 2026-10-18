# E8-02 ACP Human Interaction and Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans after implementation authorization. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Complete the existing ACP-04 permission/question consumer and truthful owned-run teardown.

**Architecture:** Reverse requests run outside the ordered committed-event reducer. Decisions are correlated to connection/session/run/prompt-generation/review identity; core remains sole authority. Cleanup reuses owned prompt state while FaceHost/Control remains live.

**Tech Stack:** Same E8-01 SDK pin, Go, stdio, current Review/Approval/Question RPC.

**Spec:** ACP branch canonical contract §12.6-12.9 and existing `issue1-acp-face/ACP-04.md`, corrected by D2 as recorded in architecture.md. Immediate predecessor: E8-01.

## Global Constraints

No automatic approval, client terminal/filesystem calls, adapter persistence, privacy facet or substitute answer. Preserve protected core ToolHost, committed evidence and source ownership. Allowlisted display content is bounded; SDK wire-error hygiene is separate from generic user-text redaction, which is not added.

## Review Focus

Permission before tool display; unsupported/unknown choice accidentally granting; accepted form with empty/extra fields; late replies after cancel/expiry/new generation; EOF or failed cancellation falsely reported as clean. Pin these cases below.

## Files and interfaces

Proposed `plugins/acp/{interactions,interactions_test,lifecycle_test}.go`; modify existing `agent.go`, `prompt.go`, `projection.go`, `reduce.go`, `transport.go`, `module.go` only at owned state/reducer/cleanup seams. Read actual current `internal/rpc/control.go` and `schemas/events/payloads/{tool.approval_*,user.question_*}.json`; no core endpoint addition is expected.

Consumes E8-01 `promptScope`, `promptState`, committed events, opaque tool IDs and agent connection. Preserve the original ACP-04 private contract:

```go
func (a *agent) startInteraction(ctx context.Context, job interactionJob) error
func (a *agent) cancelInteractions(scope promptScope)
func validateAnswer(content map[string]any) (string, error)
```

Proposed `interactionJob` carries prompt scope plus committed interaction/review/tool-call IDs and expiry, using existing branch ID types. Define it beside the consumer and pass it from the reducer; no second event registry or durable state. Pending entries are bounded to 16, own a cancel function and once-only completion.

## Tasks

### 1. Permission requests

- [ ] Add `TestPermissionOnceOnly`, `TestPermissionPreviewFailsClosed`, `TestPermissionLateReply` to proposed interactions tests. Assert committed tool_call precedes request; `review/get` run/tool IDs match; action/target/preview describe the reviewed request without raw Arguments serialization; unusable essential preview fails closed.
- [ ] Pin exactly `allow-once`/`reject-once`, kinds `allow_once`/`reject_once`. Only the live matching selected option may call `approval/respond` once. Unknown choice, cancelled dialog, missing client support and reverse failure cannot approve. If prompt cancellation already won, discard reply and allow runtime cancellation to settle.
- [ ] Run `go test -run '^TestPermission' -count=1 -v` inside `plugins/acp`, observe missing-consumer failure, then implement the private methods through existing RPC. Successful RPC is not a synthetic approval event: durable-looking UI status advances only on committed terminal approval event. Rerun green.

### 2. Questions and late/ambiguous replies

- [ ] Add `TestQuestionFormActions`, `TestQuestionAnswerValidation`, `TestQuestionUnavailableForm`. Use `mode=form`, real sessionId, opaque toolCallId and schema `{type:object,properties:{answer:{type:string}},required:[answer]}`. `validateAnswer` accepts exactly one `answer` string, non-whitespace, at most 65536 encoded UTF-8 bytes. Empty/whitespace/nonstring/extra fields/oversize produce no answer.
- [ ] `accept {answer:"hello"}` calls `question/respond` once with hello. Decline/cancel/unavailable/invalid reply uses existing `review/respond` action=cancel with a safe distinct reason, never an empty answer. URL/nested/sensitive-data forms are outside this pilot. Verify exact RPC argument names against current schemas.
- [ ] Add `TestInteractionExpiryAndCapacity`, `TestInteractionAmbiguousDecision`, `TestInteractionDoesNotBlockCancel`. Barrier-delayed replies after runtime expiry, prompt cancellation, terminal event or new generation produce zero new decisions/effects. At 16 requests the 17th fails and cancels its own affected run; no other request is evicted. On ambiguous response timeout, inspect review/committed state, then cancel if still uncertain; never retry a different choice.
- [ ] Run `go test -run '^Test(Question|Interaction)' -count=1 -v` red, implement at existing consumer/reducer seams, run green and `go test -race -run '^Test(Permission|Question|Interaction)' -count=1`. Reverse waits must not stall event order or cancellation.

### 3. All-owned-run cleanup

- [ ] Add `TestOwnedRunCleanupOnEOF`, `TestCleanupCancellationFailure`, `TestBrokenPipeWithPendingReview` in proposed lifecycle tests. Two owned sessions plus one unrelated Host run: EOF/fatal transport cancels only owned runs, detaches waits/subscriptions/reducers and returns under one 10s total budget. Exercise signal, blocked/broken writer and cancellation-vs-completion; no replacement success after write failure.
- [ ] Run the tests red against existing `boundFace.Run`, which currently closes connection but lacks completed ACP-04 all-owned-run cleanup. Close admission first; retain a separate live host cleanup context; cancel interactions and bounded `run/cancel`; await committed terminal outcomes only while budget remains; join workers and close SDK streams; return to FaceHost for app/storage close. Use remaining deadline, not fresh nested 10s budgets.
- [ ] Cancellation failure/timeout returns incomplete-cleanup error with original cause; it cannot claim durable cancellation or undo an authorized external effect. Repeated cleanup finalizes once. Retain #80's normal-cancel evidence limitation until a real candidate test proves otherwise.
- [ ] Run all plugin tests, plugin race suite, relevant FaceHost/runtime tests and `just ci`. Record real EOF/failure traces and exact candidates in future Story log. Independent reviewer checks lock order, capacity release, stale reply fencing and host-live teardown. Commit explicit paths under human attribution.

**Acceptance:** Positive permission and form answer resume governed work once; invalid/unsupported/stale paths never authorize or fabricate answers; cancellation/transport failure affects only owned runs and reports incomplete cleanup honestly.

**Stop conditions:** Missing E8-01 SDK/candidate proof, unsupported core decision semantics, inability to correlate committed ownership, or a reproduced core cancellation fault outside this adapter. Return exact evidence to the existing owner; do not bypass it or expand #80 here.
