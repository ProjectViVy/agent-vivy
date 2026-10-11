# Code SSH action plan

**Issue:** [#15](https://github.com/ProjectViVy/agent-vivy/issues/15).
**Branch:** `code-ssh`. **Baseline:** `ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666`.
**Design:** [SSH-D1](../../specs/2026-10-11-code-ssh-design.md).
**Contracts:** [SSH-C1](contracts.md). **Evidence:** [acceptance matrix](acceptance.md).

This index is the sole authority for Story state and immediate dependencies.
The design owns architecture and requirements; contracts own shared signatures;
Story files own execution steps; acceptance owns required product evidence.
All source paths marked proposed are absent at the baseline. Documentation
delivery does not authorize product code or a public/internal Host change.

## Epics and executable Stories

| Epic | Outcome | Stories |
| --- | --- | --- |
| E0: feasibility and contract | Prove public Host composition or identify the smallest blocked exception; freeze a worker-owned wire contract | SSH-00, SSH-01 |
| E1: remote execution endpoint | Root-confined files, durable receipts, bounded processes and truthful cancellation | SSH-02, SSH-03 |
| E2: plugin integration | Governed SSH transport, coherent remote tools, project resources and GUI | SSH-04, SSH-05, SSH-06, SSH-07 |
| E3: optional product closure | Explicit Recipe, removable assembly, installation and real/fault acceptance | SSH-08 |

| Story / plan | Requirements | Observable output | Immediate predecessor and consumed output | State | Release condition |
| --- | --- | --- | --- | --- | --- |
| [SSH-00](SSH-00.md) | R1, R4-R8 | H1-H6 executable feasibility ledger and exact exception proposal if needed | none | Planned | Execution authorization; current source inspection; isolated probe environment |
| [SSH-01](SSH-01.md) | R1, R3-R5, R8 | SSH-C1 typed wire, strict codecs, version/capability fixtures | none | Planned | Execution authorization; SSH-D1/SSH-C1 accepted for implementation |
| [SSH-02](SSH-02.md) | R2, R4-R5 | Worker filesystem and durable admission/result receipts | SSH-01: codecs, payloads and digests | Planned | Accepted SSH-01 evidence and Linux filesystem test environment |
| [SSH-03](SSH-03.md) | R3-R5 | Worker process/job/lease lifecycle and executable | SSH-02: workspace, receipt store and dispatch ownership | Planned | Accepted SSH-02 evidence and Linux process/fault environment |
| [SSH-04](SSH-04.md) | R1, R4-R5, R8 | Plugin-owned OpenSSH client with governed target, cleanup and receipt lookup | SSH-00: approved Host authority path; SSH-01: wire | Blocked | H1/H2/H3/H5/H6 proof; any exception approved and implemented with evidence |
| [SSH-05](SSH-05.md) | R1-R5, R8 | Governed remote ToolWorld catalog and stable-operation reconciliation | SSH-03: real worker; SSH-04: approved transport | Blocked | H1-H3 proof; exact Host signatures added to this plan before work |
| [SSH-06](SSH-06.md) | R1-R2, R5-R6 | Bound project instructions, previews and digest-checked deliverables | SSH-05: authoritative binding and operation path | Blocked | H4 proof and exact authorized resource/instruction consumer path |
| [SSH-07](SSH-07.md) | R1, R3, R5-R6, R8 | Backend-owned target controls, status, jobs and remote GUI indicator | SSH-06: coherent remote resources and shared owner | Blocked | H5/H6 proof; no constructor duplication or Action grant bypass |
| [SSH-08](SSH-08.md) | R1-R9 | Sealed optional Generation, installation instructions, real loop and omission evidence | SSH-07: complete selected contribution set | Blocked | All predecessor evidence; SDK/UI/platform/live acceptance environment |

The table contains immediate edges only. Derived topological waves:
`{SSH-00, SSH-01}`, `{SSH-02, SSH-04}`, `{SSH-03}`, `{SSH-05}`,
`{SSH-06}`, `{SSH-07}`, `{SSH-08}`. A wave is structural eligibility, not
execution authorization or a claim that blocked Stories can start.

## Gates and decision order

1. At execution start, fetch main and compare the pinned baseline, repository
   rules, supported Ports, SDK construction and overlapping branch changes.
   Update this package before implementing against changed contracts.
2. Run SSH-00 first to settle H1-H6. Worker contract/work can progress independently
   after execution authorization; it does not prove complete issue feasibility.
3. If all gaps are proven, record `ZERO-INTERNAL-PROVED` and link executable
   evidence. Otherwise record `HOST-EXCEPTION-REQUIRED`, exact files/signatures,
   consumers and alternatives. Do not implement that exception without a user
   decision. No SSH-specific code goes into `internal/` in either disposition.
4. If an exception is approved, add independently reviewable Host Stories to
   this index before coding, attach them as immediate prerequisites, and
   regenerate waves/coverage/conflicts. An approval alone is not passing evidence.
5. Release one integration Story only when its exact consumed contracts and
   predecessor results exist. No current Story is marked Ready or implemented.

H1-H6 are defined once in SSH-D1 §4. SSH-00's ledger records actual producer,
consumer, Grant, failure test and disposition for each. A namespaced remote
tool demo, Context Source bytes, successful SSH login or UI mock cannot waive
the corresponding gate.

## Ownership and conflicts

| Shared surface | Writers | Coordination |
| --- | --- | --- |
| `extensions/ssh-worker/wire/` | SSH-01; later changes by SSH-02/03/06 | Freeze SSH-C1 first; update every consumer on schema changes |
| Worker dispatch, receipts and lifecycle | SSH-02/03/06 | Serial commits; SSH-06 adds only artifact methods to the closed dispatcher |
| Plugin descriptor, owner, manager and tests | SSH-04/05/06/07 | One serial write lane; never create a package-global singleton |
| Optional Recipe, SDK packing, UI projection | SSH-08; SSH-07 consumer checks | Serial; generated files are not manually edited |
| Any generic Host contract or normative catalog | No authorized writer | Separate approved plan and evidence before release of dependents |
| This index, SSH-D1/SSH-C1 and acceptance | Coordinator | Change together; invalidate dependent readiness when contracts move |

Prefer native execution, one Story at a time. Separate read-only reviews may
check protocol and Host feasibility, but this package does not require parallel
agents or grant overlapping write ownership. Commit one reviewed concern using
the authenticated human repository identity, with explicit staged paths.

## Requirement traceability

| Requirement | Implementing Stories | Acceptance |
| --- | --- | --- |
| R1 | SSH-00/01/04/05/06/07 | A01, A05, A06, A07, A09 |
| R2 | SSH-02/05/06 | A03, A05, A06, A09 |
| R3 | SSH-01/03/07 | A04, A07, A09 |
| R4 | SSH-00/01/02/03/04/05 | A01, A02, A03, A04, A05, A10 |
| R5 | SSH-00 through SSH-08 | A01-A10 as applicable |
| R6 | SSH-06/07/08 | A06, A07, A09 |
| R7 | SSH-00/08 | A08 |
| R8 | SSH-00/01/04/05/07/08 | A01, A08 |
| R9 | SSH-08 | A09, A10 |

## Execution handoff and completion

Hand off the index, SSH-D1/SSH-C1 from one commit, assigned Story, predecessor
evidence and root/UI rules. Before a Host change read the repository's
`vivy-kernel-ci` skill; before UI implementation use the required frontend skill;
Eino adapter changes require `vivy-eino`. Read `.agents/skills/vivy-plugin/SKILL.md`
and its four normative architecture documents in all integration Stories.

Each Story returns its commit, red/green results, fixture/environment, accepted
interfaces, failure evidence and unresolved deviations. Stop only the dependent
tasks on a failed gate; do not substitute raw process/network/RPC/database paths.
Unknown outcomes remain unknown. This package is planning evidence only.
Issue closure requires A01-A10, seven-artifact closure, full CI and real product
acceptance; skipped or unavailable checks remain explicitly unverified.
