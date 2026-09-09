# Verification record — generation-parameter demo moved into 「Settings → General → Advanced Features」

Work branch: `feat/settings-genparams-provider` (developed in a worktree; uncommitted
changes in another lane's root tree were left untouched). The direction was reversed once
(provider-panel version → General Advanced Features per-model version), and before delivery
`git reset --soft` reorganized it into the final single commit.

## Commands and results

| Step | Command | Result |
| --- | --- | --- |
| Full gate (first run `pnpm build` to produce real `ui/dist` for go:embed) | `just ci` | ✅ passed (fmt-check / vet / go test / headless-compile / typecheck / vitest / vite build) |
| New unit tests (demo generation parameters independent per model) | `pnpm test` (11 `demo-api.test.ts` tests inside ci) | ✅ passed |
| Browser smoke test (embedded UI path) | `pnpm exec playwright test e2e/genparams-advanced.spec.ts` | ✅ 1 passed |

## Browser smoke test (Playwright, exercised in a real browser)

Added `ui/e2e/genparams-advanced.spec.ts` (submitted with the delivery as a regression
specification), and exercised `http://127.0.0.1:8799` (e2e-created backend + this build's
embedded `ui/dist`):

1. `/settings` General tab: the 「Advanced Features」 card appeared; the full page had no
   standalone card-level 「Generation parameters」 heading.
2. After seeding two selected models (gpt-4o-mini / gpt-4o), the dropdown defaulted to
   gpt-4o-mini: temperature 0.7, Max Tokens 4096, and the notice 「Editing generation
   parameters for gpt-4o-mini」.
3. Save 8192 → 「Saved locally」; the `vivy.demo.gen-params` key
   `openai/https://api.openai.com/v1/gpt-4o-mini` was written.
4. Switch the dropdown to gpt-4o → default 4096 loaded, with the gpt-4o-mini key
   unaffected; after saving 1024, both keys coexisted without overwriting each other.
5. Refresh → gpt-4o-mini remained selected by default and the saved 8192 was read;
   `vivy.demo.gen-params` persisted.

## Notes

- The root-tree :3015/:8787 belonged to another parallel lane (compaction section merged
  into General); this branch used the `ui/e2e` self-created port 8799 for smoke testing and
  did not interfere with the root tree.
- The first `just ci` run in a new worktree with 「no real `ui/dist`」 fails
  (`go:embed all:dist` / embed test 503); after `pnpm build`, all checks were green—an
  environment gap, not a code issue.
- Existing issues are unrelated to this change (recorded on the shared board): stale copy
  assertions at `ui/e2e/runtime.spec.ts:84` and `welcome-wizard.spec.ts:33`, see
  `docs/TODO.md` §0.1 `UI-E2E-STALE`.
