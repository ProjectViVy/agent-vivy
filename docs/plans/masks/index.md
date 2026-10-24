# MASK-43 implementation package

Epic: optional persona-subordinate session masks, P1, issue #43.
Code baseline: `5253f77dfcec6c0d212c2f41d386aeeb7619e1ef`.
Architecture baseline: local design commit `a0f892c`; [current design](../../specs/2026-09-21-mask-subsystem-design.md).
Authorization: the maintainer requested detailed planning after receiving the design,
then authorized implementation in the active session. Publication, push, merge, and
issue comments remain outside this work.

## Read order and authority

1. Read repository `AGENTS.md`, applicable local skills and the linked design.
2. Read [shared contracts](contracts.md), then the assigned Story.
3. Use [acceptance](acceptance.md) for evidence IDs and release gates.

Architecture owns decisions and requirements. Contracts owns exact proposed shared
signatures/wire shapes. This index alone owns Story state and dependencies. Story
plans own implementation steps. Acceptance owns the combined evidence matrix.
Symbols and files in the remaining planned stories are proposals until their
implementation lands. No hidden chat context is required. Do not interpret a
code snippet in a plan as existing code.

## Story graph and current state

| Story | Requirements | Outcome | Immediate predecessor and supplied output | Plan | State | Evidence / next gate |
| --- | --- | --- | --- | --- | --- | --- |
| MASK-1 | M2, M5, M7 | Closed contract, pure built-in catalog and generation wiring foundation | none | [MASK-1](MASK-1.md) | Implemented | Commits `d45f84c` through `b12014`; covered by the 2026-09-22 full Go verification. |
| MASK-2 | M3, M4 | Revisioned storage, actions and selection/fork serialization | MASK-1: contracts and typed factory binding | [MASK-2](MASK-2.md) | Implemented | Commits `c790520` through `bc2ae44`; covered by the 2026-09-22 full Go verification. |
| MASK-3 | M1, M2, M3, M6, M7, M8 | Atomic admission, Markdown composition, checkpoint-bound immutable instruction | MASK-2: storage/CAS and authorized control operations | [MASK-3](MASK-3.md) | Implemented, local verification complete | Runtime, App gate, symmetric edit-marker retry and prompt middleware are committed; full Go tests and conformance pass. `just ci`, browser smoke and live-model release gates remain. |
| MASK-4 | M4, M5, M9 | Optional Web UI, independent code mode and complete Generation evidence | MASK-3: truthful runtime capability and immutable capture | [MASK-4](MASK-4.md) | Implementation slice landed, release pending | Typed header slot, independent code mode and removable `vivy/masks-ui` source are committed. Recipe selection, legacy shell removal, artifacts, browser smoke and focused rule approval remain pending. |

Topological waves: `{MASK-1}`, `{MASK-2}`, `{MASK-3}`, `{MASK-4}`. This table is the
DAG source; no separate diagram or duplicated machine-readable graph is maintained.
No unknown/self/transitive edges are needed. All shared-file edits are serial:
App/Assembly in 1–4; storage in 2–3; Runtime/RPC in 2–4. No execution parallelism
is proposed. Independent read-only review does not confer write ownership.

MASK-1 is enabling work, not a public feature or SUPPORTED Port. Do not select an
incomplete module in a production Recipe. Seven-artifact closure happens only when
MASK-3's real runtime consumer and MASK-4's product evidence are accepted. Tests may
instantiate internal contracts directly before release selection becomes legal.

## Scope and readiness

Preserve the architecture's defaults: organism-wide custom catalog, 16 KiB body,
CAS conflict at admission, same-Generation prompt-v1 resume, database-held custom
Markdown, no new persona/memory backend. Planning uses these defaults following
the request to refine the delivered design. No repository instruction is changed.

At execution start, fetch/reconcile the actual authorized target branch, read new
rules and compare the exact overlapping files with this baseline. Record any
changed contract here before implementing dependent tasks. Especially inspect #42,
#51, PLAN/GOAL prompt assets and packaging hash changes. Do not blindly rename the
existing `recipes/minimal.vivy.yml`; lite naming belongs to its product track.

Ready requires execution authorization, accepted predecessor evidence, compatible
current source, and usable verification environment. The workspace now has a
verified Go toolchain; missing `just`, PowerShell/browser tooling, or live
credentials still blocks the corresponding release gate, not planning. Do not
promote evidence based on skipped tests. Implementation evidence is recorded in
the MASK-1/MASK-2 commits and the active MASK-3 branch.

## Handoff and changes

Give the worker this index, contracts revision from the same git commit, design,
assigned Story, predecessor evidence, root/UI rules and acceptance IDs. It owns
only its Story's listed files; source conflicts require a coordinated plan update.
Preserve cause chains internally and safe errors publicly. Never push, merge or
post issue comments without conversation authorization.

If a shared signature changes, update contracts and every consuming Story, invalidate
dependent readiness, then resume. Review cannot waive mandatory tests or rewrite
architecture merely to make a fixture pass. Use one focused commit per independently
reviewable deliverable, human repository identity only, explicit paths staged.

Recommended execution method after review: native, one Story at a time, because
shared admission and generation contracts make simultaneous write lanes costly.
