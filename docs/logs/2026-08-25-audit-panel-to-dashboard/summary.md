# Move the audit section from the top-right drawer into the dashboard

Date: 2026-08-25
Status: complete

## What changed

The audit section used to live behind a top-right header button that opened a
right-rail Sheet (`AuditDrawer`). It is now embedded directly in the dashboard
(Dashboard, `/dashboard`) as its own card.

- Removed the audit Sheet, its `auditOpen` state, and the `FileText` icon from
  `ui/src/routes/_layout.tsx`.
- Renamed/relocated `ui/src/components/audit/AuditDrawer.tsx` to
  `ui/src/components/audit/AuditPanel.tsx` (component `AuditPanel`; the drawer
  framing in the doc comment and the date-input id were updated).
- `ui/src/components/demo/DashboardDemoView.tsx` renders a new "Audit" card
  after "Recent Activity" containing `<AuditPanel/>` at a fixed 480px height. The card
  sits outside the snapshot error branch because the audit preview is static
  and independent of the demo dashboard API.

## Unchanged

- Audit content, tabs (Structured Events / Gateway Logs / UI Logs), date picker, and
  refresh behavior are identical; data still comes from the static
  `DIVA_AUDIT_EVENTS` preview in `ui/src/components/settings/diva-preview-data.ts`.
- The other header drawers (Sessions, To-do Items) and the approval center are untouched.

## Scope

UI only. No Go kernel, RPC, or storage change.
