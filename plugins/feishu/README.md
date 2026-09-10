# plugins/feishu — Vivy’s Feishu / Lark ear (one-on-one text, persistent WebSocket)

`feishu` is a seam-channel plugin (VIVY-CHANNEL-PACK.md §9), started and
consumed by the kernel’s ChannelHost; it is not a model tool. Its directory
structure and Start/Stop/Send skeleton follow the shape template established by
`plugins/telegram`, and its lifecycle follows the supervised redial pattern in
`plugins/dingtalk`.

## Initial slice scope (what it does / does not do)

**Does:**

- One-on-one chats (`chat_type == "p2p"`) **receive/send plain text**
- Transport: **persistent outbound WebSocket** (Feishu event gateway, manifest
  `transport: "poll"` + grant `channel.poll`). The documentation describes a
  webhook callback, but this implementation uses a persistent connection—the
  plugin never listens on a port and never implements a webhook (contract §14.3:
  WS counts as poll)
- Ignore group chats, rich text / cards / images and other non-text messages,
  and the bot’s own messages (`sender_type == "bot"`) (to prevent echo loops);
  reaction replies are a separate event type, are never registered, and therefore
  never reach the ear
- Send replies through OpenAPI `im.v1.messages` (`receive_id_type=chat_id`,
  `msg_type=text`, `content={"text":...}`), collecting the platform’s returned
  `message_id`; the SDK manages tenant_access_token from the app credentials,
  and the plugin never touches any token

**Does not do (later slice):** group triggers, rich text / interactive cards,
media, reaction replies, reply threads and topics, webhook mode, or the full
markdown suite.

## Permission and policy boundaries

| Item | Owner |
|---|---|
| `allow_from` sender allowlist | Enforced by **Host (kernel)**; the plugin has no separate allowlist |
| Secret resolution | Host’s `Secret`; this plugin needs **two** credentials (app ID + app secret), which do not fit in the envelope’s single `token_env`, so their names are declared by the top-level `app_id_env` / `app_secret_env` keys in settings (CH-C6/D2 pattern), with Host approval |
| channel settings | Strictly decoded by the plugin (unknown fields fail closed); the kernel does not know the keys in `settings` |
| Listening port | None. The persistent connection is outbound-only; the SDK’s automatic reconnect is disabled and replaced by the plugin’s own ctx-aware supervision loop (a new client per redial, never revived after Stop, with no goroutine leaks) |

Senders are written as `feishu:<open_id>` (open_id first, falling back in
order to user_id and union_id when missing) and must **exactly match** an
`allow_from` entry in the configuration (no wildcards; `"*"` is not allowed).

## encrypt_key and verification_token

- `settings.encrypt_key` is a **regular settings value** under §14.3 (Host
  does not decode it). Events pushed over the persistent WebSocket are already
  plaintext, and the SDK’s WS dispatch path does not decrypt them; the key is
  carried and passed to the SDK event dispatcher as required by the contract
  (webhook-mode decryption is unused in WS mode), for contract completeness.
- **No `verification_token`**: URL challenge verification is the handshake for
  webhook mode and does not apply to persistent connections; picoclaw passes the
  key but likewise leaves it unused in its WS flow, so this plugin removes it
  from the settings surface.

## is_lark switch

`settings.is_lark: true` switches the gateway and OpenAPI domains from Feishu
(`https://open.feishu.cn`) to the international Lark version
(`https://open.larksuite.com`). `settings.open_base_url` can override the entire
domain (loopback stub for testing / dedicated gateway) and has the highest
priority, following the precedent of dingtalk’s `open_api_host`.

## 64-bit constraint

This plugin is compiled for **64-bit platforms** (amd64 / arm64). The lark SDK’s
dependency tree is not guaranteed to work for the 386 target—**a 386 build is
expected to fail; this is a hard constraint, not a bug**. Do not add a 386
compatibility shim for it.

## Configuration example (fixed envelope, opaque settings)

```yaml
channels:
  feishu:
    enabled: true
    allow_from: ["feishu:ou_xxxxxxxxxxxxxxxx"]
    # token_env may be empty: settings declares all credential names (see below)
    settings:                    # Opaque to the kernel; decoded by this plugin
      app_id_env: FEISHU_APP_ID          # Required: environment variable name for the app ID
      app_secret_env: FEISHU_APP_SECRET  # Required: environment variable name for the app secret (must differ from the above)
      # is_lark: false                   # Optional: true = international Lark domain
      # encrypt_key: "..."               # Optional: regular settings value (Host does not decode it; WS mode does not decrypt)
      # open_base_url: "https://open.feishu.cn"  # Optional: domain override (testing / dedicated deployment)
```

Secrets travel only through environment variables (D-010; values never appear
in configuration, logs, or event payloads):

```text
export FEISHU_APP_ID=cli_xxxxxxxxxxxx
export FEISHU_APP_SECRET=xxxxxxxxxxxxxxxxxxxxxxxxx
```

Unknown `settings` fields are **rejected**—a key the current lib does not know
requires a version bump and a new pack; it cannot be forced in through configuration.

## Packaging and verification

```text
vivy-sdk verify plugins/feishu          # Static rules + linkability
vivy-sdk pack --with feishu --out dist/ # Produce candidate EXE (linked with oapi-sdk-go)
vivy-sdk inspect-artifact dist/<gen>/   # recipes.plugins contains feishu
```

The standalone go.mod (`example.com/vivy/plugins/feishu`) is mandatory:
the default `just ci` and the species’ `go build ./cmd/vivy` import graphs do
not reach `github.com/larksuite/oapi-sdk-go`—only the generation produced by
pack has the ear in its body.

## Module dependencies

This module may import only `agent-vivy/sdk/module`, its focused `sdk/port` + the standard library +
`github.com/larksuite/oapi-sdk-go/v3`. Imports of
`agent-vivy/internal/...`, eino, picoclaw, or `.workspace` are forbidden;
`net.Listen` is forbidden; blank `init()` imports are forbidden.

## SDK version

The pinned version is `github.com/larksuite/oapi-sdk-go/v3 v3.11.0` (the v3
series’ rewritten WS client: `Start` observes ctx and returns, workers are
collected by a WaitGroup, and the client enters a terminal state after stopping).
The v3.9.4 WS client pinned by the picoclaw reference implementation blocks
forever in `Start` with `select{}` and leaks a never-exiting pingLoop
goroutine on every successful Start. That conflicts with this plugin’s
supervised lifecycle and hard no-leak requirement, so v3.11.0 is used as the
“most recent stable version with WS support.”
