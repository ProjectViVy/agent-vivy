# Verification

## Baseline and scoped D00 revalidation

- Source baseline: `ce5489256ed30bfb1cc83769d87325105dc8df21` on stable;
  implementation branch: `codex/issue91` in `.worktrees/issue91/agent-vivy`.
- Independent Laputa checkout: `ff3936f44ff8cf08c12af2cf698c194cfe474fd3`,
  matching `laputa-source.lock.json`; `scripts/ensure-laputa.ps1 -Quiet` passed.
  Local clone used `--no-hardlinks`, with its own working directory and origin.
- Toolchain: Go 1.26.4 windows/amd64, Node 24.14.1, pnpm 10.33.0. Go is added
  to process PATH only; GOPROXY is process-local. No `go env -w` was used.
- Eino capability check: existing v0.9.21
  `adk.NewTypedChatModelAgent[*schema.AgenticMessage]`,
  `adk.NewTypedRunner` and typed WrapModel middleware remain
  (`internal/runtime/engine.go`). No orchestration capability is added.
- Audit ownership checked from default App wiring through runtime hook/sink
  and the active memory Host contract. The Channel hook and
  actionhost.JournalAuditSink are separate and retained.
- D01 changes no persisted format or affected upstream API; the later
  approval/checkpoint/BML/Studio conversion gates are not accepted by this cut.

## Observed checks

| Command/check | Result |
| --- | --- |
| `pnpm install --frozen-lockfile` in ui | Passed; locked dependencies, independent node_modules |
| Initial `pnpm build` | Failed in esbuild temporary-file deletion, before production audit edits |
| Standalone esbuild transform, small input vs >1 MB input in default TEMP | Small passed; large reproduced the same Access denied failure without Vivy source |
| Same large-input diagnostic with worktree-local TEMP/TMP | Passed |
| `pnpm build` with worktree-local TEMP/TMP | Passed; actual Vite assets and index.html built, no placeholder |
| `go test ./internal/app -run '^TestAppRunsWithoutDigestAudit$' -count=1 -timeout 35m` before deletion | Expected RED: causal/result/terminal/idle assertions passed first; failed only because the retired audit log was still emitted |
| `go test ./internal/app ./internal/runtime -run 'TestAppRunsWithoutDigestAudit\|TestToolHook\|TestNudgeAcceptance\|TestRecoverAcrossModelGeneration' -count=1 -timeout 35m` after deletion | Passed: app 3.852s; runtime 22.529s |
| `gofmt -l internal/app/assembly_audit_test.go` | Empty output |
| `git diff --check` / staged diff check | Passed |
| `just ci` with worktree-local TEMP/TMP | Failed in the full Go test stage; 55 top-level tests failed across nine packages. Bootstrap, UI typecheck/build/i18n, 82 Vitest files / 643 tests and Go vet passed |
| `just fmt-check` after staging the new/deleted Go paths | Passed; the initial CI enumeration had reported deleted paths before they were staged |
| `just headless-compile` and `just plugin-ci` | Passed separately after the aggregate stopped at its test stage |
| Isolated split pair: `just run` on :8788 and `cd ui; pnpm dev` on :3015 | Started with an empty explicit config and worktree-only VIVY_USER_HOME; healthz and Vite GET returned 200. Opened the Vite origin; backend recorded bootstrap 200 and RPC WebSocket 101. Both processes stopped afterward |
| Fresh independent read-only review | Passed: no substantiated findings in the implementation snapshot; did not count pending CI as passed |

The build workaround is process environment only: TEMP/TMP point into this
checkout's ignored build scratch. No dependency source, build recipe or
product code was patched. Vite's existing >500 kB chunk advisory remains.

## Full-gate failures and baseline comparison

The failed packages were internal/app, internal/app/settings,
internal/embedded, internal/runtime, sdk/codeclient, sdk/host/v1,
sdk/internal, sdk/internal/assembly and sdk/internal/conformance. The
[failure inventory](ci-failures.txt) preserves all 55 top-level test names.
The full gate is not passed, and D01 is not accepted.

Failures include Windows symlink privileges, child compiler access to the
global Go cache, SQLite cleanup locks, loopback connection/timeouts and shell
assumptions. The checkout-local TEMP workaround also makes the two project
instruction discovery tests see the repository above their temporary roots;
that is a verification-environment side effect, not a proven baseline defect.
The full run additionally failed TestNudgeAcceptanceResumeNoStaleNotice
with an approval-durability timing error, despite its focused run passing.
These failures have not all been independently diagnosed.

A detached, unchanged ce548925 comparison checkout with its own pinned
Laputa reproduced TestPlanGoalIntegratedReviewAndTwoRounds,
TestPlanGoalIntegratedRoundLimitBlocksDurably,
TestAppShutdownStopsGoalAdmissionBeforeChannelDrain and
TestAppShutdownBounded. The round-limit failure differed (loopback request
failure on baseline versus missing schema_meta in the candidate); only the
test failure, not a common root cause, is established. The same selected
code-face, concurrent-save and embedded tests passed on baseline.

The following candidate retry also passed (app 34.589s, settings 4.270s,
embedded 25.037s); this does not erase the full-suite failures:

```text
go test ./internal/app ./internal/app/settings ./internal/embedded -run '^(TestAppRunsWithoutDigestAudit|TestCodeFacePrintModeStreamsAssistantText|TestCodeFaceJSONModeEmitsOrderedRecords|TestCodeFacePrintModeCancelsBlockedRunLoudly|TestSaveConcurrentWritersKeepDocumentValid|TestPollCollectsRunEvents|TestPollOverflowReportsGap)$' -count=1 -timeout 35m
```

### D01-induced conformance evidence mismatch

This is a direct consequence of the source change, not an environment-only
failure. The current producer hashes every internal source/test file. Its
computed identity is now
`21f917fbd373940eacae7eaa41796db050ea9bc0276edffb87d29a22e17c07f9`;
the checked-in conformance artifact still binds
`58e65c9389ffe8eda3349e2a02e8004487c7d5e0b8de604af989250fbd96b621`.

On the unchanged baseline, this hash check passes, but the actual producer
suite then fails its UI packing checks on this host. The disposable baseline
has no node_modules, so its producer failure is not an equivalent complete
candidate acceptance run. The candidate's UI pack failures independently
show missing symlink privilege. No conformance artifact was edited to claim
that unexecuted or failed suites passed. Required follow-up is a successful
producer run and evidence refresh on a working verification host, followed by
the full required gate. Until D06 replaces this mechanism, its existing gate
remains authoritative; D01 does not bypass it.

## Residue and scope

Runtime/app/current-contract searches find no active AuditHook, AuditRecord
or SlogAuditSink. The test retains retired log strings as negative assertions.
Checkpoint's private `payload_sha256` remains owned by D08, not this audit
facility. Necessary actionhost Journal auditing remains.
The actionhost package retains its distinct AuditRecord/SlogAuditSink vocabulary
for required bounded action auditing; these are not the removed runtime types.
The active harness roadmap no longer requires digest-only event-audit records.

The independent review verified the retained Channel/run-observer/actionhost
wiring, nil-slice safety, real App/provider/file-tool test path and synchronized
log capture. Unchanged operation-ID conflict rules, migrations and broader Face
compatibility were outside this cut; they retain their own #91 owners and are
not accepted by this review. The reviewer ran no duplicate suites.

No live network is used by the new test; its provider endpoint is loopback
and its file/Journal state is under test-owned temporary directories. No
production Journal was opened. Schema, pins and the three retained loop
components are unchanged. Product-wide #91 acceptance is not claimed.
