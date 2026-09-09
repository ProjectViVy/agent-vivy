# Acceptance

A later agent or human working in this repo should:

1. Open Dashboard and find Token Stats between the KPI row and Recent Activity.
2. Treat the numbers as demo data (`getDemoTokenUsage`). Do not present them as
   Journal-backed usage.
3. Keep one Token surface on this page: the dedicated section, not a duplicate
   KPI card.
4. Run `just ci` as the gate for follow-up UI changes here.

Open remainder: `UI-TOKEN` in `docs/TODO.md` §0.1 (real ledger).
