# Verification

| Step | Command | Result |
|---|---|---|
| Format + vet | `gofmt -l internal && go vet ./internal/storage/... ./internal/runtime/` | clean |
| TT-1 regression tests | `go test ./internal/runtime/ -run TestServiceSessionPin -v -count=1` | 2 PASS |
| Full storage suites (incl. CN-19) | `go test ./internal/storage/... -count=1` | sqlite ok (21.9s), postgres ok, conformance CN-01..CN-19 green |
| Discrimination probe | drive seeding replaced with `(*tools.MountedTools)(nil)`, rerun positive test | FAIL "never reached status completed" (expected); restored → PASS |
| Race, full runtime package | `go test ./internal/runtime/ -race -count=1` (after 60s walkthrough deadline fix) | two consecutive greens; verbose run: `ok agent-vivy/internal/runtime 106.291s`, zero `--- FAIL` |
| Product gate | `just ci` (root) | CI-EXIT:0 (tail-checked) |

UI note: this slice changes no UI-visible behavior (backend tool-admission only), so no 3015 browser smoke — per smoke-for-user-visible-change the gate applies to user-visible or executable behavior; none changed here.
