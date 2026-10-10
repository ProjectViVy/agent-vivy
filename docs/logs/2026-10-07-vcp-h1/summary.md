# H1 — Bundle consolidation (taxonomy landing) + full `just ci`

## What shipped

The placement taxonomy (SPEC §2.1 + vivy-plugin skill "Placement rule") landed
physically. Applying the universality rule ("universal = selected by every
Generation") dissolved the original H1 tool-migration list: every non-protected
`internal/tools` entry is universal and stays internal. What moved:

| Move | From | To |
|---|---|---|
| `vivy/local-llm` | `plugins/coding/local-llm` | `plugins/infra/llm` |
| `vivy/lsp` | `plugins/lsp` | `plugins/coding/lsp` |

Module IDs are source-independent and unchanged (`vivy/local-llm`, `vivy/lsp`).
All reference sites updated: vivy-module.yaml + Descriptor `source.ref`, both
plugin `go.mod` module lines (lsp's `replace` corrected to `../../..`), root
go.mod require+replace, `repoSourceDirs` (lsp's diagnostics/
languageServerStatuses flags moved with it), generate-default extern,
conformance table + results JSON, frontend_v1_test paths. Digests re-pinned
at the self-referential fixed point:
`vivy/lsp` = `6220b77f…818067b`, `vivy/local-llm` = `9a8e049a…e13a90d`.

`vivy/lsp` was activated into `recipes/vivy-code.vivy.yml` (it was resolvable
but selected by NO recipe — dead inventory). Owner-approved capability change:
the coding generation now gains the LSP tool-world with fs.read/fs.write and
a command-allowlisted proc.spawn (gopls, typescript-language-server,
pyright-langserver, rust-analyzer).

## Incidental fixes this gate surfaced

- `justfile` `plugin-ci` only scanned direct children of `plugins/`/`faces/`;
  nested modules (`plugins/coding/*`, `plugins/infra/*`) were invisible to CI.
  Now recurses `Get-ChildItem -Recurse -Filter go.mod` — all 18 modules run.
- `scripts/i18n-cross-face-contract.json` gained 21 Web + 21 TUI face-specific
  key classifications accumulated by B2/D1/F3/G stories (steering lanes,
  compact UI, thinking settings, TUI dialogs/search/editor/model-scope).
- `generate-default` gofmt fix.
- Environment note (not a repo change): `ensure-laputa` rejects the Devin
  git-proxy origin (`url.insteadOf` rewrite); `just ci` here ran under
  `GIT_CONFIG_GLOBAL=/dev/null`. Linux-VM-only concern; Windows unaffected.

## Deviation vs original H1 plan

Original plan moved ~20 universal tools to `plugins/coding/*`. Superseded by
the owner-approved taxonomy: universal tools stay `internal/`; only
coding-specialized assets migrate (recorded in the rewritten plan).
