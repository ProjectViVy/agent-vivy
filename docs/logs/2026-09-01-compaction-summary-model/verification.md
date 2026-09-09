# Verification

| Command | Result |
|---|---|
| `go build ./...` + `go vet` (config/runtime/app/provider) | Green (one D-007 intercept-draft iteration: the app's direct `eino` import was moved behind the opaque `runtime.SummaryModel` seam; the `TestEinoImportsQuarantined` rule remained intact) |
| `go test ./internal/config -run TestCompaction -count=1` | Green: the default `summary_model` is empty; `\n`/`\x00` are rejected; TrimSpace normalization works; YAML parsing is covered by `TestCompactionSummaryModelParses` |
| `go test ./internal/runtime -run 'TestEngineSummaryModel\|TestEngineSummarization\|TestEngineReduction' -count=1` | Green: `TestEngineSummaryModelPreferredWhenHealthy` (the summary model is called exactly once, the main model only once for the main loop, and the input contains CHEAP-SUMMARY-cc) + `TestEngineSummaryModelFailsOverToMain` (override failure → main model called exactly twice: failover summary input starts with a system instruction, has no duplicate system message, and contains the original feed; the main-loop input contains the failover summary) |
| `go test ./internal/runtime -run 'TestEngine\|TestCron\|TestService' -race -count=1` | Green (57.6s) |
| `just ci` (all gates, including plugin-ci) | Green |
| `just ui-e2e` | 10 passed / 1 skipped (the pre-existing cron-tasks skip requires a real provider); the engine-reload paths (model-refresh / mcp / sandbox / compaction behavior) remain regression-free |

The `summary_model` call was not smoke-tested against a real provider (a key is
required); failover semantics are covered by scripted contract tests.
