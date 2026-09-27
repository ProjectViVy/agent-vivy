# Verification — chat transcript rows (Phase 1)

Environment: split dev pair already running (`vivy` backend `127.0.0.1:8787`,
Vite dev UI `127.0.0.1:3015`), Windows.

## Unit / static

| Command | Result |
| --- | --- |
| `cd ui; pnpm typecheck` | OK (`tsc --noEmit`, exit 0) |
| `cd ui; pnpm test` | OK — 43 files / 363 tests (25 new: 10 `run-rows`, 8 `tool-presentation`, 4 `ToolRow`, 3 `ReasoningRow`) |
| `cd sdk/ui; pnpm test` | OK — 27 tests (fixture updated for the new face members) |
| `just ui-ci` (repo root) | OK — frozen-lockfile install, typecheck, tests, `vite build`, i18n completeness and cross-face checks |

Covered by the new unit tests: fold ordering (reasoning → text → tool → text),
reasoning-lane restart after text, streaming/running flags, tool identity and
timing, envelope stripping, `error` / `awaiting-approval` / `stopped` derivation
(a run that ends with an unfinished call), compaction notices, the three
transcript-merge paths (event-sourced run, fallback run, live run not yet
projected), variant classification and summaries, exit-code/signal parsing,
diff totals (including "unparsable diff reports no totals"), head/tail folding,
collapsed-by-default markup, and error-replaces-summary.

## Live UI (`http://127.0.0.1:3015`, real console session)

Probe: `ui/.dev-workdir/rows-smoke.mjs` (gitignored) driving Chrome against the
dev server, selecting the existing session `你好` (its latest run's events are
loaded by `selectSession`; two older runs are prefetched).

```text
toolRows=14  names={"tool_search:ok":9,"echo_info:ok":1,"list_dir:ok":2,"task_list:ok":1,"glob:ok":1}
reasoningRows=10  runningReasoning=0
assistantBubbles=13  fallbackCards=2   (two runs older than the prefetch window)
firstToolText="工具调用tool_search · select:echo_info"
```

- Expanding the first tool row produced exactly one `[data-tool-body]` with
  `输入{"query":"select:echo_info"}` / `输出{"matches":["echo_info"]}`.
- Expanding the first reasoning row produced `[data-reasoning-body]` with the
  model's actual reasoning text (English), proving reasoning now survives the
  end of a run.
- Screenshots: `ui/.dev-workdir/rows-collapsed.png` (collapsed rows: 思考过程 ·
  summary, 工具调用 · name · summary, then the answer bubble) and
  `ui/.dev-workdir/rows.png` (expanded reasoning + expanded tool body).
- Console errors: only a pre-existing unrelated `404`.

Not exercised live: the diff card (this session has no file-mutation result) and
the shell exit-code pill. Both are covered by unit tests
(`tool-presentation.test.ts`, `ToolRow.test.tsx`) and reuse the shipped
`DiffView`.

## Notes

- The reasoning rows prove that `model.reasoning_delta` events do reach the
  Journal for the current DeepSeek path, which contradicts the still-open
  `DEEPSEEK-REASONING-CONTENT` row in `docs/TODO.md` §0.1 (recorded 2026-09-16).
  That row belongs to the provider track; it was not edited here.
- `just ci` still stops at `sdk/internal/conformance` for the reason already
  diagnosed in `docs/logs/2026-09-19-vivy-ui-markdown-empty-bubbles/verification.md`
  (the checked-in `internal` digest predates uncommitted `internal/workflow/`
  work in another lane). The UI half of the gate (`just ui-ci`,
  `headless-compile`, `plugin-ci`) is green.
