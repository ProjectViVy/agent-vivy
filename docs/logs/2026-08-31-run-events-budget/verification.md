# Verification — 2026-08-31 run-event budget exemption

## Commands and results

```text
go test ./internal/runtime/ -run 'TestReserveMappedBudget|Budget' -count=1
  -> ok  agent-vivy/internal/runtime  1.283s

just ci
  -> EXIT=0 (fmt/vet, all Go tests, 175 UI tests, and UI build all passed)
```

## Added test

`TestReserveMappedBudgetSkipsStreamingDeltas`（internal/runtime/service_test.go）：

- 600 mapped `model.delta` events produce no budget error on a MaxEvents=5
  ledger (streaming chunks no longer consume the event budget);
- semantic events (`model.completed` / `model.usage`) are still accounted for normally;
- a semantic-event batch exceeding MaxEvents still trips the breaker with `ErrBudgetExceeded`.

## Real-path evidence (the problem before the fix)

- `data/logs/vivy.log.2026-08-31`: two entries at 14:36:36 and 14:43:35
  for `run budget circuit breaker opened kind=events limit=512` (the two smoke runs).
- Real-path verification of the Settings → Tools card save path: select
  list_dir/read_file → `tools/set-active` → `data/settings.yaml` contains
  four `tools_enabled` entries → the card echoes the literal
  "Currently using a custom override". The path
  works; it was then cleared with `Restore configuration defaults`
  to return to config-default semantics.

## Pending manual re-verification

After the user restarts `just run`, the request "What tools do you have now? Can you see what is in the
workspace?" should complete within the 512 budget (multiple tool-call rounds + streaming reply),
without another safety-budget interruption.
