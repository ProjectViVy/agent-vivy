# SSH-06 remote project resources implementation plan

> **For agentic workers:** Use superpowers:executing-plans for native execution. Steps use checkboxes; follow the index's readiness gates before coding.

**Goal:** Load the bound remote project's instructions and expose authorized previews/deliverables with exact provenance.
**Architecture:** Plugin-owned bounded resource translation through the proved instruction/resource consumers; ContextHost and delivery authority remain canonical.
**Tech Stack:** Public Context Source/Resolver where sufficient, worker artifact methods, Go integration fixtures.
**Spec:** [SSH-D1](../../specs/2026-10-11-code-ssh-design.md) §4/7; [SSH-C1](contracts.md); baseline `ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666`.
**Epic / requirements:** E2 / R1-R2, R5-R6. State and predecessors: [index](index.md).

## Global constraints and review focus

BLOCKED on H4: current instruction/preview/delivery code uses local paths.
Exact accepted producer and consumer signatures are required before Ready.
A Context Source candidate is data, not proof of system-instruction semantics.
No custom raw download route, local-path mirror/cache or plugin card bypass.
Review instruction imports, scope denial and artifact replacement after presentation.

## Prerequisites and contracts

Consume SSH-05's authoritative binding and real governed remote operations.
Existing public contracts: contextsource.Provider.Query(ctx,Request)(Page,error)
and Resolver.Resolve(ctx,ResolveRequest)(Resource,error); ResourceReference is
scoped and versioned, ContextHost rechecks authority. Produce instruction/resource
providers only through H4's accepted path plus artifacts.stat/read on the worker.
No new SDK resource interface is assumed by this Story.

## Task 1: Project instructions and bounded remote previews

**Files:** NEW proposed plugin `context.go`, `resources.go`, `context_test.go`,
`resources_test.go`; UPDATE descriptor/manager. READ existing
`internal/runtime/project_agentsmd.go` and `workspace_files.go`; modifications
belong only to a separately approved generic Host Story.

- [ ] Write `TestRemoteInstructionsUseBoundProject`,
  `TestInstructionImportsStayBoundAndLimited`, `TestLocalInstructionsNotSubstituted`,
  `TestResourceScopeAndVersionDenied`. Use contradictory local/remote AGENTS.md
  sentinels, traversal imports, cycles, missing/oversize bytes and stale references.
- [ ] Run plugin focused tests for red. Implement through accepted H4 signatures;
  preserve current instruction authority, precedence, import budget and provenance.
  Refuse unsupported/unavailable remote instruction semantics; never silently
  add local project instructions or promote external data to system messages.
- [ ] Previews resolve relative paths under immutable binding and exact version.
  Check current tenant/workspace/session at the Host, then worker root/digest.
  Read once under byte bound; changed or inaccessible references fail clearly.
- [ ] Run plugin tests/vet, SDK verify and accepted real instruction/resource
  consumer tests. Confirm local sentinel is absent from the admitted remote
  instruction snapshot. Commit as `feat: bind SSH project instructions and previews`.

## Task 2: Explicit digest-bound deliverables

**Files:** NEW proposed worker `workspace/artifacts.go`, `workspace/artifacts_test.go`;
UPDATE `worker/dispatch.go`; NEW plugin `artifacts.go`, `artifacts_test.go`.
READ `internal/runtime/deliverable_transfer.go`; no internal edit in this Story.

- [ ] Write `TestArtifactDigestCheckedAtEveryChunk`,
  `TestArtifactReplacementNeverReturnsNewBytes`, `TestArtifactBoundsAndScope`,
  `TestRemoteDeliveryUsesExistingAuthority`. Fixture artifact exceeds inline
  output but stays inside item/set bounds; inject change between stat and read.
- [ ] Run worker `go test ./workspace ./worker -run Artifact -count=1` and plugin
  `go test ./... -run '(Artifact|RemoteDelivery)' -count=1` for red.
  Implement stat/read with SSH-C1 exact-digest/offset/chunk rules and allowed
  project roots. Handle partial reads and digest changes honestly; an explicit
  presentation creates a scope/binding/digest reference, never a raw host path.
- [ ] Route download through the accepted delivery authority with permissions,
  item/set budgets and safe filename/media metadata. Worker state/output/private
  receipts cannot be presented as project artifacts. UI consumes this result in
  SSH-07 without alternate blob/file access.
- [ ] Run both modules' tests/vet and actual selected preview/download consumer
  fixture. Record A06 byte/digest comparison and denial/change cases. Commit as
  `feat: deliver exact remote SSH workspace artifacts`.

## Acceptance and handoff

Return A06, exact instruction/resource/delivery signatures and scope/version
evidence for SSH-07. If any existing authority cannot consume a remote resource,
remain blocked and update H4's concrete seam proposal. Do not weaken R2/R6 to
cards or opaque context bytes. Rollback removes only owned references/fixtures;
no operator project, credential or receipt deletion.
