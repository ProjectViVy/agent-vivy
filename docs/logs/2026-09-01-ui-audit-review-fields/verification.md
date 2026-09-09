# Verification

## Gates

- `just ci` — passed (exit 0): Go fmt/vet/test, headless compile, six plugin-ci
  modules, and UI install + `tsc --noEmit` + `vitest run` + `vite build` all
  green (8 new approvals i18n keys are aligned in en/zh, and the detail `<dl>`
  extension passes type checking).
- `just ui-e2e` — passed (exit 0, 10 passed / 1 skipped, 27.3s): real browser +
  real control plane; `runtime.spec.ts` covers the main Review path (opening the
  Review sheet), and the appended detail `<dl>` rows did not regress existing
  assertions.

## Smoke notes

- The new approval-detail fields require terminal/expired records for full
  observation (pending records only show the always-present created/expires rows),
  and there is no component-specific spec that can seed the data; following the
  CH-C1-N3 precedent, the full e2e suite is the smoke substitute, with the
  manual observation path in acceptance.md.
- WebSocket RPC transport (`/rpc/bootstrap` → WS upgrade) has no curl smoke
  path.

## Static review evidence

- `ui/src/lib/api.ts` `ReviewItem`: `actor?/created_at/expires_at/decided_at?/
  precondition_hash?/stale_reason?/decision_reason?/error?` are all existing
  wire-format fields; this change is front-end rendering only, with no RPC/backend
  change.
- `ui/src/components/approvals/ApprovalsView.tsx`: the detail `<dl>` adds 8
  categories of rows after trust, all conditional on field presence; times use
  `new Date(x).toLocaleString(dateTimeLocale())` consistently (zh/en locale-aware,
  from the same source as PersonaMemoryView/CronTaskManagementView).
- i18n: `approvals.{createdAt,expiresAt,decidedAt,actor,precondition,
  staleReason,decisionReason,errorLabel}` is added in sync to en/zh after the
  trust anchor.
