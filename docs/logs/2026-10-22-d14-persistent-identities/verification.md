# Verification

Candidate base: `a12748a7114bb54fd0bdf663bfb1f01fcec90802`.
Isolated managed checkout: `issue91-d14-identity/agent-vivy`.
Own sibling Laputa pin: `9acc999301d5d575da2cea902a1c6d213763f381`.

Native commands use `C:/Program Files/Go/bin/go.exe` and `-p 1` to keep focused
verification separate from the lead's heavy CI lane.

| Command | Result |
| --- | --- |
| `node scripts/ensure-laputa.mjs --quiet` | Pass after scoped sibling-write approval; own clean pin |
| `go test -p 1 ./internal/channelhost ./internal/observerhost ./internal/modules/memory` | Pass; 84.713s / 2.221s / 2.934s |
| `go test -p 1 . -count=1` from `bml/` | Pass; 8.337s, including source ambiguity, ID conflict, reopen and concurrent allocations |
| `go test -p 1 . -run TestAppendHistory -count=1` from `bml/` after strengthening projection-preservation assertion | Pass; 1.125s |
| Broader storage run, `go test -p 1 ./internal/storage/migrations ./internal/storage/sqlite ./internal/storage/postgres ./internal/modules/memory ./internal/observerhost -count=1` | Migrations pass 37.434s; SQLite pass 367.023s; PostgreSQL package compiles, real cases skip without DSN. Interrupted after those packages; final two packages covered separately below |
| `go test -p 1 ./internal/channelhost ./internal/modules/memory ./internal/observerhost ./internal/storage/sqlite ./internal/storage/postgres -run 'Test(ChannelSession\|EnsureSession\|IngestIdentity\|ProviderIngest\|CursorKey\|ConvertedObserver)' -count=1` | Pass; 3.624s / 2.055s / 0.399s / 0.966s / 0.115s; PostgreSQL real fixture skipped without DSN |
| `go test -p 1 ./internal/storage/migrations -run 'Test(ChannelIdentityMigration\|IdentityPreflight)' -count=1` | Pass 4.134s; injected migration failure rolls back all 047 columns and marker, retaining history |
| `go test -p 1 ./internal/storage/postgres -run TestChannelSessionIdentityAndPreflightPostgres -v -count=1` | Compiles; explicit skip because `VIVY_POSTGRES_TEST_DSN` is unset |
| `gofmt -l` on all changed Go files | Pass; empty output |
| `git diff --check` | Pass |
| Focused producer residue search for SHA imports/helpers, sanitizer and truncated-ID constants | Pass; no matches |

Initial test attempts could not access the default Go cache under the sandbox
and did not execute tests. Approved native commands above ran the actual suites.
PowerShell PATH also lacked Go, so the explicit installed binary was used. No
global environment or dependency settings changed.

Integrated Linux `just wiring-ci` / `just kernel-ci`, real PostgreSQL, and final
copied-target artifact acceptance are not claimed here; the lead owns those
lanes after merging this deliverable. PostgreSQL tests require a real disposable
`VIVY_POSTGRES_TEST_DSN`; an absent DSN is a skip. There are no UI behavior
changes; the lead owns integrated product smoke. No release was produced.
