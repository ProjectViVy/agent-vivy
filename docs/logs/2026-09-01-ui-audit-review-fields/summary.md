# Summary

## Topic

UI-AUDIT-REVIEW-FIELDS: fill in audit fields in the Review Center details,
eliminating the gap where expired/stale reasons could not be audited.

## Background and audit finding

The 2026-08-31 UI audit confirmed that the backend `ReviewItem`
(`ui/src/lib/api.ts`) already carried audit fields such as `actor`, `created_at`,
`expires_at`, `decided_at`, `precondition_hash`, `stale_reason`,
`decision_reason`, and `error`, but the `ApprovalsView.tsx` detail `<dl>` only
rendered run/action/target/effect/reversibility/scope/trust—there was nowhere to
see why an approval expired, what the rejection reason was, or who approved it.

## Changes

- `ui/src/components/approvals/ApprovalsView.tsx`: the detail `<dl>` appends
  the following after trust (all rendered conditionally on field presence):
  - `actor` (initiator)
  - `createdAt` / `expiresAt` (always shown, using
    `new Date(x).toLocaleString(dateTimeLocale())`, the same i18n-aware format as
    PersonaMemoryView/CronTaskManagementView)
  - `decidedAt` (decision time, shown only when `decided_at` exists)
  - `precondition_hash` (`<code>` + break-all, hash unchanged)
  - `staleReason` / `decisionReason` (stale/decision reason)
  - `error` (red `text-destructive` text)
- `ui/src/i18n/en.ts` + `zh.ts`: adds 8 keys after the `trust:` anchor in the
  `approvals` block (createdAt/expiresAt/decidedAt/actor/precondition/staleReason/
  decisionReason/errorLabel), synchronized across both languages.

## Explicitly not done

- List (master-side) rows do not gain fields—the detail page is the audit surface,
  and the list remains scannable.
- The pending-action area/workflow is unchanged; this item adds read-only audit
  display only.
- UI-AUDIT-REVIEW-INSPECTOR (Inspector inline review) and
  UI-AUDIT-REVIEW-BUSY-SCOPE (per-item lock) are separate items and are not
  mixed in here.
