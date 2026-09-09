# Dashboard tab reorder: Token / Trajectory / Sessions

## Changes

The `/dashboard` tabs were adjusted as requested:

- The default selection changed from `Overview` to `Token`, so entering the dashboard immediately shows Token statistics.
- The tabs from left to right are **Token, Trajectory, Sessions** (Token, Trajectory, Sessions).
- The former first tab, `Overview`, was renamed to `Sessions`; its content is unchanged (runtime status + recent activity).
- The page subtitle was also changed to the literal `Token usage, trajectory, and session status are shown in separate sections.` and the English was synchronized.

## Changed files

- `ui/src/components/demo/DashboardDemoView.tsx` — `defaultValue="token"`; TabsTrigger/TabsContent reordered to token → trajectory → overview.
- `ui/src/i18n/zh.ts` — `dashboard.overview`: '概览' → '会话'; subtitle reordered.
- `ui/src/i18n/en.ts` — `dashboard.overview`: 'Overview' → 'Sessions'; subtitle reordered.

## Explicitly not done

- The `Sessions` tab retains the original overview content (runtime status and recent activity); it was not connected to real session-list data.
- The trajectory panel still uses demo data (`trajectory-demo-data.ts`); it is unrelated to this change.
- There are no changes to routes, backend RPC, or data structures.
