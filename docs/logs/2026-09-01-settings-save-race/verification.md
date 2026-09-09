# Verification commands and results

| Command | Result |
| --- | --- |
| `go test ./internal/app/settings/` | Passed (including the new concurrency test) |
| `go test ./internal/app/settings/ -race -count=3` | Passed; before the fix it could reproduce `settings: commit: rename ... Access is denied` |
| `just ci` | Exit code 0 (golangci-lint + gofmt + go test ./... + UI tsc/eslint/vitest/build; run twice after the fix) |
| `just ui-e2e` | `1 skipped / 9 passed`, exit code 0; the model-refresh internal error no longer appears |

## Evidence chain

1. `ui/test-results/model-refresh/e2e-model-add-model/error-context.md`:
   the providersError text is literally `internal error`, and the registry row
   is missing the newly added model.
2. `internal/rpc/control.go` `internalError()` always returns the literal
   "internal error" and intentionally discards detail, pointing to a server-side
   Load/Save failure rather than business validation.
3. `data/e2e-home/vivy.log` (the e2e workdir): settings parse failures and dial
   records match the timeline of the case.
4. Local reproduction before the fix: the first run of the concurrent Save test
   immediately reported `settings: commit: rename ... Access is denied`
   (Windows); including Load's read window in `fileMu` made it pass.
