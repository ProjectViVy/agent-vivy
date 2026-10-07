# A2A-03 verification

## Backend fault matrix (sqlite + postgres)

`TestChannelTaskAnswerAtomicRace` /
`TestChannelTaskAnswerAtomicRacePG`,
`TestQuestionTransitionRollback`, `TestQuestionTransitionLocal`:

- remote answer commits question CAS + answered envelope (v2) + receipt
  in one transaction; receipt `accepted_seq` = event seq
- identical retry returns original receipt, `NewlyCommitted=false`,
  exactly one answered event
- same message id + different body → `ErrConflict`
- local (`CommitQuestionTransition`) vs remote race → exactly one
  winner, one answered event, question settled once
- losing message on a settled question → `ErrConflict`, no receipt, and
  a later pending question on the same run stays untouched
- run mismatch → `ErrNotFound`; run with committed terminal →
  `ErrRunClosed`; expired question → `ErrConflict`; unknown scope →
  `ErrNotFound`; none leave receipts
- trigger-forced mid-transaction abort → question stays pending, no
  event, no receipt

Runs:
- `go test ./internal/storage/sqlite -run 'ChannelTaskAnswerAtomicRace|QuestionTransition' -count=1` ok
- `go test -race` same → ok
- `VIVY_POSTGRES_TEST_DSN=... go test ./internal/storage/postgres -run ChannelTaskAnswer -count=1` ok
- `go test -race` postgres → ok

## Runtime dispatch / approval isolation / crash

`TestChannelTaskAnswerDispatchAndAtomicity` — channel-admitted run
suspends on ask_user; remote answer (parts `["  blue ","sky"]` →
`"blue \n\nsky"`) settles question, resumes run to `RunCompleted`, actor
`channel:a2a:p-1`, answered event payload version 2; retry → original
receipt, no second event; approvals untouched.

`TestChannelTaskAnswerSelectorsAndLosers` — run selector mismatch and
ghost question id → `ErrNotFound`, no receipts; loser message on the
settled question → conflict, no reopen.

`TestA2AApprovalIsLocalOnly` — `/approve` settles the question it names
while a real pending approval on the same run stays
`ApprovalPending` with zero `tool.approval_decided` events; `/deny`,
`/pending` texts conflict on the settled question; approval id as
question selector → `ErrNotFound`, no receipt.

`TestChannelTaskAnswerCrashRecovery` — store-level committed answer +
dropped pending slot → `Recover` emits exactly one `run.failed`
classification (`no pending approval`), question stays answered, exactly
one answered event, zero tool executions after crash; subsequent answer
→ conflict, no reopen.

`TestChannelTaskAnswerConcurrentWithLocal` — local `AnswerQuestion` vs
remote `SubmitChannelTask` on the same pending question → exactly one
winner, one answered event.

- `go test ./internal/runtime -count=1` ok (58s, full package)
- `go test -race` answer-focused subset ok
- `go build ./...` clean

## just ci

Pending final run for the story boundary; prior known main regression
`TestClientAgainstRealVivyCode` resolved on this branch via merged
conformance fix (#38) while PR #37 (queued-turn codeface) still awaits
owner merge. Digest repinned to post-change `a30ba39e…` for the five
internal-rooted conformance rows.
