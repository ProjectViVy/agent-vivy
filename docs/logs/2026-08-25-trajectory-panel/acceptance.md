# Acceptance

A later agent or human working in this repo should be able to verify by eye at
`http://127.0.0.1:3015/dashboard` (split pair, `just dev`):

1. The Dashboard tabs are **Overview / Token / Trajectory**—there is no Audit
   Tab, Audit card, or top-level Audit entry; `ui/src/components/audit/` does not exist.
2. The Trajectory Tab shows three blocks, structurally matching the DeepSeek
   Harness trajectory view:
   - Toolbar: an “Actual Duration” toggle, Collapse All Runs, Collapse All Calls,
     and a “Search” box on the right;
   - Three-lane timeline (Input/Model/Tools, 44px label bar + 50px plot area):
     assistant spans use TTFT/Decoding gradient segments; press-drag on the chart
     selects a range (the ledger highlights only records inside the range and
     dims those outside it); hovering any span shows a KIND · start/end time ·
     Total · TTFT · Decoding tooltip; Escape clears the selection.
   - Ledger: record rows grouped by run (SYSTEM/USER/CONTEXT/ASSISTANT/TOOL/
     SUBTOOL/COMPACTED type badges, with a `Request #N` jump button on ASSISTANT
     rows), and error rows in red with `text → result`; after collapsing/expanding
     all runs and calls, the ledger becomes a `… Collapsed · N records` summary
     row; search terms filter the ledger and dim unmatched spans on the timeline.
   - Click a row or `Request #N` to open the right-side detail: request-level
     summary/usage/timing (Status/Provider/Model/tool calls/retries, Token
     details, TTFT/Decoding/total duration), and record-level input/output/thinking.
3. After switching languages (Settings → Language), Trajectory-panel copy switches
   between Chinese and English with no bare i18n keys.
4. `just ci` is the gate for follow-up changes; run `pnpm test` first when adding
   trajectory data/projection pure-function changes (`trajectory-utils.test.ts`
   covers projection, collapsing, formatting, and data invariants).

## Known remaining items (explicitly not done here)

- The Trajectory panel uses demo data only (static constants outside `vivy.demo`);
  connecting real runtime trajectories requires kernel log/replay RPCs and a
  separate task (see `docs/TODO.md` §0.1 UI-TRAJ).
- The right-side detail panel is fixed at 320px and does not support drag or
  keyboard resizing (the DSH original does); add it later if needed.

## Leftover findings

- A fresh checkout’s `just ci` must run `pnpm build` first to generate `ui/dist`
  (the Go-side `ui/embed.go` `go:embed all:dist` target); see
  `docs/TODO.md` §0.1 UI-CI-BOOTSTRAP.
