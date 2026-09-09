# Verification

| Command | Result |
|---|---|
| `go test ./internal/app/settings/ -race -count=1` | ok 2.481s (including 4 new Update tests) |
| `go vet ./internal/rpc/ ./internal/app/settings/` | clean |
| `go test ./internal/rpc/ -count=1` | ok 17.655s |
| `just ci` | Green (golangci-lint + go test ./... + UI tsc/eslint/vitest, 195 tests + vite build) |
| `just ui-e2e` | 10 passed / 1 skipped (pre-existing cron-tasks skip), 21.6s |

## New tests (`internal/app/settings/settings_test.go`)

- `TestUpdateConcurrentUpsertsAllSurvive` — SET-RMW regression: 8 goroutines
  each Update-upsert one unique `(id, base_url)` provider entry; the final Load
  preserves all 8. Under the same scheduling, Load→modify→Save loses entries
  (last-writer-wins).
- `TestUpdateFnErrorAbortsWrite` — the `fn` error passes through unchanged and
  the document remains unchanged (`errors.Is` assertion + before/after
  `DeepEqual`).
- `TestUpdateRejectsInvalidCandidate` — candidate-document validation fails →
  `IsValidationError` is true, `errors.As` extracts the underlying error, and the
  document remains in its previous state.
- `TestUpdateReturnsPersistedDocument` — starts from a missing file and an empty
  document; the return value equals the Load result (`DeepEqual`).

## Notes

- The first version of `TestUpdateReturnsPersistedDocument` hit existing YAML
  round-trip behavior: a nil `Models` slice marshals as `models: []` and decodes
  as an empty slice, so `DeepEqual` fails (the same happens with Save and was not
  introduced by Update). The test avoids this by giving the entry a non-empty
  `Models` value.
