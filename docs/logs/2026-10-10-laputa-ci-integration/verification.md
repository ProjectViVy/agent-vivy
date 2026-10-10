# Verification

Base: main 59a673ac738368eb4039f24c463df084776f8daf. Existing GitHub Actions run 38057626444 failed all three lanes on the same missing Laputa APIs.

- Before the change, `just ensure-laputa` installed the old source pin. `go test -run '^$' -tags vivy_headless ./internal/modules/diva-cognitive` reproduced the missing API compile errors.
- After the change, `just ensure-laputa` advanced the clean checkout to 4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24. The identical compile check passed.
- `go test -tags vivy_headless ./internal/modules/diva-cognitive` passed.
- A separate plain clone with the candidate lock and no sibling Laputa checkout passed `just setup`, including source bootstrap and dependency download. The environment exports the official Go proxy; the prior Go proxy setting was restored after setup.
- The six existing bootstrap tests passed as part of `just ci`.
- `git diff --check` passed.

Full local `just ci` and exact-commit Windows backend, UI, fresh-clone dev startup, and browser gates are pending at initial publication. Their terminal results belong in the draft PR verification record; this log does not claim they passed. Linux local checks use Go 1.26.4, Node 24.19.0, pnpm 11.19.0, PowerShell 7.6.6, and just 1.58.0. Optional PostgreSQL/live-provider/Studio checks are outside this dependency repair.

## CI follow-up

The first candidate's Windows UI lane passed 598 tests plus typecheck/build/i18n; the browser lane passed both Playwright tests. Fresh-clone setup and independent cold dev startup also passed (run 38060015686).

The first local full gate exposed the stale internal-source conformance digest and memory recall tests using the wrong generated body. No internal source had changed at that stage. The canonical `go run ./sdk/internal/cmd/source-hash internal <old-digest>` produced `182ede26e698d87eaedd1534b40ee7d44ba7a0069268354229a76a6afc864797`; only the five matching evidence identities were refreshed. The unchanged conformance reproduction gate must execute successfully before accepting them.

A standalone validation clone successfully packed the DIVA Recipe and passed both SDK memory-loop overlay preparation/identity tests. The previously failing `TestMemoryLoopRecallAfterProcessRestart/profile-1` passed with unchanged assertions under that verified overlay. Full `just ci` and Windows exact-commit checks remain in progress at follow-up publication; the draft PR records terminal outcomes.

Local worktree execution also exposed Go VCS discovery treating the managed ancestor `.git` mount as the repository. Validation uses a standalone clone and disposable `/var/tmp` fixtures under the environment's approved execution override, without disabling VCS stamping. Local Chromium download returned `403 Domain forbidden`; Windows CI provides browser verification. No access restriction was bypassed.

Run 38061869472 passed UI and both browser tests; backend remains pending. The superseded Windows backend log revealed `TestMemoryLoopCrashC04PartialEffectBatchStopsAtUnknown` failing on a transient file sharing violation. The fixture now retries that error without extending its deadline. The new real-lock regression requires Windows CI and was not executed locally. These test-source edits change the canonical internal digest to `5562095fddc63038324b6874a235cfbc44e90fdf55df315cc566c6018d127766`; five evidence identities are refreshed, with their conformance reproduction still required.
