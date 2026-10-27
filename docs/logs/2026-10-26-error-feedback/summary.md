# Error feedback: one quiet hierarchy instead of four red boxes

Date: 2026-10-26. Lane: worktree `.worktrees/ui-overhaul/agent-vivy`, branch `feat/ui-overhaul`.

## What changed

Failures now use one of three levels instead of a mix of heavy red cards, raw text
against the composer, bare red `<p>` strings and page banners:

1. **Toast** (`ui/src/lib/toasts.ts` + `ui/src/components/feedback/ToastViewport.tsx`)
   — top-right, portal to `document.body`, `z-[300]`, for transient action failures.
   Newest first, max 3, repeats merge with a `×N` badge, `danger`/`warn` persist,
   `ok` 2500 ms, `info` 3500 ms, hover/focus pauses the remaining time, Escape
   dismisses the top item, `role="alert"` for danger/warn and `role="status"`
   otherwise. The store is a separate zustand instance so timers never enter the
   JSON-RPC domain store, and `store.ts → lib/toasts.ts` is the only import
   direction.
2. **Persistent notice** (`ui/src/components/feedback/FailureNotice.tsx`) —
   `full | compact | inline` on `bg-raised` with a danger rail and folded
   diagnostics. Replaces the deleted `RecoverableError.tsx` at the root error,
   chat history/run failures, work controls, settings, MCP, cron, skills,
   approvals, lifecycle, files, the welcome model step and every first-party
   plugin surface.
3. **Field / dialog inline** — unchanged placement where the error belongs to one
   input or dialog body.

Supporting changes:

- `ui/src/lib/failure.ts` gained `looksTechnical` and `technical` on
  `ClassifiedFailure`. Machine noise (`rpc error`, `code =`, JSON, URLs,
  `TypeError`, stack frames) no longer becomes the main sentence; the classifier
  substitutes `feedback.unknownDetail` and keeps the raw text behind
  `详情 / 复制诊断`. Human backend copy (provider HTTP status split) stays primary.
- `API_KEY_MARKERS` narrowed from `api key` to `api key missing|rejected|invalid`
  so the wizard's own "Enter an API key…" guidance is no longer rewritten as a
  generic key-missing failure.
- Header connection pill: localized `connecting / reconnecting / idle /
  disconnected`, tone `ok | warn | danger | sunken`, icon-only on mobile, and one
  `errors.disconnected` toast when the connection transitions into `error`
  (suppressed while `runError` already explains the failure).
- Settings errors have one owner (`SettingsView`); the duplicate renderings in
  `LanguagePicker`, `NetworkToolsCard`, `SandboxSettingsCard` and
  `ModelSettingsCard` are gone. Rename/delete/permission, message actions,
  clipboard, model switch, MCP toggle, cron row actions and skill toggles report
  as toasts instead of invisible state.
- All 7 first-party plugin UIs (masks, memory, notebook, persona, workflow,
  evolution, session-tree) use `FailureNotice` for block-level errors and pass
  raw error strings through the classifier, so plugin panels fold diagnostics too.
- i18n: `feedback.*` (5 base keys + 14 `feedback.operation.*`) and
  `layout.connection.*` added to `en.ts`, `zh.ts`, and the sorted
  `scripts/i18n-cross-face-contract.json` web projection.
- Audit: `ui/scripts/audit/screenshots.mjs` gained a `toast` surface that imports
  `/src/lib/toasts.ts` in the dev pair and seeds danger(+technical)/warn/ok toasts
  before capture, so the rendered-ink sweep measures the new component.

## Explicitly not done

Socket-close detection and reconnect policy; toast persistence across reload;
global `unhandledrejection` capture; backend string localization; TUI feedback;
new dependencies; plugin catalog additions beyond existing strings. No
`docs/TODO.md` rows were added because nothing from this plan was deferred.
