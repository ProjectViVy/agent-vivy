# Issue 58 recovery checkpoint

This is a new recovery lane, not product acceptance. Repository access and the attached recovery record were confirmed on 2026-10-16 Asia/Shanghai. The original development environment is not assumed available.

## Source reconciliation

| Classification | Evidence | Disposition |
|---|---|---|
| Remote available | implementation branch `66fabfd27bd83d8de201268ccadc6a0d298d7d6a`; approved revision-3 design and S0–S6 plans; S1 owner/facade and S2 Tasks 1–3 source | Preserve and revalidate, not inherited GREEN |
| Latest main | `5715950ddfc87e05cf60e3d2a0ca54983bef05eb` | Integrated locally at `b91096cca2028a8e8b127f60d41da600d64bb7eb`; provider, thinking and persona updates retained |
| Historical only | accepted source `c113cac50b37e09b9439251bbc6d3320a22bc4a4`, checkpoint `858af73230fd6927fb93cdfe722d23face429fa7`, later S3 files and tests | Both commit fetches rejected as not-our-ref; neither named extant checkout has objects; no source or passing conclusion imported |
| Recovery remnant | cleanup staging contains original directory names | Copied before inspection; zero regular files and no `.git`; no recovered source |
| Reimplementation required | S2 Task 4 input provenance, strict persisted request decoder and grouped assistant derivation; S3 input projection/receipt/capacity and native fold adapter | Test-first rebuilding; historical reports are not acceptance |
| Not delivered | S4 management/UI, S5 selected BML projection and legacy exit, S6 candidate acceptance | Continue in existing plans |

The attachment `issue58-recovery-checkpoint.md` is a continuation record, not a backup. Its instruction to wait for the old mount is superseded by the owner's explicit current recovery instruction. No old mount, commit, review or GREEN is assumed.

## Current findings

1. `internal/storage/subjectsql/evidence.go` rejects Notebook-derived evidence wholesale; retain exact evidence and explicitly classify automatic input availability/exclusion. Unknown dominates exclusion.
2. `internal/runtime/subject_tool_evidence.go` saves full output but text replay returns a compact projection; S3 must prove the next real HTTP request receives the full authorized live output.
3. Existing `prepareRequest` history fallback/folding and budget path remain suitable only for ordinary Coding. LIFE requires fail-closed exact inputs and commit-before-retirement.
4. Invalid UTF-8 must be detected before JSON serialization can substitute bytes. Artifact UTF-8 checks after serialization do not establish exact original arguments.
5. #66 installed singleton producer is absent. Its trusted consumer seam exists; do not expand #58 into sidebar implementation. Installed LIFE acceptance remains blocked.

## Compatibility

Main migrations end at 043; recovery branch has paired044/045, so no current numbering collision. Go module declares Eino0.9.20 and Laputa bb57e93. Committed sibling replacements still apply; actual source SHA must be recorded, not inferred from the module version. No dependency version change is authorized by this checkpoint. Persona uninitialized unnamed admission must remain available; initialized Frozen Core remains authority.

## Execution ownership

One implementation writer, GPT-6.1 Sol High, rebuilds missing S2 provenance. Root owns integration, status and evidence; a separate read-only reviewer audited the baseline. No main merge, deployment, tenant migration or external messaging is authorized. Checkpoints go to `feat/issue-58-recovery-20261016`.

## Latest recovery progress

S2 provenance was reimplemented, independently reviewed and fixed at8ae1db9d;
focused producer/race gates pass without inheriting old local GREEN. Broader
regression failures and the safer-run boundary are recorded in verification.md.
Historical intermediate: S3 Task1 was in progress at locale69e9d59 (exact remote treebea63855), not accepted at that cut. The latest accepted cut is recorded below.
The initial findings above describe the recovered baseline, not current completion.

## Latest accepted source and next work

S3 Task1 was reimplemented and accepted at96adc4f7 / exact-tree remotedb474a19
(treeef33f382), after independent review, TDD repairs, actual local provider HTTP,
SQLite/owner reopen and native checkpoint/refusal replay tests. Exact isolated
normal/race gates each205 passing test events;14PG no-DSN skips are not execution.
The input acceptance/report contains full commands and retained failed attempts.
Native Task2 has since been reimplemented and its scoped runtime interface accepted
at3e13019b / exact-tree remotecdc2e16c (treef482293e). One independent review
found two Important issues; one consolidated TDD pass repaired historical tool
fragment representation and explicit summary cancellation/drain/model/engine
fences. Exact3e130 normal/race each228 tests pass without failure/skip/race; root
also reran the three original unchanged reviewer probes, all passing. Broader
default runtime/App passes1501 tests plus2 package events,3 intentional skips.
Raw completed attempts, source hashes and reviewer RED/fix evidence are archived.

S4 is now the sole implementation writer for same-owner actions, protected deferred
tools and management UI. S5 exact source expansion/BML/single-sink/legacy exit and
S6 integrated product gates remain. DIVA/MemoryLoop verification is running on a
clean local clone of exact3e130 after a detached-worktree VCS detection failure;
full justci/new UI/final native Windows/live-model/#66 installation are not accepted.
No PostgreSQL DSN means no real PostgreSQL execution. Scoped S3 interface acceptance
does not establish whole S3 or centralized product acceptance.


Historical Task2 WIP checkpoint local340bc959 / remoteee053ead (exact tree1eb88a3f)
preserves the first native adapter and pending-intent implementation. It is not an
accepted Task2 cut. Subsequent tests exposed real bound-port CAS error translation
and explicit-summary observation on a closed source run; both were repaired and
reverified at3e130. The integration rulings in S3.md bind pending recovery and synthetic control
Journal use without introducing another owner or fake conversation lifecycle.

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

## Current scoped handoff after S4 fix

Fixed7ab14d5/tree5d4e0a7 (remote4c7a9ebc) resolves the independently reproduced
actual App ToolEvidence omission. Root unchanged original App/runtime probes PASS7
plus2 packages; normal PASS318 plus5 packages/2opt-in skips; full Issue58 race
PASS256 plus5 packages/1opt-in split-fixture skip, no failures/races. See the
fixed-source manifest, original RED and unchanged-probe archive. S4's scoped
interface is accepted for S5 implementation, not whole Story or product acceptance.
UI707-test/typecheck/build evidence remains explicitly fixed168; no later full
candidate UI/browser execution is invented. S5 exact historical source expansion,
loaded status, BML native receipts/selection/single sink and legacy exit remain.
S6 justci/one-candidate native Windows/browser/installed/live quality and PostgreSQL
execution remain unaccepted. The first released S5 source increment is the minimal
native C5 queue/v3 compatibility fix; its producer review/publication/pin precede
all BML consumer activation. No main merge, deployment or tenant migration.

## Required CI wiring checkpoint

Source2e6273d8 / remotef1d65162 / tree89dbbe29 adds nested BML, pinned native
Windows and the existing Recap browser E2E as required gates, with immutable
artifact uploads. Root fixed config/BML-task/PowerShell17/collection1/strict-TS
checks pass; actual Windows/browser execution remains pending. See
[s6-ci-wiring.md](s6-ci-wiring.md). S5's inspected routing ruling now explicitly
preserves the old profile/raw route, selects one App-owned C2 projection adapter
and requires pre-read qualification of legacy recall/tools. No consumer
activation or installed DIVA acceptance is implied.

## Task1a checkpoint and actual Windows failures

Task1a f6bcae7d / remote3f547b58 / tree28c871b5 is pushed and root-fixed normal
380/race81/vet/nativeMVS162 pass, with documented opt-in skips and zero external
forwarding. [Fixed checkpoint](s5-task1a-fixed-checkpoint.md) records source seals,
all retained failures and pending independent review. Task1b/Task2 remain open.
[Actual Windows run and remediation](s6-windows-execution-and-remediation.md)
records UI707PASS, native failures/BUILDFAIL, new browser setup failure and backend
cancellation after a real runtime failure. No whole Windows/browser/CI GREEN.

## Task1a independent review correction

Source-only review at f6 is complete: 0Critical/1Important/0Minor. Root reproduced
I1 (negative expansion budget panic) on the exact candidate in1.324s with the
unchanged ordinary local native-record fixture. Task1a is changes-required until
the minimal bound check and fixed-candidate rerun. Broad prior greens are not
acceptance of that boundary. See s5-task1a-fixed-checkpoint.md and its separately
sealed original-RED and independent-review archives.
