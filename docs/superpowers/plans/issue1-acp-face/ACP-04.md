# ACP-04 Human Interaction and Failure Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Track the ordered steps with checkboxes; delegation is optional and requires the applicable authorization.

**Goal:** Route permissions and questions through the existing review authority and close every owned prompt safely on disconnect.
**Architecture:** Run reverse requests outside the ordered reducer. Correlate replies with connection/session/run/generation/review IDs; use one bounded cleanup path without adapter persistence.
**Tech Stack:** Go 1.26.4; ACP wire 1 / schema-v1.21.0; existing FaceHost and Control; SDK pin accepted by ACP-01.
**Spec:** [Reconciled detailed design](../../../architecture/ACP-STDIO-FACE.md), executable baseline dd78fcf142f384d47ce5cfefb43738fdb9a7346d.
**State / dependencies:** [Single index](index.md#story-status-and-dependencies). Requires accepted ACP-03 session/prompt/reducer contracts.

## Global Constraints

Follow the [shared constraints](index.md#global-constraints), proposed limits in design section 10, and repository plugin/kernel workflows. This conditional plan does not approve its public interfaces or schedule G1. ACP-01 freezes them and revises downstream steps before execution. No product data access or generated-assembly hand edits.

## Review Focus

RF-4/5: invalid and late replies, unusable previews, ambiguous decision RPC, unsupported forms, EOF and failed cleanup.

## Files and interfaces

Proposed plugins/acp/{interactions.go,interactions_test.go}; extend prompt.go and projection.go at their interaction handoff only.
Read internal/rpc/control.go review/get, review/respond, approval/respond and question/respond; schemas/events/payloads/tool.approval_*.json and user.question_*.json. Reuse committed-event fixture shapes; do not introduce adapter persistence.

Consumes ACP-03 interactionJob, promptScope, opaqueToolID, sanitizer and terminal cancellation.
Produces:

- func (a *agent) startInteraction(ctx context.Context, job interactionJob) error, reserving capacity and returning immediately.
- func (a *agent) cancelInteractions(scope promptScope), idempotently cancelling only that prompt's pending reverse work.
- func validateAnswer(content map[string]any) (string, error): exactly one key answer, a string, non-whitespace, <=65536 encoded UTF-8 bytes; reject extra keys/types.
- Private pendingInteraction entries keyed by scope plus committed interaction ID, holding cancel function, expiry and once-only completion state.

## Task 1: Implement meaningful once-only permissions

- [ ] Write TestPermissionOnceOnly, TestPermissionPreviewFailsClosed and TestPermissionLateReply. Verify the tool_call is written before the permission request; fetch review/get and cross-check run/tool ownership. A title alone is not an adequate approval preview.
- [ ] Run go test -run 'TestPermission' -count=1 in plugins/acp. Initial failure: interaction consumer missing.
- [ ] Build bounded action/target/preview/risk text from allowlisted review fields and sanitize complete units. Do not serialize Arguments or truncate an essential command/target into ambiguity. If presentation is unusable, deny through the existing approval path with a safe reason.
- [ ] Send exactly option IDs allow-once and reject-once with kinds allow_once and reject_once. Translate only a live selected matching option to approved/denied. Unsupported UI, unknown option, cancelled dialog or reverse error cannot approve. If session cancellation already won, let Runtime cancellation settle instead of issuing a contradictory decision.
- [ ] Update durable-looking tool status only after the committed approval terminal event, not merely after an RPC success.

## Task 2: Implement form answers and cancellation

- [ ] Write TestQuestionFormActions, TestQuestionAnswerValidation and TestQuestionUnavailableForm. Pin the response table:

~~~text
accept {"answer":"hello"} -> question/respond once with "hello"
accept {"answer":""}, whitespace, nonstring, extra fields or >65536 bytes -> no answer
decline / cancel -> review/respond action=cancel, distinct safe reason
unknown action / no form support / reverse method failure -> question cancellation, no fabricated answer
expired or terminal prompt -> discard reply, zero new decision calls
~~~

- [ ] Run go test -run 'TestQuestion' -count=1. Implement mode=form, real sessionId, opaque toolCallId and requestedSchema {type:object, properties:{answer:{type:string}}, required:["answer"]}; no arbitrary extra business fields or secret collection.
- [ ] Validate SDK response content before question/respond. For declined/unavailable/invalid form, use review/respond {review_id, action:"cancel", reason}; do not invent an empty answer.
- [ ] Rerun TestQuestion and confirm no client filesystem/terminal/URL elicitation calls occur.

## Task 3: Reconcile lifetime and concurrent outcomes

- [ ] Write TestInteractionExpiryAndCapacity, TestInteractionAmbiguousDecision and TestInteractionDoesNotBlockCancel. Use barriers to return a reverse reply after cancellation, completion, expiry and a newer prompt generation. Assert zero resumed effects/answers in every stale case.
- [ ] At 16 pending requests, reject the 17th and cancel its affected run. Do not evict another session's decision, queue without bound or auto-approve.
- [ ] On a decision RPC timeout, query review/get or await its committed terminal event. If still uncertain, cancel the run; never retry with another decision. A stale/conflict -32009 is consumed as stale state.
- [ ] Connect cancelInteractions to all terminal/cleanup paths and remove ACP-03's interim unsupported-interaction fallback. Release capacity exactly once even if cancellation races with the response.
- [ ] Run go test -race -run 'Test(Permission|Question|Interaction)' -count=1, then just ci; record the full AC-08/09 evidence and commit explicit paths with: feat(acp): route permissions and questions through runtime authority.

## Task 4: Close all owned work on transport loss

**Files:** Extend plugins/acp/{agent.go,transport.go,prompt.go,interactions.go} only at their existing lifecycle seams; add plugins/acp/lifecycle_test.go.
**Interfaces:** boundFace.Run consumes the launcher-owned streams/context and the live Host; prompt finalization, cancelInteractions(promptScope) and unsubscribe remain idempotent. Do not add a second lifecycle registry.

- [ ] Write TestOwnedRunCleanupOnEOF, TestCleanupCancellationFailure and TestBrokenPipeWithPendingReview. Run two owned sessions and an unrelated Host run; EOF cancels only owned work. Exercise signal, blocked/broken writer, pending reverse request, expiry and cancellation-versus-completion.
- [ ] In plugins/acp run go test -run 'Test(OwnedRunCleanup|CleanupCancellation|BrokenPipeWithPendingReview)' -count=1; observe missing or incorrect cleanup.
- [ ] On EOF/fatal transport, close admission and mark active prompt generations draining; cancel their reverse waits and invoke run/cancel under the remaining shared launcher budget while Control is live. Await durable outcomes while time remains, detach subscriptions/reducers and return so FaceHost closes the App. For an unwritable pipe send no replacement response.
- [ ] Report failed Control cancellation or timeout as incomplete cleanup; do not claim a durable cancelled Run or rollback of an already-authorized effect. Repeated cleanup must not duplicate authorization/answer/finalization.
- [ ] Rerun focused and race suites, then just ci. Record real lifecycle traces and commit with: fix(acp): close owned prompts on transport loss.

**Acceptance:** Successful live permission and form replies resume governed work once; invalid, unsupported, cancelled and stale paths never authorize or fabricate an answer. Pending UI cannot block cancel/exit. Cleanup failure is visible and never recorded as successful product acceptance.
