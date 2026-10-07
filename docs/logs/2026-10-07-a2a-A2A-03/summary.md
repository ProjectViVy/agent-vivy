# A2A-03 summary — atomic ordinary answers and honest crash recovery

Outcome: the remote ordinary answer now rides the same atomic native
question transition the local Review/Face path uses, and interrupted
settlements classify as one durable failure instead of silently retrying.

## Shipped

- `internal/storage/question_transitions.go` — the shared transition
  contract: `QuestionTransitionCommit` (captured QuestionID + RunID +
  outcome + actor/reason + runtime-built journal event),
  `QuestionTransitionStore.CommitQuestionTransition` for local
  answer/cancel/expiry, and `ChannelTaskAnswerCommit` adding scope,
  message id, input hash and the captured question to the same commit.
- `ChannelTaskStore.CommitChannelTaskAnswer` (both backends) —
  scope lock → receipt pre-check → question CAS → answered envelope →
  receipt, one transaction. PostgreSQL takes
  `pg_advisory_xact_lock(hashtextextended(runID, 0))` first so the
  transition serializes with `Journal.Append`; a row lock alone would
  not. SQLite commits under its single-writer transaction.
- `internal/runtime/service.go` — `AnswerQuestion`, `cancelQuestion` and
  `expireQuestion` route through `CommitQuestionTransition` when the
  store supports it (legacy lifecycle APIs preserved for other callers);
  `dispatchQuestionResume` is the shared pending-slot resume handoff.
- `internal/runtime/channel_tasks.go` — `SubmitChannelTask` dispatches
  messages carrying `question_id` to `submitChannelTaskAnswer`:
  `channel-task/v1` hash with op `answer` binding context/run/question
  selectors, joined + TrimSpace'd parts, answered envelope at payload
  version 2 with actor `channel:a2a:<principal>`, receipt operation
  `answer`. Resume is scheduled only for `NewlyCommitted` winners.
- `schemas/events/payloads/user.question_answered.json` v2 — optional
  `actor`; v1 events remain valid subsets.

## Contract settled

- Remote answers are ordinary answer text, never commands: `/approve`,
  `/deny`, `/pending` settle only the named pending question and never
  touch `ApprovalStore`; an approval id as the question selector is
  ErrNotFound with no receipt.
- Receipt PK is (instance, principal, message); identical retry →
  original receipt, no second transition; a fresh message naming a
  settled question is ErrConflict and cannot reach a later question on
  the same run.
- Committed-answer/process-loss gap: restart recovery's default branch
  classifies the run as one durable failure (`no pending approval`),
  retaining question + answered-event evidence; terminal runs are never
  reopened.
