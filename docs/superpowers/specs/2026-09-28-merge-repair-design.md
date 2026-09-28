# Merge-chain repair — design

Status: repair baseline for the rapid merge chain on `main`.
Code baseline: `6aa00d24` (main tip at audit start), the re-uploaded repository
whose history was recreated after the identity incident on the previous remote.

## Background and findings

Between `a74ad98d` (PR #66) and `6aa00d24` a burst of merges landed five feature
lines at once: channel-tier1 (#36), goal-plan (#57), session-continuity (#60),
vivy-code-init (#61), memory (#62), and the issue-39 DAG orchestration (#64),
plus three mid-flight "merge main" syncs (`d62780f6`, `d9ac9491`, `9dd56889`,
`f22002d1`, `6aa00d24`). Conflict resolution was textual only; semantic
integration was never re-verified, and the repository was force-recreated, so
every suspicious hunk had to be re-derived rather than trusted.

### Two-generational damage

The breakage predates the mega-merge. `d62780f6` (main → work-60) dropped
`lockMessageSession` and `currentMessageWorkSeq` from
`internal/storage/sqlite/messages.go` while sibling code still called them, so
every revision from `d62780f6` through `915177cd` (`9dd56889^2`, the main tip
merged in) does not compile. `9dd56889` re-added the helpers by taking the DAG
side's newer file — a partial heal, not a deliberate repair.

### Pattern

Every real defect is a cross-feature collision: one side landed an invariant or
contract and the other side's code (or its stale copy) silently violated it.
Legitimate resolutions (migration renumbering 026–033, dead-file removal) were
distinguished from drops by tracing each lost symbol to its introducing commit.

## Defects and repairs

| ID | Defect | Merge | Repair |
| --- | --- | --- | --- |
| D1 | `internal/rpc/control.go`: duplicate `ControlDeps` fields and duplicate `RunWithOptions` literal keys (compile error) | `9dd56889` | keep single union of both field sets |
| D2 | `internal/app/app.go`: duplicate `Crons`/`Channels`/`Titles` block (compile error) | `9dd56889` | single union |
| D3 | `internal/domain/event.go`: `EventToolMounted` dropped from `EventTypes` (47-entry contract) | `9dd56889` | restore entry |
| D4 | `internal/storage/migrations/runner_test.go`: stale pins (version 33, count 33) after renumbering | `9dd56889` | re-pin to manifest 001–033 |
| D5 | channel plugins (`dingtalk`, `discord`, `feishu`, `qq`, `telegram`): `MaxMessageRunes`, both `CapabilityTarget()` methods, and matching source digests dropped — every SDK pack/inspect test failed on `source hash mismatch` | `28def9c2` resolved toward the older main-side file | restore `^1` module_v1.go, re-run `go mod tidy`, recompute `sourcehash.Tree` digests, re-pin `module_v1.go`, `vivy-module.yaml`, `conformance_results.json`, `reproduction_test.go` |
| D6 | `faces/headless` digest pin stale after `8d8e26a5` (v2-only `model.completed`) | stale golden, not a merge defect | re-pin digest |
| D7 | `config.Tools.Enabled` default lost the union of work-control tools (`agent`, `enter_plan_mode`, `submit_plan`, `get_goal`, `create_goal`, `report_goal`, `workflow`, `reply_parent`, `child_inbox`) | `9dd56889` | restore union |
| D8 | durable tool-operation coordinator ate governance refusals and interrupts: `compose.IsInterruptRerunError` was converted to a plain NodeRunError (approvals/plan review broke); `ErrGoalArmed`/`ErrStaleGoalReference`/`ErrSandboxDenied` refusals became run-fatal or lost their `policy.evaluated` journal | `fbed2ea1` + `9dd56889` interaction | `tool_operation.go`: interrupts bypass completion recording; refusal text recorded as operation result **and** the typed refusal is still surfaced to `InvokableRun` so failure marking + `policy.evaluated` journaling run; model-work tools (`enter_plan_mode`, `submit_plan`, `get_goal`, `create_goal`, `report_goal`) exempted from durable-op admission entirely (`isModelWorkTool`) because `CommitWork` self-dedups via `modelWorkIdentity` |
| D9 | `rebuildPendingChild` created `pendingRun` without a context, so resume hit a nil-parent panic at `service.go:3757` | `fbed2ea1` | create `context.WithCancel`, register in `s.active` and `s.pending` |
| D10 | resume failure on a child run emitted `run.failed` instead of `child.failed` | `fbed2ea1` + `9dd56889` | use `s.terminalEvent(ctx, m, err)` classification |
| D11 | `session.primary-run` uniqueness invariant (`runs.go`) collided with fixtures that stack active runs or `AppendMessage` without a session row | invariant introduced on main side | fixtures updated: complete the prior primary run before starting a new one (`service_test.go`, `reference_context_test.go`); seed `CreateSession` before `AppendMessage` (`deliver_recovery_test.go`, `control_test.go`); `narrow_run_selection` uses terminal `RunCompleted` rows |
| D12 | `messages.session_id+position` UNIQUE violated by bare `INSERT INTO messages` in `runs.go`/`run_admission.go`/`work_control.go` (sqlite + postgres) | `029 history_positions` migration vs older writers | all writers allocate `position` via `sqliteNextMessagePosition`/`postgresNextMessagePosition` under `lockMessageSession` |
| D13 | `reference_lifecycle_test.go`: `MaxContextBytes: 256` is below the reserved static-instruction floor (~882 bytes) — test was unreachable code | pre-existing latent defect exposed by compile fixes | raise budget to 8192 so the fold trigger still fires |
| D14 | `channelhost` requires `Deliveries` (migration 026 durable outbound intents); shutdown test fixture predates it | `3dd224e5` | pass `a.backend` as `Deliveries` |
| D15 | `sdk/internal/assembly/channel_capability_test.go` referenced the abandoned `example.com/vivy/plugins/...` module path | canonical-path rename `922b6f15` | fix to `agent-vivy/plugins/...` |

## Decisions

- **Union semantics win.** Where both sides changed the same surface
  (deps fields, tool lists, event vocabulary), the repair takes the union —
  no feature was reverted.
- **Digest authority = content.** `sourcehash.Tree` digests are recomputed from
  the restored trees and every pin (descriptor, `vivy-module.yaml`, golden
  artifacts, generated assembly) follows the content — never hand-edited to a
  hoped-for value.
- **Durable ops keep their promise.** Completed ops replay their recorded
  result; governance refusals are results (not failures) but must still fire
  the invocation-failure mark and `policy.evaluated` journal on the live path.
- **No main pushes.** All repairs land on a repair branch PR authored by the
  human maintainer (`mastwet`), per the repository identity rule.

## Verification

- `go build ./...` clean.
- `go test ./...`: 74 packages, zero failures (sqlite + in-memory paths).
- `sdk/internal/conformance`: full 23-suite provider conformance re-executed
  and byte-matched against the regenerated `conformance_results.json`.
- Postgres-dsn-backed conformance, `just ci` (PowerShell `justfile`), and UI
  end-to-end remain out of scope of the local gate set.
