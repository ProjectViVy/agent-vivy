# Verification

## Automated

- `just ci` — PASS: formatting, UI typecheck (24 files / 201 tests), UI build, Go vet, all main-module tests, headless compile, and every plugin/face module test.
- `go test ./internal/storage/sqlite ./internal/storage/postgres -count=1` — PASS; SQLite executes CN-22/CN-23 idempotence/conflict/concurrency conformance, Postgres package passes and its DSN-gated conformance remains conditional.
- `go test -race ./internal/runtime -run 'TestMapperBoundsLargeNonStreamingAssistantPayload|TestMessageProjector|TestServiceApprovalResumePersistsChunkBeforeProviderEOF' -count=1` — PASS.
- `go test -race ./sdk/tui/stream ./internal/tui -run 'PayloadVersion|V2|Completed' -count=1` — PASS.
- `go test ./internal/rpc -run TestSessionHistoryRepairsDurableAssistantProjection -count=1` — PASS; both `session/messages` and `session/get` repair a deliberately missing v2 projection from Journal without duplication.
- Approval-resume exact-once and loopback approved-conversation tests were repeated to expose stream observer races; both pass after preventing tool-result events from consuming post-tool model stream markers.
- Final LUNA MAX gate findings for the 64-byte completion envelope and child-worker v1 bypass were fixed; focused config/worker tests verify the 128-byte floor and bounded child delta/v2 output.
- `go test -race ./internal/runtime -run TestDeleteSessionSealsActiveRunAgainstResurrection -count=5` — PASS; deletion drains the active run, leaves no rows to resurrect, and rejects a later run on the tombstoned id.
- `go test ./internal/runtime -run 'TestDeleteSession(SealsActiveRunAgainstResurrection|RejectsLateExternalWorkerEvent)' -count=10` — PASS; native and supervised-worker producers cannot append or create a child after session deletion.
- `go test ./internal/runtime -run 'Test(MessageProjector|BudgetLedgerReplay|DeleteSession|RunShellRejects|ServiceContextStatusAndCompactSession)' -count=1` — PASS; covers strict v2 metadata, replay budget parity, native/worker deletion fences, shell tombstones, and deletion-safe synthetic compaction.
- `go test ./internal/app -run TestChildModelDeltasDoNotExhaustSemanticEventBudget -count=1` — PASS; 600 deltas fit under a one-semantic-event ledger and the v2 completion consumes that single slot.
- Broad gate attempts exposed an 80 ms one-shot scheduling fixture race: under load, startup recovery could correctly classify the not-yet-fired test job as an already-missed one-shot. Both fixtures now use a 500 ms startup margin and a bounded 15-second settle window; `go test ./internal/runtime -run '^TestCronAtJob(DisablesAfterRun|DeletesAfterSuccessfulRun)$' -count=10` passes.
- `go test ./internal/runtime -run 'TestDeleteSessionSealsRecoveredShellPendingRun|TestRunShellApprovalRecoversProtectedStateAfterRestart' -count=3` — PASS; restart-recovered shell runs retain their session fence and cannot append a late terminal after deletion.
- `go test ./internal/app -run 'TestDeleteSessionCancelsLiveChildWorker|TestChildModelDeltas' -count=5` — PASS; deletion reaches the live worker cancel handle and long child streams retain budget parity.
- `go test ./internal/runtime -run 'TestServiceContextStatusAndCompactSession|TestDeleteSession' -count=3` plus `go test ./internal/storage/sqlite ./internal/storage/postgres -count=1` — PASS; synthetic compaction events are removed with the session on both storage implementations.

## Real path

- `just vivy-code` — PASS; produced the independent headless-tagged terminal binary.
- `.\vivy-code.exe --help` — PASS.
- Non-PTY `.\vivy-code.exe` — PASS for the guard path (`tui: an interactive terminal is required`).
- Native PTY launch was attempted, but the host rejected `CreateProcessW` with OS error `-1073283067` before Vivy started. No interactive-screen claim is made. The real Bubble Tea/event-loop and loopback-control paths remain covered by the passing TUI and `internal/app` suites.
- No browser surface changed in this delivery; a split browser smoke was therefore not used as substitute evidence for the terminal protocol change.

## Review

- GPT-5.6-LUNA MAX specialists audited event architecture, storage idempotence, consumer compatibility, final correctness, and the delivery gate. Their late-producer and replay-budget findings were fixed before the final gate.
