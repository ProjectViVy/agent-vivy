# Verification: PR #79 review fixes

Environment: Linux sandbox, uid 0, Go 1.26.4, Laputa checkout at the pinned
`27da7b9` (symlinked as `../laputa`). Windows and PostgreSQL were not available.

## Reproduced before fixing (test fails without the change, passes with it)

| Defect | Test |
|---|---|
| Ordinary session unreadable after non-UTF-8 tool arguments | `TestMessageProjectorToleratesInvalidToolArgumentsOutsideRecap` |
| Recoverable tool error killed a Recap run | `TestIssue58RecoverableToolErrorStaysModelVisible` |
| Stuck run blocked the subject's next model request | `TestIssue58ExcludedRunDoesNotBlockRecapContext`, `TestIssue58UnresolvedToolRunIsSealedExcluded` |
| Thinking label said default while sending `max` | `drops a picked level once the model no longer declares it` |
| SQLite orphan `message_copy_sources` | `TestDeleteSessionRemovesMessageCopySources` |
| Laputa: cancel before replacement wedged the owner | `TestRecapRecoveryCancelBeforeReplaceDoesNotPoisonOwner` |

Existing tests whose old expectation was the blocking behavior were updated:
`TestIssue58EvidenceSaveFailureBlocksContinuation`, `TestIssue58FailedReceiptNeverAdvancesModel`,
`TestIssue58InvalidUTF8StreamRetainsArgumentPrefixAndIdentity` (now assert an excluded seal with no originals),
and `TestIssue58InvalidArgumentsSkipObserverDelivery` / `TestIssue58AutomaticIngestProvenance` (skip and advance).

## Commands and results

| Command | Result |
|---|---|
| `go test -p 2 -timeout 40m -skip '^TestMemoryLoop' ./...` | 81 packages ok; 3 failures, none from these changes (below) |
| `go test ./internal/eval -count=1` (re-run on pinned Laputa) | ok |
| `go test ./internal/storage/...` | ok (PostgreSQL tests skipped: no server) |
| `cd plugins/vivy-memory && go test ./...` | ok |
| `cd ui && pnpm exec vitest run` | 98 files, 743 tests passed |
| `cd ui && pnpm exec tsc --noEmit` | clean |
| `ui/playwright.issue58.config.ts` (real Go fixture + Vite + Chromium) | 2 passed |
| Laputa: `go test ./actmem` | ok |
| Laputa: `go test ./garden/internal/ingest ./garden/internal/runtimecore ./garden/agentapi` | ok |

Playwright ran through a temporary config pointing at the sandbox Chromium
(`/opt/pw-browsers/chromium`, `--no-sandbox`); the config was deleted afterwards.

## Failures that are not caused by these changes

- `internal/runtime TestDeliverableReadSnapshotWriteFailure`: its fixture uses `chmod 0500`, which uid 0 ignores.
- `ui TestEmbeddedHandlerAnswers`: needs a built `ui/dist`; only `.keep` exists here.
- `internal/eval TestEvalMissingFileArtifactDoesNotFallBack` failed once during the long run because the shared Laputa checkout was mid-edit; it passes on rerun.
- `VIVY_RELEASE_CONFORMANCE=1 go test ./sdk/internal/conformance -run TestCheckedInProviderConformanceMatchesExecutedSuites` fails identically on the unmodified PR #79 head (release digest of `vivy/context-source` differs from checked-in evidence). The checked-in release evidence was not advanced here.

## Not run

- `just ci` as a whole (PowerShell steps), the native Windows lane, PostgreSQL conformance, live-provider runs.
- The generated UI assembly was regenerated with `ui/scripts/stage-ui-assembly.mjs` (only the memory sidebar source hash and catalog hashes changed).

## Laputa

The Laputa commit lives in a local clone and is delivered as a patch
(`0001-fix-actmem-do-not-poison-the-Recap-owner-when-recove.patch`). Apply it to
Laputa PR #10's branch, then re-pin VIVY (`MEM-58-LAPUTA-REPIN`).
