# Verification

The required D06 and D01 scoped obligations pass at product candidate
`cc74e7d8ea6474190c8c182d42f51496c4d7fccc`. The final delivery commit changes only these records,
Story plans/status and browser evidence. Historical failed attempts and explicit
skips remain recorded; no full-product release CI is claimed.

## Candidate and environment

- Baseline: `e2c1518e`. Isolated Windows branch: `codex/issue91-d06` at
  `C:/Users/Administrator/.codex/worktrees/issue91-d06/agent-vivy`.
  Root D03 edits were never copied or modified. After its human commit
  `c065a82b` was available, it was integrated sequentially as `398fe41b`;
  paired target 037 and final regeneration/checks followed that integration.
- Independent Linux-filesystem checkout:
  `/home/mastwet/work/vivy-issue91-d06/agent-vivy`, fetched through a Git bundle
  and reset to the exact candidate, with its own Laputa
  `ff3936f44ff8cf08c12af2cf698c194cfe474fd3`.
  Go 1.26.4, Node 24.21.0, pnpm 10.33.0. Native Windows Go 1.26.4, Node 24.14.1.
- Studio was actually bootstrapped and inspected at
  `31452417737b1bf643f9181e15a1f3c5226a2b94`. Confirmed consumers were fixed at
  `abc3ba4765ebd6d24bcf519ad3c7777d3891ef04`, the integrated compatible gitlink.
  Both repositories remain local; publish them together before remote reuse.
- Native Git/module/package/TLS integrity remains enabled. Fetch-dependent final
  checks use process-scoped `GOPROXY=https://goproxy.cn,direct` and normal
  GOSUMDB. No global proxy or checksum-verification setting was weakened.

## Final exact-candidate results

| Command / check | Observed result |
| --- | --- |
| `just wiring-ci` | PASS: 16 Node guards and 105 assembly cases, no skips |
| `just kernel-ci` | PASS: vet, 1,715 owner cases; one optional ripgrep skip; separate actual minimal Pack/Inspect/boot passes |
| `just d02-ci` | PASS: 81 runtime/RPC, 175 shared Face/CODE, one causal App, 13 shutdown/headless App and nine standalone headless cases; no skips |
| `just d02-web-ci` | PASS: typecheck, 102 Web cases, two actual selected-UI/lite artifact cases; no skips |
| `go test ./sdk/... -count=1 -timeout=35m -json` | PASS: 1,079 executed cases; one manual TUI preview skip. Full SDK internal package 336.465s; strict conformance package 64.617s |
| `go test ./internal/rpc ./internal/studio ./internal/studiocore ./internal/transition/studioledger/... ./internal/eval -count=1 -timeout=15m -json` | PASS: 281 cases; Windows-only ADS case skips on Linux and passes natively |
| D01 causal/hooks/nudge/recovery selector below | PASS: 17 cases, no skips |
| Historical App/settings/embedded selector below | PASS: 13 cases, no skips; every historical failure mapped in [d01-failure-map.md](d01-failure-map.md) |
| `go generate ./internal/generated/assembly` and `pnpm --dir ui run stage:ui`, twice | PASS on exact Linux candidate: explicit `authoring-default`; all 77 emitted files byte-identical before/after both passes; tracked generated projections have no diff |
| Native `just studio` | PASS on exact candidate: actual executable build |
| Native `go test ./sdk/internal/assembly ./sdk/generation ./sdk/conformance -count=1 -timeout=10m` | PASS on exact candidate: 9.952s / 0.046s / 0.046s |
| Independent behavior and residue follow-ups | No findings on exact candidate; read-only, no duplicate suite execution. See [review.md](review.md) |

The SDK run actually packs and inspects all ten checked-in Recipes, full UI,
standalone external Module, hello-fs, LSP and Go-host fixtures. No required variant
is unavailable. Strict reproduction executes all 23 Provider suites without an
update flag; no failed or unexecuted outcome was rewritten as passing.
[recipes.md](recipes.md) records exact inputs, targets and opaque allocations.
Manifest tests include duplicate/escaped/case-variant/nested fields, invalid UTF-8,
real embedded Requirement round-trip, schema/framing refusal and shared identity.

## Additional owning evidence retained

These checks preceded the final Manifest-only structural correction; their
owning production files are unchanged by that correction. Final integrated
owner/SDK/gate runs above cover its affected behavior.

- Full UI: `pnpm --dir ui exec vitest run` passed 83 files / 654 tests after
  core DTO/card removal. `pnpm --dir ui build` passed, actual production build
  4.06s; existing chunk-size advisory only.
- Native Studio/converter/conformance packages passed; native core migrations
  and lifecycle/RPC subset passed (41.250s / 19.704s). Native Windows ADS and
  ripgrep tests both passed; Linux skips remain skips.
- Native shell/fixture Node tests passed 17 cases with no skips (12 workbench,
  five fixtures); `client.js` syntax check passed.
- Core SQLite and real disposable PostgreSQL fresh/reopen/copied-035/history/
  failure rollback and retired-column refusal passed. Existing v14/copied-035
  paths reach 037; unlisted intermediate 036 refuses before writes.
  [Core commands and support boundary](../2026-10-21-d06-core-generation-identity/verification.md).
- The separate Studio ledger converter preserves six-table history, events,
  artifacts and snapshots; fresh/reopen/release/install/rollback and refusal
  fixtures passed. [Studio commands](../2026-10-21-d06-studio-identity/verification.md).
  The lead reran integrated native and Linux owning suites.

## Exact focused commands

Native subprocess PATH includes `C:/Program Files/Go/bin`.

```text
go test ./internal/app ./internal/runtime -run 'TestAppRunsWithoutDigestAudit|TestToolHook|TestNudgeAcceptance|TestRecoverAcrossModelGeneration' -count=1 -timeout=10m -json
go test ./internal/app ./internal/app/settings ./internal/embedded -run '^(TestCodeFacePrintModeStreamsAssistantText|TestCodeFaceJSONModeEmitsOrderedRecords|TestCodeFacePrintModeCancelsBlockedRunLoudly|TestPlanGoalIntegratedReviewAndTwoRounds|TestPlanGoalIntegratedRoundLimitBlocksDurably|TestAppShutdownStopsGoalAdmissionBeforeChannelDrain|TestAppShutdownBounded|TestSaveConcurrentWritersKeepDocumentValid|TestPollCollectsRunEvents|TestPollOverflowReportsGap|TestPlanGoalIntegratedReadsConvertedVersion23SQLite)$' -count=1 -timeout=15m -json
go test -tags=vivy_headless ./internal/studiocore ./internal/studio ./internal/transition/studioledger/... ./sdk/conformance -count=1 -timeout=10m
go test -tags=vivy_headless ./internal/storage/migrations ./internal/rpc -run 'TestGeneration|TestApplyPreflight|TestStudioRPCEmptyListAndPromote|TestSpeciesInspectBuiltinThenPromoted|TestGenerationsRejectRPC|TestEvalsStartUnknownSuiteAndFailedProbe|TestControlShellStartIsGovernedAndStrict' -count=1 -timeout=10m
go test -tags=vivy_headless ./internal/rpc ./internal/runtime -run 'TestResolveProjectAttachmentsRejectsNativeWindowsADS|TestBackendGrepRipgrepHonorsGitignore' -count=1 -v -timeout=5m
node --test scripts/check-plugin-v1-fixtures.test.mjs studio/dsh-species-workbench/species.test.mjs studio/dsh-species-workbench/confirm.test.mjs studio/dsh-species-workbench/recipe.test.mjs
```

## Real browser and reproduction

Final split pair: `go run ./cmd/vivy` plus `pnpm dev` in `ui`; Vite at
`:3015` proxies the loopback control plane. Fresh explicit home/config:
`.workspace/d06-browser-cc74` in the dedicated Linux checkout. Actual browser
interaction on exact candidate verified Lifecycle `authoring-default`, absence
of hash rows, Refresh and Generation list reading on target 037. Earlier
English-to-Chinese switching confirmed the compiled catalog after backend
acknowledgement/reload. The final screenshot is
[browser-final-lifecycle.png](browser-final-lifecycle.png).
No provider key or tenant store was used. Task-owned split servers were stopped.

Only producers changed generated files. Raw final commands/results, candidate,
JSON events and receipts are retained in the Linux checkout's
`.workspace/d06-final-evidence` and the Windows worktree's
`.workspace/acceptance-evidence`; earlier failed/native logs remain in
`.workspace`. No custom source-hash evidence mechanism is introduced.

## Retained failures, skips and limits

- Invalid early timeout flags executed no tests; no pass was inferred. Early
  builds exposed remaining descriptor/Recipe/RPC hash fields, since removed.
- Initial SDK packing exposed missing selected CODE/LSP native Go requirements.
  Packer requirements now derive from compiled selected source owners. Actual
  full Recipes pass. An old copied WSL Git index still named deleted sourcehash
  code; exact bundle checkout fixed that verification setup.
- First kernel run failed `TestINOFYWorkflowCancelPropagates` (cancelled versus
  recovery_required); cause not established. Final full kernel gate passes.
- Native converter map-key and file-URI drive parsing failures were corrected
  with portable slash-form keys and leading-slash file URIs. Absolute evidence
  paths failed RED before portable slash/drive/colon rejection.
- Full RPC exposed a stale pre-D03 shell-label expectation. Test-only alignment
  to `bash (script redacted)` retains the raw-script negative assertion.
- Earlier SDK runs failed retired error wording, then the shared JSON guard's
  catalog diagnostic label. The final guard keeps catalog framing-only behavior
  and its original typed-decoder error authority; full SDK now passes.
- Historical App selection exposed stale automatic-v23-upgrade expectations.
  The test now proves read-only refusal, unchanged source and explicit copied
  conversion before App open. CODE JSON mode failed once without captured cause;
  diagnostic additions, independent 10/10 repetition and final history selector
  pass. No runtime fix or root cause is claimed for that isolated failure.
- Manifest ambiguity checks failed RED for duplicate/escaped keys; independent
  review added case aliases and embedded Requirement coverage. Exact schema tags,
  promoted fields and invalid UTF-8 refusal now pass integrated suites.
- Default external-fixture proxy timed out at checksum fetching; direct fetching
  hung and was stopped. Supported reachable proxy passed with normal GOSUMDB.
- Linux skips: optional ripgrep, Windows-only ADS, manual
  `TestDumpComposerPreview` (requires `TUI_PREVIEW=1`). The first two passed
  on Windows. Manual preview is outside product acceptance.
- Native UI build hit esbuild temporary-file Access denied, including local
  TEMP/TMP retry. Final native regeneration hit an OS user-mapped-file lock on
  `ui/src/generated/assembly.ts`. No source workaround or generated hand edit
  was used. Actual Linux production UI build and exact-candidate two-pass
  generation pass; native Studio and contract suites also pass.
- Restarting the earlier disposable browser home refused its outstanding lease;
  no lease was bypassed. Final browser used a new dedicated home.
- No Windows full-product CI, release gate, real-token smoke, daily install,
  upgrade publication or overall #91/D14 closure is claimed. No push occurred.

All branch authors/committers are the configured human `mastwet`. The existing
Story index remains the sole Story status record. Final document/link checks and
`git diff --check` pass; task-created disposable runtime/build output is removed.
