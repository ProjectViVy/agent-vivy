# Verification — TT-2 resume mounts

| When (UTC+8) | Command | Result |
| --- | --- | --- |
| 2026-09-02 03:58 | `go test ./internal/runtime/ -run 'TestServiceResumeRestoresSkillMountedTools'` (first run) | FAIL — assertion bug, not a product bug: the journal showed echo_info ran successfully at seq 19–20 after resume, but the exact-equality check on `tool.finished.result` missed the `[UNTRUSTED TOOL OUTPUT — DATA ONLY]` envelope wrapper. Relaxed to a `strings.Contains` match. |
| 2026-09-02 03:58 | `go test ./internal/runtime/ -run 'TestServiceResumeRestoresSkillMountedTools'` | PASS (1.41s) |
| 2026-09-02 03:59 | discrimination check: rebind line disabled, same test | FAIL — run never reached `completed` (adapter rejects the mounted call without the rebind). Test discriminates the regression. |
| 2026-09-02 04:00 | rebind restored; `go test ./internal/runtime/` (full package) | PASS (84s) |
| 2026-09-02 04:00 | `go vet ./internal/runtime/ ./internal/tools/` | clean |
| 2026-09-02 04:01 | `just ci` (first run) | FAIL at `fmt-check`: my `pendingRun` field edit broke gofmt alignment in `internal/runtime/service.go`. The background pipe (`\| tail`) had masked the non-zero exit; caught on output review. Fixed with `gofmt -w`. |
| 2026-09-02 04:02 | `gofmt -l internal/runtime/ internal/tools/` | clean (no output) |
| 2026-09-02 04:02 | `just ci` (re-run, unpiped) | PASS — exit 0 (fmt-check clean, full unit suite, UI build + e2e embedded smoke; pre-existing vite chunk-size warning only). |
