# Verification record

Date: 2026-09-01 | Environment: Windows (worktree `agent-vivy-vc0`, branch
`feat/vc1a-bash-tool`)

## Commands and results

| Command | Result |
|---|---|
| `gofmt -l internal/tools internal/app internal/worker` | Clean (after one formatting correction in agent_test.go) |
| `go vet ./internal/tools/... ./internal/app/... ./internal/worker/...` | Passed |
| `go test ./internal/tools/ ./internal/worker/ ./internal/app/ -run 'Agent\|ReadOnly\|SystemPrompt\|Oversize' -count=1` | All three packages `ok` |
| `go build ./...` | Passed (implementation completed and verified before this window) |
| `just ci` | **Passed** (full Go tests + vet + UI vitest, 24 files / 195 cases + vite build) |

## New tests

- `internal/tools/agent_test.go`
  - `TestAgentToolDelegatesTaskAndMask`: trims task/mask before passing them to
    the seam and returns the result unchanged
  - `TestAgentToolValidatesArguments`: three rejection paths—missing task, empty
    task, and non-JSON input
  - `TestAgentToolBoundsTaskAndMask`: rejects task >64 KiB and mask >2 KiB
  - `TestAgentToolWithoutOperationsFails`: fails when the seam is unassembled
  - `TestAgentToolSpec`: Readonly, required task, and present mask
  - `TestRegistryRegistersAgentWithOperations`: registers only with a seam, not
    without one
- `internal/app/agenttool_test.go`
  - `TestReadOnlyToolNamesFiltersSurface`: filters the read-only subset (removes
    write_file/bash, removes mcp_call even when marked readonly, removes agent
    itself, and preserves order)
  - `TestAgentSystemPromptIncludesMask`: no mask line in the base; a mask adds a
    hint as an extension of the base prefix
  - `TestStartAgentTaskRequiresWiredManager`: unarmed reports "not wired"
  - `TestStartAgentTaskRequiresRunScopedParent`: no run context reports
    "run-scoped"
  - `TestStartAgentTaskRejectsBeforeSpawning`: when the parent concurrency limit
    is full, StartChild rejects before spawning (reuses the existing harness and
    creates no real subprocess)
- `internal/worker/server_test.go`
  - `TestRunTurnLoopThreadsSystemPrompt`: `System` becomes the first system
    message through turnLoop, followed by a user message (net.Pipe + fake parent
    captures ModelRequest)
  - `TestRunRejectsOversizeSystemPrompt`: System >64 KiB is rejected by
    "bounded harness limits"

## Smoke exception

Under the `smoke-for-user-visible-change` rule, the agent tool's real end-to-end
path needs a real provider key (the subagent calls a real model through the model
broker). This environment has no key, so two layers of real protocol tests are
the substitute and are recorded here:

1. Worker protocol layer: real `worker.Run` server + real JSONL RPC peer
   (net.Pipe), asserting system-message threading and bound rejection—the real
   subagent harness behavior.
2. App assembly layer: `agentToolRef` guard path + `StartChild` rejection path
   through the real service/policy/storage combination (`newChildBrokerTest`
   harness, constructed like the existing child-run tests).

Browser 3015 smoke is not applicable: this slice changes the kernel tool surface
  and has no UI changes (the UI is untouched).
