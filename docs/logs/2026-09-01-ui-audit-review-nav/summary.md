# UI-AUDIT-REVIEW-NAV: `/approvals` main-navigation entry

## What changed

- Add an `/approvals` item to `ConversationSidebar`'s main-navigation
  `NAV_ITEMS` between Dashboard and Cron (ShieldCheck icon, matching the
  ApprovalsView title). The full Review Center route was previously discoverable
  only as a sheet opened by the chat shield (2026-08-31 audit conclusion).
- i18n: `nav.approvals` is `en=Approvals` / `zh=审批中心`, consistent with the
  existing `layout.reviewCenter` / `approvals.title` copy and introducing no new
  semantics.
- Add e2e `ui/e2e/approvals-nav.spec.ts`: provider-independent main-navigation
  click → `/approvals` URL + title/subtitle assertions + no demo localStorage +
  reachable entry in the 390px mobile drawer; it pins main-navigation reachability
  against regression.

## What was explicitly NOT done

- Run Inspector's Review inline/tab (UI-AUDIT-REVIEW-INSPECTOR) is a separate
  item.
- The ApprovalsView itself and review-sheet logic are unchanged; the navigation
  item has no pending badge (the existing RPC has no lightweight count field, so
  the scope is not expanded for this).
