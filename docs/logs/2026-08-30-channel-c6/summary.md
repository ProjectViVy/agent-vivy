# CH-C6 — `plugins/dingtalk` Stream direct-message text (summary)

Date: 2026-08-30. Branch `feat/channel-c6` (cut from `feat/channel-c5`
d3d4342; sequential slices reused the same worktree).
PLAN: `docs/plans/channel-epic/CH-C6.md`. Contract:
`VIVY-CHANNEL-PACK.md` §9/§11/§14.1 (dingtalk row). Reference:
`.workspace/picoclaw/pkg/channels/dingtalk` (rewritten, zero imports).

## What changed

Second real ear: DingTalk Stream (outbound WS client) direct-message text loop, with
the directory shape modeled on the telegram template. The default EXE has zero
DingTalk dependency.

1. **Independent module** `plugins/dingtalk` (`dingtalk-stream-sdk-go v0.9.1`;
   direct dependencies are only agent-vivy + SDK, with gorilla/websocket remaining
   indirect).
2. **Stream lifecycle**: SDK automatic reconnect is disabled (its background reconnect
   loop survives Stop), and a plugin-side 3s context-aware redial supervisor takes over;
   Stop is idempotent with bounded waiting and closes the racing socket after stopping;
   **late-callback fence** (the SDK dispatches frames with `context.Background()`;
   frames arriving after Stop are dropped directly, without rememberWebhook or
   PublishInbound—the review should-fix was applied and pinned by tests).
3. **Inbound**: direct-message (`conversationType "1"`) plain text only;
   `content.content` fallback, bot self-loop protection (`ChatbotUserId`, one
   layer beyond picoclaw), and sender fallback order match picoclaw;
   `Sender = "dingtalk:<staffId>"`. sessionWebhook is stored only in adapter
   memory (latest per session), never in kernel config/envelope/RPC; webhook URLs are
   redacted from the error chain (query contains access_token, including the NewRequest
   parse-failure path, closing both D-010 routes).
4. **Outbound**: `Send` POSTs to the stored sessionWebhook
   (`{"msgtype":"text","text":{"content":...}}`; the initial draft's character
   shape was corrected against the SDK `SimpleReplyText`); handles
   errcode/errmsg; an unknown session (no webhook after restart) fails closed. No
   markdown/cards/groups.
5. **Credential extension (common envelope fix)**: `hostEnv.Secret` now accepts
   the envelope `token_env` **or** a top-level settings `*_env` declaration
   (the standing-command `token_env / *_env` pattern); dingtalk declares the two
   secrets `client_id_env` + `client_secret_env`; nested/non-string
   declarations are ignored (fail-closed), and values never enter logs. Telegram's
   existing semantics are unchanged (its settings token_env == envelope name).
6. **CH-C2-N1 closeout**: added the four verify fixtures (transport=webhook / duplicate
   grant / tool seam claiming a channel-family grant / negative max_message_runes),
   asserting each corresponding rule; `go test ./sdk/...` is green.
7. **Real SDK loopback test**: hand-written RFC 6455 gateway (stdlib, zero new
   dependencies) runs the real SDK client end to end: ticket acquisition → handshake →
   CALLBACK frame → ack → PublishInbound → Send → no redial after Stop. `-race`
   is clean.

## Explicitly not done

- Group triggers / @ lists / markdown replies / cards / media (§14.3 explicitly excludes
  them from the first slice).
- HTTP webhook bot shape (rejected by the Octos baseline); `channel.webhook` grant
  is not enabled.
- Silent redial for a dead ear (the SDK default logger emits nothing and the plugin has
  no logging surface)—recorded in §0.1 CH-C6-N1.
- Settings `*_env` names have no strict `^[A-Z_][A-Z0-9_]*$` validation
  (currently any string is a declaration; the adapter's strict decode constrains actual
  use)—recorded as the §0.1 CH-C6-N2 hardening candidate.
- The maximum ~50s mutual-exclusion window between Stop and an in-flight redial (SDK
  mutex + gorilla dial ctx blind spot)—documented, with no leak (-race verified).
- Real DingTalk organization manual smoke was not run (no credentials; does not block
  CI; rollback = recipe does not name dingtalk).
