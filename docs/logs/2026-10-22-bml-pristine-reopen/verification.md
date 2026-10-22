# Verification

Base: `1b8da8cca150c7a4a7734c6e480fa41e24b230cc`, fast-forwarded without reset in
the existing isolated `issue91-d14-identity/agent-vivy` checkout. Own sibling
Laputa remains clean at `9acc999301d5d575da2cea902a1c6d213763f381`.

All commands below used native `C:/Program Files/Go/bin/go.exe` from `bml/`;
tests used `-p 1`. Temporary stores were created by `testing.T.TempDir` outside
the Git checkout and cleaned by the test owner.

| Check | Result |
| --- | --- |
| RED: `go test -p 1 . -run TestHomeWarmupAfterCancelledFirstInitialization -count=1` before the implementation change | Failed in 0.071s with the exact expected `no such table: schema_meta (1)` after the real cancelled transaction rollback |
| GREEN: `go test -p 1 . -run 'TestHome(Warmup\|PathsAndLazyStore\|OpenExistingDoesNotRunSchemaOnForeignFile)' -count=1` | Pass, 0.133s; pristine reads do not mutate bytes, later write initializes/reopens, unknown/schema1 sources refuse unchanged |
| `go test -p 1 . -count=1` | Pass, 7.363s, in the parent-granted bounded BML-only slot |
| `go vet .` | Pass |
| `gofmt -l home.go home_pristine_test.go` | Pass; empty output |
| `git diff --check` | Pass |

Optional BML race was not run: this lane has no owned warm Linux-filesystem
checkout, and creating a fresh environment would exceed the bounded optional
slot. No full runtime, core SQL, browser, PostgreSQL or product CI gate was run
in this lane. The parent owns integrated App reproduction, broader acceptance,
final runtime-pin resealing and fresh independent audit checks.
