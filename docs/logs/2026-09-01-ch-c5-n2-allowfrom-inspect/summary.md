# CH-C5-N2: allow_from on the inspect face + UI error/empty distinction

## What changed

- `internal/channelhost/host.go`: `ChannelStatus` gains `AllowFrom` — the
  startup-effective envelope's allowed-sender summary, populated in
  `Inspect()` next to `Configured`/`Enabled`/`TokenEnv`. Sender IDs are
  not secrets (D-010) and already cross `channel/get`.
- `internal/rpc/control.go`: `channelStatusResult` gains
  `allow_from` (always a JSON array, never null — normalized in
  `toChannelStatusResult`). The UI's pending-restart derivation now has
  document truth (`channel/get`) vs process truth (`channel/inspect`)
  for allow_from too, so a pure allow_from edit is no longer invisible.
- `ui/src/lib/api.ts`: `ChannelStatus.allow_from: string[]`.
- `ui/src/components/settings/channel-store.ts`:
  `channelPendingRestart` compares `allow_from` element-wise (both sides
  preserve write order; a reorder-only edit is also a difference and
  clears on restart).
- `ui/src/components/settings/ChannelsSettings.tsx`: inspect failure now
  renders a distinct error panel (AlertTriangle + raw error + refresh
  button) instead of falling through to the "no ears in this generation"
  empty state, which misread an RPC failure as an empty generation. The
  redundant toolbar error span is removed; the list sidebar only renders
  with `statuses.length > 0` (also hides it while loading).
- i18n: `channels.inspectError` (en/zh).

## Tests

- Kernel: `TestInspectNotesRecordStartAllDecisions` asserts the configured
  channel's `AllowFrom` summary; `TestInspectNotesForSkips` asserts nil
  for an unconfigured channel.
- RPC: `TestChannelInspectRPC` asserts `allow_from: ["alice"]` on the
  wire; `TestChannelInspectAllowFromAlwaysArray` pins nil → `[]`.
- UI: `channel-store.test.ts` gains the pure allow_from-edit pending case
  (`['alice','bob']` and `[]` both pending) and the equal-fixture case
  now covers allow_from equality.

## Companion fix in the same session

- `internal/rpc/accesslog_test.go` had a pre-existing data race (plain
  buffer read while the websocket middleware logged from the server
  goroutine) that blocked the `-race` gate; fixed and committed separately
  (9c86023, `docs/logs/2026-09-01-accesslog-ws-race/`).

## Not done

- Server-push of the pending-restart state: the badge still appears on
  the next `refreshChannels()` (save path already re-pulls), not on a
  timer.
- Grouping/normalizing allow_from order in the editor; order-sensitive
  comparison is intentional (document truth vs process truth).
