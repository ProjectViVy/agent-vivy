# A2A-04 Verification

- `go test ./internal/journalview ./internal/storage/sqlite` (TestTaskTextProjectionEquivalence, TestJournalPageWatermark): green.
- postgres `TestJournalPageWatermarkPG`: green under VIVY_POSTGRES_TEST_DSN.
- `go test ./internal/channelhost` (incl. -race, 47s): green —
  TestTaskOperations (scoped submit/replay, foreign denied, revocation),
  TestTaskStateMap (INPUT_REQUIRED+safe prompt, answered precedence,
  AUTH_REQUIRED no args leak, terminal wins + unsafe content exclusion),
  TestTaskListAndCancel (totals, sorted pages, tamper/foreign/expired
  tokens → cursor_invalid, lazy + idempotent cancel, not_cancelable),
  TestTaskStreamReplayLive (snapshot-first, live artifacts+status to
  terminal EOF, interrupt EOF, cursor tail replay, malformed/foreign/
  too-new cursor rejects, 1s-tick catch-up of a silent commit).
