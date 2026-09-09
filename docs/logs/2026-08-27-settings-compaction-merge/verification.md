# Verification record (2026-08-27, merge Compaction into General)

## Commands run and results

- The repository-root `just ci` (fmt-check → vet → go test ./... → headless-compile →
ui-ci[pnpm install --frozen-lockfile → typecheck → vitest → vite build]) **passed, exit code
0** (two runs: first for the section reorganization; second rerun after adding the invalid-
tab deep-link clamp, still all green). UI unit tests: 105 passed (15 files), including
`diva-preview-data.test.ts` (2 updated compaction-exclusion tests); the Vite production build
succeeded (2201 modules).
- Browser real-path smoke test (against the running split layout `http://127.0.0.1:3015`
  Vite dev server + `:8787` control plane, a temporary Playwright spec connected directly
  to 3015 and was deleted afterward):
  - The 「Compaction」 tab disappeared from the settings tab list (tablist now contains only
    General / Model / Tools / Vivy Features / Language / Channels Preview / Network Preview /
    Self-evolution Preview / Sandbox Preview).
  - The 「General」 section shows a 「Context compaction」 card:
    `max tokens=8192 / compaction threshold (%)=80 / retain recent messages=12`, usage
    `6,340 / 8,192 tokens`, and buttons 「Run compaction preview / Restore preview defaults」.
  - Changing the threshold to 50 → the threshold-overrun state shows 「Compaction threshold
    reached」, with feedback 「Compaction threshold preview updated.」; clicking 「Run compaction
    preview」 → usage changes to `2,880 / 8,192 tokens`, feedback 「Simulated one context
    compaction preview.」; clicking 「Restore preview defaults」 → inputs return to 8192 / 80 /
    12, feedback 「Compaction configuration restored to preview defaults.」.
  - Deep link `/settings?tab=compaction`: no Compaction tab is selected; it falls back to the
    「General」 section and shows the 「Context compaction」 card (after the fix).
  - **Existing defect found and fixed by smoke testing**: invalid tabs such as `?tab=bogus`
    were not actually filtered by `validateSearch` in this version, so `Route.useSearch()`
    returned the invalid value unchanged; `activeTab` then became invalid, leaving Radix Tabs
    without a match and the settings page blank (pre-existing and unrelated to removing the
    Compaction section, but `?tab=compaction` fell into the same dead end). `SettingsView` now
    clamps invalid values with `isSettingsTab(initialTab)` back to 「General」, and the smoke
    test passes completely.

## Verification conclusion

- `just ci` is all green; user-visible behavior at 3015 (tab removal / merge into General /
  interaction feedback / deep-link fallback) all passed through the real Playwright path.
- Not verified: the full `just ui-e2e` was not run (two existing specs,
  `runtime.spec.ts` / `welcome-wizard.spec.ts`, are known to fail; see `docs/TODO.md` §0.1
  `UI-E2E-STALE`, unrelated to this change); this change did not touch the relevant components.
