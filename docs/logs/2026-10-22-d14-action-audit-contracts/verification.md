# Verification

Native Windows focused owner verification, using an independent managed
worktree and its own clean Laputa checkout pinned to
`9acc999301d5d575da2cea902a1c6d213763f381`.

- `node scripts/ensure-laputa.mjs --quiet`: passed after an approved retry to
  create the isolated sibling checkout.
- `go test -p 1 ./internal/actionhost -run
  'TestActionHost(AuditsBoundedMetadataWithoutPayloads|AuditSinkFailuresRemainClosed)'
  -count=1 -timeout 5m`: test-first red on the original implementation because
  serialized audit records still exposed the retired SHA-256 fields.
- `gofmt -w internal/actionhost/host.go internal/actionhost/audit_metadata_test.go`:
  passed.
- `go test -p 1 ./internal/actionhost -count=1 -timeout 5m`: passed after the
  final test addition (`ok agent-vivy/internal/actionhost 5.778s`). Includes
  existing authorization, schema, secret-redaction, timeout, ownership,
  concurrency and bridge failure cases as well as the new metadata cases.
- `go vet -p 1 ./internal/actionhost`: passed.
- `git diff --check`: passed.
- Focused tracked-tree scan found no remaining consumers of `InputSHA256`,
  `OutputSHA256`, `input_sha256`, `output_sha256` or `digestPlaceholder` outside
  historical records and this delivery's explanation. No hash producer remains
  in `internal/actionhost/host.go`.

The Go executable was invoked as `C:/Program Files/Go/bin/go.exe` with
`GOMAXPROCS=2`. Plain `go` was absent from PATH. The sandbox denied access to
the existing Go build cache and gofmt temporary writes; approved retries used
the isolated worktree and cache. These setup failures were not test failures.

The owner requested focused, low-parallel tests while a native full CI lane
was active. This lane did not run full CI, Linux/WSL wiring/kernel gates,
race, PostgreSQL, Recipe pack/Inspect, split UI smoke, live-provider E2E or
format-converter acceptance. The change is not UI behavior and does not start
another split server. Those omissions are not passes, and this record makes
no D14 acceptance or all-hash-retirement claim.
