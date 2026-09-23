# Nudge Delivery Index

Revision ND-D1, 2026-09-23. Issue [#58](https://github.com/ProjectViVy/agent-vivy/issues/58).
Authoritative engineering contract: [NUDGE-DESIGN](../../architecture/NUDGE-DESIGN.md).
User authorized detailed architecture and an executable plan package. Product implementation has not been authorized in this turn. These documents are complete planning instructions; they do not claim accepted implementation evidence.

## Epic and Story DAG

One Epic: **recover from supported tool failures inside the existing Run, with bounded advisory correction and auditable stop behavior**.

| Story | Requirements | Deliverable | Immediate predecessors / accepted input | Status | Plan / gate |
| --- | --- | --- | --- | --- | --- |
| ND-0 | N3, N5 | Executable pinned-Eino ordering and correlation evidence | None; inspected baseline | Planned | [ND-0](ND-0.md); toolchain + execution authorization |
| ND-2 | N2, N3, N4, N5 | Typed outcome records, single detector, durable batch settlement | ND-0: verified event/model ordering | Blocked | [ND-2](ND-2.md); predecessor evidence |
| ND-1 | N1, N3 | Selective failure producers across direct/enhanced/MCP | ND-2: run-local metadata/state and Journal contract | Blocked | [ND-1](ND-1.md); predecessor evidence |
| ND-3 | N2, N3, N4, N5 | One transient audited nudge at model handoff | ND-1: classified failure producers integrated with ND-2 state | Blocked | [ND-3](ND-3.md); predecessor evidence |
| ND-4 | N1–N5 | Integrated CI, real-path recovery and handoff evidence | ND-3: complete integrated feature | Blocked | [ND-4](ND-4.md); predecessor evidence |

Computed topological waves: **{ND-0} → {ND-2} → {ND-1} → {ND-3} → {ND-4}**.
Story IDs retain their responsibility labels; execution order follows dependencies, not numerical ID. The table is the authoritative DAG. No unknown IDs, self-edges, cycles or redundant transitive edges.

Use one native sequential lane. ND-2 owns the outcome type definitions and state before ND-1 adds classifiers/producers; this removes a metadata dependency that would otherwise be hidden in parallel work. ND-2 owns mapper/Service/schema settlement, ND-1 owns tool adapters and MCP, ND-3 owns middleware wiring. They touch shared runtime files sequentially. No parallel implementation lane is released by this package.

## Shared contract and review focus

All workers consume ND-D1, AGENTS.md, this index, their plan and actual predecessor evidence. All signatures live in design §4; Story snippets are behavioral pseudocode unless expressly called Go. Do not copy competing contract definitions between plans.

Five concrete review risks and owners:

1. Model request begins before tool results are durably recorded: ND-0, ND-2, ND-3.
2. Softening cancellation, storage failure or uncertain mutation into a safe retry: ND-1, ND-4.
3. MCP IsError or enhanced media loses typed failure identity: ND-1, ND-4.
4. Parallel ordering, Journal failure or terminal stop strands a waiting model: ND-0, ND-2, ND-3.
5. Retry/resume/compaction duplicates synthetic reminders or turns them into user authority: ND-3, ND-4.

## Execution environment and gates

Required toolchain: Go 1.26.4 (go.mod), just, the PowerShell environment expected by justfile, Node/pnpm for ui-ci, and dependencies required by the existing CI workflow. Inspect `.github/workflows/ci.yml`; do not rewrite it for this feature. Current planning environment has Eino module source but no Go or just on PATH. Commands below are future gates, not recorded passes.

ND-0 becomes Ready when execution is authorized and its toolchain is available. ND-2 becomes Ready only after ND-0's evidence resolves the source-backed assumptions; ND-1 additionally requires the accepted ND-2 state contract. Later waves require predecessor acceptance. A plan existing or a unit test passing alone does not release a Story.

Final repository gate: `just ci`; focused commands are development checks, not replacements. Relevant race command: `go test -race -timeout 20m ./internal/runtime -run 'TestNudge|TestToolFailure' -count=1` on a supported race-toolchain host. Real smoke uses an isolated temporary workspace/database, never production data.

## Scope and change control

All five Stories have file ownership, contracts, steps, failure cases, verification and return evidence. No Story may implement steering, Goal scheduling, automatic retries, a public ToolWorld schema change or generic guard registry. Escalate public contract/scope changes. A failed ND-0 integration assumption requires a source-backed design revision before downstream work; do not improvise a second execution path.

When a shared contract changes, revise the authoritative design, affected plans and this index together, and invalidate downstream evidence. Only accepted runtime evidence changes status to Done. This task performs no background supervision.

## Coverage and handoff

N1 → ND-1/4; N2 → ND-2/3/4; N3 → ND-0/1/2/3/4; N4 → ND-2/3/4; N5 → ND-0/2/3/4.

First execution task: ND-0. Recommended method: native sequential execution, since the pipeline shares lifecycle/Journal contracts and yields little benefit from extra coordination. Implementation awaits the user's next instruction.

Planning evidence: [verification](../../logs/2026-09-23-nudge-design/verification.md).
