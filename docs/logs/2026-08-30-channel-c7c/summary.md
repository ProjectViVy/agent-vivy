# CH-C7c — `plugins/discord` text, no voice (summary) — closing slice

Date: 2026-08-30. Branch `feat/channel-c7c` (cut from `feat/channel-c7b` d5f4506; the sequential slice reused the same worktree).
PLAN: `docs/plans/channel-epic/CH-C7c.md`. Contract: `VIVY-CHANNEL-PACK.md` §14.1 (discord row). Compared with picoclaw discord (**`voice.go` not ported**).

## What changed

The fourth international ear, the M-CH4 closing slice: pure text for Discord Gateway WS DM + text channels.

1. **Independent module** `plugins/discord` (**upstream** discordgo v0.29.0—picoclaw uses a fork replacement, we do not use the fork, and the README notes this).
2. **Session interface isolation**: discordgo has no clean endpoint injection (`Session.gateway` is unexported and the endpoint is process-global) → a narrow `session` interface (Open/Close/ChannelMessageSend) plus a factory seam; tests are all fake/loopback. Settings only has `token_env` (single credential, with envelope-pinned semantics matching telegram).
3. **Lifecycle (verified against discordgo v0.29 source)**: `Open()` synchronously completes READY (bad token / 4014 intent rejection fails on first connection); `reconnect()` ignores Close and loops forever → `ShouldReconnectOnError=false` + a fresh session for every redial; death signal = a synthesized DISCONNECT event (the double trigger is real → `sync.OnceFunc`); **ear/API separation**—a dedicated session that is never opened uses pure REST `ChannelMessageSendComplex` (replies remain available while the ear redials). Stop is latch+cancel, bounded wait, and never Close from the Stop path (`Open` holds the lock through the handshake, so Close from Stop can deadlock—same shape as qq).
4. **Inbound**: DM + guild text channels; bot self-echo / empty content / system and interaction types are dropped; `ReplyTo` is captured inbound (ignored outbound in this slice); `Sender = "discord:<id>"`. No deduplication fence (the Discord gateway does not redeliver already-dispatched events—verified against source, recorded in the README). A named function type was used as the handler (discordgo's `handlerForInterface` type switch does not recognize named function types—a real bug caught by tests, now commented).
5. **Ban pion in SDK verify (CH-C7c §6.2 requirement)**: `bannedImportPrefixes` adds a **full-prefix** ban on `github.com/pion/`, effective across every seam (tool plugins cannot smuggle it in either); the `bad-pion-import` fixture was rejected in a real test. Zero porting of `voice.go` / webrtc / TTS / slash suite (grep all zero; `ShouldReconnectVoiceOnSessionError=false` is pinned by a test).
6. **Secrets**: explicitly pin `LogLevel` to `LogError` (discordgo prints Identify packets containing the token at LogDebug—do not rely on the library default; the review note is implemented). `Message Content Intent` is a privileged intent, and the README states the enablement prerequisite.

## Explicitly not done (non-goals)

- voice / WebRTC / TTS / typing indicators / reactions / edits / embeds / media / forum and thread management / slash commands and interaction handling—the contract prohibits them or defers them to a later slice.
- RESUME continuation (session ID/seq are not exported in v0.29) → every redial sends IDENTIFY again, with bounded event loss during the redial gap (prioritizing deadlock avoidance over zero loss; recorded as an intentional deviation).
- Group trigger policy (all readable text goes through the Host `allow_from` gate).
- Real Discord smoke testing was not done (no credentials; enabling the Intent requires the developer dashboard; rollback = omit discord from the recipe).

## Gates and evidence

`just ci` exit 0 (24 Go packages + 172 UI tests); `vivy-sdk verify plugins/discord` ok; `bad-pion-import` rejected; `pack --with discord` candidate EXE links discordgo v0.29.0 and has **0 pion**; the species `go.mod`/`go.sum` are byte-identical; `go list -deps ./cmd/vivy` has zero discordgo; plugin tests + `-race` are green.

## Structural gap (registered in §0.1)

`just ci` does not compile/test `plugins/*` (the fmt-check's rg glob scans only cmd internal sdk ui; `go test ./...` does not cross independent module boundaries)—all five ears' gofmt/vet/test rely on manual execution inside each slice. There are now five plugins; a follow-up `plugin-ci` recipe should run per module (recorded as CH-C7c-N1).
