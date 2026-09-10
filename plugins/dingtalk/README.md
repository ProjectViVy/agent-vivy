# plugins/dingtalk — Vivy’s DingTalk ear (one-on-one text, Stream mode)

`dingtalk` is a seam-channel plugin (VIVY-CHANNEL-PACK.md §9), started and
consumed by the kernel’s ChannelHost; it is not a model tool. Its directory
structure and Start/Stop/Send skeleton follow the shape template established by
`plugins/telegram`.

## Initial slice scope (what it does / does not do)

**Does:**

- One-on-one chats (`conversationType == "1"`) **receive/send plain text**
- Transport: **outbound WebSocket** (DingTalk Stream gateway, manifest
  `transport: "poll"` + grant `channel.poll`), with no webhook and no listening port
- Ignore group chats, cards / non-text messages, and the bot’s own messages (to prevent echo loops)
- Store the session webhook (the reply URL DingTalk sends with each message) as
  plugin-side runtime state keyed by conversationId (the latest one wins); on
  reply, POST a text message through `ChannelEnv.HTTP()` and validate
  `{errcode,errmsg}`

**Does not do (later slice):** group triggers, cards and interactive media, the
full markdown / actionCard suites, webhook mode, reply threads, or topics.

## Permission and policy boundaries

| Item | Owner |
|---|---|
| `allow_from` sender allowlist | Enforced by **Host (kernel)**; the plugin has no separate allowlist |
| Secret resolution | Host’s `Secret`; this plugin needs **two** credentials (app key + app secret), which do not fit in the envelope’s single `token_env`, so their names are declared by top-level `*_env` keys in settings (CH-C6/D2), with Host approval |
| channel settings | Strictly decoded by the plugin (unknown fields fail closed); the kernel does not know the keys in `settings` |
| Listening port | None. The Stream connection is outbound-only; the SDK’s “permanent reconnect” is disabled and replaced by the plugin’s own ctx-aware supervision loop (it never revives after Stop) |

Senders are written as `dingtalk:<senderStaffId or senderId>` and must
**exactly match** an `allow_from` entry in the configuration (no wildcards;
`"*"` is not allowed).

## Session webhook (sessionWebhook)

DingTalk does not provide a “send a message by conversation” API—each incoming
message carries a time-limited `sessionWebhook`, and replies can only be POSTed
to it. Therefore:

- It is **plugin-side runtime state**: it exists only in memory, stores the
  latest value by conversationId, and is cleared on restart;
- It **never enters** kernel configuration, the configuration envelope, or logs
  (D-010: the URL query string contains a session token, so the query string is
  stripped from outbound errors before they enter the error chain);
- `Send` **fails closed** for a conversation without a webhook—after a process
  restart, the other party must send the bot a message before Vivy can reply.

## Configuration example (fixed envelope, opaque settings)

```yaml
channels:
  dingtalk:
    enabled: true
    allow_from: ["dingtalk:manager1234"]
    # token_env may be empty: settings declares all credential names (see below)
    settings:                    # Opaque to the kernel; decoded by this plugin
      client_id_env: DINGTALK_CLIENT_ID        # Required: environment variable name for the app key
      client_secret_env: DINGTALK_CLIENT_SECRET # Required: environment variable name for the app secret
      # open_api_host: "https://api.dingtalk.com"  # Optional: gateway override (testing / dedicated deployment)
```

Secrets travel only through environment variables (D-010; values never appear
in configuration, logs, or event payloads):

```text
export DINGTALK_CLIENT_ID=dingxxxxxxxxxxxx
export DINGTALK_CLIENT_SECRET=xxxxxxxxxxxxxxxxxxxxxxxxx
```

Unknown `settings` fields are **rejected**—a key the current lib does not know
requires a version bump and a new pack; it cannot be forced in through configuration.

## Packaging and verification

```text
vivy-sdk verify plugins/dingtalk          # Static rules + linkability
vivy-sdk pack --with dingtalk --out dist/ # Produce candidate EXE (linked with dingtalk-stream-sdk-go)
vivy-sdk inspect-artifact dist/<gen>/     # recipes.plugins contains dingtalk
```

The standalone go.mod (`example.com/vivy/plugins/dingtalk`) is mandatory:
the default `just ci` and the species’ `go build ./cmd/vivy` import graphs do
not reach `github.com/open-dingtalk/dingtalk-stream-sdk-go`—only the generation
produced by pack has the ear in its body.

## Module dependencies

This module may import only `agent-vivy/sdk/module`, its focused `sdk/port` + the standard library +
`github.com/open-dingtalk/dingtalk-stream-sdk-go`. Imports of
`agent-vivy/internal/...`, eino, picoclaw, or `.workspace` are forbidden;
`net.Listen` is forbidden; blank `init()` imports are forbidden.
