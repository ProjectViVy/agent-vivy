# Face Pack Proposal Summary

Date: 2026-08-29
Status: complete (proposal only; not adopted)

## Outcome

Recorded the 2026-08-29 discussion as a product-contract proposal:
`docs/architecture/VIVY-FACE-PACK.md`. Faces (`web` / `tui` / `headless`)
become a first-class packed organ, parallel to the existing channel
proposal. No kernel, SDK, or UI code changed.

## Delivered

- `docs/architecture/VIVY-FACE-PACK.md` — F0 contract: one face per
  generation, factory `faces/`, user `seam: face`, coding species without
  web, Android as a downstream kernel consumer.
- Cross-links from `VIVY-ASSEMBLY.md`, `VIVY-PLUGIN-SPEC.md`,
  `SELF-EVOLVING-GATEWAY.md`, `VIVY-CHANNEL-PACK.md`,
  `VIVY-GATEWAY-AND-STUDIO.md`, `ACP-REMOTE-CONTROL-PROPOSAL.md`,
  `docs/research/README.md`.
- Open board: `docs/TODO.md` §0.1 `FACE-0`.

## Explicitly not done

- Adopting the proposal (that is `FACE-0`).
- `sdk/plugin` seam expansion, `faces/` packages, Bubble Tea, `vivy run`.
- Changing the default `vivy.exe` from the web gateway.
- Android / APK / gomobile.
