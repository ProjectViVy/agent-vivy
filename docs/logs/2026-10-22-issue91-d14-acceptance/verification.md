# D14 verification

Executable candidate: `5b0b572da80602f46d7496d004aa9d88fc6f5eeb`.
Laputa `9acc999301d5d575da2cea902a1c6d213763f381`; INOFY
`6f1397e417fc524ad266634247dd632e2ed93ec0`; Eino v0.9.21.
The closing record-only commit does not change this executable tree. Relative to `73ef094c`, the final candidate deletes exactly five obsolete independent Module test assertions and corrects two UI locale descriptions; Go/runtime behavior is unchanged, and the locale edits only clarify when saved credentials apply. Executions below retain their actual source points.

## Venues and final affected checks

Linux verification uses `/home/mastwet/work/vivy-issue91-acceptance/agent-vivy`,
an independent Linux-filesystem WSL checkout with its own pinned Laputa.
Go1.26.4, Node24.21.0, pnpm10.33; raw output remains in its
`.workspace/verification/`. Native verification uses
`C:/Users/Administrator/Desktop/morediva/newdiva/agent-vivy/.workspace/issue91-native/agent-vivy`,
with independent dependencies, cache and disposable home. No production Journal
or tenant workspace was accessed. Stable root source remains untouched.

| Final gate / real path | Candidate and observed result |
| --- | --- |
| `just wiring-ci` |09aad07d:16 Node +111 Go pass, zero skips. Raw final-wiring.log. |
| `just kernel-ci` |09aad07d:1964 pass plus actual minimal pack/Inspect/boot1. One existing optional TestBackendGrepRipgrepHonorsGitignore skip because Linux rg is unavailable. Raw final-kernel.log. |
| Broad control race |73ef094c:158 tests/subtests pass, zero failures/skips,384.640s. Selector covers approval, question, cancel, recovery, resume, pending, registration and orchestration. Raw final-control-race.jsonl. |
| Independent BML full race |73ef094c:121 pass, zero failures/skips,3.201s. Raw final-bml-race.jsonl. |
| Real PostgreSQL |73ef094c: `just test-postgres`:144 pass, zero skips on actual disposable Postgres16. Candidate/pin/clean checkout confirmed by doctor; container removed. Raw final-postgres.log. |
| Split3015 owner browser |73ef094c:6 pass/28.3s: original skill toggle, workflow draft/validate/publish, Work reload/leave, notes exact native tool continuation, cold Persona CAS/retry/save/reload, and live skill approve/stale refusal/deny. Raw browser-owner.log. |
| Rebuilt minimal-chat |73ef094c: actual SDK verify/pack/Inspect, same-ID UI staging, and3 browser cases pass/7.6s: protected list/read exact result/call continuation and narrow viewport, cancellation, provider failure. Actual Generation6935ac5d518f2680630466016483e7a1. Raw final-minimal-* and final-browser-minimal.log. |
| Native focused owner correction |73ef094c:34.475s pass; isolated own Linux focused race106.155s pass with no race reports. See d14-workflow-approval-registration/verification.md. |
| Independent final behavioral checks |Executed73ef094c:30 pass, zero failures/skips: independent actual-workflow GREEN1/0.276s, runtime26/18.184s, App3/0.451s. Exact clean73ef checkout, GOMAXPROCS2/GOFLAGS=-p=1/count1; final source-only behavior/residue reviews accept5b0b572d. See review.md. |
| Native full-product CI |5b0b572d: `just ci` completed successfully. UI has 86 files/685 tests pass; typecheck, production build, locale and semantic checks pass. Full root Go vet/tests, SDK/internal (573.161s), SDK/internal/assembly (10.099s), headless compile, and per-module plugin/Face vet/tests pass. Raw `.workspace/verification/final-ci.log` in the independent native checkout. |
| Native backend/SDK checkpoint |73ef094c: UI typecheck/build/i18n and685 cases, full App83.874s, RPC66.621s, runtime266.664s, SDK/internal600.323s and headless compile pass. Its final independent Module stage failed only the five obsolete Source.SHA256 test references; fixed at09aad07d. |

The root approval/question correction was driven by an independent deterministic
RED reproduction at 9214f200. Its owner then used real durable admission,
Service.drive, native Eino checkpoint creation and actual continuation in new
regressions. The change waits for existing private suspension registration
before the irreversible decision/answer CAS; it adds no runtime, checkpoint
format, Journal or LLM loop. Shell and native-orchestration owners retain their
existing routes. Timeout/cancellation preserve the first-writer slot when no
live continuation is available. See the owner log and independent review.

Eino capability check: pinned adk.TypedRunnerConfig.CheckPointStore,
adk.NewTypedRunner, TypedRunner.Run and ResumeWithParams already own native
suspension. Engine.Resume delegates to runner.ResumeWithParams. This repair
addresses Vivy's publication/registration ordering, not an upstream capability
gap requiring a replacement mechanism.

## Applicable prior executions

Evidence below retains its actual checkpoint. Final affected checks above and
the complete native gate cover later changes; unchanged storage, protocol,
generator and optional-owner evidence is carried rather than relabeled.

| Gate / real path | Observed result and applicability |
| --- | --- |
| Full `just race-ci` | 1b8da8cc: 1946 pass, no race, one optional rg-dependent skip. This is not a claim of a complete final-candidate race-ci run. Final registration paths and BML are separately rerun under race. |
| `just d02-ci` | 1b8da8cc: protocol/Face 175, causal App 1, shutdown 13 and standalone CODE 9 pass, zero skips. No later protocol change. |
| Optional cognitive/memory/masks/ActionHost/RPC | 1b8da8cc package suites pass; RPC 24.054s. f6 memory full race: 59 pass, zero skips. Final App/kernel gates cover current integration. |
| Real PostgreSQL | 441c4266 `just test-postgres`: actual disposable Postgres16, 144 pass, zero skips. SQL/storage owners unchanged by later warming, BML, Persona and registration fixes; container removed. |
| Web protocol/artifacts | 441c4266 `just d02-web-ci`: 106 Web cases and 2 actual selected-UI/lite artifact packs pass. Later changes do not alter this protocol. |
| UI/generation | Actual same-ID SDK staging twice is byte-identical; actual generator/locale output committed. 441c4266: 683 UI cases, build 5.09s; 1580 locale keys and 162 placeholders per locale plus 13 cross-Face units. Persona adds 2 regressions; final native CI repeats UI typecheck, all 685 cases and build. |
| BML pristine correction | Deterministic cancelled-initialization RED, then full BML 7.363s and vet pass. At f6 full BML race 121 pass, zero skips, 3.298s; final repeat recorded above. Unknown/schema1 still refuse unchanged. |
| Full App at f6 | 159 pass; two opt-in skips: TestCognitiveActionFixtureCapture (export off), TestRealProviderSmoke (no live credentials; human tiny-Face check is last). Final native CI repeats the full package. |
| Causal approval | f6 corrected actual provider-entry conformance passes under race, count5. Completed durable operation and exact call/name/result precede provider continuation; asynchronous display projections may lag under design 11.4. |
| Continuity and memory browser | f6 split3015: continuity 4 pass/27.6s; actual MEMRULES stale revision/refresh and BML creation 1 pass/5.2s. These owner contracts are unchanged. |
| Owner browser | 2364 split3015: all 6 pass/29.6s, including Persona cold load/CAS/retry/save/reload, skill approval/stale refusal/denial, workflow publication, Work, and notes exact tool continuation. Final repeat recorded above. |
| Masks selected Recipe | Actual packed/inspected/staged2364: all original 11 cases pass/59.2s. Committed5d readiness helper's two affected cases rerun: 2 pass/24.5s. Assertions/timeouts/product behavior unchanged. |
| Minimal chat Recipe | Actual packed/inspected/staged2364: 3 pass/6.3s, protected list/read exact continuation and narrow viewport, cancellation, provider failure. Final rebuilt artifact/browser repeat recorded above. |
| Native official Recipes | Complete SDK suite at2364 passes651.432s, conformance118.834s, assembly7.262s, including all 9 official executable Recipes, DIVA/shared and real minimal/hello-fs causality. Final native gate repeats its applicable suite. |
| Finite047 SQLite runtime bridge | Pass0.036s: source refused unchanged; copied target reopens twice, preserving complete binary workflow results, original channel SessionID/tuple, observer raw0007/version29. |
| Finite047 real Postgres runtime bridge | Same actual owner checks, pass0.756s; temporary container/overlay removed. SQL/mapping owners unchanged by final registration repair. |
| Shipped BML converter -> actual BML | f6 pass0.008s, two reopens and original EventID replay preserve historical ID/content/revision4/store revision7/count1. No duplication or payload rewrite; source/export unchanged; overlay removed. |

## Native host constraints and failed gates

Initial full-gate failures are retained as diagnostics, not passing verdicts.
Managed-checkout esbuild temporary deletion, pnpm store mismatch, nested Go->Git
startup0xc0000142 and global sumdb write denial were independently reproduced.
The independent checkout uses its installed store with CI=true, private
GOPATH/GOCACHE, existing verified GOMODCACHE and normal native process
permissions. VCS stamping, checksum verification, ACLs, machine policy and
global Go configuration remain unchanged. The actual external SDK consumer
probe passes5.406s and unchanged full eval passes18.713s.

Go TEMP is outside Git to avoid inheriting project instructions; Node/esbuild
TEMP is inside the task workspace where actual deletion succeeds. A job-local
pnpm wrapper and NODE_OPTIONS preload cover direct SDK Node children. Actual
UI build passes4.65s and SDK pack/Inspect probe21.517s. The already assigned
SeCreateSymbolicLinkPrivilege is enabled only for CI and restored in finally;
the actual symlink-escape case passes0.176s. Raw cmd.exe redirection preserves
stderr and just ci's actual exit status after PowerShell5 discarded buffered
output in an earlier attempt. No gate command is reduced.

The2364 full gate passed UI685, typecheck/build/vet and the complete SDK suite
but failed an approval fixture timeout. The original fixture independently
passed count10/15.958s. The5d serial retry then exposed the concrete raw SQL row
before durable approval event at the same fixture; it was stopped after the
known failure. Commit9214 corrects the readiness helper, removes all redundant
not-durable retries, retains the5s deadline, and avoids secret payload diagnostics.
Its held-Append RED and 6 final owning cases pass20.915s. This helper issue is
separate from the real root registration P1 subsequently fixed and reviewed.

The73ef094c gate reached independent Module verification after all root Go/SDK
and headless stages passed, then failed exactly five obsolete Source.SHA256
assertions. Focused vet and tests pass for all five after deletion; declaration
identity, Port, source Ref, catalog and lifecycle checks are retained. Full
tracked-source review finds no active consumers of that retired field class.
The final candidate changes no Go/runtime behavior, format or generation input;
its only UI behavior-tree delta is the two corrected static locale descriptions.

## Separate finite upgrade artifact

Final separate artifact commit `d2345e4c357485120abc52ae07c7ddf708659350`
pins runtime `5b0b572da80602f46d7496d004aa9d88fc6f5eeb`, schema source
`aeb685d257da7f557ac3da0a949f5a9854c34468`, and head47. Release folder:
`C:/Users/Administrator/.codex/worktrees/issue91-upgrade-final/agent-vivy/.workspace/artifacts/finite-upgrade-047-5b0b572d`.
The source ZIP SHA256 is
`8ef73737852aa1d47917318dd68b4a69249bbefab4b58d7ff87ea899f8941c61`; the
delivery manifest SHA256 is
`2a368e6aa6ee559329c5aa6d67159b81b33ed3d4931d75936bc72ef87e41baa0`.
Native and Linux core CLI hashes are `e7c2fba1c1f864f4a31b8839a3b30a374fdc6f33a6defe2a2d00c3951f5aadd8` and
`161e48800f3a297d28435d4719c201a08b00bc5c4711c4237ce6df50a91be0d9`.
The nine manifest-listed delivery files and all four previous release manifests
were independently hash-checked after sealing. The source ZIP differs from the
prior package only at `pin.json`, `storage/README.md` and `verification.md`;
readers, SQL, byte mappings and BML/Studio files remain identical.

Final focused gates pass: all 94 direct Git SQL blobs match their exact catalog;
native manifest tests pass in0.095s; native and independent Linux CLIs report
the exact runtime/schema/head pins, inspect and copy an admitted fixture without
changing the source, and refuse existing targets without changing their bytes.
The outputs match the accepted schema, counts, protected workflow bytes and
identity evidence. The unchanged full converter, BML and PostgreSQL matrices
were not repeated for this pin-only reseal; their actual prior results remain
listed above. The actual schema047 runtime bridge evidence remains attributed
to `441c4266`, with SQLite at 0.036s and PostgreSQL at 0.756s.

The unchanged full converter matrix passed SQLite/CLI11.899s/0.192s, actual
Postgres58.524s/0.191s, BML Linux0.179s/native20.499s, native CLI14.329s and vet.
The original standalone Studio ledger converter passes Linux0.045s and
Windows4.893s. This certifies that finite owner, not the complete external shell.
All94 paired SQL files match their Git owners; SQL001..038 remain immutable.
The final schema has001..047, without live migration checksums.

Converters retain frozen old readers outside the final runtime and accept only
enumerated source families. They convert copies, preserve full original bytes,
history and identity, require explicit recovery for unbound old pauses, and
refuse unknown shapes before writes. Original databases and programs remain
available for rollback. Existing sealed releases are preserved byte-for-byte.
Delivery hashes are native artifact integrity, not runtime content authority.

## Limits and manual gate

The older planned scripts/memory-loop.mjs and just diva-recall-test do not exist
at this baseline and were not run. Available owner suites, actual official
DIVA/shared Recipe and split UI provide their scoped evidence. Evolution remains
explicitly demo/deferred UI-EVO; D10 uses the existing live generic skill review
path and Persona actions. Later #93 extraction/physical omission and absent
later-baseline capabilities are not claimed.

Temporary source overlays/browser specs/configs and Postgres containers are
removed after checks. Disposable artifacts and raw logs remain for review.
Human real-model acceptance is LAST and pending. Candidate5b0's actual packed
native minimal-chat artifact and its exact inspected Generation
`6b41cd840bde56a3e97312ed00674327` are running with the matching staged UI on
3015 and a private disposable workspace at `.workspace/d14-manual-5b0b572d/`.
Backend `/healthz`, tiny UI and fresh-origin auxiliary model settings at3017
each return HTTP200; the existing RPC created Session `sess_c5533c0a93858efa`
bound to that workspace. DeepSeek OpenAI-compatible settings are expanded and
the API Key field is empty. The setup issued no Run or provider request. The
first pack attempt could not write Go overlays to the
app-owned visualization temp path; it passed after moving Go temp files under
the independent native checkout's ignored `.workspace/temp`. No product source
changed for this environment-only adjustment. No credential is requested in
chat or recorded. D14/#91 stay open until the human outcome is recorded. No push,
stable-root merge or publication.
