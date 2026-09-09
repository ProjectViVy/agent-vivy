# 2026-08-30 — Super Channel contract (C0)

## Goal and background

Turn `VIVY-CHANNEL-PACK.md` from the 2026-08-25 factory `channels/` proposal into the
adopted Super Channel contract. The decision came from the same day's discussion:

- ChannelHost + the normalized envelope + the optional-capability matrix are the world
  entry, not a collection of five bots.
- telegram / discord / Feishu / DingTalk / QQ in this batch are **all plugins**
  (`plugins/<name>/`, `seam: channel`).
- Do not create a new `channels/` directory or `RegisterChannels()`; reuse ADR-015's
  `Register()` overlay.
- A2A / NeuroLink come later with the same envelope and Host; Face / ACP remain
  independent.
- Empty `allow_from` is fail-closed. Each channel plugin has an independent `go.mod`.

## Changes

- `docs/architecture/VIVY-CHANNEL-PACK.md` — rewrite the source of truth; status is now
  adopted direction.
- Cross-references: `VIVY-ASSEMBLY.md`, `VIVY-PLUGIN-SPEC.md`,
  `SELF-EVOLVING-GATEWAY.md`, `VIVY-GATEWAY-AND-STUDIO.md`, `VIVY-FACE-PACK.md`,
  `ACP-REMOTE-CONTROL-PROPOSAL.md`, `docs/research/README.md`,
  `docs/research/OPEN-ITEMS.md`.
- `docs/TODO.md` §0.1: `CH-0` DONE; annotate `CH-A`/`CH-B`/`CH-C`/`UI-CHANNELS-BE`
  under the new contract; record one item in §10.

## Explicitly not done

- Do not change `sdk/plugin`, `internal/`, `ui/`, or event schemas.
- Do not implement the five channel plugins, Host, A2A, or NeuroLink.
- Do not change the Settings page (`UI-CHANNELS-BE` remains open; the contract requires
  correcting the allow_from copy and removing email / neuro-link add options when the
  backend is connected).
- Do not adopt Face Pack (`FACE-0` remains OPEN).
