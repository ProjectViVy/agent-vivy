# Verification

Commands used the shared Go 1.26.4 toolchain via
`PATH=/workspace/agent-vivy/work/bin:$PATH`. UI dependencies were linked to the
parent's installed dependency tree; the temporary link is not committed.
Generated extensions were staged with `node scripts/stage-ui-assembly.mjs` before
TypeScript and full UI checks, without editing generated sources.

## RED and GREEN

Before production edits:

- `go test ./sdk/tui/live -run TestLiveDequeuedTurnResendsCapturedOptionsAfterTextEdit -count=1`
  failed for idle and busy resend: recalled thinking remained `auto`, expected
  `high` (0.021s).
- `vitest run src/components/chat/ChatInput.restore.test.tsx` failed because the
  recalled image thumbnail was absent (0.300s). The existing text preset alone
  could not restore its captured DTO.

After the adapter change:

- Full UI Vitest: **78 files, 600 tests passed**, 29.64s. A final focused composer,
  store and SDK compatibility run passed **72 tests**, 1.46s, following the added mode control, localization and StrictMode attachment regression.
- `tsc --noEmit`: passed after assembly staging and the SDK store contract update.
- `go test ./sdk/tui/live ./sdk/tui/stream ./sdk/tui/view ./sdk/tui/i18n -count=1`:
  passed. Tests cover edited idle/busy resend, queue ID preflight, unsupported
  DTO preservation, separate abort recovery, raw-byte privacy, failed resend,
  image removal, captured file visibility, and a newer in-flight draft.
- `go test -race ./sdk/tui/live -run 'TestLive(DequeuedTurnResends|RecallPreflights|AbortRecall|RestoredResend|FailedQueuedRecall)' -count=10`:
  passed (1.182s), including the queued-failure edit regression. The final
  focused run additionally exercised successful follow-up resend (0.024s).
- `go vet ./sdk/tui/live ./sdk/tui/stream ./sdk/tui/view ./sdk/tui/surface ./sdk/tui/i18n`:
  passed.
- `node scripts/check-i18n-completeness.js`: passed, en/zh each 1569 keys and
  163 placeholders; runtime copy audit clean.
- `gofmt` applied to changed Go sources/tests; `git diff --check` passed.

## Limits

The parent explicitly owns integrated `just ci` and real browser tests after
landing this patch; this lane does not claim those gates passed. The parent
reported a browser RED reproducing lost high thinking and image after
reload/recall/resend before this adapter patch.

Editor recovery items are ephemeral per-session drafts, with durable original
payloads retained in the authoritative journal. This patch adds no persisted
face draft service. Legacy text-only responses/events keep their existing text
restore fallback. Unknown future option values are retained rather than
withdrawn by an editor that cannot represent them.
