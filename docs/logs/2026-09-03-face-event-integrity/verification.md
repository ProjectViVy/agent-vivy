# Verification

- `go test ./...` in `faces/headless`: PASS
- `go test ./...` in `faces/tui`: PASS
- Covered synchronous subscribe replay, a 512-delta burst, terminal delivery, and retryable approval/question failures.
- Full `just ci` is recorded in the final delivery log after all three commits.
