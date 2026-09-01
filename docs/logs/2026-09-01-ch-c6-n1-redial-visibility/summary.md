# CH-C6-N1 — supervised redial visibility (ChannelEnv log face)

## What changed

The supervised redial loops retried silently: dingtalk's supervisor
swallowed every `stream.Start` error, qq's swallowed dial/auth/handshake
failures after the first attempt and exited silently on the terminal
cannot-identify give-up. A revoked credential or dead gateway looked
exactly like a healthy channel from the outside — the inspect note only
records Start failures, and the Host cannot observe anything that happens
after Start returns.

CH-C6-N1 gives adapters a kernel-structured logging surface:

- `sdk/plugin/channel.go`: new **optional** face `plugin.ChannelLogger`
  (`Logger() *slog.Logger`). Optional by design — envs without it keep
  adapters silent, so the ChannelEnv ABI stays additive and the wire
  protocol grows no log channel until out-of-process channels need one.
- `internal/channelhost/channelenv.go`: `hostEnv.Logger()` returns the
  Host logger pre-scoped with `channel=<name>`, so adapter lifecycle
  lines land in the kernel's structured log under the right channel
  without the adapter naming itself.
- `plugins/dingtalk`: captures the face at Start (set-once before the
  supervisor goroutine exists; nil-safe); `supervise` warns per failed
  redial (`failures`, `err`) and logs the recovery (`failed_attempts`).
  `streamRedialDelay` const → var so lifecycle tests can shrink it (the
  qq `shrinkRedialDelay` pattern).
- `plugins/qq`: same set-once face capture; `supervise` warns each failed
  attempt with a `stage` field (`session` / `dial gateway` /
  `authenticate` / `handshake`), logs the recovery, and — the previously
  completely silent terminal path — **errors on the cannot-identify
  give-up** ("the ear stays deaf until the channel restarts"), which is
  the operator's one visible signal that the ear is dead for good.

Values logged are gateway errors and attempt counts only; adapter code
never logs secrets through the face (D-010 holds).

## Explicitly not done

- The pluginhost wire protocol (out-of-process channels) gains no log
  channel: a wire env without the face keeps its adapter silent, exactly
  like before. A log-capable wire face belongs to the slice that first
  runs channels out-of-process.
- DingTalk silent network death (NAT timeout with no disconnect frame —
  the ear stays deaf without any redial ever being attempted) is a
  different finding and stays open as CH-C6-N3 (dead-link detection).
- feishu's supervise loop is untouched: its silence shape is CH-C7a-N1's
  stop-during-start finding plus the same redial silence; adopting the
  face there is mechanical once that slice touches the file.
