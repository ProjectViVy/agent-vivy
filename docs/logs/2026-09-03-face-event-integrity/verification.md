# Verification

- `go test ./...` in `faces/headless`: PASS
- `go test ./...` in `faces/tui`: PASS
- Covered synchronous subscribe replay, a 512-delta burst, terminal delivery, and retryable approval/question failures.
- `go test -race ./...` in both Face modules: PASS.
- Repository `just ci` reached only the host's pre-existing missing-WSL bash/job failures after format, UI, vet, and affected tests passed.
