# Verification record — 2026-08-28 provider-direct-write

Scope: independent worktree `feat/provider-direct-write` (the root tree was being merged with channels-ui and
had an unfinished merge, so development was isolated under the parallel-worktree-isolation hard rule).

## Commands and results

| Command | Result |
|---|---|
| `go build ./internal/app/settings/... ./internal/rpc/...` | ✅ |
| `go test -tags vivy_headless ./internal/app/settings/...` | ✅ added registry round-trip/validation/conflict/ActiveKey/Upsert tests |
| `go test ./internal/rpc/...` | ✅ added settings/providers*, update-preserves-registry, and write-time env callback tests |
| `go test -tags vivy_headless ./internal/config/...` | ✅ user home directory defaults (`t.Setenv(VIVY_USER_HOME, tmp)`) |
| `go test -tags vivy_headless ./internal/app/...` | ✅ (including settings_overlay_test.go, after gofmt) |
| `go test -tags vivy_headless ./...` | all green except `internal/eval`, `internal/studiocore`, and `sdk/internal`; these three fail because the **`ui/dist` build artifact is missing** (embed `all:dist`), not because of this change |
| `cd ui; pnpm install --frozen-lockfile` | ✅ |
| `cd ui; pnpm typecheck` | ✅ all green |
| `cd ui; pnpm test` | ✅ 122/122 (custom-providers rewrite 15, saved-models 18, store 4, etc.) |
| `cd ui; pnpm build` | ✅ produces `ui/dist`; afterward eval/studiocore/sdk tests no longer lack embed |
| `just ci` (fmt-check + vet + test + headless-compile + ui-ci) | see below |

## Final `just ci` result

- The first run was blocked by `internal/app/settings_overlay_test.go` not being gofmt-formatted (a pre-existing
  baseline issue in that file, fixed with `gofmt -w`).
- After the fix, the rerun was: fmt-check ✅ · vet ✅ · `go test ./...` (without tags) ✅ (eval/
  studiocore/sdk no longer fail because `ui/dist` is now built) · headless-compile ✅ ·
  ui-ci (install/typecheck/test/build) ✅.

## Known deviations (recorded accurately)

- Browser smoke (`http://127.0.0.1:3015`): this worktree did not start a backend+frontend process pair;
  root-tree resources were also occupied by a parallel lane. Per repository convention, manual acceptance steps are provided
  in acceptance, for the user to verify in a session with an available port (consistent with previous iterations).
