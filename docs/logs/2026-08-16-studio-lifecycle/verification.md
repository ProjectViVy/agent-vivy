# Studio Lifecycle Closure Verification

Date: 2026-08-16

## Deterministic boundary

Every test runs against temp worktrees; tenant paths (`data/vivy.db`,
`data/workspaces/`) are represented by sentinel files the tests assert are
never created or written. The candidate EXE for eval is built from
`./cmd/vivy` (same pattern as the existing eval runner tests); the Studio
core spawns it directly — no species RPC, no `evals/start`.

## Commands

| Check | Result |
|---|---|
| `gofmt -l .` | pass; no files reported |
| `go vet ./...` | pass |
| `go test ./...` | pass |
| `just ci` | pass |
| `just studio` | builds `vivy-studio.exe` |

## ST acceptance evidence

| Slice | Acceptance | Evidence |
|---|---|---|
| ST-5 | Studio packs and evaluates itself; live species zero participation | `internal/studiocore/service_test.go` `TestEvalSpawnsCandidateItself` (candidate booted by the Studio, EvalRun in Studio ledger, tenant paths untouched); `TestRecordGenerationRecordsPackOutput` |
| ST-7 | Human-gated release → install to daily location; next launch is the new body | `TestReleaseRequiresHuman` (`machine` refused), `TestInstallAndRollbackRoundTrip` (daily `vivy.exe` hash == released generation; `install.json` present) |
| ST-8 | Rollback to previous Release; tenant Journal untouched | `TestInstallAndRollbackRoundTrip` (rollback restores body A, `install.rolled_back` event, sentinel Journal bytes unchanged), `TestRollbackWithNothingToRestore` |

## Live-process safety

`install`/`rollback` only copy files into the target and the Studio-owned
snapshot dir; they never start or kill a process, so a running `vivy.exe`
keeps its old mapping until exit (NG-3).
