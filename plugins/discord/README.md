# plugins/discord — Vivy’s Discord ear (Gateway text receive/send, DM + server text channels)

`discord` is a seam-channel plugin (VIVY-CHANNEL-PACK.md §9), started and
consumed by the kernel’s ChannelHost; it is not a model tool. Its directory
structure and Start/Stop/Send skeleton follow the shape template established by
`plugins/telegram`, and its lifecycle follows the supervised redial pattern in
`plugins/dingtalk` / `plugins/feishu` / `plugins/qq`.

## **This plugin connects only to official Discord Bots**

- **Not a user account (self-bot)**: it does not log into any personal Discord
  account, emulate the client protocol, or use a user token. The only
  credential is the **Bot Token** issued in the developer portal at
  [discord.com/developers](https://discord.com/developers/applications) (the
  plugin adds the `Bot ` prefix).
- **Not a webhook service**: there are no webhook callbacks or listening ports.
  Transport is an **outbound Gateway WebSocket** (manifest `transport: "poll"`
  plus grant `channel.poll`, with WS counting as poll under contract §14.3) plus
  the official REST message endpoint (`POST /channels/{id}/messages`).
- **No voice / no pion / no TTS**: Discord voice is not supported at all
  (voice.go, pion/webrtc, WebRTC, and TTS are untouched—since CH-C7c the SDK
  validator **rejects any plugin importing `github.com/pion/*`**, and the
  `sdk/internal/testdata/bad-pion-import` fixture pins this rule).
- **No slash suite**: no application command / interaction handlers are
  registered—slash commands are independent interaction events in discordgo;
  this plugin registers only `MESSAGE_CREATE`, so interactions cannot reach it.
  Message-side command payloads (types 20/23) are also discarded by the shape
  filter.
- The SDK uses the **upstream `github.com/bwmarrin/discordgo v0.29.0` release**.
  The picoclaw reference implementation pins the same version with a fork
  replacement (`yeongaori/discordgo-fork`); this plugin **does not** include
  that replacement and the species links only the upstream module.

The code and documentation make the same declaration; see the `plugin.go` package
comment (no voice/no pion/no TTS/no slash are all stated first).

## Message Content Intent (operations prerequisite)

Since 2022, Discord has classified message content as a **privileged intent**:

- The plugin requests `IntentsGuildMessages | IntentsDirectMessages |
  IntentsMessageContent` on every IDENTIFY;
- **The bot owner must first enable "MESSAGE CONTENT INTENT" in the developer
  portal (Bot page)**, or the gateway rejects the session with close code 4014.
  The first `Open` fails immediately and Start fails closed (there is no ear
  left that appears “started” but cannot receive content);
- Without the intent, DM text is still pushed (DM exemption), but `content` is
  always empty in server text channels. The plugin discards it under the “do not
  publish empty content” rule, effectively becoming deaf.

## Initial slice scope (what it does / does not do)

**Does:**

- Receive plain text from **DMs + server text channels** (`MESSAGE_CREATE`,
  with Message Content Intent; see above)
- Send plain text: `POST /channels/{id}/messages`, sending content **as-is**—
  Discord renders markdown natively, and the plugin **neither strips nor adds**
  formatting; it does not use embeds or force markdown
- Reply addressing: ChatID is the **channel ID** (in a DM it is the DM channel
  ID and can be used directly as the destination—unlike passive-reply platforms,
  Discord lets a bot send proactively to a visible channel, so there is **no
  passive reply window** and it can reply after a restart)
- `ReplyTo` slot: an inbound type-19 reply’s `referenced_message.id` is put into
  the envelope (recorded only; no thread behavior)

**Does not do (later slice):**

- **The full voice stack** (voice.go / pion / WebRTC / TTS / speech
  transcription)—a Species-level prohibition, not “deferred”
- slash commands, buttons/components, modals, context menus, or interactions of any form
- Group trigger-word/@ filtering (all readable text in server channels passes
  through Host’s allow_from allowlist gate), embeds, media/attachments,
  reactions, typing indicators, message editing/deletion, forum posts, or thread management

## Permission and policy boundaries

| Item | Owner |
|---|---|
| `allow_from` sender allowlist | Enforced by **Host (kernel)**; the plugin has no separate allowlist |
| Secret resolution | Host’s `Secret`; Discord has only **one** bot token, with the settings-side `token_env` pinned to the envelope’s `token_env` as a pair (C3/C4 pattern); they must match or fail closed |
| channel settings | Strictly decoded by the plugin (unknown fields fail closed); the kernel does not know the keys in `settings` |
| Listening port | None. The Gateway is outbound-only WebSocket; sending is outbound-only REST |

Senders are written as `discord:<user ID>` (`author.id` from `MESSAGE_CREATE`),
and ChatID is the channel ID. It must **exactly match** an `allow_from` entry in
the configuration (no wildcards; `"*"` is not allowed).

Shape filter (local discard before publishing; a second gate beyond the allowlist):

- **Discard bot authors directly** (echo guard): the gateway also pushes the
  bot’s own messages (and messages from other bots) as `MESSAGE_CREATE`; keeping
  them would create an echo loop
- Discard command-type messages (`CHAT_INPUT_COMMAND` 20 /
  `CONTEXT_MENU_COMMAND` 23) and system messages (join notices, pin notices, and so on)
- Discard empty content (images/attachments/embeds/stickers only)—media receive is a later slice
- Discard messages missing a sender / message ID / channel ID (Host dispatch would discard them anyway)

**No deduplication**: the Discord Gateway **does not redeliver** events it has
already dispatched (unlike QQ). RESUME only backfills events after the last
**received** sequence number, and discordgo advances the sequence number as
soon as it receives a frame (before dispatch), so frames already processed by
this process are not replayed. After a process restart it is a new session
(fresh identify), so there is nothing to replay. This plugin therefore has no
msg_id deduplication barrier like QQ.

## Lifecycle: do not use discordgo’s built-in reconnect (source conclusion)

Three facts from reading the discordgo v0.29.0 source (`wsapi.go`) determine this
plugin’s lifecycle shape:

1. **`Open()` completes the full handshake synchronously**: fetch the gateway
   address via REST → dial WebSocket → HELLO → IDENTIFY → read back the READY
   frame. A nil return from `Open` means the gateway accepted the session—token
   rejection and a disabled intent (4014) are both exposed here, so Start is
   naturally fail-closed (a first attempt that does not complete the handshake
   does not report started).
2. **The built-in reconnect cannot be disabled or stopped**: when
   `ShouldReconnectOnError=true` (the default), `reconnect()` is an **infinite
   loop** (backoff capped at 600s), and `Close()` **does not** stop it (v0.29 has
   no internal flag flip). The ear revives after Stop. Therefore the plugin
   **creates a new session for every supervision attempt** and pins
   `ShouldReconnectOnError` to `false` (also
   `ShouldReconnectVoiceOnSessionError=false`, sealing off the voice revival
   path), then redials in its own ctx-aware supervision loop—the same tradeoff
   as dingtalk/feishu disabling SDK automatic reconnect.
3. **Death signal**: every SDK disconnect path (read-loop error, heartbeat
   failure, Gateway op7) calls `Close()` first and then the no-op `reconnect()`,
   while `Close` dispatches a synthetic **`DISCONNECT` event**. With SDK
   reconnect disabled, this event is the death signal awaited by the supervision
   loop. On natural death, discordgo’s own goroutine closes the socket; the
   supervision loop only replaces the session. `Stop` only latches + cancels and
   never calls `Close` itself (while dialing, `Open` holds the session mutex and
   a Stop-side Close would block—the supervision loop cleans up, allowing Stop
   to return within a bound).

**Cost (an intentional, documented deviation)**: the Gateway session ID and
sequence number are unexported fields in v0.29, so a new session cannot inherit
the old session’s RESUME state. Every redial performs a fresh IDENTIFY, and
**messages sent by others during the redial gap are lost** (the gap is bounded
by the redial delay). Stoppability > resumption; this is the same tradeoff as
qq/feishu. The send path is unaffected (the REST send client is created once at
Start and never calls Open, separate from the ear—the qq api/ear split mode).

## Configuration example (fixed envelope, opaque settings)

```yaml
channels:
  discord:
    enabled: true
    allow_from: ["discord:123456789012345678"]
    token_env: DISCORD_BOT_TOKEN       # Envelope-side declaration (used by Host to pin Secret)
    settings:                          # Opaque to the kernel; decoded by this plugin
      token_env: DISCORD_BOT_TOKEN     # Same name on the plugin side (C3/C4 pin: must match)
```

Secrets travel only through environment variables (D-010; values never appear
in configuration, logs, or event payloads):

```text
export DISCORD_BOT_TOKEN=MTxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

Unknown `settings` fields are **rejected**—a key the current lib does not know
requires a version bump and a new pack; it cannot be forced in through configuration.

## Message length

manifest `max_message_runes: 2000`: the content limit for a single Discord
message is 2000 characters, matching the picoclaw reference implementation’s
`WithMaxMessageLength(2000)`.

## Packaging and verification

```text
vivy-sdk verify plugins/discord           # Static rules + linkability
vivy-sdk pack --with discord --out dist/  # Produce candidate EXE (linked with discordgo)
vivy-sdk inspect-artifact dist/<gen>/     # recipes.plugins contains discord
```

The standalone go.mod (`example.com/vivy/plugins/discord`) is mandatory: the
default `just ci` and the species’ `go build ./cmd/vivy` import graphs do not
reach `github.com/bwmarrin/discordgo`—only the generation produced by pack has
the ear in its body.

## Module dependencies

This module may import only `agent-vivy/sdk/module`, its focused `sdk/port` + the standard library +
`github.com/bwmarrin/discordgo` (its go.mod transitive dependencies
gorilla/websocket and x/crypto do not appear directly in business-code imports).
Imports of `agent-vivy/internal/...`, eino, **the pion suite**, picoclaw, or
`.workspace` are forbidden; `net.Listen` is forbidden; blank `init()` imports
are forbidden.

## SDK version and deviations

The pinned version is the **upstream** `github.com/bwmarrin/discordgo v0.29.0`—
the same version as picoclaw but **without its fork replacement**. Intentional
deviations (all recorded above and in the package comment):

1. **Do not use its built-in reconnect loop** (`ShouldReconnectOnError=false`):
   at the source level it is infinite and `Close` cannot stop it, so the ear
   revives after stopping;
2. **No RESUME continuation**: session ID / sequence are unexported, so every
   redial performs a fresh IDENTIFY and events during the redial gap are lost
   (within a bound);
3. **No slash/interaction support**: register only `MESSAGE_CREATE`; voice/
   media/embed/reaction/typing/edit are all outside this slice.
