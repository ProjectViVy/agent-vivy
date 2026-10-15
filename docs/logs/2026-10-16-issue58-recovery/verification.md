# Recovery verification

Candidate baseline: `b91096cca2028a8e8b127f60d41da600d64bb7eb`.

- PASS: authenticated connector read of Issue58 and git clone of repository; remote heads checked via `git ls-remote`.
- PASS: read specified revision3 design, index, S2/S3/S4 plans and supplied recovery attachment.
- PASS: `git diff --check` before local main integration; conflict resolution preserves main onboarding/avatar imports and removes obsolete Coding/LIFE placeholder import. The already-fixed Eino race board row is not resurrected.
- UNAVAILABLE: original c113cac/858af73 fetch returns not-our-ref; original cleanup remnant has zero files, no objects/source.
- NOT RUN: fresh Go baseline, provider wire, race, migration, UI and platform checks. Go1.26.4 installed; exact Laputa source bootstrap is in progress.
- NOT RUN: PostgreSQL real execution (DSN availability not yet checked), native Windows, real-model quality and installed #66 LIFE.

Old `/tmp` command logs are historical and are not counted as new verification. No implementation pass or S3 acceptance is claimed.

## Fresh fixed-baseline results

These runs use isolated worktree source candidate `b91096cca2028a8e8b127f60d41da600d64bb7eb`, exact sibling Laputa bb57e93, Go1.26.4, Eino0.9.20. They are not results for later S2 WIP source or a complete Issue58 candidate. Raw logs are compressed losslessly in [evidence](evidence/manifest.json), with source hashes and byte counts.

| Command | Actual result | Qualification |
|---|---|---|
| `go test -json ./internal/storage/... ./internal/runtime ./internal/app ./internal/rpc -run '^TestIssue58' -count=1` | PASS108 test/subtest events, FAIL0, SKIP13 | All13 skips are PostgreSQL tests without DSN; not real PostgreSQL execution |
| `go test -json ./actmem -count=1` in exact pinned Laputa module | PASS114 test/subtest events, FAIL0, SKIP0 | Linux owner source only; native Windows not run |
| `pnpm --dir ui build` | PASS, exit0 | Normal staging hook used; bundle size warning retained |
| `pnpm --dir ui typecheck` | PASS, exit0 | Baseline UI only |
| `pnpm --dir ui test` | PASS692 tests in92 files | Normal staging pre-hook; missing Recap UI is not implemented |
| `just bootstrap-test` | PASS11 tests, SKIP0 | Current source lock/dependency bootstrap behavior |
| `go test -json ./internal/app ./internal/provider ./internal/runtime ./internal/rpc -run 'Test(CustomProvider|StoredSelection|ResolveStored|ResolveCustom|ToolAdapterInfo|ControlThinking|Catalog)' -count=1` | PASS24 test/subtest events | Narrow main compatibility regression selection only |

NOT RUN: full `just ci`, candidate race, real PostgreSQL, installed LIFE/#66, real model quality, native Windows. Source hashes cannot turn skipped/unrun gates into passes.

## Published checkpoints

- `cf9dbe18e2b40f41324d40ca80b95d8da22b72ca`: recovery audit + main integration. Tree77f1cc77 equals local ab382bff.
- `dac4036c0bb80134be4330896f55075b427fcab2`: WIP S2 provenance. Tree2e438d8f equals local4e27122. NOT ACCEPTED: native checkpoint serialization, raw invalid-UTF8 producer path and remaining affected regressions were unresolved at this checkpoint. Later edits are not covered by its evidence.

CLI push has no HTTPS credential. Connected GitHub API publication creates exact source trees with explicit parentage; author and committer were confirmed as the responsible human. Remote commit IDs therefore differ from local metadata. No forced ref update is used; a lease rejects concurrent branch movement.

## S2 new-source gates and review

Producer source candidate `0f27910a` (remote tree-equivalent `6694dc15`) passed
`go test ./internal/runtime ./internal/app ./internal/storage/... -run '^TestIssue58|^TestNotebook|^Test.*Provenance' -count=1 -timeout=120s` and the same selected focused race gate as reported in its raw logs. These results do not close independent findings I1/I2. The [independent review](reviews/s2-provenance-review.md) requests changes.

BLOCKED, not test PASS/FAIL: the attempted broad command
`go test ./internal/runtime ./internal/app ./internal/storage/... -count=1 -timeout=180s`
was rejected by automatic approval review for HTTPS to an untrusted/non-approved
destination with unknown payload, advising approved/local dependency sources. The
command log is empty and its session no longer exists. No full/justci retry or
indirect bypass was attempted. Read-only inspection has not identified the exact
external destination; external-looking test URLs are not proof of an actual request.
Previously local focused validation is distinct from this rejected broad action.

Additional fixed-baseline main regressions: TestRegisteredCustomProviderRunsFromItsOwnEndpoint and all three TestModelThinking tests passed (4 tests, no skips). This closes the exact test-name gap in the earlier broad compatibility regex; it is not live-model acceptance.

## Accepted focused S2 producer cut

Source8ae1db9d / treea76e8d96 closes I1/I2 by one TDD fix pass. Exact commands and
all attempted logs, including initial compile/test failures and the empty blocked
broad log, are preserved losslessly in evidence/s2-reimplementation-logs.tar.gz.

- PASS: offline focused producer selection after fixes.
- PASS: offline focused race: runtime20.301s, app11.680s, storage1.025s, SQLite2.414s.
- PASS: root independent execution of TestIssue58InvalidUTF8StreamRetainsArgumentPrefixAndIdentity and TestIssue58InvalidArgumentsBlockObserverDelivery, with actual run events in evidence/s2-fix-verification.jsonl.gz.
- SKIP: PostgreSQL real DB cases, no DSN.
- BLOCKED: broad packages/justci after automatic rejection; no retry, no result.
- NOT RUN: S3 actual HTTP/capacity/fold/CAS/receipt, new Recap UI, installed #66, native Windows, real-model quality.

This accepts the focused producer interface for S3 implementation only. S2 full
regression and centralized product acceptance remain open. Existing data, migrations
and dependencies were not rewritten; no046 is introduced because existing fields
and Journal can represent the evidence. Any discovered persistence gap invalidates
that ruling and must be repaired before dependent activation.
