# CH-C3 — ChannelHost + Fake-Plugin TCK

## 1. Identity

| | |
|---|---|
| ID | CH-C3 |
| Stage | C World Entry |
| Person-days | 3 |
| Milestone | M-CH1 |
| Dependency | CH-C2 |
| Successor | CH-C4 (template); C6/C7* can branch separately |
| Branch | `feat/channel-c3` |
| Contract | §7 ChannelHost, §8 capability matrix, C3 |

## 2. Goal

A fake channel sends a text → `channel.inbound` is recorded → `Message(source=channel)` → `Service.Run` → terminal-state `Send` back to the fake adapter. Empty `allow_from` rejects `Start`. The default EXE still has no real protocol.

This is the first slice in which the ear fan truly exists. All optional capability interfaces must be **declared** in this slice (the fake plugin need not implement them).

## 3. Current State (After C1+C2)

- Message has provenance; there is a `channel.inbound` event.
- sdk/plugin has Channel/Env; Adapt skips channels.
- No `internal/channelhost`.
- `internal/app/app.go` sends only `pluginhost.Adapt(Register())` into the tools table.
- `Service.Run(ctx, sessionID, userText)` is available.

## 4. Target Structure

```text
internal/channelhost/          # Must not import eino*
  host.go         StartAll/StopAll；fail-closed
  session.go      (channel, chat_id[, topic_id]) → Session
  dispatch.go     PublishInbound implementation: ingest → Run → subscribe to terminal state → Send
  capabilities.go all optional capability interface types + Discover
  fake/           test Channel; not a plugins/ product

internal/app
  Register() routing: tool → pluginhost.Adapt; channel → channelhost.Host
  Host holds Service's Run callback; do not make channelhost import the full runtime loop in depth
```

Sessions: local UI Sessions and channel Sessions **do not merge**. New Sessions carry the sandbox default.

Configuration: if `channels.<name>` is not in the compiled-in set, startup fails. Empty allow_from → the channel does not Start; record an error and do not allow it.

## 5. File Inventory

**Create** `internal/channelhost/` (including tests and fake)

**Modify** `internal/app/app.go` assembly; startup validation in `internal/config` (if C2 only implemented parse); use `importlint` to confirm channelhost has no eino.

**Do Not Touch** `plugins/telegram`; platform SDKs in species go.mod; direct NewRunner in `engine.go`.

## 6. Steps

1. Create the package. Host dependency interfaces: Journal, Messages, Sessions, `Run(sessionID, text)`, and the config envelope.
2. List all optional interfaces from evolution document §5 in `capabilities.go`. Discover them with type assertions.
3. Fake Channel: after Start, call `env.PublishInbound` with one text; record Send in memory.
4. TCK:
   - Empty allow_from → Start fails.
   - Non-empty allow_from + sender not on the list → do not Run.
   - Sender on the list → journal has inbound; Message.Source=channel; Run is called.
   - Run reaches terminal state → fake.Send is called.
   - Unknown configuration name → startup error.
5. App assembly: when there are no channel plugins, Host.StartAll is a no-op. Default Register() nil → behavior remains unchanged.
6. `just ci`.
7. Log to `docs/logs/YYYY-MM-DD-channel-c3/`.

## 7. Acceptance

- All of the above TCKs are green.
- `internal/channelhost` has no `github.com/cloudwego/eino` import (importlint).
- Default `just run` still has no ears and does not initiate Telegram HTTP.
- The optional-interface file exists and is referenced by Discover, even if the fake implements none of them.

## 8. Prohibitions

- Real protocol SDKs.
- Have Listen bind `:8787` `/rpc`.
- Have the Host know `parse_mode` / Lark encrypt.
- Allow empty allow_from.
- `"*"` allow_from.
- Let plugins hold `*runtime.Service`.

## 9. Risks and Rollback

- Service.Run and Host lifecycle: Run is asynchronous. TCK must wait for the terminal state or inject a fake Run.
- Avoid the channelhost → runtime → channelhost cycle: inject a function from app.
- Rollback: remove the app assembly to make the ears disappear.

## 10. Handoff

The next AGENT should prioritize [CH-C4.md](CH-C4.md). C6 may open another worktree in parallel, but it must be based on the merged Host ABI.

> **DONE 2026-08-30** — Branch `feat/channel-c3`. Host ABI finalized: `internal/channelhost` (Deps{Journal,Messages,Sessions,Run func,Channels,Config,Logger}, Discover/`Capabilities`, `chanin_*` recording, `sess_ch_<hash>` session mapping, `RunOptions.Provenance`). C4 consumes these symbols; do not rename them. Also hardened §0.1 CH-C3-N1/N2. Filing: `docs/logs/2026-08-30-channel-c3/`.

Starting with C4, implement channels formally: first read the corresponding picoclaw package (the most complete Go sample), rewrite it read-only, and do not import it. See `00-standing-orders.md`.
