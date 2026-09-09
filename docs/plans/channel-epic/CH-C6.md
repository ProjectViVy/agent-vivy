# CH-C6 — `plugins/dingtalk` Stream Private-Chat Text

## 1. Identity

| | |
|---|---|
| ID | CH-C6 |
| Stage | F Domestic Overnight |
| Person-days | 2 |
| Milestone | M-CH3 |
| Dependency | CH-C3 (Host ABI); recommended to wait for the C4 shape |
| Parallelism | Can use a separate worktree from C4 |
| Branch | `feat/channel-c6` |
| Contract | §14.3 dingtalk; Stream is not an Octos webhook |

## 2. Goal

Independent `plugins/dingtalk`. Complete the Stream WS private-chat text loop. `session_webhook` enters only plugin settings / runtime, not the kernel Config. The default EXE has no DingTalk SDK.

## 3. Current State

C3 Host generic envelope. DIVA: retain Stream; do not regress to a webhook text bot.

**Note:** Before formally writing the DingTalk adapter, first read picoclaw—it is the **most complete** Go sample among the channel implementations. Read-only rewrite; imports are prohibited. Reference: `.workspace/picoclaw/pkg/channels/dingtalk` or `C:\Users\Administrator\Desktop\morediva\.workspace\picoclaw\pkg\channels\dingtalk`. Use Stream WS; do not copy it into an Octos webhook text bot. See `00-standing-orders.md`.

## 4. Target Structure

Copy the directory shape from [CH-C4.md](CH-C4.md). `vivy-plugin.json`: `transport: poll` (the outbound WS client counts as poll for this batch). grants: `channel.poll` + `secret.read`. Credentials: the env_key for `client_id` / `client_secret`.

## 5. File Inventory

**Create** `plugins/dingtalk/**` (independent go.mod).

**Prohibited:** Teach the Host about DingTalk cards; add the DingTalk SDK to a species go.mod; turn it into an HTTP webhook bot.

## 6. Steps

1. Independent module; rewrite the picoclaw Stream client.
2. Inbound PublishInbound; outbound Send uses the session webhook carried by the inbound message (stored on the plugin side).
3. verify + pack --with dingtalk.
4. The default `just ci` path has no such SDK.
5. Log to `docs/logs/YYYY-MM-DD-channel-c6/`.

## 7. Acceptance

- Candidate private-chat text works; the default body has no DingTalk dependency.
- Empty allow_from is rejected.
- No public webhook mode.

## 8. Prohibitions

- Use an Octos-style webhook text bot as the baseline.
- Cards / media (later slice).
- `channel.webhook` grant.

## 9. Risks and Rollback

- Stream protocol changes: prioritize reliably receiving private-chat text.
- Rollback: do not name it in the recipe.

## 10. Handoff

[CH-C7a.md](CH-C7a.md). Do not change the envelope slots in this slice.

> **DONE 2026-08-30** — Branch `feat/channel-c6`. Package shape is fully isomorphic with telegram; `hostEnv.Secret` supports top-level `*_env` multi-secret declarations in settings (feishu/qq app_id+app_secret directly reuse this pattern); this package contains the handling template for sessionWebhook-style "inbound-carried reply endpoints". Filing: `docs/logs/2026-08-30-channel-c6/`.
