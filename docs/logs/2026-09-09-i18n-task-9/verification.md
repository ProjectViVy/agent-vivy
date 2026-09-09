# Verification — 2026-09-09

Commands run from the i18n worktree root unless marked `ui/`. Direct Node
entrypoints are the installed executables used by the package's test,
typecheck, and build scripts; no dependency installation was needed.

| Command | Result |
|---|---|
| `ui/`: `node node_modules/vitest/vitest.mjs run` (baseline) | Exit 0; 29 files, 256/256 tests |
| `ui/`: `node node_modules/vitest/vitest.mjs run src/lib/reveal-engine.test.ts src/i18n/completeness.test.ts` (RED) | Exit 1; 17 passed, 2 expected failures: English diagnostic still Chinese and scanner still allowed it |
| Same focused command after the fix (GREEN) | Exit 0; 19/19 tests |
| `ui/`: `node node_modules/vitest/vitest.mjs run` (full final suite) | Exit 0; 30 files, 259/259 tests |
| `ui/`: `node node_modules/typescript/bin/tsc --noEmit` | Exit 0 |
| `ui/`: `node node_modules/vite/bin/vite.js build` | Exit 0; 2,274 modules; large-chunk warning (1,303.17 kB JS, 397.16 kB gzip), not suppressed |
| `node scripts/check-i18n-completeness.js` | Exit 0; en/zh each 1,388 keys and 138 placeholders; maintained runtime copy clean |
| `go test ./...` | Exit 127, once: `go: command not found` |
| `just ci` | Exit 127, once: `just: command not found` |
| `git diff --check` | Exit 0 |
| `git check-ignore .env` / `git ls-files .env` | Ignored / no tracked entry |
| `git ls-files .github/workflows` and directory existence check | No entries; directory absent |

Static Node assertions compared `justfile` with `git show 1b1fdd3:justfile`:
all six original dependencies and their recipe bodies are unchanged;
`i18n-check: ui-ci` runs the root completeness command and is reachable from
`ci`; `ui-ci` still precedes Go compilation. Assertions passed (exit 0).
This is not just/PowerShell execution or a substitute for a green product gate.

## Untranslated-string audits

```bash
rg -n '[\p{Han}]' ui/src --glob '*.{ts,tsx}' --glob '!**/i18n/zh.ts' --glob '!**/*.test.*'
rg -n '[\p{Han}]' sdk/tui --glob '*.go' --glob '!**/*_test.go' --glob '!**/i18n/catalog_zh.go'
rg -n '[\p{Han}]' README.md docs --glob '*.md'
rg -n '[\p{Cyrillic}\p{Arabic}\p{Hiragana}\p{Katakana}\p{Hangul}]' README.md docs --glob '*.md'
git grep -n -P '[\p{Han}\p{Cyrillic}\p{Arabic}\p{Hiragana}\p{Katakana}\p{Hangul}]' -- '*.md' ':!docs/**' ':!README.md' ':!.agents/**' ':!.superpowers/**'
```

- Web `rg`: exit 0, 769 lines across 63 files. Full TypeScript syntax-tree
  traversal classified every Han character: 694 comment lines, 61 trajectory
  fixture lines, 5 mock input-match data lines, 4 protocol-recognition lines,
  4 channel brand-name lines, and 1 native-language name. No unclassified
  characters or uncatalogued Han host messages remain.
- TUI `rg`: exit 1 means no matches, not a tool error. Catalog and `_test.go`
  exclusions are explicit; no Go/TUI execution is implied.
- README/docs Han audit: exit 0, 22 lines of exact localization values,
  historical selectors/search fixtures, or protocol stream examples. No
  non-English prose found. The other-script audit and tracked Markdown audit
  outside README/docs both returned exit 1 (no matches).
  A supplementary scan of all non-ASCII letters across 859 README/docs
  Markdown files found exactly those same 22 lines and no additional scripts.
- `COMMENT` excludes non-runtime source comments from runtime host-copy checks,
  not from the AGENTS English authoring policy. The historical Task 9 plan's
  English-comment instruction still applies: non-English text is preserved
  only as exact localization/fixture/protocol data, not explanatory prose.
  This classification neither grandfathers existing non-English comments nor
  certifies authoring compliance. Exact data/protocol/fixture exclusions were
  inspected, not broadened. Task 9 removed one scanner exception; this wording
  cleanup changes no runtime scanner or rejection rule.

## Limitations

No Go compiler, Go/TUI tests, pack run, live backend/browser/TUI smoke,
headless compile, plugin-module tests, or successful `just ci` execution is
claimed. There are no repository GitHub workflows to provide remote evidence.
The narrow diagnostic failure path was exercised with real catalogs and DOM
under happy-dom, not a live browser. Cross-face canonical catalog-shape
conformance remains unproven; per-face parity must not be represented as that
proof. The large-bundle warning is pre-existing and no chunking refactor was
made. Open verification/conformance work is tracked in `docs/TODO.md` §0.1.
