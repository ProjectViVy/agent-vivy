# SSH-00 public Host feasibility implementation plan

> **For agentic workers:** Use superpowers:executing-plans for native execution. Steps use checkboxes; follow the index's readiness gates before coding.

**Goal:** Produce evidence for each H1-H6 gap and release integration only with a real public authority path.
**Architecture:** Existing Host/Port/Assembly authority; read-only source analysis and isolated public-Module probes, never a speculative internal SSH subsystem.
**Tech Stack:** Go 1.26.4, existing SDK verifier/compiler and Host conformance suites.
**Spec:** [SSH-D1](../../specs/2026-10-11-code-ssh-design.md) §2/4; baseline `ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666`.
**Epic / requirements:** E0 / R1, R4-R8. State and predecessors: [index](index.md).

## Global constraints and review focus

No production source or normative contract change is in this Story. Do not add
an `internal/ssh` package, raw network/process calls, a core Port, reserved tool
override, global singleton or fresh random ID in pre-tool middleware. Verify
actual generated constructors and runtime consumers, not just descriptor support.
Two identical intentional commands must remain distinguishable from one retry.
Status must not create a connection. Review every approval/resume boundary.

## Prerequisites and contracts

Read root AGENTS.md, the plugin skill and four standards. Existing inputs:
`toolworld.Provider.Invoke(context.Context, Host, string, json.RawMessage)`,
`toolworld.Host.Spawn(context.Context, SpawnSpec) (Proc,error)`,
`controlaction.Host.InvokeTool` and `StartRun`, Context Source/Resolver and Status
contracts. None currently exposes a verified Run/operation scope to ToolWorld.
Output is a ledger, exact existing signatures if proven, or an exception proposal;
no proposed accessor becomes available merely by being written in the ledger.

## Task 1: Inspect and reproduce each gate

**Files:** READ `sdk/port/{toolworld,pretool,controlaction,contextsource,status}/*.go`,
`sdk/port/catalog.go`, `sdk/internal/assembly/runtime_generate.go`,
`sdk/internal/source_verify_v1.go`, `internal/app/assembly_worlds.go`,
`internal/runtime/{tool_operation,tooladapter,isolation,project_agentsmd,workspace_files,deliverable_transfer}.go`.
NEW proposed `docs/logs/YYYY-MM-DD-code-ssh-00/{summary,verification,acceptance}.md`
and `docs/superpowers/plans/code-ssh/host-gate.md`. Public probe sources live in
a disposable directory outside production; preserve their exact source in the log.

- [ ] Reconcile current branch/main and list changed inspected paths. Capture
  exact revisions and scheduled Port phase. Run existing contract suites:
  `go test ./sdk/port/toolworld ./sdk/port/controlaction ./sdk/port/contextsource ./sdk/port/status -count=1`.
  They establish baseline behavior, not H1-H6 completion.
- [ ] For H1-H3, trace admission → approval → middleware rewrite → persisted
  revalidation → Provider.Invoke → settlement. Record what the Provider actually
  receives. Reproduce target/argument tampering and interrupted-operation behavior
  using existing coordinator fixtures. Run
  `go test ./internal/runtime -run '^(TestToolOperationCrashMatrix|TestApprovalResumeReusesPreparedToolOperation|TestBrokerOperationCompletionWriteFailureBlocksReplay|TestToolApprovalArgumentsHashCanonicalizesJSON)$' -count=1`.
  Confirm these baseline names with `rg` after branch reconciliation.
  Do not invent test names or add a fake Host accessor to make a probe pass.
- [ ] Verify that revalidation recomputes effective arguments from persisted
  middleware inputs; document why a regenerated random ID breaks equality.
  Trace the supported source of stable identity. Absence is a failing gate,
  not permission to let the model supply operation_id.
- [ ] For H2/H6, trace net.client/profile and proc.spawn enforcement into actual
  Spawn and ActionHost bridges. Probe denied executable/target, unauthenticated
  action and unsupported Action proc.spawn/secret.read Grants through current
  compiler/Host fixtures. Require denial before any process/network side effect.
- [ ] Run baseline `go test ./internal/app -run '^TestGeneratedToolWorldEnforcesGrantConstraints$' -count=1`,
  `go test ./internal/actionhost -run '^(TestActionHostDeniesMissingEffectiveGrant|TestNotebookActionHostUnauthenticatedWritesNothing)$' -count=1`
  and `go test ./sdk/internal/assembly -run '^TestGenerateRuntimeAssemblyReusesExplicitAuxiliaryToolWorldProvider$' -count=1`.
  These tests prove only their existing cases, not missing remote target/profile
  enforcement. Record the exact untested path as GAP. Include the literal host-
  key/credential persistence criterion and resolver-owned safe output handling
  in H2's decision; a normal known_hosts file is not a no-cleartext attestation.
- [ ] For H4, use different local/remote fixture instruction and file sentinels.
  Trace instruction import, preview and deliverable resolution. A path that reads
  local bytes or lacks a public remote consumer records GAP; Context Source
  returning remote bytes alone does not prove instruction authority.
- [ ] For H5, generate a disposable public Module selecting the exact proposed
  ToolWorld/Action/Context/Status/UI contribution set with current `sdk verify`
  and compile/pack fixture paths. Count constructors/Close calls and inspect the
  generated imports/projection. Assert one explicit owner, once-only cleanup and
  status reads without Spawn. No globals or hand-edited generated output.

## Task 2: Decision and exact integration handoff

**Files:** NEW proposed `host-gate.md`; UPDATE this index and SSH-04 through SSH-08
only when their inputs are verified. Proposed exception details belong in the
ledger, not in production `sdk/` or `internal/` files.

- [ ] Ledger rows H1-H6 contain: exact producer/consumer/signature, authority,
  Grant/profile, fixture/command/result and satisfied/gap. Attach source excerpts
  and executable failing evidence where current APIs permit a reproducer. If an
  API is absent, identify the absent producer and concrete blocked consumer;
  do not claim a passing mock supplies it.
- [ ] Choose exactly `ZERO-INTERNAL-PROVED` when all six rows pass, otherwise
  `HOST-EXCEPTION-REQUIRED`. For every required exception propose the smallest
  generic signature and exact files, all consumers, seven artifacts, lifecycle,
  crash/replay semantics and no-change alternative/cost. Keep SSH details out of
  generic core. Public source must still import no internal/Eino.
- [ ] Self-review protected IDs, trust levels, no raw storage/credentials and
  optional omission. Run `git diff --check`; link executed evidence in A01.
  Commit documentation as `docs: record SSH public Host feasibility gates`.

## Acceptance and stop conditions

Return A01, exact baseline and gate disposition. This Story can finish with a
truthful exception-required result, while full integration remains blocked.
Only the user can approve a necessary Host exception; then add Host Stories and
exact accepted signatures to every consumer before execution. No exception is
implemented here. Delete only disposable fixtures/owned processes on rollback;
do not alter local defaults, operator credentials or issue status.
