# ND-1 verification

## Scoped run (plan §"Validation commands")

```
go test -timeout 20m ./internal/runtime ./internal/mcphost ./internal/toolhost ./internal/app -run 'TestToolFailure|TestMCP|TestEnhanced|TestToolAdapter' -count=1
```

```
ok  agent-vivy/internal/runtime   0.715s
ok  agent-vivy/internal/mcphost   0.005s
ok  agent-vivy/internal/toolhost  0.005s [no tests to run]
ok  agent-vivy/internal/app       0.020s [no tests to run]
```

## New tests

- `internal/runtime/tool_failure_test.go`
  - `TestToolFailureClassification` — the §5 table: invocation ArgError
    (plain and wrapped) → recoverable/invalid_arguments/unknown; readonly
    invocation wrapping fs.ErrNotExist → recoverable/not_found/none; write
    invocation wrapping fs.ErrNotExist → unclassified (fatal); cancelled
    parent ctx + ArgError, context.Canceled, context.DeadlineExceeded, and
    a fabricated native interrupt → all unclassified; MCP
    ToolExecutionError → recoverable/remote_tool_error/unknown; unknown
    transport and storage errors → unclassified.
  - `TestToolFailureMarkGatesOnBoundLeg` — no bound state → mark is a
    silent no-op (non-model shell callers unchanged); bound state with no
    call ID → invariant failure.
  - `TestToolFailureCorrection` — Service-level scripted-model run through
    Host.Invoke and governedTool: one turn with four calls observing
    invalid_arguments, not_found, command_failed, remote_tool_error; the
    durable `tool.finished` rows carry outcome/reason/effects plus the
    diagnostics; the CommandResult keeps exit_code/stdout/stderr; the
    model corrects the next action and the same Run completes.
  - `TestToolFailureRefusalNeverExecutes` — deny-table-refused write:
    finished row is refused/policy_denied/not_executed and the tool ran
    zero times.
  - `TestToolFailureUnknownEffectsCountOnce` — write invocation ArgError
    under an auto-approve session: recoverable/invalid_arguments/unknown,
    invocation count exactly one (no replay).
  - `TestToolFailureTransportErrorStaysFatal` — unlisted transport error:
    run fails, no soft-converted outcome.
- `internal/runtime/enhanced_tooladapter_test.go` —
  `TestEnhancedAdapterPreservesFailureOutcome`: the soft conversion and
  its typed row survive the multimodal adapter; the diagnostic reaches the
  model as a text part.
- `internal/mcphost/toolworld_test.go` — IsError → typed
  `*ToolExecutionError` carrying the untrusted remote text; transport
  error stays untyped; success path unchanged. `fakeSession` gained a
  `callResult` field for the IsError case.
- `internal/runtime/nudge_contract_test.go` — harness gained an
  `autoApprove` option (needed so an effectful stub reaches its own
  invocation instead of the approval interrupt).

## Full gate

`just ci` — fmt-check, ui-ci (typecheck + 392 vitest tests + vite build +
i18n audits), `go vet ./...`, `go test -timeout 20m ./...` (all packages,
including the ~230s sdk/internal pack suite), headless compile, plugin-ci:
all green.

## Digest

`go run ./sdk/internal/cmd/source-hash internal ""` →
`67b6aa4b…9f4f`; applied to the five `internal`-rooted `sourceSha256`
entries in `sdk/internal/assembly/conformance_results.json` via sed
(compact-array formatting preserved — a 5-line diff).

## Notes

- The unrelated `ui/src/generated/assembly.ts` sourceHash churn from
  pnpm install regeneration (Linux vs Windows) is left out of the commit.
- The known upstream `EINO-TOOLSNODE-ERR-RACE` (docs/TODO.md §0.1) is
  unaffected — `just test` runs without `-race`.
