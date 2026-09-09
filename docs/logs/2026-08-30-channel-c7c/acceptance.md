# CH-C7c — acceptance (how a person can tell it is done) — this slice closed

Date: 2026-08-30.

## Product view: completing the international community ears; five ears closed this slice

Send text in a Discord server/DM → Gateway WS receives it → record it → Run → reply in the channel. No voice, no WebRTC, no TTS, and no slash-command suite—the picoclaw `voice.go` did not cross the river at all.

## Points a person can verify by hand

1. **The gate is green**: `just ci` exits with code 0.
2. **The ban is real**: write a plugin that imports pion; `vivy-sdk verify` rejects it directly ("pion/webrtc is banned in plugins (no voice in Vivy channels)")—the rule applies to every seam, and tool plugins cannot smuggle it in either. No pion module appears in the candidate EXE.
3. **Packaging is enough**: `pack --with discord` → the candidate links upstream discordgo v0.29.0 (**not** the picoclaw fork); the settings page automatically shows a Discord card.
4. **A bad identity fails on first connection**: a wrong token / disabled Message Content Intent → the failure reason is visible when Start fails (inspect note); it does not leave a falsely online ear behind.
5. **Replies do not depend on the ear being online**: outbound uses an independent REST session—when Run finishes during an ear redial, the reply is still sent (ear/API separation).
6. **Token discipline**: discordgo's LogDebug prints an Identify packet containing the token—the plugin explicitly pins LogLevel, and neither the default nor explicit paths leak it.

## Acceptance items explicitly outside this slice (do not pursue them here)

- Voice/TTS/embeds/reactions/edits/slash → prohibited or deferred to a later slice.
- Enabling Message Content Intent → an operational prerequisite (developer dashboard), not a code issue.
- Real Discord smoke testing → manual pre-release acceptance (rollback = omit discord from the recipe).

## Slice-closing note (M-CH4)

All eleven C1–C7c slices landed: the ledger recognizes the world entry, the SDK has a seam, Host is in the kernel, five real ears are independent modules, and the settings page sees only compiled-in bodies. The default `just ci` path has zero platform SDKs. Later slices (C8 subprocess / C9 A2A/NeuroLink) are memo lines in `docs/TODO.md`, not a start order.

## Rollback

Omit discord from the recipe and it disappears; revert this branch and this ear is gone.
