# Official Eino v0.9.20 native-fix report

## Outcome and exact candidate

The official dependency remediation removes the captured shared error from enhanced
ToolsNode stream conversion without a Vivy scheduler/converter workaround. The exact
required retained-Plan race command and existing unretained Plan baseline pass.
Broader native runtime and corrected app/persona compatibility results are below.
No production adapter, S2 source behavior, orchestration loop, tool concurrency,
capability, test assertion or test exclusion changed.

Base: `4a914f531b341202211e8df8764060190a16ae29` (complete S2 Task2 source).
Source/pin/test/guide/amendment commit: `d0f1e487baa3ba74d4b0ec7471a7547592b2809c`.
This companion evidence commit changes docs/logs and TODO reconciliation only;
it does not claim its own later SHA was the tested source binary. Both commits use
human identity `mastwet <1136523089@qq.com>`. Parent-owned plan index/ledger is excluded.
No push, merge, deployment, tenant journal access or release packaging occurred.

## Official module and dependency closure

Normal `go mod download -json github.com/cloudwego/eino@v0.9.20` reports official
origin `https://github.com/cloudwego/eino`, tag `refs/tags/v0.9.20`, commit
`b9539ec114a14c32aa568f05eda5cde1e83fcfc1`, version time
`2026-09-20T09:34:37Z`. GOPROXY is `https://proxy.golang.org,direct` and GOSUMDB is
`sum.golang.org`; no private exclusion, VCS/toolchain bypass or replacement was added.
Normal `go get github.com/cloudwego/eino@v0.9.20` resolved the official module.
Its unrelated go.mod/go.sum sorting was removed from the patch. The dependency
change is exactly one root require line plus these two authenticated sum entries:

```
github.com/cloudwego/eino v0.9.20 h1:Qini5wHK2gI+QfQb4QuO2IKnvS5LpUc+CctPjZ1euNA=
github.com/cloudwego/eino v0.9.20/go.mod h1:OBD1mrkfkt/pJa4rkg1P0VnaMeOVl7l8IAdEqY//3IQ=
```

The previous Eino sum remains historical. Upstream v0.9.13/v0.9.20 go.mod checksums
are identical; before/after graph comparison changes only the root Eino edge and
Eino's version-qualified outgoing edges. Inofy and Laputa retain their own v0.9.13
minimum requirements under Go MVS. No transitive version, EinoExt pin or local
replacement changed. OpenAI v0.1.13, Claude v0.1.25, MCP v0.0.9 and ACL OpenAI v0.1.17
remain pinned. Published Laputa modules and clean local replacement checkout remain
`4c7c6b20859375d0d764d3ba2a0188926cf73c9e`.

Root `go mod verify` returned exit 1 for missing ziphash on the existing replaced
local `agent-vivy/* v0.0.0` modules. This is not reported as a successful root gate.
To verify the downloaded official Eino cache without those unrelated replacements,
a disposable module containing only `require github.com/cloudwego/eino v0.9.20`
and copied authenticated sums ran `GOWORK=off go mod verify`: exit 0,
`all modules verified`. The official module and sources were not patched or vendored.

## Native source and API compatibility inspection

Source inspected directly in `/root/go/pkg/mod/github.com/cloudwego/eino@v0.9.20`.
The official enhanced stream converter in `compose/tool_node.go:1365` uses local
`parts, convertErr := tr.ToMessageInputParts()` and assigns parts only after success.
There is no shared outer error write in that concurrently called closure. The
non-stream Invoke conversion still uses its own sequential local function error;
it is not the raced stream closure.

The same native ToolsNode change adds a mutex-protected stream checkpoint tracker.
Each source collects completed chunks; `core.ErrStreamCanceled` marks that call for
rerun instead of treating a partial output as completed. At merged EOF the tracker
returns a native composite interrupt with original input, completed normal/enhanced
outputs and unfinished call IDs in input order. `compose/checkpoint.go` recognizes
that typed stream checkpoint error, preserves its interrupt persistence maps and
producer input, and reruns the producer through the existing restore path. This is
upstream checkpoint state, not a new Vivy receipt or orchestration authority.

| API inspected | Compatibility and ordering evidence |
|---|---|
| `adk/middlewares/summarization.New`, `TypedMiddleware.Summarize` | Same configuration and typed message shapes as the old pin; existing compaction middleware compiles and its regressions run. |
| `GenModelInput` | Called to construct exact model input before summary model invocation; S3 can supply Recap plus complete selected originals. |
| `Finalize`, `Callback`, internal events | Summarize snapshots state, emits optional before event, generates summary, calls Finalize, then Callback, then after-summary event, then returns messages. Callback/event errors can follow a durable Finalize commit; S3 must still resolve the operation receipt. |
| `BeforeModelRewriteState` | Replaces a copied state's Messages only after successful Summarize. The only source diff in this middleware versus v0.9.13 is returning the original ctx rather than nil on error. |
| `adk.WithHistoryModifier`, chat-model resume `HistoryModifier` | Existing native resume history hook remains; scope/epoch validation is still the Vivy admission/model-boundary obligation. |
| `RunnerConfig.CheckPointStore`, `Runner.ResumeWithParams`, compose checkpoint APIs | Existing bridge compiles and checkpoint/approval/child/Plan resume coverage runs; native canceled-stream producer state is preserved through this same bridge. |
| `tool.EnhancedInvokableTool`, `InvokableTool`, `StatefulInterrupt`, `compose.IsInterruptRerunError` | Tool interface source hash is unchanged. The existing enhanced adapter still invokes its inner governed tool before compact/multimodal normalization; typed post-invoke Plan interrupts remain native. |
| `compose` tool callbacks and `utils.runWithCallbacks` | Existing onStart → tool endpoint → onError or onEnd/stream-output callbacks remain; a stream-output callback receives a copied reader. The new tracker acts on the native output stream after endpoint creation. No Vivy callback/receipt ordering was moved. |

No structural API conflict or source adapter expansion was needed. Import quarantine
runs in final app compatibility. S3 design/S3/S6 plans carry a dated v0.9.20 capability
amendment; original v0.9.13 baseline claims and historical reference fixtures are
preserved. Current Eino guide/cache path and only the two test-only constants that
explicitly track the current module pin are updated. Guide validation passes.

## Preserved RED evidence and independent fixture correction

Original actual v0.9.13 required race command and baseline command are retained in
`task-2-report.md`, `task-2-required-race.jsonl` (SHA256
`200ccb3036f50c6687b323375a28693583f58c97a6810a6824932ddf36d23ed6`) and
`task-2-upstream-baseline-race.jsonl` (SHA256
`4d586171447282f7af70487d265a832fefad87e1de5c8837b8e8e391fe576dff`). Both exited 1
with captured outer error writes at upstream compose/tool_node.go:1253. The retained
Plan revise subtest and its parent failed; the existing unretained submission-fence
test also failed. These saved RED runs were reused without needless repetition.

The broad app compatibility run separately exposed
`TestDefaultPersonaAuthorityPersistsAndFreezesPerSession` at default_persona_test.go:28:
`app: cognitive factory: diva-cognitive: open garden owner: recap binding: storage:
subject binding mismatch`. The test supplied nil SnapshotStore to the shipped
factory. Isolated v0.9.20 and original v0.9.13 alternate-modfile runs both reproduced
the same pre-model initialization error (exit 1); these are actual raw logs below.
This is an existing S2 producer-dependency fixture gap, not an Eino API regression.

Controller explicitly authorized one fixture-only correction: open an isolated
SQLite core database and reuse `backend.Snapshot()` for both factory open/reopen
calls. All original uninitialized-authority, no WORLD injection, frozen existing
session, saved persona and new-session projection assertions remain unchanged.
No nil-store fallback or relaxation of ResolveSubjectBinding was introduced. The
same focused persona test provides GREEN evidence for this fixture correction.
The later broad app diagnosis has the separate cancellation timeout recorded below.

## Actual final and diagnostic gates

Environment: Go `go1.26.4 linux/amd64`, PATH starts with
`/workspace/scratch/243d1cbb859f/toolchains/bin`. Counts are Go JSON terminal test
entries, including subtests/group entries. No failed/excluded subset substitutes for
the original required command. All gates used official Eino v0.9.20 except the
explicit old-pin persona diagnostic. The full runtime/storage and focused gates
ran after the final dependency pin; subsequent changes are the two current-pin
metadata constants and isolated persona test setup only. Runtime/storage source
and tests are byte-identical to the source candidate. Corrected final app checks
cover the subsequent test-only changes at that candidate.

### required-race

```sh
go test -race ./internal/runtime ./internal/storage/sqlite -run '^(TestIssue58|TestMessageProjector|TestToolOperationCrashMatrix|TestApprovalResumeReusesPreparedToolOperation|TestServiceCancelConcurrentIdempotent)' -count=1 -json
```

Actual exit 0. storage/sqlite: 24 pass; runtime: 55 pass.
Raw output SHA256: `092fcda5ef6bdfc46743de0758fb41246c24393d99dfbf14c6583b419c615478`.

### baseline-race

```sh
go test -race ./internal/runtime -run '^TestPlanGoalProbeSubmissionFencesLaterToolInSameBatch$' -count=1 -json
```

Actual exit 0. runtime: 1 pass.
Raw output SHA256: `3eeca1f4b600261f6b181d1e01e76c7f2a5f21de24304916493c3ed25fd0ae92`.

### full

```sh
go test ./internal/runtime ./internal/storage/... -count=1 -json
```

Actual exit 0. storage: 21 pass; storage/migrations: 40 pass/5 skip; storage/postgres: 1 pass/36 skip; runtime: 1130 pass/1 skip; storage/sqlite: 209 pass.
Raw output SHA256: `42fff95420fba7c95c6495518769d922d1052b1cc48a5cc9d196e9fce9197dfc`.

### focused

```sh
go test ./internal/runtime ./internal/storage/... ./internal/app ./internal/rpc -run '^TestIssue58' -count=1 -json
```

Actual exit 0. storage/postgres: 0 pass/8 skip; storage: 1 pass; rpc: 10 pass; storage/migrations: 2 pass; storage/sqlite: 24 pass; app: 14 pass; runtime: 39 pass.
Raw output SHA256: `c6b5a20163fe7b3836a1af36ce41c80e68ea0d724ba76c00353dff76527fa133`.

### app-compatibility-corrected

```sh
go test -race ./internal/app -run '^Test.*(Plan|Interrupt|Cancel|Checkpoint|Child|Persona|Coding|Compaction|ApprovalResume|EinoImportsQuarantined)' -count=1 -json
```

Actual exit 1. app: 31 pass/1 fail.
Raw output SHA256: `0a5e6ee934491b962728a3c00fa6128f4a1a96320b924cb18d43a6f81fb8d6ca`.

### persona-corrected

```sh
go test -race ./internal/app -run '^TestDefaultPersonaAuthorityPersistsAndFreezesPerSession$' -count=1 -json
```

Actual exit 0. app: 1 pass.
Raw output SHA256: `b98c2f512e8e405aa77ae5634d5bc5667c2a6c594060c33958f16466b8bf201d`.

### codeface-pin

```sh
go test -race ./internal/codeface -run '^Test(Prepare|Prepared)' -count=1 -json
```

Actual exit 0. codeface: 3 pass.
Raw output SHA256: `197d9b5133f8c991e6285a32d0d1addec8985f98076ac5ded747269d365cb466`.

### native-cancel-checkpoint

```sh
go test -modfile=.superpowers/sdd/EINO/upstream-tests.mod -race github.com/cloudwego/eino/compose -run '^(TestAttack_EnhancedToolStreamCancellationTracksCompletedCalls|TestCheckpointToolStreamCancellationRerunsProducerOnce)$' -count=1 -json
```

Actual exit 0. github.com/cloudwego/eino/compose: 2 pass.
Raw output SHA256: `c9031101a6bc25ad9f06aed187a49388cd47b3a6c5bbf7f7f9bc7c0cbe4c7722`.

### cancel-isolated

```sh
go test -race ./internal/app -run '^TestMemoryLoopActivityDeleteDrainsLiveCancellation$' -count=1 -json
```

Actual exit 0. app: 1 pass.
Raw output SHA256: `6e7279edbf5426b39ea939eca23402d63d3702ccb395fc789d7f83176563577a`.

### Initial/diagnostic compatibility runs (never labeled passes)

- `compatibility`: exit 1; runtime: 128 pass; app: 31 pass/1 fail. SHA256 `0f66d61c431bda50ab0bd8da00dc2c7df215e4b6bf245aa46f67aadc70f2ed86`.
- `final-app-compatibility`: exit 1; codeface: 1 pass; app: 31 pass/1 fail. SHA256 `fbdd0c795e994d0c8432952d59c8bf848194fa88a475d7c140bd857708465b22`.
- `persona-isolated`: exit 1; app: 0 pass/1 fail. SHA256 `3df07a2c23e18afd399ace9fb87695a6223be262654211cd29f1563eafdec06f`.
- `persona-baseline`: exit 1; app: 0 pass/1 fail. SHA256 `c52f0894361d33fae12d42ef916699331da80ab44cf97cb5b062ea081a49fee5`.

The first broad compatibility command's runtime package result is a final green
native compatibility check; its app package failure is superseded only by the
corrected persona test and subsequent app diagnosis, not erased. The interim app/codeface rerun likewise
preceded the fixture correction; its codeface result covers the launch consumer,
and the separate codeface pin run covers the remaining affected launch-test file.
Exact commands/counts/failure names/start/end times and every skip are recorded in
`eino-upgrade-verification.json`. Raw required/baseline GREEN, old/new persona RED,
corrected persona GREEN and native-cancel outputs are committed beside this report.
Other complete raw outputs remain at the exact scratch paths in the manifest;
counts, hashes, all failure and skip names are durable.

The corrected broad app diagnosis failed only
`TestMemoryLoopActivityDeleteDrainsLiveCancellation` at memory_loop_activity_test.go:147:
its existing ten-second session/delete RPC deadline expired. Both preceding broad
app diagnoses passed this exact test; the corrected run briefly overlapped those
processes. Load/overlap is a hypothesis, not a demonstrated root cause. The exact
case ran alone after all broad runs drained; its actual result is listed above.
This isolated result does not rewrite the failed broader command as a pass. No
production change, timeout relaxation or broad rerun chasing green was made.
Controller identified MemoryLoop as separate DIVA Recipe coverage, excluded from
the general just ci Go gate; these diagnostic matches do not replace that recipe.
A candidate-bound S6 recipe run remains required.

Full runtime/storage: 1401 pass / 42 skip. PostgreSQL's 36 server suites and five
migration checks skip because no real VIVY_POSTGRES_TEST_DSN is available. Its one
pure unit check passes. Runtime TestReportFeedbackSnapshot skips its existing
external GitHub prerequisite. No skip is PostgreSQL/provider/product acceptance.

`git diff --check` passes. The current Eino guide passes the repository skill
validator. This increment adds no version-string mirror test. Official upstream
canceled-enhanced-stream and checkpoint producer-rerun tests run unchanged through
a disposable copied modfile so native test-only dependencies cannot rewrite the
product module files. This is a focused check of the named native ordering risk,
not a full upstream or product CI claim.

## Source fingerprints, limits and handoff

Inspected official source SHA256 values:

- `compose/tool_node.go`: `9ad538059f4ecc81137dbb045829d95792dfdf3fa3905b322f4152d4b6c131bd`.
- `compose/checkpoint.go`: `503045562c7e9c2af94faac90e878adf200e31b6d3c58a766f99d9a8b76516ad`.
- `compose/pregel.go`: `b1e9d77fbfecb763dee70ed18f8e408dea45dc3d6c2429cd4ef75cc0e856bcaa`.
- `adk/runner.go`: `68667d6e57a496a0374517b30f2b637846a517e7f370e21c56439842a50b24b6`.
- `adk/chatmodel.go`: `674dc19ca9a41406d5fc68848355eb9199c39be8f3b4a2a01527e1dc41ce61a5`.
- `adk/handler.go`: `c880d5981e0f5d9961f557899b7efb71b7250d80b68db6465497292bdf53b02b`.
- `adk/middlewares/summarization/summarization.go`: `597cd74e17ee3659a8c79e66029947f823b61b0fb50c2dfa7790054b4b1eeef0`.
- `components/tool/interface.go`: `d10e710bb80923427b68fb58b90737299e7aa98b75819f4d1c46df1bde1b712e`.
- `schema/stream.go`: `b7b154c587960d9db504dfc83bf95929dfc7aa4bf8ebd2a4ba3203b0c77a15a4`.
- `compose/utils.go`: `90c8be4b3bc066d17327dea620180baa38a1478ac45c0cc38e35fda976329ee0`.

The manifest also records source candidate, base, normal module provenance/sums,
changed source fingerprints and before/after module graph hashes. Current design
and development records distinguish inspected baseline from final native-fix pin.
The three duplicate upstream ToolsNode race TODO entries are reconciled into a
completion record with historical evidence retained.

Remaining: independent scoped spec/quality review; real PostgreSQL server parity;
centralized S6 just ci and whole-candidate integration; native Windows; real provider,
installed artifact/browser LIFE/#66 and DIVA/Mentle product acceptance. None ran or
is implied here. No Eino API compile conflict or native source race remains observed in this scoped
remediation. The actual broader MemoryLoop cancellation timeout remains a qualified
compatibility concern; its isolated outcome is additional evidence, not an erased
failure or recipe acceptance. Final source acceptance belongs to controller/review gates.
No release was packaged, so a new release.md is not applicable. Rollback must not
reinstate an unsafe v0.9.13 runtime while claiming the race gate passes; preserve
current source/artifact owners and protected evidence while selecting a reviewed
compatible official runtime. S3 source implementation remains outside this slice.
