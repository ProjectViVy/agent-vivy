# plugins/qq — Vivy’s QQ ear (official QQ Open Platform bot, one-on-one text, persistent WS)

`qq` is a seam-channel plugin (VIVY-CHANNEL-PACK.md §9), started and consumed
by the kernel’s ChannelHost; it is not a model tool. Its directory structure and
Start/Stop/Send skeleton follow the shape template established by
`plugins/telegram`, and its lifecycle follows the supervised redial pattern in
`plugins/dingtalk` / `plugins/feishu`.

## **This plugin connects only to official QQ Open Platform bots**

- **Not a personal account**: it does not log into any personal QQ account,
  emulate the client protocol, or require QR codes, passwords, or device
  information. The only credentials are the bot’s **AppID + AppSecret** issued
  in the Open Platform console at [q.qq.com](https://q.qq.com).
- **Not OneBot**: it does not implement or support the OneBot 11/12 protocols,
  connect to any OneBot implementation (go-cqhttp, Lagrange, LLOneBot, and so
  on), or provide a forward/reverse WS server/client protocol.
- **Not NapCat**: it does not depend on, start, or connect to NapCat or any
  third-party keep-alive framework, and **does not start a second process**—the
  ear is the plugin itself, running in the Vivy process.
- Transport is the **official event gateway’s persistent outbound WebSocket**
  plus the **official v2 message API** (`/v2/users/{openid}/messages`), matching
  the official documentation.

The code and documentation make the same declaration; see the `plugin.go` package
comment (THIS IS NOT A PERSONAL-ACCOUNT BOT).

## Initial slice scope (what it does / does not do)

**Does:**

- One-on-one chats (`C2C_MESSAGE_CREATE`) **receive/send plain text**
- Transport: **outbound WebSocket** (the official event gateway, manifest
  `transport: "poll"`
  + grant `channel.poll`; WS counts as poll, contract §14.3), with no webhook
  and no listening port
- Replies use the official passive-reply API: `POST /v2/users/{openid}/messages`,
  with `msg_type=0` (text), `msg_id` (passive window), and `msg_seq` (reply
  sequence 1,2,3… within the same window); the SDK manages access_token from
  AppID/AppSecret, and the plugin never touches the token
- **Deduplicate repeated deliveries**: the official documentation explicitly
  warns that the same `msg_id` may be pushed more than once; the plugin keeps a
  bounded in-memory deduplication set by msg_id (5-minute TTL, hard limit of
  10,000 entries), so duplicate events do not start a second run
- On disconnect, **resume the gateway session** (session ID from READY, seq from
  the sequence number of the last dispatched event) to avoid consuming events
  twice after reconnecting

**Does not do (later slice):**

- **Group chats (`GROUP_AT_MESSAGE_CREATE`) are not supported yet**; see the
  next section for why
- The full channel/Guild feature set, rich media, markdown/ark cards, voice,
  buttons, or webhook mode

## Why this slice has no group chats

The group address field in the official group-mention event is `group_openid`,
but the botgo v0.2.1 event struct pinned by this plugin (`dto.Message`) only
decodes `group_id`. The real v2 group payload has no `group_id` at all, so the
SDK always decodes it as empty (the picoclaw reference implementation has the
same issue). Reliably receiving group text and replying cleanly would require
bypassing the SDK and parsing raw WS frames, which does not fit this slice’s
rule—only text that the official API can receive reliably is included. Group
chats therefore remain explicitly out of scope until botgo adds the field or a
new version is evaluated.

(Note: registering the C2C handler also enables the intent bit shared by group
and C2C events, so the gateway may still push group event frames; the SDK
dispatch layer has no group handler registered, so those frames are silently
dropped.)

## Permission and policy boundaries

| Item | Owner |
|---|---|
| `allow_from` sender allowlist | Enforced by **Host (kernel)**; the plugin has no separate allowlist |
| Secret resolution | Host’s `Secret`; this plugin needs **two** credentials (AppID + AppSecret), which do not fit in the envelope’s single `token_env`, so their names are declared by the top-level `app_id_env` / `app_secret_env` keys in settings (CH-C6/D2 pattern), with Host approval |
| channel settings | Strictly decoded by the plugin (unknown fields fail closed); the kernel does not know the keys in `settings` |
| Listening port | None. The persistent connection is outbound-only; botgo’s built-in session manager is **not used** (see the next section), and the plugin’s own ctx-aware supervision loop takes its place (never revived after Stop, with no goroutine leaks) |

Senders are written as `qq:user_<openid>` (`author.id` in a C2C event, meaning
the user’s openid from the application’s perspective and equal to
`author.user_openid`), and ChatID is that openid itself. It must **exactly match**
an `allow_from` entry in the configuration (no wildcards; `"*"` is not allowed).
Note that openids are **isolated per application**: changing the bot’s AppID
changes every openid.

## Passive reply window (why Send may “fail closed”)

The QQ Open Platform has **no ordinary API for proactively sending a message
by conversation** for this type of bot—every reply must carry the `msg_id` of
the incoming message it answers (the passive window lasts about 60 minutes, with
at most 4 replies per msg_id), and multiple replies in the same window are
deduplicated using increasing `msg_seq` values. Therefore:

- The plugin records the latest incoming `msg_id` and reply sequence **in
  memory** per conversation (the latest value wins); it never enters kernel
  configuration, the configuration envelope, or logs (D-010);
- For a conversation that has never sent a message (and for every conversation
  after a process restart), `Send` **fails closed**—the other party must send
  the bot a message before Vivy can reply.

## Lifecycle: do not use botgo’s session manager

botgo’s built-in local session manager is a **non-stoppable** background loop:
`Start` blocks forever on its internal reconnect queue, does not observe ctx, and
has no stop mechanism. Even when the bot is banned, it merely recovers its own
panic inside the inner loop and **retries silently forever**, wasting the
platform’s connection quota. This plugin instead uses botgo’s exported
**protocol-layer ws client** (`websocket.ClientImpl`), driven by the plugin’s
own supervision loop—one new client per attempt: dial → identify/resume → wait
for READY (a first attempt that does not complete the handshake does not report
started; fail closed) → wait while alive → redial after disconnect according to
resume state. When the bot is banned or delisted (cannot-identify close code),
it **stops redialing**—stricter and safer than botgo’s infinite retries (redials
can never succeed and only consume quota). The ear remains in the “started but
deaf” terminal state, consistent with the sibling plugins.

Likewise, **do not use** `token.StartRefreshAccessToken`: after 11 consecutive
failures in a bare goroutine it panics with no recovery, terminating the entire
process. The SDK’s token source is lazy and cached (based on expiration time),
and the WS client fetches a new token itself on an authentication-failure close
code, which is sufficient without crash risk. Start still performs one
synchronous token fetch first—fail closed when credentials are rejected.

The SDK’s default logger is also **silenced** (`botgo.SetLogger`): botgo logs
every WS frame and every HTTP request/response body at INFO by default,
including the **access token** in the identify payload and the user’s **message
content**—violating `docs/architecture/LOGGING.md` and D-010. The plugin cannot
import `internal/logging`, so only Error-level output to stderr is retained.

## sandbox switch

`settings.sandbox: true` uses the SDK’s built-in
`botgo.NewSandboxOpenAPI` (`https://sandbox.api.sgroup.qq.com`); the gateway
address is discovered through the same client and switches with it. The default
is false (production).

## Configuration example (fixed envelope, opaque settings)

```yaml
channels:
  qq:
    enabled: true
    allow_from: ["qq:user_A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3"]
    # token_env may be empty: settings declares all credential names (see below)
    settings:                    # Opaque to the kernel; decoded by this plugin
      app_id_env: QQ_APP_ID              # Required: environment variable name for AppID
      app_secret_env: QQ_APP_SECRET      # Required: environment variable name for AppSecret (must differ from the above)
      # sandbox: false                   # Optional: true = official sandbox environment
```

Secrets travel only through environment variables (D-010; values never appear
in configuration, logs, or event payloads):

```text
export QQ_APP_ID=123456789
export QQ_APP_SECRET=xxxxxxxxxxxxxxxxxxxxxxxx
```

Unknown `settings` fields are **rejected**—a key the current lib does not know
requires a version bump and a new pack; it cannot be forced in through configuration.

## Message length

manifest `max_message_runes: 2000`: v2 text-message content is limited to
7000 bytes; 2000 runes are 6000 bytes in the all-CJK case, leaving a safety margin.

## Packaging and verification

```text
vivy-sdk verify plugins/qq            # Static rules + linkability
vivy-sdk pack --with qq --out dist/   # Produce candidate EXE (linked with botgo)
vivy-sdk inspect-artifact dist/<gen>/ # recipes.plugins contains qq
```

The standalone go.mod (`example.com/vivy/plugins/qq`) is mandatory: the default
`just ci` and the species’ `go build ./cmd/vivy` import graphs do not reach
`github.com/tencent-connect/botgo`—only the generation produced by pack has the
ear in its body.

## Module dependencies

This module may import only `agent-vivy/sdk/plugin` + the standard library +
`github.com/tencent-connect/botgo` (and its go.mod transitive dependencies such
as oauth2/resty/gorilla, which do not appear directly in business-code imports).
Imports of `agent-vivy/internal/...`, eino, picoclaw, or `.workspace` are
forbidden; `net.Listen` is forbidden; blank `init()` imports are forbidden.

## SDK version

The pinned version is `github.com/tencent-connect/botgo v0.2.1`, the same as the
picoclaw reference implementation. There are three intentional deviations (all
recorded above): do not use its built-in session manager (non-stoppable, with
silent infinite retries for banned bots); do not use its background token-refresh
goroutine (a bare goroutine panics after consecutive failures with no recovery);
and do not handle group events (the DTO field name does not match the official
v2 payload).
The WS protocol itself (hello/heartbeat/identify/resume/close codes) remains
implemented by the SDK client; this plugin only supervises the lifecycle and
normalizes events.
