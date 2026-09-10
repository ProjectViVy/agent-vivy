# plugins/telegram — Vivy’s first real ear (private-chat text)

`telegram` is a seam-channel plugin (VIVY-CHANNEL-PACK.md §9 / §14.3),
started and consumed by the kernel’s ChannelHost; it is not a model tool. It is the
**shape template** for the later feishu / qq / discord / dingtalk adapters—copy
the directory structure and the Start/Stop/Send skeleton instead of inventing
another one.

## Initial slice scope (what it does / does not do)

**Does:**

- Private chats (`chat.type == "private"`) **receive/send plain text**
- Outbound long-poll (`getUpdates`), with no webhook and no listening port
- Platform-side filtering with `AllowedUpdates: ["message"]`: edited messages,
  channel posts, and so on never reach the ear
- Ignore the bot’s own messages (to prevent echo loops), forwarded messages, and non-text messages

**Does not do (later slice):** group triggers, media, command menus, the full
MarkdownV2 / HTML suites, voice, webhooks, reply threads, or topics.

## Permission and policy boundaries

| Item | Owner |
|---|---|
| `allow_from` sender allowlist | Enforced by **Host (kernel)**; the plugin has no separate allowlist |
| `token_env` secret resolution | Host pins `Secret` to the `token_env` declared in the envelope; the plugin may resolve only that name |
| channel settings | Strictly decoded by the plugin (unknown fields fail closed); the kernel does not know the keys in `settings` |
| Listening port | None. long-poll is an outbound-only connection |

Private-chat senders are written as `telegram:<numeric user ID>` and must
**exactly match** an `allow_from` entry in the configuration (no wildcards;
`"*"` is not allowed).

The two occurrences of `token_env` are **intentional**: the
`channels.telegram.token_env` in the configuration envelope is the declaration
Host audits, while `settings.token_env` is the name the plugin actually resolves
through `Secret`. If the two differ, `Secret` fails closed and Start refuses to
launch.

## Configuration example (fixed envelope, opaque settings)

```yaml
channels:
  telegram:
    enabled: true
    allow_from: ["telegram:123456"]
    token_env: TELEGRAM_BOT_TOKEN
    settings:                    # Opaque to the kernel; decoded by this plugin
      token_env: TELEGRAM_BOT_TOKEN   # Must match the envelope token_env
      # base_url: "http://127.0.0.1:8081"   # Optional: local Bot API sidecar
      # proxy is intentionally unsupported: all traffic uses the Host client
```

Secrets travel only through environment variables:
`export TELEGRAM_BOT_TOKEN=123456:AA...` (D-010; the token never appears in
configuration, logs, or event payloads).

Unknown `settings` fields are **rejected**—a key the current lib does not know
requires a version bump and a new pack; it cannot be forced in through configuration.

## Packaging and verification

```text
vivy-sdk verify plugins/telegram          # Static rules + linkability
vivy-sdk pack --with telegram --out dist/ # Produce candidate EXE (linked with telego)
vivy-sdk inspect-artifact dist/<gen>/     # recipes.plugins contains telegram
```

The standalone go.mod (`example.com/vivy/plugins/telegram`) is mandatory:
the default `just ci` and the species’ `go build ./cmd/vivy` import graphs do
not reach `github.com/mymmrac/telego`—only the generation produced by pack has
the ear in its body.

## Module dependencies

This module may import only `agent-vivy/sdk/module`, its focused `sdk/port` + the standard library +
`github.com/mymmrac/telego`. Imports of `agent-vivy/internal/...`, eino,
picoclaw, or `.workspace` are forbidden; `net.Listen` is forbidden; blank
`init()` imports are forbidden.
