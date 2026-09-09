# CH-C6 — acceptance (how a person can tell it worked)

Date: 2026-08-30.

## Product view: the first domestic long-lived ear

A DingTalk user sends direct-message text to the bot → the Stream long connection
receives it → the ledger records `channel.inbound` → Run → the reply returns
through sessionWebhook. No public address or webhook server is needed—the meaning of
"domestic long-lived" is precisely this.

## Human-verifiable points

1. **The gate is green**: `just ci` exits 0; the default EXE dependency graph
   contains no DingTalk SDK.
2. **Packaging is enough**: `vivy-sdk verify plugins/dingtalk` → ok;
   `vivy-sdk pack --with dingtalk` → candidate EXE (inspect lists dingtalk); the
   product tree's `go.mod` is byte-for-byte unchanged.
3. **Full loopback demonstration** (inside CI, no real network): tests start a local
   DingTalk protocol gateway; the real SDK client completes ticket acquisition, WS
   handshake, and direct-message text frame → the plugin emits the normalized envelope
   (Channel=dingtalk, Sender=dingtalk:<id>) → the reply POSTs back through
   sessionWebhook as `{"msgtype":"text",...}` → no redial after Stop.
4. **The three fail-closed checks remain**: empty allow_from refuses Start; an
   unlisted sender is dropped; missing `client_id_env`/`client_secret_env`
   in settings or an unset env → Start fails with a visible reason (Host inspect note).
5. **Key discipline**: `client_secret` exists only in the environment; even the
   sessionWebhook access_token is stripped from error messages (both failure phases have
   pinned tests).
6. **Settings recognizes it automatically**: the C5 Channels page is inspect-driven—
   pack dingtalk into a generation and that generation's Settings page shows a DingTalk
   card without any UI change.

## Explicitly not part of this slice's acceptance

- Group chat/@ triggers/markdown/cards/media → later work.
- Real DingTalk organization send/receive → pre-release human acceptance (requires
  enterprise-internal app credentials; rollback = recipe does not name dingtalk).
- Visible alerting for a dead ear (revoked credentials) → §0.1 CH-C6-N1.

## Rollback

Omit dingtalk from the recipe and it disappears; revert this branch and this ear is
gone.
