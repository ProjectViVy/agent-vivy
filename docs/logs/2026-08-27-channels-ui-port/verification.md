# Verification record

## Execution environment

- Code: isolated worktree `../agent-vivy-channels-ui` (branch `feat/channels-ui`,
  based on main `8e18293`); the root tree was not written, in accordance with the
  parallel-lane rules.
- All commands were run from the worktree root / `ui/`.

## Commands and results

| Step | Command | Result |
|---|---|---|
| Dependencies | `cd ui && pnpm install --frozen-lockfile` | Passed (pnpm 10.33, 365 packages) |
| Types | `cd ui && pnpm typecheck` | Passed (tsc --noEmit, 0 errors) |
| Unit tests | `cd ui && pnpm test` | Passed: 17 files / 138 tests; added `channel-schema.test.ts` (19), `channel-store.test.ts` (14), updated `diva-preview-data.test.ts` (2); i18n zh/en parity check passed |
| Build | `cd ui && pnpm build` | Passed (vite build, 2211 modules; only the usual chunk>500kB warning) |
| Full gate | `just ci` (fmt-check / vet / test / headless-compile / ui-ci) | **Passed (exit 0)**: Go fmt/vet/test all green (internal/runtime, sqlite, rpc, ui, etc. all ok), ui-ci (typecheck + 138 tests + build) passed |

> Note: the first `just ci` failed once because it raced in parallel with the store
> snapshot-stability fix (`channel-store.ts` changed from cloning on every read to
> 「immutable writes + stable-reference snapshots」 to satisfy `useSyncExternalStore`).
> The full run passed after the fix. The fix also aligned the old tests' 「deep-copy on the
> read side」 assertion with the 「retired-channel write」 semantics (`saveChannel` is a
> no-op for retired channels).

## Added test coverage

- `channel-schema.test.ts` (19 tests): platform coverage and knownness, field defaults
  (discord / email / unknown platform), type-based coercion (boolean / number /
  string-list / text), `splitIdList` / `joinIdList` round trip,
  `normalizeChannelConfig` (known-field normalization + preservation of unknown fields
  and enabled), required fields and `validateConfig`, and `fieldsByGroup` grouping.
- `channel-store.test.ts` (14 tests): localStorage read/write and bad-data filtering
  (bad JSON / discard non-object entries / hide retired channels on read / deep copy on
  write), `toggleChannel` / `removeChannel` (retired-channel no-op), custom event emitted
  by writes, Discord read normalization (gateway_url / intents=37377 / boolean and list
  defaults), `channelStatusFor` readiness approximation (all required fields → ready /
  missing fields listed), and SSR guard.

## Smoke test (user-visible behavior, real Chromium)

Vite dev (temporary port **3016** in the `ui` directory, with `VIVY_BACKEND_ADDR`
pointing to the existing 8787 backend; 3015/8787 were occupied by another parallel lane
and therefore not taken over). The Playwright script exercised the full flow at
`http://127.0.0.1:3016/settings?tab=channels`:

1. Empty state 「No channel configurations yet」 appeared ✓ (and the toolbar's 「Add
   channel」 button was visible).
2. Added Telegram through the wizard: 「Channel configuration wizard」 → platform card
   selected → quick guide 「How do I obtain Telegram credentials?」 → entered Bot Token
   → (verified the tutorial dialog: 「Telegram configuration tutorial」 title + Long
   Polling access-method overview, closable) → Next → 「Configuration complete」 → Done
   → Telegram appeared in the card grid ✓
3. Disabled/enabled the card: title 「Disable」 → 「Disabled」 → 「Enable」 → 「Enabled」 ✓
4. List view: selected telegram in the left column → changed token → 「Save configuration」 ✓
5. Refreshed the page: Telegram remained, with `telegram.token = smoke-token-456` in
   `vivy.ui.channels` (persistence effective) ✓
6. Deleted: confirm 「Are you sure you want to delete channel "telegram"? This action
   cannot be undone.」 (`{{name}}` interpolation correct) → returned to empty state ✓

Conclusion: smoke test passed (supplementary screenshots were in temporary smoke-test
artifacts and were removed under the delivery cleanup rules, so they are not included in
the deliverable).

## Full-gate conclusion

`just ci` exit 0. There were no corresponding Go-side changes after the UI changes; Go
tests served as regression confirmation.
