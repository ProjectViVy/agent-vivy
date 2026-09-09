# Verification — CH-C1-N3 provenance

| Command | Result |
|---|---|
| `go test ./internal/rpc/ -run TestControlMessageProvenanceProjected -count=1` | ok 0.688s (both RPCs project channel turns; UI turns have no provenance; payload key assertion included) |
| `just ci` | All green: fmt-check + vet + go test ./... + headless-compile + plugin-ci (6 modules) + UI tsc/eslint/vitest 195 + vite build |
| `just ui-e2e` | 10 passed / 1 skipped (cron-tasks pre-existing skip; requires a real provider) |

## Notes

- Real-browser verification of the badge requires a real channel turn (Telegram,
  etc.); the e2e stack has no channel injection surface. Coverage therefore
  comes from the RPC contract test (projection shape), the ui-e2e regression
  (10/1), and the manual steps in acceptance.md. On the UI side, the change
  produces zero rendering for UI turns because the field is omitted entirely.
