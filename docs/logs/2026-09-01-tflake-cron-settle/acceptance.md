# Acceptance: how to verify manually

- `go test ./internal/runtime/ -run TestCronSettle -count=1` passes
  deterministically within seconds and can be repeated freely: the two
  contracts—delete on success and retain/disable on failure—are directly
  readable.
- Repeated full-load `just ci` runs are no longer brought down by intermittent
  timeouts in the cron contract; even if the end-to-end canary times out under
  extreme load, `TestCronSettle*` still proves that delete-after-run is correct,
  narrowing the failure cause from "suspect contract" to "environmental
  scheduling noise".
