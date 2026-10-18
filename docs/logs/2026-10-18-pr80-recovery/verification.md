# Verification — PR #80 recovery completion

## Candidate and environment

- Review baseline: `dfa755da63c45694528130aa6af00d4dde686f56`.
- Latest main checked through GitHub and git remote at start:
  `7b42b2920f72de6f37de8b9e4ff4bf6a110a762f`.
- Merge: `ad1aab43`; runtime fix: `e7255a7f46652541fe87716154d0e7cfa0207a95`;
  expanded faults and corrected policy: `0c083f12ad986b2a7ff3767c4839206f2e6eb846`.
- Final executable/test-harness source candidate: `d10d9f50f54eb45c3485f09157e072c54fedfadc`;
  workflow-only integration correction: `a82a6ef09690fc622ea0dc8f7ac23845af5a2686`.
  Later delivery evidence changes do not alter executable sources.
- Windows native isolated lane `.worktrees/pr80-recovery/agent-vivy`, own
  sibling Laputa `4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24`.
  Go 1.26.4, Node 24.14.1, pnpm 10.33.0, current main justfile.
- Independent Linux filesystem checkout:
  `/home/mastwet/pr80-verification/agent-vivy`, own Laputa/dependencies/scratch;
  Go 1.26.4, Node 24.21.0, pnpm 10.33.0, gcc 14.
- Tenant journals and daily home were not read or written. Test state is isolated.

## Recovery fault evidence

`go test ./internal/runtime -run '^TestRecoverCleanup' -count=1` on the
pre-fix runtime failed all seven initial injected cases because Recover
returned nil. See [recovery-red.txt](recovery-red.txt).

After the fix, `go test ./internal/runtime -run '^TestRecoverCleanup' -count=1 -v`
passed all 15 scenarios (33.043s): intent creation/deletion; Approval and
Question row write, cancellation event append, successful event append with
lost acknowledgement, terminal append, successful terminal append with lost
acknowledgement, terminal status projection; post-terminal Plan cancellation
with SQLite close/reopen. Every retry asserts exactly one cancellation and
one terminal. See [recovery-faults.txt](recovery-faults.txt).

`go test ./internal/runtime -run '^TestRecoverAcrossEngineVersion|^TestRecoverWithMissingCheckpoint|^TestRecoverCleanup' -count=1`
passed (19.589s, before the later test-only expansion). The four original
engine-version tests exercise new-code behavior, not actual historical rollback.

Independent read-only review of `ad1aab43..e7255a7f` found no substantiated
important runtime defect and confirmed the pinned Eino capability check. Its
three suggested verification gaps are now covered by lost-ack/status tests,
SQLite reopen and the separately recorded actual-binary acceptance. Unchanged
child/shell/expiry/Goal recovery and empirical PostgreSQL behavior were outside
that targeted review; no broad cleanup claim is made for those paths.

## Current gates

The integrated Windows lane completed the repository's current **`just ci`**
with **exit 0**. Full executable behavior was stable after `e7255a7f`; later
`0c083f12` changes added fault tests and corrected the Recover comment/policy.
Those final tests were also run directly and under Linux race. The final
binary/harness candidate is `d10d9f50`; `git diff --name-only d10d9f50 HEAD`
through `a82a6ef0` shows only `.github/workflows/ci.yml`, with no executable
source changes. Delivery evidence after that point is documentation only.

Windows command environment: prepend `C:/Program Files/Go/bin` to PATH and
set process-local `VIVY_GOPROXY=direct`, then run `just ci` at the lane root.
No substitutions for recipes and no extra test skips were used.
[Windows command output](windows-ci.txt) retains every non-artifact-JSON line;
large generated manifest/Inspect JSON is represented by length and SHA256, and
the complete raw log hash is recorded. The raw log remains `.workspace/ci.log`.

| Current local check | Result |
|---|---|
| `just ci`: ensure-laputa, bootstrap-test, fmt-check, ui-ci, vet, full test, memory-loop, DIVA recall, headless compile, plugin-ci | **PASS**, exit 0 |
| UI suite / i18n | 93 files, 702 tests pass; 1,575 en/zh keys and 14 shared semantic units conform |
| Full ordinary Go suite | PASS; runtime 920.903s, SQLite 256.951s, SDK internal 424.545s |
| Real DIVA memory-loop overlay | PASS, command-line-arguments 885.694s; `memory-loop: passed` |
| DIVA recall / headless / plugin-ci | PASS; all 22 independent modules passed |
| `just fmt-check` after the complete gate | PASS, 1,224 Go files formatted |
| `git diff --check` | PASS |
| Final `just doctor` | 26 OK, 1 warning, 0 failures; warning is unavailable Windows Docker CLI |
| Actual native Windows binary matrix | PASS; details below |
| Full Linux `just race-ci` | **FAIL**, exit 1; Claude upstream race, fresh main control below |
| Final focused recovery race | **PASS**, exit 0, 20.085s |

The split pair was intentionally stopped after verification. The owned backend
termination caused the dev wrapper's expected child-exited error; it is teardown,
not a startup acceptance failure. Both owned listeners (:8788/:3016) are closed.
No Docker packaging or real PostgreSQL gate was run locally; neither is claimed
by this Windows `just ci` result. Remote CI is tracked separately at delivery.

### Linux race and fresh main control

The canonical command was run unchanged on the Linux filesystem checkout:
`just race-ci` at `e7255a7f` → **FAIL, exit 1**. Full output:
[linux-race.txt](linux-race.txt). `internal/runtime` passed (442.297s), including
parallel tool paths; `storage/sqlite` passed (186.993s). The only failing package
was `internal/provider`: `TestStreamCompleteReplyEndsCleanly/anthropic-messages`
and `TestStreamCutOffIsAcceptedKnownBug/anthropic-messages` reported the same
EinoExt Claude v0.1.25 race (`claude.go:446` versus `:480`). No affected tests
were skipped; the full command ran to its final nonzero exit.

After checkout of `0c083f12`, including the extra lost-ack/status/reopen cases:

```text
go test -race -count=1 -timeout 20m -run '^(TestRecoverCleanup|TestRecoverAcrossEngineVersion|TestRecoverWithMissingCheckpoint|TestServiceRecover)' -v ./internal/runtime
```

→ **PASS**, 12 top-level tests, 20.085s, exit 0. See
[linux-recovery-race.txt](linux-recovery-race.txt). Subsequent changes affect
only the binary harness, workflow pin/comment and delivery evidence.

Same-host control at untouched main `7b42b292` used:

```text
go test -race -count=1 -timeout 15m -run '^(TestStreamCompleteReplyEndsCleanly|TestStreamCutOffIsAcceptedKnownBug)$' -v ./internal/provider
```

Untouched main contains no matching tests: its zero exit is **not** counted as
passing recovery/race evidence. Then only the exact PR diagnostic test file was
copied into that independent main checkout (test-only overlay, an untracked
`internal/provider/provider_stream_test.go`; no production source or pin changes).
File SHA256:
`a0b133718cc105994492c6963f013b488b4bda2c97172443858e7bbb7b9bc7de`.
The identical command then reproduced both Claude data races and failed, exit 1.
Main pins Eino v0.9.13 and Claude EinoExt v0.1.25. See
[linux-main-race-control.txt](linux-main-race-control.txt).

This proves the race does not require the new Eino core pin. It **does not waive
or green the failing full gate**. `EINOEXT-CLAUDE-STREAM-RACE` remains open in
`docs/TODO.md`; the full Linux/remote aggregate remains a release blocker until
that separately scoped upstream issue is resolved. No EinoExt upgrade, local
substitute, weakened assertion or full AgenticModel migration is included here.

## Actual binary acceptance

The reproducible harness uses real streamed tool calls and the ordinary RPC,
Service.Run and Eino checkpoint path, then force-kills only its own processes.
It waits for the natural organism lease expiry; it does not delete leases,
rewrite checkpoint headers or override engine versions. Read-only SQLite
snapshots validate durable rows/events as well as RPC projections. Historical
binary source is untouched main `7b42b292` (Eino v0.9.13).

Both final Windows binaries were built **without headless tags** from a
standalone acceptance clone, with its own pinned Laputa and a copy of this
candidate's built static UI assets. Neither tracked source tree was dirty.
Commands (from `.worktrees/pr80-native-acceptance/agent-vivy`):

```text
git switch --detach d10d9f50f54eb45c3485f09157e072c54fedfadc
go build -o .workspace/vivy-candidate.exe ./cmd/vivy
git switch --detach 7b42b2920f72de6f37de8b9e4ff4bf6a110a762f
go build -o .workspace/vivy-historical.exe ./cmd/vivy
go version -m .workspace/vivy-historical.exe
git switch --detach d10d9f50f54eb45c3485f09157e072c54fedfadc
go version -m .workspace/vivy-candidate.exe
node scripts/pending-run-binary-smoke.mjs --old .workspace/vivy-historical.exe --new .workspace/vivy-candidate.exe --output .workspace/final-binary-matrix --go "C:/Program Files/Go/bin/go.exe"
```

Final matrix: **PASS**, exit 0, 2026-10-18 04:13:38Z–04:17:24Z; exactly nine
loopback scripted model requests for nine genuine pauses. No recovery invoked
the model. Node's experimental SQLite notice is informational.

| Actual binary | Source / embedded VCS | Eino | SHA256 |
|---|---|---|---|
| Historical | `7b42b2920f72de6f37de8b9e4ff4bf6a110a762f`, `vcs.modified=false` | v0.9.13 | `52a6acb43c27d86cde23f42221ede4326436528ea7975e09c55590860ae26475` |
| Candidate | `d10d9f50f54eb45c3485f09157e072c54fedfadc`, `vcs.modified=false` | v0.9.21 | `e1bdb0972a49863f7e006877adafdfecf584cc2a292e08d66465019d21535c99` |

| Matrix | Durable result after first and repeated restart |
|---|---|
| Historical pauses → fixed upgrade | All three Run rows failed once with internal_error/engine-change text; Approval/Question cancellation event precedes terminal; Plan cancellation once; no intents; checkpoint pointers/checksums unchanged |
| Candidate pauses → direct historical rollback (negative control) | Old binary fails all three Runs but leaves Approval, Question and Plan **pending**, including after another restart. This proves the unsupported boundary |
| Candidate pauses → cancel under current → current restart → historical restart twice | All three Runs remain cancelled; one Run terminal and one interaction/Plan cancellation each; no active/pending work/intents; checkpoint pointers/checksums unchanged |

[Derived durable snapshots and event sequences](binary-results.json) contain
binary metadata, hashes, all nine Run outcomes and exact cancellation/terminal
counts. The raw report hash is retained there; raw scratch report lives in the
standalone acceptance clone's `.workspace/final-binary-matrix/report.json`.
No production data appears in this record.

A preliminary nested-worktree binary run also passed the matrix, but Go 1.26.4
embedded the common root VCS revision rather than that worktree's commit. It
is **not** used as final binary identity evidence. The independent clone above
resolves provenance for this acceptance; the tooling issue is recorded as
`GO-NESTED-WORKTREE-VCS` in `docs/TODO.md` §0.1.

## Main integration and workflow check

Only `docs/TODO.md` conflicted when merging main. The completed ToolsNode race
record was retained; main's development-environment scripts/recipes and
work-control fixes were not replaced wholesale.

The main Linux race job referenced a truncated pnpm Action SHA. Exact API
checks in this session:

- `gh api repos/pnpm/action-setup/commits/b906affce14559ad1aafd4ab0e942779e9f58b1 --jq .sha`
  → HTTP 422, no commit found; `git show origin/main:.github/workflows/ci.yml`
  confirms this was the integrated baseline value.
- The same request with `b906affcce14559ad1aafd4ab0e942779e9f58b1`
  → resolves that exact commit (HTTP 200). The job now uses the existing full
  SHA already pinned by the other jobs; no action version or permission change.
- `node --check scripts/pending-run-binary-smoke.mjs` and `git diff --check`
  → pass.


## Support boundary and remaining work

Upgrade to the fixed v0.9.21 binary can cancel incompatible paused
Approval/Question/Plan continuations. Direct historical rollback with pending
work is unsupported: the old binary lacks this fix. Before rollback, stop new
admission, complete/decide/cancel using the current binary, verify zero active
and pending requests/reviews/intents with complete event counts, and restart
the current binary to verify again. A cleanup error blocks rollback.
No compatibility rollback binary is supplied.

This recovery protocol does not generalize every normal `run/cancel` caller or
migrate legacy terminal orphans that were created without an intent. Source
inspection found that the unchanged normal cancellation helpers still ignore
`journalReviewEvent` failure; this is recorded as `HITL-NORMAL-CANCEL-EVENT`.
For that reason the pre-rollback contract requires **event evidence as well as
cancelled row states**; successful RPC return or an empty rendered queue alone
is insufficient. Incomplete evidence blocks historical rollback. No general
normal-cancellation failure/retry guarantee is claimed by the happy-path binary
matrix above.

No full AgenticModel migration or #79 Recap/Subject work. S4 B5 is still P2
usage-projection work; B6/B7 still need authorized live-provider checks. No
real provider keys were used. Local success is not a remote CI result.

The split pair started successfully at backend `:8788` / Vite `:3016` with
`.workspace/dev/home`; an attempted real browser open was denied by the browser
tool's saved origin permission/managed policy, including repeated user-requested
retries. Browser inspection is therefore unverified, not a pass. No workaround
was attempted. No UI behavior was changed by this follow-up.

Automatic approval review rejected cleanup of the binary-smoke agent's failed
probe scratch as blocked by policy. No alternate deletion was attempted. Those
ignored temporary files remain under the isolated
`.worktrees/pr80-binary-smoke/agent-vivy/.workspace/pr80-binary/` lane only;
no production files or unrelated root changes were touched.
