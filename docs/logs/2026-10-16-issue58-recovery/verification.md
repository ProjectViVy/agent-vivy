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

## Subsequent safer broad probe — FAIL / incomplete

Fixed source8ae1db9d was subsequently tested with a materially safer local-only
runner: cached dependencies only (`GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local`),
credential-bearing child environment names removed, and a loopback non-forwarding
HTTP/HTTPS deny proxy. Local fixture requests remain available; external requests
are rejected rather than forwarded. This is distinct from the original rejected
action, whose BLOCKED record above is retained. No rejection bypass was used.

The same package command exited1. The proxy rejected2 external requests and
forwarded none. Raw output and runner metadata are preserved in
[evidence manifest](evidence/s2-localonly-full-manifest.json).

- FAIL: runtime `TestModelCallPayloadSchemas/actual_v3_request_validates` and
  `TestServiceRunHappyPath` expected event version3; production correctly emitted4.
  Test-only repair c308029e asserts strict v4 provenance and retains historicalv3.
  Its focused passing result is not a rerun of this broad command.
- FAIL: app `TestCodeFaceJSONModeContinuesNewestSession` changed the ACTMEM root
  on its second startup while retaining the same Journal. Independent read-only
  diagnosis confirmed the existing owner-path binding correctly rejects it.
  Repair the fixture to reuse the organism root; do not weaken production binding.
- INCOMPLETE: the app package reached its180s aggregate limit during C05. C05
  had run25s and was waiting for the killed fixture's real30s SQLite lease;
  earlier crash cases each consumed about41s. This does not establish a C05
  deadlock, pass or product failure. Rerun crash recovery with its real lifecycle
  budget on a later fixed candidate. Do not delete leases or alter the clock.
- PASS in this probe: storage, migrations and SQLite packages. PostgreSQL DSN
  cases remain skipped and are not real database execution.

Full `just ci`, native Windows, actual model quality and installed #66 acceptance
remain open. Default and DIVA Recipe composition must be reported separately.

## S3 input recovery checkpoints

- Remote4a553700 / tree8e804182 (local2c428600): shared strict projection receipt
  codec and automatic evidence projection helpers; not Task1 acceptance.
- Remote773dc741 / treeebadeca (localec579296): trusted per-run output cap, applied
  after provider thinking options; local OpenAI/Claude HTTP fixtures exercise the
  exact requested cap and unchanged thinking. No physical model limit is invented.
- Remotef27dbe162731cda202afd0713b29ce53ac12f923 / treebea63855191e2538e0563f9a6b4c4c270a6a0fb9
  equals locale69e9d59 and includes the c308029e test-only repair. It checkpoints
  governed run input wiring and actual local-wire fixtures, while remaining
  restart/certificate/refusal/isolation work and independent review are pending.
  Mandatory reconciliation still fails before provider invocation until Task2.

The supported projection certificate and requested4096-token LIFE output reserve
are implementation refinements documented in the plan/design. Unknown provenance,
model window, non-text cost or unsupported protocol remains unavailable. Ordinary
Coding has no new cap default. Local HTTP wire evidence is not live model quality.

## Subsequent fixed-candidate evidence and interruption

- PASS: exact658fc30b targeted `TestMemoryLoopCrashC05EffectsDoneBeforeWatermark`,
  local-only/offline/credential-stripped runner,120s package budget,71.429s wall
  including build. No external requests. Actual lease wait remains unchanged.
- INTERRUPTED: a new25m package-budget probe at658fc30b lost its execution session
  after transport disconnect; no final exit/runner metadata exists. Its raw partial
  log confirms the old positive LIFE tool fixture failure, later repaired at3d.
  Partial passes do not certify app or full packages. See broad-658-interruption.json.
- Reviewed3d / remote4c4e605a, treee7e37350: independent runtime^Issue58 PASS7.002s;
  provider FAIL on nil-tool fixture. Three Important source findings and that Minor
  fixture finding are recorded in reviews/s3-input-review.md.
- Fixf8 / remotef8cb53ba10367523772f7976fe88630117ef9140, treeaae24648: root exact
  six-group local-only regression PASS21 events, exit0, external0. Fixes include
  missing-pin first/next-leg/resume denial, LIFE FaceCode Frozen Core and worker
  isolation; no independent second review or broader pass is inferred.
- FAIL: authorf8 prescribed focused gate,146 pass events/5 fail events (three
  positive app cases plus parent/package): ContinuityOnlyComposition eligible
  request, ExactTextInteractionSealsThroughServiceRun, FormattedGrantAuthorizedRead.
  Test-only real C1/capacity fixture repair is pending; assertions are preserved.

All S3 attempted logs at this snapshot, including RED and compile failures, are
losslessly retained in s3-input-attempts-pre-acceptance.tar.gz with its manifest.
Root/reviewer raw logs, the interrupted broad output and the non-forwarding runner
source are retained under s3-root-evidence-manifest.json. These are evidence, not
Task1 acceptance. Focused/race gates on the repaired candidate are still required.
