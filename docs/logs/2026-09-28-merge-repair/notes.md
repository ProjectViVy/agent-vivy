# 2026-09-28 merge-repair — iteration log

## Scope

Audit and repair the merge chain `a74ad98d..6aa00d24` on the re-uploaded
`main` (channel-tier1 #36, goal-plan #57, session-continuity #60,
vivy-code-init #61, memory #62, DAG orchestration #64, plus mid-flight syncs
`d62780f6`, `d9ac9491`, `f22002d1`, `9dd56889`, `6aa00d24`).

## Audit findings

- `d62780f6` (main → work-60) dropped `lockMessageSession` /
  `currentMessageWorkSeq` from `internal/storage/sqlite/messages.go` while
  sibling code still called them: every revision `d62780f6`..`915177cd` does
  not compile. `9dd56889` partially healed it by taking the DAG side's newer
  file.
- `9dd56889` left textual-conflict artifacts: duplicated `ControlDeps` fields,
  duplicated `RunWithOptions` literal, duplicated app fields, dropped
  `EventToolMounted`, stale migration pins (26 → real 33).
- `28def9c2` resolved all five channel `module_v1.go` files toward the stale
  main side: lost `MaxMessageRunes` + both `CapabilityTarget` methods, and the
  embedded source SHA256 was set to a value matching no real tree — every SDK
  pack/inspect test failed on `source hash mismatch`.
- Durable tool-operation coordinator vs governance refusals/interrupts:
  `InterruptRerunError` was flattened to a NodeRunError (approval/plan-review
  suspension broke); governance refusals became run-fatal or silently lost
  their `policy.evaluated` journal.
- New invariants collided with older writers: one-active-primary-run per
  session; `messages.session_id+position` UNIQUE without position allocation;
  `AppendMessage` requiring a session row; `channelhost` requiring a durable
  `Deliveries` store.
- Latent defect exposed: `reference_lifecycle_test.go` used
  `MaxContextBytes: 256`, below the reserved static-instruction floor — it
  could never have run green on the revisions that contained it.

## Verification

- `go build ./...` clean.
- `go test ./...`: 74 packages, zero failures.
- `go test ./sdk/internal/conformance/`: 23 provider suites re-executed and
  byte-matched to the regenerated `conformance_results.json`.
- `pnpm install --frozen-lockfile` + `pnpm run stage:ui` used for the SDK/UI
  toolchain tests (Node 24.9.0, pnpm 11.19.0, matching CI).

Not covered locally: `just ci` (PowerShell `justfile`), Postgres DSN-backed
conformance (`VIVY_POSTGRES_TEST_DSN` unset), browser E2E.
