# Verification

Base: main 59a673ac738368eb4039f24c463df084776f8daf. Existing GitHub Actions run 38057626444 failed all three lanes on the same missing Laputa APIs.

- Before the change, `just ensure-laputa` installed the old source pin. `go test -run '^$' -tags vivy_headless ./internal/modules/diva-cognitive` reproduced the missing API compile errors.
- After the change, `just ensure-laputa` advanced the clean checkout to 4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24. The identical compile check passed.
- `go test -tags vivy_headless ./internal/modules/diva-cognitive` passed.
- A separate plain clone with the candidate lock and no sibling Laputa checkout passed `just setup`, including source bootstrap and dependency download. The environment exports the official Go proxy; the prior Go proxy setting was restored after setup.
- The six existing bootstrap tests passed as part of `just ci`.
- `git diff --check` passed.

Full local `just ci` and exact-commit Windows backend, UI, fresh-clone dev startup, and browser gates are pending at initial publication. Their terminal results belong in the draft PR verification record; this log does not claim they passed. Linux local checks use Go 1.26.4, Node 24.19.0, pnpm 11.19.0, PowerShell 7.6.6, and just 1.58.0. Optional PostgreSQL/live-provider/Studio checks are outside this dependency repair.
