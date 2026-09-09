# CH-C7c — `plugins/discord` Text (No Voice)

## 1. Identity

| | |
|---|---|
| ID | CH-C7c |
| Stage | G International Completion / Phase Close |
| Person-days | 2 |
| Milestone | M-CH4 |
| Dependency | CH-C3; C4 should be merged |
| Branch | `feat/channel-c7c` |
| Contract | §14.3 discord; voice.go / pion / TTS prohibited |

## 2. Goal

Independent `plugins/discord`. DM / text-channel text + Message Content Intent. The default EXE has no discordgo. This is the phase-closing slice.

## 3. Current State

**Note:** Before formally writing the Discord adapter, first read picoclaw—it is the **most complete** Go sample among the channel implementations. Read-only rewrite; imports are prohibited. Reference: `.workspace/picoclaw/pkg/channels/discord` or `C:\Users\Administrator\Desktop\morediva\.workspace\picoclaw\pkg\channels\discord`. picoclaw contains `voice.go` / pion—**do not port those files**. See `00-standing-orders.md`.

## 4. Target Structure

Copy C4. grant `channel.poll` + `secret.read`. token_env. Do not implement the full slash-command feature set.

## 5. File Inventory

**Create** `plugins/discord/**`. Prohibit pion/webrtc, voice.go, TTS probing, and adding discordgo to a species go.mod.

## 6. Steps

1. Rewrite the Gateway WS text path. Explicitly do not copy voice.
2. Have verify scan `pion` imports as a failure (recommended addition).
3. pack --with discord.
4. `just ci` default path.
5. Log to `docs/logs/YYYY-MM-DD-channel-c7c/`. Add the TODO M-CH4 phase-close note.

## 7. Acceptance

- Candidate DM/text-channel text works.
- The dependency graph has no pion.
- Empty allow_from is rejected.

## 8. Prohibitions

- `voice.go`, WebRTC, the full slash-command feature set, or TTS.
- Public webhook.

## 9. Risks and Rollback

- discordgo may easily make voice the default example: inspect imports carefully during diff review.
- Rollback: do not name it in the recipe.

## 10. Handoff

Implementation closes this phase. For later slices, read [CH-C8.md](CH-C8.md) / [CH-C9.md](CH-C9.md) (notes, not work orders).

> **DONE 2026-08-30** — Branch `feat/channel-c7c` (based on c7b). C1–C7c all landed in this phase, closing M-CH4: default body `Register()=nil` with zero platform SDKs; five ears each use an independent module; pion is blocked by verify. Work on later slices requires the user to name one (C8) or a capability proposal (C9). Filing: `docs/logs/2026-08-30-channel-c7c/`.
