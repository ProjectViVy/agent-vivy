# 2026-09-01 — VC-3 slice 2: lsp_definition / lsp_references / lsp_symbols

## What changed

The second batch of VC-3 `lsp_*` tools (all effect read; no kernel changes required):

- `plugins/lsp/protocol.go` — LSP Location/definitionParams/referenceParams
  types; two response shapes, documentSymbol (hierarchical) and symbolInformation (flat);
  `parseSymbols` heuristically supports both based on whether a `location` key is present.
- `plugins/lsp/tools.go` (new) — shared `syncOpen`/`syncFile` prelude (validate the
  workspace-relative path + 1-based line/column → 0-based LSP position, read from disk via
  env.OpenRead → synchronize with didOpen/didChange, and reuse the manager connection), with
  three tools:
  - `lsp_definition` — textDocument/definition (Location | Location[] | null)
  - `lsp_references` — textDocument/references (`include_declaration` optional,
    default false)
  - `lsp_symbols` — textDocument/documentSymbol (hierarchical indentation rendering,
    with the SymbolKind 1..26 name table; flat shapes render as `kind name path:line:col`)
- `plugins/lsp/vivy-plugin.json` — tools increased to 4.
- `plugins/lsp/plugin_test.go` — the fake language server adds responses for three methods;
  end-to-end coverage includes single-hop definition, multi-line references, indented symbols,
  flat-shape parsing, null/single-Location rendering, and rejection of line 0.

Not done: lsp_rename/replace_symbol (next batch, effect write through approval); diagnostic
backfill; file-version history (awaiting O1..O6); UI.

## Crush alignment

Crush is FSL-1.1-MIT: definition jumps, reference lookup, and symbol lists align with
Crush's existing LSP capabilities, with zero code copied and no surface added that Crush lacks.

## Verification command

See `verification.md`.

## Results

- Plugin-module gofmt/vet/`go test -race` all passed;
- Five-step path: `vivy-sdk verify plugins/lsp` passed; `pack --with lsp` produced
  gen_4bb127429f3049aa, and generation.json tools contains all four lsp_* tools.
