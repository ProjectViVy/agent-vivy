# agent-vivy

Implementation home of the Vivy species (`vivy.exe`) and the first-party
Studio overlay. Canonical product rules: `docs/architecture/VIVY-STUDIO.md`.

## Venue

ST-6 is done (2026-08-16). All further Vivy development (species, Studio,
skills, recipes, product-contract docs, tests) happens inside **Vivy
Studio**. Do not use an external IDE / agent as the main implementation
path. Daily `vivy.exe` is a tenant product, not an IDE.

The only remaining outer-loop exception is Studio itself failing to
start (`VIVY-STUDIO.md` §2.2): restore boot, write it down, do not
slip in features.

## Air gap (ST-2)

The Studio engine's workspace is this repository root. It is not
`data/` and it is not a tenant install.

Do **not** read or write production journals from a Studio session:

- `data/vivy.db`
- `data/demo/`
- `data/workspaces/`

Studio's own DSH home is `data/studio-home/` (sessions, profile, settings).
That directory is the engine's scratch, not the species Journal.

## How to verify

- Kernel / docs / UI: `just ci`
- User plugin: `vivy-sdk verify plugins/<name>` then `vivy-sdk pack --with <name>`
- Studio lifecycle (pack → eval → release → install → rollback):
  `just studio` builds `vivy-studio.exe`; the ledger lives at
  `data/studio-home/studio.db` (Studio-owned, not the species Journal).
  See `.agents/skills/vivy-studio-lifecycle`.
- Do not treat a hand-rolled `go test` as the product path when `just ci` exists
- Do not open `internal/runtime/engine.go` to "install" a plugin

Prefabricated Studio skills: `.agents/skills/vivy-plugin-five`,
`.agents/skills/vivy-kernel-ci`, `.agents/skills/vivy-studio-lifecycle`.
