# agent-vivy

Implementation home of the Vivy species (`vivy.exe`) and the first-party
Studio overlay. Canonical product rules: `docs/architecture/VIVY-STUDIO.md`.

## Scope Separation: Vivy vs Vivy Studio

**Default scope is VIVY (the species/kernel).** When the user mentions "Vivy" without "Studio", develop the Vivy kernel/species itself — not the Studio overlay. Only when the user explicitly says "Studio", "Vivy Studio", or "工作室" should you work on the Studio overlay/shell.

- **Vivy (default)**: Kernel, engine, UI, skills, recipes, plugins, product-contract docs. Use `just ci` for verification.
- **Vivy Studio (explicit only)**: First-party IDE shell, skin, theme, lifecycle. Use `just studio` for builds. See `.agents/skills/vivy-studio-lifecycle`.

This separation prevents accidental cross-contamination between the species runtime and its development environment.

## Development environment

**Vivy Studio is the first-party daily development IDE and the recommended
author path. It is not an exclusive execution venue.** A developer tool or
agent that has been authorized to read this workspace should use its own
native editing, testing, and automation capabilities directly in the current
workspace. Do not transfer or replay that work inside Vivy Studio merely to
satisfy a venue rule.

The same repository contracts and verification commands apply regardless of
which authorized development tool performs the work. Daily `vivy.exe` remains
a tenant product, not an IDE.

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

## DSH harness reference source

DeepSeek Harness source lives in `.workspace/deepseek-harness/` (currently:
`deepseek-harness/` working clone + `upstream/` mirror of
`https://github.com/deepseek-ai/deepseek-harness.git`). It is reference
material for engine internals — do not edit it as the implementation path.

If the checkout is missing and you need it, clone it into `.workspace/`:

```text
git clone https://github.com/deepseek-ai/deepseek-harness.git .workspace/deepseek-harness/upstream
```

`.workspace/` is gitignored (see `.gitignore`) — never commit it. The
installed `@deepseek-ai/dsh` npm package is built JS, not source; treat this
tree, not `node_modules`, as the source of truth for harness behavior.
