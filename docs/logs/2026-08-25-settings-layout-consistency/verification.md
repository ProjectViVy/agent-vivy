# Verification — 2026-08-25 Settings-page layout and description consistency

## `just ci` (repository root)

Result: **passed**. The first run was blocked by 8 UI type errors from parallel
WIP (`MASK_OPTIONS` renamed in masks, `t` undefined in LifecycleView); after the
fixes, everything was green:

- `go vet ./...` / `go test ./...`: all ok (cached)
- `go test -tags vivy_headless ./cmd/vivy ./ui`: ok
- UI: `pnpm typecheck` passed, `pnpm test` **all 31 tests in 8 test files passed**,
  and `pnpm build` succeeded (Vite build, 2188 modules)

Fixed blockers: `ui/src/components/chat/MaskAndModelSwitcher.tsx`,
`ui/src/components/masks/MaskManagementView.tsx` (`MASK_OPTIONS` →
`maskOptions()`), and `ui/src/components/lifecycle/LifecycleView.tsx` (added
`useTranslation()` inside `GenerationSelect`).

## Browser smoke (http://127.0.0.1:3015, split Vite)

Before the fixes the app was blank (`MASK_OPTIONS` was undefined, so React did not
mount); after the fixes it mounted normally and each item was verified:

| Check | Result |
|---|---|
| Six preview tabs in the tab bar carry a “Preview” marker | ✅ Channels / Network / Language / Compression / Self-Evolution / Sandbox all display it |
| General-page DemoNote removed | ✅ The General page now has only Application Info, Theme, and the Agent-Diva migration-preview area |
| “General & About” restored | ✅ All three cards—Chat Display / Cache & Runtime Status / About Vivy—are present |
| Network page “Current Preview Summary” has a description | ✅ “Summarizes the selections above; does not represent actual network-tool configuration.” |
| Compression page “Compression Configuration” has a description | ✅ “Changes affect only this page’s preview; they are not written to runtime configuration.” |
| Vivy Features page card headers unified | ✅ Lifecycle and Run Inspector both use “icon + title + description” |

## Unverified items

- Visual details (the screenshot was checked as a text snapshot; the model cannot
  read images); layout correctness is based on DOM text structure.
- Theme/language-switch interactions were not tested (parallel-stream files, not
  touched in this round).
