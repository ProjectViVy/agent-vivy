# Integration verification

Venue: sole Windows stable writer; independent Linux-filesystem Debian WSL
checkout `/home/mastwet/work/vivy-issue91-runtime/agent-vivy`, clean pinned Laputa
ff3936f44ff8cf08c12af2cf698c194cfe474fd3. Go1.26.4, Node24.21.0, pnpm10.33.0;
GOFLAGS=-p=2, vivy_headless owner checks. Other dirty checkouts were not reused.

- RED: combined Generation tests exposed missing retired-column refusal, missing
  accepted037 source recognition and legacy Generation shape drift detection.
  GREEN: full `go test -tags=vivy_headless ./internal/storage/migrations -count=1 -timeout 3m`.
- `just wiring-ci`: PASS,16 Node tests and105 assembly cases,zero skips.
- `just kernel-ci`: PASS,vet and1734 owner cases; actual minimal artifact
  Pack/Inspect/boot also passed. Existing optional ripgrep case skipped in WSL.
- `go test -tags=vivy_headless ./internal/rpc ./internal/studio ./internal/studiocore ./internal/eval ./internal/transition/studioledger/... -count=1 -timeout 15m`: all five owners PASS; cmd package has no tests.
- Real disposable PostgreSQL16, loopback55439, no tenant store:
  `just test-postgres` PASS,125 cases,zero skips. An initial copied-fixture
  assertion wrongly expected upgrade_required while its deliberate extra view
  remained; preflight correctly refused that unexplained view. The fixture now
  distinguishes clean intermediate source from deliberate drift. A broad
  test-text edit briefly changed baseline035's expectation; it was narrowed,
  the named conversion test passed, then the whole real gate passed.
- `just d02-web-ci`: PASS,typecheck,102 Web cases and2 actual selected-UI/lite
  artifact cases,zero skips. `go test ./sdk/conformance` also passed its DTO tests.
- Actual strict23-Provider behavior reproduction: PASS,68.500s,without update flags or evidence mutation. The first attempt lacked ui/dist in untagged child tests; a real pnpm UI production build (5.89s) supplied the assets, then reproduction passed. No placeholders were used.
- Go formatting, unchanged stable001–037 SQL, canonical candidate source comparison and final staged diff checks: PASS.

Donor D06 already supplies full ten-Recipe/external fixture/native Studio and
independent review evidence; see its original verification. The combined gates
above revalidate changed shared owners. Broader release/full CI and native
Windows full CI were not run. The owner selects scoped WSL wiring/minimal gates.
