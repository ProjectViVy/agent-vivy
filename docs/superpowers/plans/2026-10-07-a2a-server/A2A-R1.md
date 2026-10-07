# A2A-R1 — Conditional Exact Event Replay Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans only after G0 explicitly adopts option B, freezes the extension wire/errors, and the index releases this Story. Consolidation is not implementation authorization.

**Goal:** Recover retained public committed task events across disconnect, terminal transition and restart for an enhanced client that durably deduplicates them.

**Architecture:** Reuse the accepted native Journal reader, projector, TaskHost, Host listener and standard adapter. Add one optional replay operation on that listener and thin SSE framing with durable cursor IDs. Standard SDK operations remain unchanged; no second event store, queue, runner, listener, SDK fork or shipped client product.

**Tech Stack:** Go 1.26.4, pinned a2a-go/v2 v2.6.0 event models, net/http SSE, native SQLite/PostgreSQL Journal and existing artifact test driver.

**Spec:** [Design section 8.2](../../specs/2026-10-07-a2a-server-design.md#82-option-b-retained-exact-event-replay-proposal), adopted G0 revision and the published wire specification it requires. This replaces the old branch's A2A-06; it does not preserve that Story ID as an alias.

**Epic / requirements:** E2 / R10, R5, R9. **Status/dependencies:** [index](index.md#epics-and-story-authority).

## Global Constraints

- B requires an explicit one-extension exception to issue #2; option A leaves this Story unselected.
- Route is POST `/a2a/extensions/task-event-replay/v1` on the existing Host listener. Host authenticates before cursor/history inspection.
- Agent Card declares the adopted URI optional, `required: false`, only while the implementation and mount are healthy.
- Replay has no initial current-state snapshot. Exclusive `after` controls resumption; Last-Event-ID cannot override it.
- Cursor survives restart and represents projection version, Run, Journal sequence and public-event ordinal. Possession is not authorization.
- Terminal tasks drain retained events and close. Historical interrupted statuses do not suppress later committed events; an interrupted current tail closes after delivery.
- Use the existing limits and page reader; batch exhaustion must page or return the adopted explicit error, never silently lose events.
- No exactly-once transport claim. Client event application and cursor persistence must be atomic, with duplicate IDs ignored.

## Review Focus

Cover the actual boundary failures: terminal during a gap; append during catch-up; multiple public events from one Journal entry; private filtered rows; replay-unavailable history; deleted/foreign tasks and revoked credentials; duplicate delivery and a client crash before durable save.

## Prerequisite handoff

A2A-06 supplies the accepted standard adapter, native Host/projector/reader,
listener and selected/omitted artifact driver. G0 supplies the exact wire
schema and HTTP/pre-header/post-header error mapping, extension URI, cursor
compatibility/deletion contract and issue exception. These remain blocking
until adopted; an implementer must not invent them from this plan.

### Task A2A-R1.1: Native replay operation and optional wire adapter

**Files:** Modify proposed predecessor files `sdk/port/channel/task.go`, `task_test.go`, `internal/channelhost/tasks.go`, `task_stream.go`, `task_stream_test.go`, `http.go`, `http_test.go`, `internal/app/channels.go`, `channels_test.go`, `plugins/a2a-server/handler.go`, `card.go`, `card_test.go`; create proposed `plugins/a2a-server/replay.go`, `replay_test.go`. These predecessor files must exist at this task's accepted baseline; do not create replacement subsystems if they do not.

**Interfaces:** Add the sixth conditional TaskHost method `ReplayTaskEvents(context.Context, TaskReplayQuery) (TaskStream, error)` and `TaskReplayQuery{TaskID, ContextID, After string}` from design 8.2. Preserve the five standard methods. Grant wrappers forward replay only with the same real Host capability and channel.a2a grant. Consume existing TaskUpdate, JournalPageReader and protocol mappers. Private plugin constructor `newReplayHandler(tasks channel.TaskHost) http.Handler` implements only the frozen extension operation; extend the existing router's exact allowlist rather than adding HTTPMounts or another listener. G0's safe discovery feature tells the card whether the replay URI can be advertised.

- [ ] **Step 1:** Write `TestTaskReplayRetainedHistory`, `TestTaskReplayGrantWrapper`, `TestA2AReplayWire` and `TestA2AExtensionCard` with these assertions:

```text
missing after => retained public events from origin, no initial Task snapshot
exclusive cursor => next ordinal/event, deterministic IDs survive restart
terminal task => retained tail through terminal then EOF
historical input/auth-required => later retained events still delivered
interrupted current tail => waiting status delivered then EOF
filtered entries / page boundary / concurrent append => no skipped public record
explicit after and conflicting Last-Event-ID => body after remains authoritative
route/Card claim => present only with healthy adopted B implementation
existing standard subscription => snapshot first; already-terminal still unsupported
```

- [ ] **Step 2:** Run `go test ./internal/channelhost ./internal/app ./sdk/port/channel -run 'TaskReplay' -count=1 -v` and `go -C plugins/a2a-server test ./... -run 'A2AReplayWire|A2AExtensionCard' -count=1 -v`. Expect missing replay behavior to fail, not an unrelated environment failure.
- [ ] **Step 3:** Extend the existing native reader/projector with replay mode. Register notification before capturing head; page through committed history and tail on the same connection; use the inherited fallback wakeup and backpressure. Validate current scope before decoding detailed positions. Emit full public event mappings with stable cursor ID and flush each complete bounded frame. Close/release on transport failure without cancelling the Run. Freeze and use G0's pre-header and post-header errors; no raw storage cause reaches the wire.
- [ ] **Step 4:** Repeat focused tests with `-race -count=1`, plus all accepted official-client regression cases. Require deterministic event/cursor pairs, no iterator leak, no cross-principal disclosure and unchanged standard SSE framing. `go -C plugins/a2a-server test ./... -count=1` must include the actual named tests rather than a zero-test selection.
- [ ] **Step 5:** Commit this capability and evidence: `feat(a2a): add optional retained event replay`.

### Task A2A-R1.2: Durable client recovery and final artifact acceptance

**Files:** Extend `plugins/a2a-server/replay_test.go`, `internal/channelhost/task_stream_test.go`, `sdk/internal/a2a_artifact_test.go` and existing backend conformance suites. Add `plugins/a2a-server/testdata/replay_client.go` only if an isolated executable fixture is clearer than in-test code; it is never a shipped client. Update the design/index/TODO and Story iteration logs with actual results. Refresh source-bound conformance only from executed evidence.

**Interfaces:** Consume the adopted replay operation, real composed native task path and A2A-06 artifact driver. The enhanced test client durably stores its applied event IDs/state and latest cursor in one transaction; reuse a test persistence facility or a single atomically replaced test-state file. It has no native database access. Produce B recovery evidence and new selected/omitted artifact identities after the extension change.

- [ ] **Step 1:** Write `TestA2AReplayDurableClient` and `TestA2AReplayDenials`. Start a real native task, persist applied events/cursor, disconnect during catch-up, commit more events and terminal state, restart both server and fixture, then resume. Assert the complete expected public ID sequence with no double-applied event. Inject duplicate delivery, duplicate/missed wakeups and client failure before durable save. Test malformed/oversized/foreign/future/unknown-version cursors, context mismatch, unauthorized principals, unavailable retained history, deleted Session, token revocation and slow/oversized output. Each must match G0's exact error contract; foreign and unknown tasks remain indistinguishable.
- [ ] **Step 2:** Run `go -C plugins/a2a-server test ./... -run 'A2AReplayDurableClient|A2AReplayDenials' -count=1 -v` through the real artifact driver's endpoint fixture. Expect failing recovery/security assertions before completing the fixture; skipped server/client persistence or absent PostgreSQL is a blocked check, never a passing result.
- [ ] **Step 3:** Complete the smallest durable fixture and any missing replay failure behavior. Replay-unavailable may permit a separate GetTask state inspection if accessible, but must never certify the old event log complete. Run the scenario against both SQLite and an actual disposable PostgreSQL instance. Reuse the existing mapper/reader; do not add retained shadow output or a replay daemon to satisfy the test.
- [ ] **Step 4:** Repeat the focused recovery tests, all standard-client regressions, and A2A-06.3's verify/pack/Inspect/conformance matrix plus `just ci`. Pack into new directories; prove current default/non-A2A artifacts omit both standard and extension surfaces and dependency closure. Record durable client apply-once evidence separately from server delivery order, along with revisions, exits and skips. Old pre-extension source hashes do not certify the new artifact.
- [ ] **Step 5:** Commit final B acceptance evidence: `test(a2a): verify durable replay and refreshed artifacts`. Update the single index and Story logs; G2 may be accepted only when all selected B and base requirements pass. No merge, deployment or issue closure is implied.

## Acceptance and handoff

Return adopted wire contract, real standard/enhanced client transcripts,
server/client restart evidence, both-backend results, grant/security failures,
current artifact/Inspect identities and omission rollback. Keep native history
and receipt tombstones on removal. Rejected or unavailable replay must be
honest; state convergence alone cannot close B acceptance.
