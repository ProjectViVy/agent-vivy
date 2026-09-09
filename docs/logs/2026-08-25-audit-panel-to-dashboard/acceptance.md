# Acceptance

A later agent or human working in this repo should:

1. Find the audit section only in the dashboard (Dashboard) "Audit" card —
   `ui/src/components/demo/DashboardDemoView.tsx` renders `AuditPanel`.
2. Not look for a top-right Audit header button or `AuditDrawer.tsx`; both are
   gone. Reintroducing a header drawer would need a fresh product decision.
3. Keep the audit preview static (`DIVA_AUDIT_EVENTS`); wiring it to real
   journal data is a separate, later task.
4. Run `just ci` as the gate for any follow-up UI change in this area.

No known leftover findings; `docs/TODO.md` §0.1 unchanged.
