# Dashboard Audit → Trajectory panel (restored DeepSeek Harness trajectory design, demo data)

Date: 2026-08-25
Status: complete

## What changed

The Dashboard (`/dashboard`) Audit feature was removed entirely and replaced by
the Trajectory panel (`TrajectoryPanel`), restoring the visible structure and
interactions of DeepSeek Harness `packages/client/ui-trajectory` as closely as
possible (referenced from the `.workspace/deepseek-harness/upstream/` working
clone; the upstream was not changed).

- **Removed**
  - `ui/src/components/audit/AuditPanel.tsx` (the entire directory was deleted).
  - `DIVA_AUDIT_EVENTS` / `DivaAuditTab` (`ui/src/components/settings/diva-preview-data.ts`)
    and its test assertions (`diva-preview-data.test.ts`).
  - i18n: removed the top-level `audit.*` and unreferenced
    `settings.preview.audit.*` dead-key blocks from `ui/src/i18n/zh.ts` /
    `en.ts`; replaced `dashboard.audit*` keys with `dashboard.trajectory*`.
- **Added** `ui/src/components/trajectory/`
  - `trajectory-demo-data.ts` — deterministic demo data (24 records, 3 runs, 8
    requests + 1 compaction; all system/user/context/compacted/message/tool/
    subtool types, 1 tool failure, and 1 retry), with timestamps derived from a
    fixed baseline and no module-level randomness.
  - `trajectory-utils.ts` — pure projection/formatting utilities: three-lane
    timeline projection (`sequence` uses equal widths / `duration` compresses idle
    time based on actual duration, reproducing DSH `timeline.ts` semantics), run/
    request-start indexes, collapsed display-row projection, and duration
    formatting; no React dependency.
  - `TrajectoryToolbar.tsx` — sticky toolbar: Actual Duration toggle, Collapse All
    Runs, Collapse All Calls, and trajectory search box (reproducing DSH
    `TrajectoryToolbar`).
  - `TrajectoryTimeline.tsx` — Chrome-Network-style 44px label bar + 50px
    three-lane plot area: Input/Model/Tools, assistant spans with TTFT/Decoding
    gradient segments, drag-to-select ranges, hover tooltip (KIND · start/end time
    · Total · TTFT · Decoding), Escape to clear, and dimmed search misses
    (reproducing the DSH `TrajectoryTimeline` + `views.module.css` layout).
  - `TrajectoryLedger.tsx` — two-column ledger (event column: run `#N` labels,
    type-badge icons, `Request #N` jump button, selected/run vertical rails;
    content column: inline `text → result` preview with errors in red), run and
    assistant-call-chain collapsing, search filtering, dimming outside the
    timeline range, and keyboard accessibility (Enter/Space selects), reproducing
    the DSH `TrajectoryTable` row structure.
  - `TrajectoryDetailPanel.tsx` — right-side detail panel: request-level
    (summary/usage/timing: Status/Provider/Model/tool calls/subcalls/errors/retries,
    Token details, TTFT/Decoding/total duration) and record-level
    (input/output/thinking `<pre>`).
  - `TrajectoryPanel.tsx` — composed root component and state machine (collapse,
    search, range, selection).
- **Wiring**: changed the “Audit” Tab in
  `ui/src/components/demo/DashboardDemoView.tsx` to the “Trajectory” Tab
  (`value="trajectory"`), rendering `TrajectoryPanel` inside the Card.
- Added the `trajectory` i18n namespace (matching zh/en structure, with keys for
  toolbar, timeline, ledger, details, collapsing, and type copy).

## Unchanged

- The Overview / Token Tabs, `TokenStatsPanel`, and `getDemoDashboard` snapshot are unchanged.
- No changes to the Go kernel, `src/lib/api.ts` / `src/lib/rpc.ts` /
  `src/lib/store.ts`; no backend or RPC was added.
- Other Settings-page preview sections (channels/network/self-evolution/sandbox)
  and the Evolution-page “auditable evolution governance” copy are unaffected.

## Scope

UI only (`ui/src`), demo data, no backend. Development was completed in the
separate worktree
`../agent-vivy-trajectory` (branch `feat/trajectory-panel`) and merged back
through `parallel-worktree-isolation`.
