# CH-C7b — `plugins/qq` Official Bot Text

## 1. Identity

| | |
|---|---|
| ID | CH-C7b |
| Stage | F |
| Person-days | 2 |
| Milestone | M-CH3 |
| Dependency | CH-C3; C4 should be merged |
| Branch | `feat/channel-c7b` |
| Contract | §14.3 qq; official open-platform bot, not a personal account, not OneBot |

## 2. Goal

Independent `plugins/qq`. Private-chat/channel text that the official Bot WS can reliably receive. The default EXE has no botgo.

## 3. Current State

DIVA: no personal account / NapCat add-on.

**Note:** Before formally writing the QQ adapter, first read picoclaw—it is the **most complete** Go sample among the channel implementations. Read-only rewrite; imports are prohibited. Reference: `.workspace/picoclaw/pkg/channels/qq` or `C:\Users\Administrator\Desktop\morediva\.workspace\picoclaw\pkg\channels\qq`. It is an official open-platform bot, not a personal account and not OneBot. See `00-standing-orders.md`.

## 4. Target Structure

Copy C4. Credential env keys: `app_id` / `app_secret`. transport poll (outbound WS).

## 5. File Inventory

**Create** `plugins/qq/**`. No OneBot bridge, personal account, or botgo in a species go.mod.

## 6. Steps

1. Rewrite the official WS; inbound PublishInbound.
2. Scope is limited to "text reliably received by the official API"; do not invent the full Guild feature set.
3. verify + pack --with qq.
4. `just ci` default path.
5. Log to `docs/logs/YYYY-MM-DD-channel-c7b/`.

## 7. Acceptance

- Candidate text loop works end to end; the default has no botgo.
- Empty allow_from is rejected.
- Both code and documentation state: not a personal account, not OneBot.

## 8. Prohibitions

- Personal accounts, OneBot, NapCat, large-file base64, or voice.
- A second body (external qq.exe).

## 9. Risks and Rollback

- Official API event coverage may be incomplete: narrow the scope; do not change the Host.
- Rollback: do not name it in the recipe.

## 10. Handoff

[CH-C7c.md](CH-C7c.md).

> **DONE 2026-08-30** — Branch `feat/channel-c7b` (based on c7a). Template notes: botgo's ChanManager/token self-starting goroutine is unusable (verified in source), so use a self-driven `websocket.ClientImpl` + resume + supervised redial; passive replies rely on inbound msg_id (memory window); group-event addresses cannot be decoded in botgo v0.2.1, so do not attempt it. Filing: `docs/logs/2026-08-30-channel-c7b/`.
