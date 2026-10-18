# Read-only dependencies and cross-package coordination

This ledger supplies the parent Epic index. It does not assign new implementation to #79/#10, #80, Laputa or DIVA, contact their authors, or change their branches/trackers.

## E5 — existing peer Recap repair

| Gate / input | Observed evidence | Required handoff / impact |
| --- | --- | --- |
| Actual peer diff | `songbai-vv/agent-vivy:claude/gallant-pasteur-g3f9qq` now resolves to `c158f2eaea29dc88ad6c6f50901c882b12a6ebe4`; ten commits ahead / zero behind #79 `b4c03754...` (53 changed files, parent receipt check) | Parent can now review actual diff. Do not retain “branch missing” as current blocker; do not claim #79 head advanced. |
| Scope of diff | Subject-pinned invalid UTF-8, provenance skip, soft tool failures, terminal-run excluded disposition, Recap capability gating, UI thinking/foreign-session approvals, limit/schema/session-delete fixes | Peer owns these changes. E6/E9 runtime/App transplants must preserve them after the selected candidate incorporates them. Exclusion semantics require explicit review: evidence cannot disappear silently to advance a cursor. |
| Delivery receipt / CI | Parent independently found no new head PR, Actions/check runs, or update to upstream #79; public verification is not aggregate CI | Wait for actual review/integration and exact-candidate CI receipt. The named Laputa patch has no publicly verified SHA/base/content; do not treat its description as reproducible delivery. |
| Main conflict | #79 metadata says mergeable=false; peer was based on #79, not current main | Compare exact trees after peer review; no wholesale merge or speculative conflict resolution in this plan. Preserve current main Provider/Toolkit/work-control fixes. |
| Windows Garden | Existing #79 declared graph fails native Garden compile; VIVY root renameio replacement is not inherited by standalone Garden | No Windows launcher/dependency script change in peer diff inventory. Gate remains open until actual native test bodies run in the supported labelled compatibility lane. Do not represent root cross-compilation as standalone Garden acceptance. |
| Browser launcher | Peer reports 2 Linux tests using a temporary Chromium config; no Windows launcher/workflow files in diff | Retain `PNPM_EXE` absolute Windows shim/setup gate; both governed Recap and historical-source/tombstone browser bodies must execute. No launcher work reassigned here. |
| Migration 046 | Peer adds paired SQLite/PostgreSQL `046_subject_interaction_disposition.sql`, runner/manifest and subject-history coverage | Review upgrade/reopen/rollback/checksum/parity through sole core Storage owner; reserve ID and prevent other packages claiming it. Existing paired migration requirements still apply. |
| Laputa patch | Peer summary says first-use cancellation recovery plus direct x/sys declaration delivered as a patch, still pinned to `27da7b9e...`. Chain reread cost was measured but cache behavior deliberately not changed; no proven replacement dependency in that record | D-E5-REPIN: wait for actual reviewed Laputa diff/full published SHA and update all three pseudo-versions plus source lock together. This is a specific dependency gate, not permission to apply its patch ourselves. |
| Candidate tests | Peer reports 743 UI tests, 2 Linux browser tests, source/storage checks; `just ci`, Windows, live PostgreSQL/provider quality not run; release-conformance mismatch remains | Record author evidence as such; inspect final candidate CI separately. Do not require DIVA installed acceptance, #66 draft activation, or paid API calls to finish unrelated engineering. |

The existing peer's terminal-run evidence disposition may affect E1's combined acceptance and E6/E9's shared `service.go` wiring. It does not create a hard prerequisite for isolated BML, ACP, diagnostics or old-proof unit work. A future combined candidate must rerun those paths and the genuine affected Recap regressions.

## E7 — existing Provider PR #80

Snapshot: head `001c563740a4ff32e1376f6bbfb2c6caaa7b22b3`, base main `7b42b292...`, open/unmerged. Existing read-only re-review completed before this planning task; this task does not repeat that claim as independent execution.

| Existing gate | Recorded disposition | Real downstream consequence |
| --- | --- | --- |
| Eino core | Upgrade v0.9.13 -> v0.9.21; EinoExt Claude remains v0.1.25 | If selected into final candidate, update Eino capability pins in E6/E8/E9 evidence and rerun shared runtime/stream/import-quarantine suites; no AgenticModel migration work here. |
| CI | Run [39099594755](https://github.com/ProjectViVy/agent-vivy/actions/runs/39099594755), PR body records Linux race FAILURE and aggregate FAILURE; Windows/Linux backend, PostgreSQL, UI/browser success | Final candidate acceptance cannot be called all-green. Existing lane owns race correction or explicit reviewed disposition; no automatic baseline waiver, skipped test or adapter swap. Re-read actual final head/run before release. |
| Recovery intent | PR introduces retryable pending-run cleanup across terminal/Plan windows | E6 review recovery and E9 approval/legacy changes must compose with this one recovery path. Preserve fault/reopen tests and do not duplicate its intent store. |
| Historical rollback | Stop work, clean under fixed binary, verify no pending/active/reviews/intents, restart and verify events plus rows | Any combined artifact rollback procedure must retain these restrictions. No rollback execution is authorized here. |
| Normal cancel limit | `HITL-NORMAL-CANCEL-EVENT` remains documented; pre-existing terminal orphans without intents not generalized | ACP cleanup and Skill cancellation tests must surface errors, not claim generalized event-write retry coverage based on #80. If a real blocker is reproduced, return it to its owner with exact test; do not absorb it silently. |
| Full migration / B5-B7 | Explicitly outside this PR; live checks require separate authorized configuration/budget | No implementation Story added; no paid test invocation. |

## Shared interfaces and file conflicts

Logical prerequisites appear only in package.md. The following are editing/acceptance conflicts; sequence these areas even when DAG nodes have no edge.

| Surface | Packages touching it | Single-writer / integration agreement |
| --- | --- | --- |
| `go.mod`, `go.sum`, `laputa-source.lock.json` | E5 #79/#10, E6 #69/#9, E7 #80, E9-02 | Parent selects one published dependency closure containing accepted required contracts. E6 must not reset #10, E5 must not drop #9, E9 must retain the published Mission gate in a compatible combined closure rather than reset to its older full source tree. No competing repins. |
| `internal/runtime/service.go`, approval lifecycle | E5, E6, E7, E9-03/06 | Sequence patches; preserve #80 cleanup intent and existing mandatory settlement. One final fault/reopen matrix; no independent fake recovery owner. |
| `internal/app/app.go`, cognitive binding | E5/E6, E9-01/02/03 | One sequential integration lane; compose peer provenance/readiness changes, Skill lifecycle and cleanup ownership. |
| Cognitive runtime/factory/contract | E6, E9-01/02/03 | E9-01 supplies intent; E9-02 supplies policy resolver signature; E6 applies typed Skill effects within that authority. No second policy/Scope owner. |
| Workflow editor/bridge/RPC/runtime/storage | E3 and E9-04 | **E3 owns the combined workflow lane.** Parent imports E9-04 tasks into that lane; one diff and evidence set. E9 does not duplicate node.inputs or idle/history launch fixes. |
| `ui/src/generated/assembly.ts`, generated Assembly, conformance results | E2/E3/E4/E5/E6/E8/E9 | Generate/attest once from final selected source after edits; no manual patching, stale evidence copying or shared staging races. |
| `internal/storage/migrations/*/046_*` | E5 peer only | Other packages reserve no 046 and edit none of the peer migrations. Snapshot intent and Skill metadata use existing stores. |
| `ui/e2e`, Playwright launch infrastructure | E1/E2/E4/E5/E6-02 | Each test owns isolated home and supported ports; share an existing launcher only through the assigned infrastructure owner. E6 creates a dedicated config only if existing configs cannot isolate its fixture. |
| Selected Face / `internal/codeface`, `sdk/port/face` / packer | E8 and possible E4 pack tests | E8 owns ACP alignment. Main `Options.In` already exists; retain default Face behavior. E4 assertions consume final packer rather than maintaining a second packer. |
| Diagnostics | E9-05 | Independent of peer feature code; preserve current main platform file identity and R1 correction. |

## Stop and change propagation

Before a later implementation branch starts, fetch/pin final source and compare touched files with this ledger. A local compatible implementation adjustment can proceed within the plan; changes to public contract, acceptance, scope, cost or dependency route require a concrete owner decision. Update affected existing plan references and invalidate candidate-bound evidence; do not reopen settled BML-first, DIVA-separate, ACP single-Face or explicit Skill approval decisions.

Final package integration requires exact VIVY/Laputa/SDK SHAs, source hashes and Generation/artifact identities, product `just ci`, actual supported platform/browser/DB evidence where required, independent review and separate human acceptance/merge decisions. No background monitoring is claimed.
