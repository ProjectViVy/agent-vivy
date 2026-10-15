# Recovery verification

Current cut: S3 Task1 input preparation accepted at96adc4f7 (remote exact-tree db474a19). Normal and race each205 passing test/subtest events; fourteen PostgreSQL DSN cases skipped. S3 Task2 is in progress; S4–S6 remain open. Earlier sections below are chronological evidence, not the latest acceptance. See [S3 input acceptance](s3-input-acceptance.md).

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

## Accepted Task1 final gates — isolated96adc4f7

| Gate | Actual result | Qualification |
|---|---|---|
| `go test -json ./internal/storage/... ./internal/runtime ./internal/app ./internal/provider -run '^TestIssue58' -count=1` | PASS205 test/subtest events, FAIL0, SKIP17, exit0;27.891s | 14 genuine PostgreSQL no-DSN skips;3 packages with no selected tests. Not real PostgreSQL execution |
| Same selection with `-race` | PASS205 test/subtest events, FAIL0, SKIP17, exit0;180.340s | Exact same immutable96/treeef33; no races; same skip qualification |
| Wider `Test(Issue58|.*Persona|.*Child|ToolSearchActivatesDeferredTool)` runtime/app compatibility | PASS200 test/subtest events plus2 package passes, FAIL0/SKIP0;95.440s | Exact1cc19c91. Later96 only refines replay proof/diagnostic validation; do not call this a96 rerun |
| Root amended replay/legacy/codec/native-empty diagnostic selection | PASS, exit0;3.196s | Exact96, own local-only runner, raw metadata preserved |

Every final runner reports zero external requests rejected/forwarded; dependencies
were offline and credential-bearing child env names removed. Built UI fixture was
copied to the isolated worktree only for Go embedding; this is not new UI acceptance.

Earlier1cc race FAILed from disk exhaustion: SQLite migration34 could not write and
runtime could not compile, so runtime race was NOT RUN. Its log is preserved. Root
reclaimed13.848GB of reproducible Go cache entries older4h (strict hex -a/-d files);
module sources/repositories/evidence were untouched. The later exact96 rerun, not
that failed attempt, supplies the race pass.

Source manifest32 files SHA256ab4af6bcee375f70e47711dcca4335fb38b86737c4faaac25f7d162919b4a48b.
See s3-task1-complete-manifest.json and s3-task1-additional-manifest.json for all
lossless logs, HTTP fixtures, source hashes, disk failure and root regression proof.

Remaining: Task2 continuous native fold/reconcile/CAS/positive receipt/restart, S4
management actions/tools/UI, S5 BML capability/single-sink/legacy exit, and S6 fixed
product candidate/justci/UI/Windows/live-model/#66. No whole S3/Issue pass is claimed.

## Intermediate native Windows CI — not a Task2 acceptance cut

GitHub Actions run132 / 38797712766 checks remoteed9d20d22f818fd05c310948160da71c1a2668f7
(exact local1a4cd31d tree796f5883) on windows-latest. UI CI and existing full-UI
browser smoke completed successfully; backend CI was still running at observation.
The fresh-clone setup and independent cold dev startup steps also passed. See
evidence/windows-ci132-intermediate.json and the linked workflow for actual status.
This WIP predates the uncommitted native Recap observer/CAS fixes, includes no new
Recap management UI, and is not whole Windows/product acceptance. Earlier NOT RUN
entries remain chronological; a final integrated source still needs its own gate.

## Native Task2 independent review — changes required

Fixed sourceb26ee856/tree982f37f5 (remoteca99c2f1) received independent review:
two Important findings, zero Critical/Minor. Actual captured correction-only tool
fragments form invalid live tool exchanges; explicit native summaries are outside
Service cancel/drain/model-switch fences. Three reviewer overlay REDs reproduce
those boundaries while the immutable candidate remains untouched. The review's
focused selection passes45 test/subtest cases but does not negate its RED probes.
See reviews/s3-native-review.md and evidence/s3-native-review-manifest.json.

Fixture-onlyf372626d/remotef34c8873 adds the existing BlobStore capability to an old
positive runtime fixture. Full Issue58 focused remains RED on three positive App
cases sharing two more missing-BlobStore fixture constructors; production App
already supplies it. Its required race selection passes separately. A consolidated
TDD fix/verification pass is underway; no native Task2 or combined S3 acceptance
is claimed. Working-tree incremental greens are not fixed-candidate passes.

## Native Task2 consolidated fixes — scoped interface accepted at3e13019b

Local3e13019b0373d8f6344bb84469fd72598f4c11ab and published
remotecdc2e16cd8f00ee06ced6ba46c4baffca5fbdeff share exact tree
f482293e05f14a955f7d0f6b5ec6644b973af3fe. Original unavailable local code and
its tests are not reused. Independent review atb26 found two Important issues;
one consolidated TDD pass fixes historical correction wire representation and
explicit summary cancellation/drain/model/engine fences. It also repairs the two
App fixture omissions without weakening mandatory capability checks. Synthetic
Journal payloads preserve JSON objects via RawMessage instead of base64 strings.

| Fixed-source gate | Actual result | Qualification |
|---|---|---|
| All runtime/App `^TestIssue58`, normal,35m timeout | PASS228 tests/subtests plus2 package events;0fail/skip,29.579s | Exact isolated3e130; existing unchanged UI embed prerequisite, not new UI acceptance |
| Same complete Issue58 selection, `-race` | PASS228 tests/subtests plus2 package events;0fail/skip/race,269.214s | Exact isolated3e130; external rejected0/forwarded0 |
| Root rerun of three original unchanged reviewer probes with test-only overlay | PASS3 tests plus1 package;0fail/skip,16.962s | Exact isolated3e130; original probe tail SHA7fa75d58abe2c658af6b94098109caf8f3014ab2f40b7889693366e9f6e0298f; no second independent review claimed |
| Broader default runtime/App,35m timeout, separate MemoryLoop gate | RUNNING | Not acceptance until exit metadata exists; parent owns result |
| Full `just ci`, final native Windows backend, new Recap UI | NOT RUN at this cut | Must qualify integrated S4/S5 candidate |
| PostgreSQL and real model quality | NOT RUN | No DSN; local HTTP fixture capture is not real-model quality |

All fixed-source runners remove credential-bearing child variables by name, use
offline dependencies and a non-forwarding external deny proxy; actual local HTTP
fixtures remain available. Each completed fixed run reports external rejected0 and
forwarded0. Original evidence remains retained on failure or uncertainty.

The earlier f372 broad aggregate FAILed after605.156s: runtime package passed, App
reached default600s package timeout (the named MemoryLoop test had only9s active).
1436 passing test events and5 skips are partial evidence, not whole-package success
or a demonstrated MemoryLoop deadlock. External rejected2/forwarded0. The new broader
run follows repository35m/default-skip-MemoryLoop policy and does not rewrite this
failed result. Repository DIVA/MemoryLoop and integrated justci remain separate.

Completed author attempts (including RED/compile failures and dirty-source greens),
source hashes, root probe overlay/provenance and safe runner are retained losslessly
in evidence/s3-native-completed-attempts.tar.gz, SHA256
d90529202c5a4e4ffe9a9844fdcf1e9590f2575c719ffdb4a9554a7f92ee81c8;
151 files indexed by s3-native-completed-manifest.json. The running broader log is
excluded until completion. The independent review archive remains unchanged.

This closes the two independent findings for the scoped native runtime interface
and releases the serialized S4 source lane. It is not whole S3/Issue acceptance.
S4 management/UI, S5 exact source expansion/BML/single-sink/legacy exit and S6 gates
remain. ContextStatus and CompactSession zero totals are not measured unified LIFE
pressure; rowless summary usage is not yet included in RunStore-joined usage UI.

## Native Task2 broader gate completion and DIVA environment cut

Exact3e130 broader default runtime/App (`-skip '^TestMemoryLoop' -timeout35m
-count=1 -json`) PASSes1501 test/subtest events plus2 package events,0fail;
3 conditional skips (CognitiveActionFixtureCapture, RealProviderSmoke,
ReportFeedbackSnapshot),68.197s. Safe runner rejected2 external proxy requests and
forwarded0; no external endpoint accepted payload. This supersedes only the RUNNING
state of that run above, not the earlier f372 timeout or any final integrated gate.

The first repository-supported MemoryLoop script attempt failed before tests:
Go1.26.4 vcsGit.RootNames recognizes a .git directory, not this worktree's .git
file, then tries unusable parent VCS metadata. `go list` therefore exits128 during
VCS stamping. Plain Git correctly resolves the worktree and fixed HEAD. The raw
failure/diagnostic is preserved without disabling stamping or changing production
assertions. Root created a separate local shared clone with a real .git directory,
checked out the same3e130, and restarted the unmodified script there. That second
run is RUNNING, not yet a DIVA/MemoryLoop pass. Generated UI embedding prerequisite
is the same unchanged existing build; this is not new UI acceptance.

Completed broad/environment raw files and runner metadata are losslessly archived
in evidence/s3-native-broad-and-environment.tar.gz with its SHA/file manifest. No
running MemoryLoop log is represented as complete evidence. Final product/DIVA,
new UI, real PostgreSQL/live quality and installed #66 gates remain qualified.

## Native Task2 DIVA/MemoryLoop script completion

Exact3e130 in clean local shared clone runs the unmodified repository-supported
`scripts/test-memory-loop.ps1` with offline dependencies and GOFLAGS=-p=1: PASS,
exit0,786.032s. Real DIVA Recipe pack/inspect and SDK artifact checks pass; the
App overlay package passes in766.065s. Earlier same-clone non-App passing package
results are valid same-source Go cache reuse. This script prints aggregate package
results without individual test/skip counts, so no invented zero-skip or50-test
pass count is claimed. Safe runner rejected1 external request and forwarded0.

Generation6431f296012517cb22a1716e6520b8e46102dff0b3578a1f03eab792a34111f0,
Recipe digestb44092c97ee9a05a2c19bff256967f0faa84bfa17b6c10ea6bc571f6ec927c17.
The script removes its temporary artifact normally; this is its real packed-body
regression gate, not the later installed S6 artifact/binary freeze or live quality.
The clone's first attempt failed SDK pack from missing offline UI dependencies;
root copied existing installed dependencies into that clone and reran unchanged.
Both completed attempts and metadata are archived in
s3-native-memoryloop-completed.tar.gz with its manifest. The earlier detached VCS
failure remains separately preserved. `test-diva-recall.ps1` is RUNNING separately.

## S4 backend checkpoint — implementation present, review/UI pending

Local94bff8014525a22ed647b23453d3b57fb9115f12 and
remote69096de73cb3b2f4618b53909a721486ee07d8a9 share exact tree
7678864b43badef22a603c38b2d67e67303003d2. Same bound C1 now serves protected
Recap actions and deferred tools. Request-scoped discovery delegates to official
Eino search over eligible tools; actual activation/invocation reauthorizes. A real
restart RED exposed native tool_search bypassing durable tool-operation evidence;
the thin adapter now records complete actual args/results through the existing
coordinator, without relaxing C2 seal. Raw malformed UTF8/surrogate/duplicate/null
inputs reject before decoder replacement. Shared ContextStatus explicitly reports
measurement unavailable instead of old session zero-token pressure.

Root reran the author backend regression selection in an isolated exact94 worktree:
317 test/subtest passes plus5 package passes,0fail,1 optional fixture-capture SKIP,
exit0,58.854s; external rejected0/forwarded0. This supplies fixed-candidate evidence;
author incremental RED/compile/FAIL/GREEN logs remain precommit source iterations.
Complete fixed Issue58 race is RUNNING. UI, independent S4 review and wider SDK/CI
are still pending; sources and loaded revision remain explicit unavailable until
S5, and ordinary usage UI omits rowless summaries. Do not call whole S4 complete.
54 completed backend/evidence files are archived with the S4 backend manifest;
source/review milestones stay separate from eventual S6 centralized acceptance.

The pinned Playwright public browser dependency install failed: repeated official
ChromeHeadless151/v1234 downloads returned empty/non-zip archives. No local browser
was installed, no dependency was downgraded, and browser E2E is NOT RUN. Install log
and metadata are preserved in the S4 backend archive; React/typecheck/build or old
Windows fixture success cannot substitute for new Recap browser acceptance.

## DIVA recall and old Windows terminal disposition

Exact3e130 separately runs unmodified `scripts/test-diva-recall.ps1`: PASS,
exit0,113.058s. Its JSON validator requires each of the6 named integration tests
to pass exactly once and rejects every skip. The SDK-generated selected DIVA
assembly is used; fixture results are not installed/live-model quality evidence.
Safe runner rejected2 external requests, forwarded0. Raw log/metadata and manifest
are retained in evidence/s3-native-diva-recall.tar.gz. Full final justci remains open.

The earlier intermediate Windows run132 is now terminal: backend CANCELLED,
UI CI PASS, existing full-UI browser smoke PASS; aggregate job named just-ci FAIL
because it requires all3 successful lanes. It did not run a local `just ci` command.
This is the oldered9d20d2/tree796f5883 WIP, not new Recap UI or final backend
acceptance. See evidence/windows-ci132-terminal.json for exact job IDs/times.
Main was re-fetched again and remains5715950; no newer main source was overwritten.

## Fixed S4 backend race completion

Exact local94bff801/tree7678864b completes root's full Issue58 race selection
across cognitive module/runtime/App/ActionHost/config: PASS255 test/subtest
events plus5 package passes,0fail/0skip, exit0,344.460s; no race diagnostics,
external rejected0/forwarded0. This completes the earlier RUNNING race state and
does not qualify the subsequently edited UI source or independent review. Raw
log/runner metadata and parsed package/event results are losslessly archived in
s4-backend-fixed-race.tar.gz with its manifest. Whole S4 and S6 remain open.

## Fixed S4 UI checkpoint — independent review running

UI source168393b0a70d2406de01c2307091c3c4c67fce3c, exact tree
372d9ff3f2e66eadb60310877616a83107d5d3d0, is published as tree-equivalent
remote0a762f25a75bfee1a7cbb3ad7bcda346e6f18f2e. Root prepared a separate
clean local clone and reran normal pnpm hooks, not just author working-source
checks:707 tests/93 files PASS, typecheck PASS, build PASS, final git diff clean.
Phase times19.982s/12.112s/8.487s/0.143s; whole safe runner exit0 in40.807s,
external rejected0/forwarded0. Existing large-chunk build advisory is retained.
The dedicated Playwright config/spec collects one browser test; browser execution
remains NOT RUN because the supported browser dependency could not be installed.

The actual split HTTP/WebSocket smoke uses only opt-in Go test code, a temporary
root and a trusted fixture resolver; it proves real Vite3015 proxy -> authenticated
ActionHost -> C1 read/edit/conflict/history/source-unavailable/clear/restore and
ordinary-session refusal. It explicitly records browser_rendered=false. Source
continues to delegate installed singleton/sidebar production to #66. No production
test endpoint, activation option or backend mock was added to the shipped UI.

All author attempts, including missing-component/concurrency REDs, type/compile
failures, partial/interrupted runs and actual smoke scripts are losslessly retained
with root's fixed-source raw results and18-file source manifest in
s4-ui-completed-attempts.tar.gz and its SHA manifest. The copied implementation
report includes supported reproduction commands. Full S4 independent review is
running on fixed168 in another isolated clean clone. Its findings/probes are not
replaced by these greens, and no S4 whole-Story or S6 product acceptance is claimed.

## S4 independent review: actual App integration blocker

Fixed candidate168393b0/tree372d9ff3 has one independently reproduced Important
inherited blocker: actual NewWithAssembly omits ServiceDeps.ToolEvidence, so the
trusted resident main conversation fails recap_unavailable before its first HTTP
request. The final probe uses catalog deepseek-flash, a known1,000,000-token window,
and successful current authorization/ContextStatus prechecks. Subsequent edit,
C2 seal and host reopen assertions were not reached. S3's earlier implementation
report claiming production App already wires the BlobStore is incorrect. Its
scoped injected-fixture/interface greens remain historical evidence; they never
established actual App admission or whole S3 acceptance.

Independent S4 review found no other substantiated finding: focused Go34 pass
events/one opt-in fixture skip; runtime probes6 pass events; ordinary discovery/
lifecycle8 passes; normal staged focused UI22 passes and independent async UI3
passes. These do not override the assembled App RED. Raw probes, all attempts,
source integrity and34 verified file hashes are losslessly archived in
s4-independent-review.tar.gz. See s4-independent-review.md. One consolidated author
TDD fix is in progress; root must rerun the unchanged original assembly probe and
affected fixed-candidate gates. S4 remains unaccepted; source for S5 remains frozen.

Exact3e130 unmodified MemoryLoop and DIVA recall scripts have since completed PASS;
the earlier RUNNING statements above are chronological history. No installed/live
quality or final justci claim follows. Main re-fetch still resolves5715950.

## S5 dependency preflight (read-only)

Vivy main remains5715950. Laputa main re-fetch resolves
f44ba00eb6ab5aa6ab9a2f5c09868373e36679f5; its three main-only commits since
common4b2bec2 contain policy documentation/AGENTS alignment, no Go source change.
Reviewed feature pinbb57 remains checked out, clean and unchanged while S4 tests
run. A new native compatibility branch must preserve both histories and apply
current policy instructions before fixing the actual legacy queue/v3 gap.
No unpublished sibling implementation or new dependency pin is active here.

Read-only native inspection confirms queued legacy activity can retry/fence after
v3 activation; archive-only native proof currently cannot be read through
LookupCaptured once the head is v3. The minimal producer change remains pending.
Any policy closure must positively verify the trusted v3 owner, preserve original
unknown effect/Before/source evidence, and never advance C1 coverage or fabricate
an applied/negative receipt. Logical ReadRecap may perform its existing same-byte
recovery publication; content/revision/coverage immutability is the correct test
boundary, not a claim that no filesystem operation occurs.

## S4 assembled App fix: fixed-candidate verification

Local7ab14d5dd0653c5dd7f0498013f2f61021d5add2/tree
5d4e0a7997d0dda654aecf1ca37bd51e8e00e03e is published as remote
4c7a9ebced190be74175c87de418e86394632b19, with the exact remote tree verified.
Production change is one dependency injection to the existing backend.Blobs();
no pending/evidence/seal guard changed. New actual Assembly regression exercises
known-capacity authenticated admission, deferred search, same-owner native edit,
exact argument/full-result blobs, one C2 seal and actual App close/reopen followed
by successful next admission and the next seal. Author RED and two intermediate
fixture failures are preserved separately from the final author GREEN.

Root reran original unchanged independent App/runtime probes in a separate clean
clone of exact7ab14d5: PASS7 test/subtest events plus2 package passes,0fail/skip,
exit0,37.729s; external rejected0/forwarded0. Only the overlay target paths changed;
original App probe SHA256f002f3d21d5ef5a5bee96511aa3c17d36539dd0c846bf847b7a7e800b26458ae
was checked before execution. Previously unexecuted real App HTTP/seal/reopen
assertions now pass. This resolves the original reproduced integration RED.

Root's same fixed candidate affected five-package normal selection PASS318
test/subtest events plus5 packages,0fail,2explicit opt-in skips
(TestCognitiveActionFixtureCapture and TestIssue58RecapSplitFixture), exit0,
36.374s; external rejected0/forwarded0. Full Issue58 race is RUNNING separately.
These qualified greens do not accept S5 sources/loaded status/BML or S6 product
behavior, browser execution, actual PostgreSQL or installed #66.

### Fixed S4 race and scoped acceptance

The root exact7ab14d5 full Issue58 race has completed PASS256 test/subtest events
plus5 packages,0fail,1explicit opt-in SplitFixture SKIP, no race diagnostics,
exit0,333.276s; external rejected0/forwarded0. Config has no matching race tests;
its package success is not invented test coverage. Source/normal/original-probe/
race raw logs, commands, full metadata and source hashes are archived in
s4-app-fix-root-fixed.tar.gz. This supersedes RUNNING immediately above.

Independent S4 review plus one consolidated TDD fix and root unchanged original
probes accept the scoped management/native-tool/App integration interface at7ab.
The actual missing production dependency is resolved, not suppressed. S4 Story
and product acceptance still await S5 exact sources/loaded composition and S6
browser/full CI/installed quality. S5 source release is now bounded to the native
legacy-consumer compatibility producer first; no BML consumer is activated before
that producer's independent review/publication/new pin.

## S5 native compatibility producer: published WIP, independent review in progress

Native local3c365207d295221c4a3c5667fc71a6a465ee59f8 / remote
1edf7b2379d7887a016c57b78e2060ad9dae1588 share exact tree
1ca98be9a8cffb18d242bf3c1aa40a613d0bbb7f. Policy-documentation merge
local1ce1c1e / remote9b25b0a9 shares treea6d45151 with verified ordered bb57+f44
parents; its71 changed blobs equal existing main objects and no Go source changed.
The new producer closes only verified v3-incompatible legacy consumers, preserving
unknown effect metadata and actual old positive archive receipts. One strict
semantic predicate covers restart, retry, watermark and archive from ordered SQL
snapshots; capsule parsing is reused with explicit strict v3 errors.

Author working-source final normal/race each281 passing tests plus4 packages,
0fail/skip/race, no external requests. Root independently checked all12 source
hashes against committed blobs. All REDs, setup failures, intermediate broad error
assertion and strengthened malformed-archive RED/GREEN are archived without
relabeling merge-HEAD working-source logs as fixed-candidate execution. Independent
Astra review is running on a clean fixed3c clone; its findings/fixed repro gates
are not replaced by author greens. Native Windows is NOT RUN. Vivy still pinsbb57;
no BML schema/adapter/consumer, new module sum or selected activation is claimed.

## Native producer independent review: two Important blockers

Fixed native3c36520/tree1ca98be9 has0Critical/2Important/0Minor findings.
I1: missing or null before.revision is silently decoded as zero; real archived
native revision3 can be reported applied revision1, clearing uncertain source/error.
Complete explicit zero and nonzero controls pass. I2: the newly reused strict JSON
helper uses Decoder.More instead of proving EOF, so trailing } or ] is accepted
by projection/Start/archive/watermark. Both original RED suites are preserved.
Independent focused Garden36+ACTMEM14 tests/3 packages pass; no other substantiated
finding. These greens do not override either RED. All19 artifact hashes verified;
27 supporting/increment source hashes match fixed committed HEAD. Candidate stays
unaccepted. One consolidated author native TDD fix is released; root will rerun
unchanged probes and affected fixed normal/race before publication/pin activation.
Vivy still pinsbb57 and no BML consumer is active. See independent report/archive.

## Native consolidated fix and exact dependency publication

Native fixeda99c25feea48e151c847cea81015bec0fdfe80bf/tree
9fff8cf9d5ca174a1af90e43a0e718281a24635e is published as remote
249a756e04c5f28af781e4dadea9e9086689bde0, exact tree verified. One local
complete-document guard and presence/non-null check reject malformed uncertain
snapshot metadata before any write; legitimate explicit zero/nonzero still recover
true native receipts. Original raw DB JSON and C1/archive bytes remain unchanged
on all six refusal paths. Shared evolution helper is unchanged.

Root runs original unchanged reviewer probes on isolated fixeda99: PASS8 tests
plus1package,0fail/skip,2.731s. Root fixed Garden normal/race each181tests+3packages
PASS9.715s/44.471s; ACTMEM normal/race each120+1 PASS0.717s/3.877s. Each lane totals
301tests+4packages with0fail/skip/race. All local-only runs rejected0/forwarded0.
Original probe hashes/overlays, raw fixed results and author RED8pass/12fail plus
GREEN20 are retained in separate archives. This accepts the native producer
interface after independent review and one consolidated TDD pass, not whole S5.

Normal git fetch of the actual published249 succeeds in5.392s. A minimal external
modfile with no replace resolves all3 new published modules using ordinary go mod
download, official Go proxy/direct and sum.golang.org: PASS80.976s. All3 signed
sumdb cache notes and actual Sum/GoModSum are retained. Every10 changed Go files
inside normal module-cache sources matches its published249 Git blob byte-for-byte.
No manually fabricated module zip or checksum was used.

Vivy pin candidatefdcb25a554df79c7938287b89a539c905804ca63, tree471a603a,
remote1d0e1268 exact verified, updates only go.mod/go.sum/source lock to
v0.0.0-20261015222145-249a756e04c5. Actual sibling HEAD is now published249, clean;
local source branch/a99 and all history are preserved. Main re-fetch remains5715950.
Root isolated fixed-fdcb affected consumer selection PASS318tests+5packages,
2explicit opt-in fixture skips,0fail,53.722s, rejected0/forwarded0. Consumer race is
RUNNING separately. No current/full UI, native Windows, BML/loaded/source-consumer
or S6 product acceptance is inferred from dependency publication or these greens.

## Fixed dependency-pin consumer race completion

At fixed Vivy fdcb25a554df79c7938287b89a539c905804ca63/tree471a603a,
with actual native sibling published249a756e/tree9fff8cf9, root isolated race
selection PASS256 tests+5packages,1 explicit opt-in SplitFixture skip,0fail,
339.059s and0race warnings. Normal selection PASS318+5packages,2opt-in skips,
53.722s. Both runners rejected0/forwarded0 external requests. This completes
only the dependency-pin consumer RUNNING state above. Full raw outputs and exact
runner commands are archived in evidence/s5-native-pin-consumer.tar.gz and its
manifest; no BML activation or whole Story/product acceptance is inferred.

Native BML schema/ownership/atomic receipt implementation is now the sole source
writer. Its SQLite1.56 dependency-cache setup failure is retained as a setup
failure, not semantic RED; official unchanged-version cache preparation passed
before offline retry. App/Garden consumer activation remains gated on independent
native BML review and any consolidated TDD fix.

## Intermediate actual Windows UI gate failure

Run38817922217 at branch head9a1a21d4 checks out GitHub PR merge008aa27b
into main5715950. UI job116707597725 FAILS: existing cross-face classification
omits chatInput.contextUnavailable and chatInput.compactUnmeasured. Its707 Vitest
tests/93files and Vite build pass; these do not make the job green. Root reproduced
the original assertion unchanged at fixedfdcb25a5: exit1/0.365s, no external
requests. Minimal existing faceSpecific.webKeys classification fix is queued after
the single native BML writer releases its index. No validator relaxation. Original
Windows decoded log and local RED are archived with SHA256 manifest under
evidence/s5-intermediate-windows-ui.*. Existing full-UI browser smoke passed in
its distinct job; new Recap E2E was not invoked. Backend was still running at
observation. This intermediate run does not qualify a final candidate.

## Intermediate actual Windows backend failure

Same run38817922217 backend job116707597299 has now completed FAIL. Many
Issue58 App/runtime/SQLite fixtures reject at storage subject binding validation;
the original plain decoded log and complete failed-test-line list are archived
under evidence/s5-intermediate-windows-backend.* with SHA256 manifest. The
actual real-Assembly HTTP regression proceeds separately in the log; this is
not a reason to ignore fixture failures. A separate read-only diagnosis traces
platform path handling and trusted fixture versus actual resolved identity in
fixedfdcb; no production/fixture change is authorized concurrently with the sole
BML native writer. No root cause or Windows pass is claimed pending that evidence.
The UI classification failure remains a separate known defect. The workflow
aggregate is not a substitute for either failed lane.

## Native BML producer checkpoint and independent review in progress

BML source92048bccda52875a798ade1bccc4e44823a564fb/treea9532ce4 is
published as5770f07ffbe4fd0b5701678846b97d4e6557eb68, exact tree verified.
Only19BML files change. Root verified63author artifacts, all19committed source
hashes and14tested Go hashes. Author final nativeSQLite1.56 normal/race and root
MVS1.59 normal each155tests+1package PASS; execution preceded commit and is
byte-bound, not mislabeled fixed-HEAD execution. Vet passes. All26raw attempts,
RED/setup/compile/fault evidence are retained in s5-bml-native-author archive.

Independent review on a clean fixed92048 clone has substantiated cross-handle
schema1 readonly upgrade leakage and single-target tombstone CAS additional
suppression REDs; review is still ongoing, no final finding count or acceptance.
The latter reuses legacy multi-supersedes but the new bound single-target CAS
must reject additional deletion targets. Root accepts that minimum boundary,
without a new bulk API. No BML consumer is active; one consolidated author TDD
pass is released only after complete independent report.

## CI portability, explicit inventory and Web classification checkpoint

Actual Windows root failures were traced to the shared conformance fixture's
Unix-only /trusted/owner path, plus the independently stale Generation inventory.
Root fetched the actual CI merge's three identity/SQL source files and confirmed
byte equality with fixedfdcb; proof is archived. Production binding/permission
guards remain unchanged. Root replaces only test-owned path construction with
filepath.Join+typed JSON/Valid check, adds the two existing Web classification
keys, and explicitly declares reviewed Recap module/five tools/seven actions in
the existing complete inventory fixture. Test assertions/validators unchanged.

Source7f10ec6649eaae1e51aa15c4ae04ee12cec6a743/tree6d70b711 is published
as be19b94f8ca633fbf9f51c327023a652aeffd83a, exact tree verified. Original
Windows failures and unchanged fixedfdcb local crossface/inventory RED remain.
Root fixed7f10 App/runtime/SQLite selection PASS269tests+3packages,1opt-in
SplitFixture skip,0fail/34.767s. Fixed crossface/completeness/eight unchanged
validator tests PASS; all local-only requests rejected0/forwarded0. Raw working
and fixed attempts are separately retained in s5-ci-fixture-fix archive. Actual
Windows rerun is pending. Neither this correction nor Linux greens accept BML
or whole S5/S6; new Recap browser/native dependency Windows/justci remain open.

## Native BML independent review complete: two Important blockers

Independent fixed92048 review is complete:0Critical/2Important/0Minor. I1 live
schema1 readonly handle across another supported upgrade discloses new bound
rows through legacy Get/List/Search/SearchVisible. I2 same-binding single-target
tombstone CAS can additionally suppress independently updated B through extra
Supersedes without B CAS; receipt names only A. Existing legacy multi-target
mechanism is inherited, but the new bound single-target boundary must reject it.

Independent focused99 plus cancellation1 tests PASS/2package results. The actual
receipt-INSERT cancellation probe preserves five tables/counters and absent
receipt, then permits exactly one retry. Both original finding probes remain
RED. Native1.56 independently confirmed; all5invocations offline/rejected0/
forwarded0. Root verified23artifact hashes and25source/support hashes against
committed92048. Original probes/logs/overlays and report are preserved in
s5-bml-independent-review archive (SHA74714381a3839717c936f25c6f7f7920657706bd19fb605e5bfb35281aee0f6b).
One consolidated author TDD pass is now released. Root will relocate only overlay
target paths and rerun unchanged probes plus fixed native/root-MVS normal/race.
No redundant second review of the same fix diff and no consumer activation.
