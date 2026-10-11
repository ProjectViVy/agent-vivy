# SSH-05 governed remote tool integration implementation plan

> **For agentic workers:** Use superpowers:executing-plans for native execution. Steps use checkboxes; follow the index's readiness gates before coding.

**Goal:** Run remote file/process/job operations under one frozen Host binding and the existing approval/Journal operation authority.
**Architecture:** Public ToolWorld Provider → plugin Manager → worker; no replacement of protected tools or parallel orchestration loop.
**Tech Stack:** Go, existing ToolHost/Policy/coordinator fixtures, SSH-C1 worker.
**Spec:** [SSH-D1](../../specs/2026-10-11-code-ssh-design.md) §4-7; [SSH-C1](contracts.md); baseline `ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666`.
**Epic / requirements:** E2 / R1-R5, R8. State and predecessors: [index](index.md).

## Global constraints and review focus

BLOCKED on H1-H3 and accepted scope/admission/reconciliation signatures from
SSH-00. Never inject random operation identity in middleware, trust model scope,
edit raw Journal or provide core Ports. Protected local IDs remain unchanged.
Namespaced tools prove a subset; coherent remote standard-tool behavior requires
the proved existing routing/composition path and H4 closure, not UI hiding.

## Prerequisites and contracts

Consume SSH-03's real executable/job/receipt behavior and SSH-04 transport/
Manager. Implement `toolworld.Provider` exactly: Definition, Discover, Invoke,
Close. Existing Invoke receives no stable scope at baseline; SSH-00 must attach
exact accepted producer signatures before Ready. Produce namespaced catalog,
binding-aware results and reconciliation through the existing local authority.

## Task 1: Typed remote catalog and tool policy

**Files:** NEW proposed plugin `tools.go`, `schemas.go`, `binding.go`,
`tools_test.go`, `binding_test.go`; UPDATE module/descriptor/manager.

- [ ] Define closed namespaced IDs `ssh_list_dir`, `ssh_read_file`,
  `ssh_search_files`, `ssh_write_file`, `ssh_patch`, `ssh_multiedit`,
  `ssh_execute`, `ssh_jobs_list`, `ssh_jobs_output`, `ssh_jobs_cancel`,
  `ssh_operation_status`. Bind schemas to SSH-C1; no schema exposes target,
  profile, Grant, Run or operation authority overrides.
- [ ] Write `TestRemoteCatalogKeepsProtectedIDs`, `TestScopeComesFromHost`,
  `TestApprovalFreezesBinding`, `TestResumeRejectsChangedBinding`,
  `TestDeniedToolNeverReachesWorker`, `TestRemoteUnavailableNoLocalFallback`.
  Use actual ToolHost approval fixtures on the accepted integration path;
  record source paths before any approved Host test edit. No internal test edit
  is authorized merely by this blocked plugin plan.
- [ ] Run plugin `go test ./... -run 'Test(Remote|Scope|Approval|Resume|Denied)' -count=1`
  for red. Implement schemas/effects/descriptions and typed payload translation.
  Mutations/execution/cancel use write/external effect treatment as supported;
  queries remain reads. Backend scope overrides no model field because such
  fields are rejected. Discover/Definition do not connect as a side effect.
- [ ] Preserve target/root/policy/build and stable IDs from admission through
  approval/resume. Use approved remote standard-tool route if it exists; otherwise
  mark full issue routing unavailable and keep dependent release blocked.
  Run plugin tests/vet, SDK verify and the accepted ToolHost/coordinator suites.
  Commit as `feat: expose governed remote SSH coding tools`.

## Task 2: Receipt-to-Journal reconciliation and real worker path

**Files:** NEW proposed plugin `reconcile.go`, `reconcile_test.go`,
`worker_integration_test.go`; UPDATE manager/binding only within accepted contracts.

- [ ] Add `TestLostReplyReturnsRetainedReceipt`, `TestLocalSettlementFailureUnknown`,
  `TestMissingReceiptNotReplayPermission`, `TestDuplicateIdentityRunsOnce`,
  `TestIdenticalCommandsWithNewIDsRunTwice`, `TestCancelUsesOriginalJobBinding`.
  Count effects in a remote fixture project and inject breakpoints between
  local claim, remote admission, side effect, reply and local settlement.
- [ ] Run focused tests for red. Implement operations.get lookup and the exact
  accepted reconciliation bridge. A terminal remote receipt is evidence offered
  to the core coordinator, not permission to write Journal or declare settled
  success independently. Missing/conflicting/unknown receipts preserve unknown.
- [ ] Exercise actual worker over verified SSH through approved Host with
  list/read/search/edit/command/jobs/output/cancel. Assert every result has the
  same origin and binding; local sentinel files remain unchanged. Foreground
  client polling uses bounded output and allows renew/cancel to interleave.
- [ ] Run plugin `go test -race ./... -count=1`, `go vet ./...`, SDK verify and
  focused existing Host/runtime suites selected from real test names. Record
  A05 and contributing A03/A04 results; commit as
  `feat: reconcile remote SSH operations through Host authority`.

## Acceptance and handoff

Return frozen binding/approval/replay evidence, ToolHost effect/Grant projection,
real worker effect counts and exact scope/reconciliation signatures for SSH-06.
No tool name alias or local-path URI trick is acceptable. Stop dependent work
if core settlement or routing is unproven. Rollback closes owned connections;
retained uncertain operations stay attached to their original target.
