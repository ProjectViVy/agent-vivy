# UI-AUDIT-DASHBOARD-LIVE — Connect the Dashboard overview to real RPCs and remove the demo snapshot

## Changes

The `/dashboard` Overview tab no longer reads the `vivy.demo.dashboard` localStorage
demo snapshot; it now calls the real RPCs in parallel:

- Session count → `session/list` (`sessions.length`)
- Active runs → `background/list` (runs whose status is not one of the terminal
  `completed/failed/cancelled` states; the audit item specified the background
  RPC because the kernel has no "list all runs" endpoint, so the background
  registry is the run list)
- Pending Review → `review/list` (the backend filters `status: 'pending'`,
  covering both approval and question)

The error state reuses `DemoLoadError` (the shared error banner, also used by
the Token tab) + retry; loading retains the skeleton screen. **The entire Recent
activity card is deleted** (the audit ruling was to delete activity items when
there is no existing endpoint—the kernel has no general activity-stream RPC, so
the demo activity items were fabricated data).

## Removed demo surface

- `demo-api.ts`: `getDemoDashboard`, `DEFAULT_DASHBOARD`,
  `STORAGE_KEYS.DASHBOARD` (`vivy.demo.dashboard`)
- `types.ts`: `DemoDashboardSnapshot`
- i18n en/zh: `dashboard.activityTitle/activityDesc`,
  `demo.dashboard.*` (activity-item copy block)
- The component was renamed to its canonical name:
  `components/demo/DashboardDemoView.tsx` →
  `components/dashboard/DashboardView.tsx` (the `_layout.dashboard` route import
  was updated as well); the remaining demo names inside the component
  (`DemoLoadError`, `TokenStatsPanel` in `components/demo/`) remain unchanged—
  they are real shared panels in use and are out of scope for this item.

## Out of scope (discovered and tracked separately)

- The Trajectory tab's `TrajectoryPanel` remains demo-only data
  (`trajectory-demo-data.ts`; the component comment says "no backend"). The audit
  item covered only the overview numbers and activity items; the trajectory panel
  is a separate surface tracked by the UI-TRAJECTORY-DEMO TODO and is not
  expanded in this slice.
- The Token tab already uses the real `stats/tokens` and needs no change.

## Already-real surface

`TokenStatsPanel` (periodic `stats/tokens` snapshots, model distribution, trends,
and export) and the route structure remain unchanged.
