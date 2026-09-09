# Verification — VC-1e

Date: 2026-08-31 | Branch: `feat/vc1a-bash-tool` (vc0 worktree)

## Commands and results

| Command | Result |
| --- | --- |
| `go build ./...` | exit 0 |
| `go vet ./internal/runtime ./internal/app ./cmd/vivy` | exit 0 (fixed one issue: `domain.RunEvent` has no `ID` field; use `Seq`) |
| `go test ./internal/runtime -run "TestEngineAgentsMD\|TestServiceAgentsMD" -count=1 -v` | 5/5 PASS |
| `go test ./cmd/vivy -run TestInit -count=1 -v` | 4/4 PASS |
| `go run ./cmd/vivy init` (smoke test in `data/tmp-init-demo/`, including main.go) | exit 0, generated AGENTS.md, and printed the "Record only non-obvious knowledge" prompt; the second run exited 1 and refused to overwrite, leaving the original file unchanged |
| `just ci` (full product gate) | exit 0 (see below) |

## just ci

- 2026-08-31 background task `b6mh8nnq4`: exit 0; all package tests + UI build passed.

## Test coverage notes

`internal/runtime/agentsmd_test.go`：

1. `TestEngineAgentsMDInjection` — real `EinoFilesystemBackend` + real workspace: two model calls (echo tool + finalization) each contain exactly one injected message, immediately before the first real user message (idempotent).
2. `TestEngineAgentsMDMissingFileInjectsNothing` — a run without AGENTS.md is completely unaffected (regression coverage for this iteration's ErrNotExist sentinel fix).
3. `TestEngineAgentsMDRunWorkspaceScoping` — run A's AGENTS.md does not leak into run B (per-run workspace isolation).
4. `TestServiceAgentsMDInjectionIsTransient` — full-stack service e2e: the model sees the injected content, while **Journal replay and message storage contain no marker** (evidence of D6 transience).
5. `TestServiceAgentsMDInjectionSurvivesApprovalResume` — approval suspension → resume flow: after the injected message crosses the checkpoint round trip, there is still exactly one (the eino Extra marker survives serialization, so idempotent de-duplication holds), and the Journal still has no marker.

`cmd/vivy/init_test.go`: template creation, empty-directory refusal (only .git counts as empty), overwrite refusal (original file unchanged byte-for-byte), and rule-file detection (`.cursorrules` + copilot are detected and included in the template).

## Smoke policy note

- `vivy init` is a real CLI-path smoke test (`go run` executes in a scratch directory inside the workspace, then cleans up).
- AGENTS.md injection itself cannot receive a 3015 browser smoke test in this lane: injection occurs during a model call and requires a real provider (this machine has no provider credential, and D-010 forbids putting keys in fixtures). The service e2e tests (#4/#5) drive the exact production engine → adapter → backend → journal stack and are the strongest evidence available in this environment.

## Deferred

- Stale-read protection / file_versions: pending O1..O6 (same record as VC-1d).
