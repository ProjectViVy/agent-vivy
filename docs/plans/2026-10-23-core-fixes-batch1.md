# Core fixes batch 1

Goal: protect live runs from operator recovery, resolve input files in the target
session workspace, preserve literal SQLite filenames, and durably retain each
queued request through restart and admission.

Design: reuse `RecoverBackground`, the session's persisted workspace selection,
and the existing Run/Journal admission boundary. Store resolved input snapshots
with queued requests and atomically carry the remaining queue in `run.started`;
this avoids a second queue database or a crash window between Run admission and
tail materialization. Follow-ups start one request per Run. Rich steering requests
degrade to follow-up because changing an admitted Run's policy/model input options
would violate its immutable admission. Text steering keeps the existing Eino path.

Eino capability check: the actual pin is Eino v0.9.21 (the skill's v0.9.13 example
is outdated). Inspected `adk/runner.go` (`TypedRunner`, `CheckPointStore`) and the
existing `adk.WithCancel`, `CancelAfterChatModel`, `ChatModelAgentResumeData`, and
`TypedRunner.ResumeWithParams` integration. These own in-run cancellation/resume;
they do not own Vivy Session queues, policy admissions, or Journal transactions.
Keep that small product seam in Vivy and retain the existing Eino engine.

- [x] Add regression cases and demonstrate failures for recovery RPC, selected
  workspace input, literal SQLite paths, and complete queued request restart.
- [x] Implement the three boundary fixes and verify focused regressions.
- [x] Extend queued request snapshots and shared RPC input preparation; propagate
  persistence failures before acknowledging enqueue; recover FIFO order and
  atomically hand pending requests to each new Run before inference starts.
- [x] Verify independent Linux wiring/kernel/D02 gates, focused Windows checks,
  and a disposable real-path smoke. Record unavailable lanes explicitly.
- [x] Review the integrated diff, write iteration evidence, and commit explicit
  paths under the configured human identity. No push is authorized.

Review focus: pending/live recovery, workspace escapes and absent sessions,
reserved URI characters and reopen, differing request options and input bytes,
enqueue failure, immediate follow-up completion, and crash after admission.

Ruling: idle removals may use Session-owned synthetic Journal tombstones because
a terminal Run cannot accept writes. This reuses the established synthetic-event
pattern, stores no second pending queue, and is deleted by both storage owners.

Ruling: keep private request bodies in the existing protected BlobStore; expose
only references in Journal/Face state. Interrupted steering remains retryable
until its Run completes successfully.
