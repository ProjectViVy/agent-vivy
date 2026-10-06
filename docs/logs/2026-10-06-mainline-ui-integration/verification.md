# Verification

- Fetched `origin/main` at `1c6e32af`; the original fix branch was clean.
- Integrated both fix commits without source-code conflicts; rebound the five internal conformance entries using `go run ./sdk/internal/cmd/source-hash internal ""`.
- Combined internal source digest: `6e082b6169f1707286be24eaf034a2cd3869763c7bfefd74b19942b8a771bed6`.
- Restored task-local Go 1.26.4, just 1.40.0, PowerShell 7.5.4 and pnpm shims after the execution environment restarted. The repository's `just ensure-laputa` passed and selected its canonical `ff3936f44ff8cf08c12af2cf698c194cfe474fd3` pin.
- Full merged-tree `just ci`: PASS, exit 0. This includes bootstrap tests (6/6), formatting, type checking, UI tests (72 files / 556 tests), production build, I18N, vet, all Go tests, headless compile, and plugin/face module gates. SDK pack/eval passed in 384.255 seconds; executable conformance passed in 69.582 seconds. Original deliveries already passed full CI and their real split-browser acceptance; this merge changes only conformance metadata and preserves their source behavior.
- A second fetch confirmed remote main stayed at the validated `1c6e32af` during CI. Push dry run passed.
- `git push origin main:main`: PASS, ordinary fast-forward publication from `1c6e32af` to merge `ed1c2173e2902b39583312cd59dfd27048ece389`.
- `git ls-remote origin refs/heads/main`: returned that exact merge commit. Both `git merge-base --is-ancestor f7262fbe HEAD` and `git merge-base --is-ancestor 4e8de423 HEAD` passed (exit 0).
- The final publication record is documentation only; `git diff --check` is sufficient under the repository editorial exception, so product CI is not repeated.
