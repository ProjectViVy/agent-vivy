# 2026-08-30 — plugins/feishu: Feishu/Lark direct-message text channel (CH-C7a)

## Change scope

Added `plugins/feishu/` (independent module
`example.com/vivy/plugins/feishu`): a seam-channel adapter for p2p
(direct-message) plain-text send/receive over the Feishu event gateway's
**outbound WebSocket long connection** (manifest `transport: "poll"`, grant
`channel.poll`), with no webhook or listening port.

- `plugin.go` — Start/Stop/Send skeleton (telegram/dingtalk-shaped template);
  supervised redial loop (SDK automatic reconnect disabled, a new client per redial,
  fail-closed first connection, no resurrection or goroutine leak after Stop); late
  event fence (stopped latch); `im.message.receive_v1` normalization (p2p +
  text + human sender only; group chat / non-text / bot echo / empty text all dropped
  locally); replies use `im.v1.messages`
  (`receive_id_type=chat_id`), with tenant_access_token managed by the SDK.
- `settings.go` — strict decode (unknown fields fail-closed):
  `app_id_env` / `app_secret_env` (required and distinct, CH-C6/D2
  `*_env` pattern), `encrypt_key` (ordinary settings value, §14.3,
  not decoded by the Host), `is_lark` (feishu↔lark domain switch), and
  `open_base_url` (loopback-test / dedicated-deployment override).
  **No `verification_token`**—URL challenge is a webhook-mode handshake and is
  not involved in long-connection mode.
- `vivy-plugin.json` — name `feishu`, seam `channel`, grants
  `[channel.poll, secret.read]`.
- `plugin_test.go` — network-free (httptest loopback only): settings matrix,
  domain-resolution matrix, event-normalization matrix, Start fail-closed matrix
  (t.Setenv set/clear credentials), full Send path (real SDK client against a
  loopback stub, asserting token and message endpoint URL/query/body), API errors
  surfaced, **real WS loopback** (SDK's own Frame codec builds frames, real
  dispatcher → PublishInbound → Send → ack assertion → zero redial after Stop),
  disconnect redial, and idempotent Stop.
- `README.md` — Chinese, mirroring the telegram/dingtalk structure.

## SDK version decision (deviation note)

Pinned `github.com/larksuite/oapi-sdk-go/v3 v3.11.0` (the rewritten v3-series
WS client), **not** the v3.9.4 pinned by picoclaw's go.mod. Reason: v3.9.4's WS
`Start` blocks forever on `select{}`, each successful Start leaks a
never-exiting pingLoop goroutine, and WS bootstrap cannot inject an HTTP client—all
conflict with the hard requirements for supervised lifecycle, no goroutine leaks, and
loopback testing. v3.11.0's `Start` observes ctx and returns, workers are
joined by WaitGroup, and the client enters a terminal state after stopping (so this
plugin uses a new client for every redial).

## Explicitly not done (later work)

Group triggers, rich text / interactive cards, media, reaction replies, reply threads
and topics, webhook mode, and the full markdown set. The 386 target is unsupported
(the lark SDK dependency tree fails to compile on 386 because `math.MaxInt64`
overflows—a hard constraint, not a bug, documented in the README).

## Not included in this slice

- Capture and commit of `docs/TODO.md` §0.1 are handled centrally by the
  coordinator; this worktree created no branch and made no commit.

## GOAL owner landing note (2026-08-30)

- Review: independent reviewer **PASS**; the SDK version deviation (v3.11.0 vs
  picoclaw v3.9.4) was verified against SDK source as necessary (old WS `Start`
  never returned from `select{}`, pingLoop could not exit, and there was no
  HTTP injection point). `just ci` exit 0.
- Recorded item: three early-return paths where `supervise`'s first connection
  overlaps Stop do not guarantee delivery of `firstErr` (unreachable in the
  Host's actual call order: Host Stops only ears whose Start completed; caller ctx
  cancellation can also unlock it)—recorded under standing order `docs/TODO.md`
  §0.1 CH-C7a-N1, with tested lifecycle code unchanged.
- Directory name normalized to `2026-08-30-channel-c7a` (formerly
  feishu-channel).
