# Trusted preset full-auto approval — verification

Environment: macOS arm64, `GOTOOLCHAIN=go1.26.4` (the repo pins `go 1.26.4`; the host default
Go 1.27.1 cannot build pinned `grpc v1.64.1` because `http2.TrailerPrefix` is gone).

## Commands and results

| Command | Result |
|---|---|
| `gofmt -l internal sdk ui` | clean (no output) |
| `GOTOOLCHAIN=go1.26.4 go build ./...` | pass |
| `GOTOOLCHAIN=go1.26.4 go test -count=1 -p 1 ./internal/config ./internal/app/... ./internal/domain` | pass (`config` 0.34s, `app` 19.6s, `app/settings` 1.1s, `domain` 0.38s) |
| `GOTOOLCHAIN=go1.26.4 go test -count=1 -p 1 ./internal/runtime ./internal/rpc` | pass (`runtime` 67.4s, `rpc` 19.2s) |
| `GOTOOLCHAIN=go1.26.4 pnpm typecheck` (in `ui/`) | pass |
| `node --test scripts/check-i18n-cross-face.test.js` | 8/8 pass |
| `git diff --check` | pass |

The first full `./internal/runtime` run surfaced one expectation tied to the old contract:
`TestPlanSubmissionSurvivesToolApprovalGate` waited for a `submit_plan` approval while the session
was pinned to `auto`, which now executes the call. The session is pinned to `ask` (the test's stated
"asking approval policy" premise), and the test passes in the final run.

## Pre-existing check failures (not caused by this change)

`node scripts/check-i18n-completeness.js` and `node scripts/check-i18n-cross-face.js` exit 1 in this
checkout on facts already present at `HEAD`:

- `ui/src/lib/failure.ts:56-58` contains Chinese status copy that is not catalogued.
- `ui/src/i18n/en.ts` declares `chatInput.sendNow`, which the cross-face script reports as an
  unclassified Web key.

Both were confirmed with `git show HEAD:<path>`; neither file is part of this diff.

## Not run

- `just ci`, `just wiring-ci`, `just kernel-ci`: require the project's Linux-filesystem WSL/Linux
  checkout and pinned Laputa, unavailable on this host.
- Split-pair browser smoke (`just run` + `pnpm dev`): not run; no runtime wiring changed, and the
  user-facing copy is covered by typecheck plus the i18n key contracts.
