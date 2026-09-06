# Verification

## Commands

Worktree: `../agent-vivy-project-instruction-scan` on `feat/project-instruction-scan`.

```
C:/Program Files/Go/bin/go.exe test ./internal/runtime/ -count=1 -timeout 120s -run "TestDiscoverProjectInstructions|TestProjectAgentsMD|TestEngineProjectAgentsMD|TestEinoSkillBackendProject|TestEngineAgentsMD"
```

Result: `ok agent-vivy/internal/runtime` (19.923s).

```
C:/Program Files/Go/bin/go.exe test ./internal/runtime/ ./internal/rpc/ ./internal/tools/
```

Result: `internal/rpc` ok, `internal/tools` ok. First full `internal/runtime` run failed only `TestDiscoverProjectInstructionsRootAgentsMD` (Windows 8.3 vs EvalSymlinks path compare); fixed to compare against `CanonicalInstructionRoot`. Re-run of the new tests passed.

`internal/app`, `cmd/vivy`, `internal/codeface` compile in this worktree requires `ui/dist` (embed). That is produced by `just ci` / `ui-ci`, not by a hand-rolled `go test`.

```
just ci
```

Result: **CI-EXIT:0** from the feature worktree (`feat/project-instruction-scan`). fmt-check, ui-ci (typecheck + 201 vitest + vite build), vet, `go test ./...` (including `internal/runtime` 507s), headless-compile, and plugin-ci all passed.

## Browser smoke

Required for the Skills origin badge. This session could not start the worktree split pair: `127.0.0.1:8787` was already occupied by the original working tree's control plane, and that process does not contain this branch. `3015` was free. Do not treat a screenshot of the old process as verification.

Human follow-up from this branch: `just run` + `cd ui; pnpm dev`, then open `http://127.0.0.1:3015/skills` and confirm `.agents/skills/*` packages appear with origin `project` and a disabled enable switch.
