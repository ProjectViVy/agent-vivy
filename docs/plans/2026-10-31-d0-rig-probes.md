# D0 implementation: Rig upgrade and persistence seam probes

## Authorized continuation: D0 completion and D1 (2026-10-31)

The owner approved implementation of D0 completion followed by D1, using grouped
append records rather than whole-history replacement. This section supersedes
the original proposal-only/no-D1 boundary below. Spec: DESIGN's accepted owner
rulings and preliminary detailed architecture. Execute inline with focused
regressions and a final independent review; do not publish or push.

- [x] Rig opt-in grouped checkpoints: complete records, exact receipts, torn-tail
  recovery, corruption refusal, preserved first failure, unchanged ordinary logs.
- [x] Rig final-outcome persistence boundary, including stop/retry/supersession,
  and saved model/tool result restoration without external redispatch.
- [x] Required saved-key/schema preflight before mutation; repeat D0 acceptance.
- [x] Vivy runtime-feature DurablePlugin and separate persistent recipe: task
  admission/deduplication, intent/result gates, recovery, uncertainty resolution,
  serial tools, durable cancellation and single-tool input handoff.
- [x] Entry/maintenance guards and explicit local Rig development builds; recipe
  run command, isolated development locks and one Rig dependency source.
- [x] Actual Windows executable crash matrix, baseline regressions, costs,
  records and independent review. D0 must pass before D1 is claimed usable.

Interfaces: Rig owns the grouped-log adapter/receipts, required-state validation
and final-result scheduling. Vivy owns DurableAgentState (stable saved schema),
submit/inspect/cancel/resolve, task_control and operation/generation policy. No
second model/tool loop or task database. Runtime capabilities remain YAML-selected.

Acceptance: kill before/after intent, after effect/before result, after result/
before publication, and after completion; reuse committed outcomes, never blindly
repeat uncertain unsafe effects. Test failed/partial writes, corrupt complete
records, missing required state, stale receipts, duplicate input, changed route,
new input between tools, cancellation with late results and competing writers.

First guarantee is ordinary process-crash recovery only. Reuse existing retry and
resource controls. Persona/Recap/memory, wake scheduling, AutoDream, delegation,
coding-session management and owner/outsider policy are out of this slice. Reject
unsupported durable fork/rewind/clone/import/history mutation before writing.
Local --rig-path integration is development-only; production pins and recipe lock
files stay unchanged. No real-provider, OS-crash or power-loss claim from fixtures.

Implementation and executed evidence are in [HANDOFF](../HANDOFF.md#d0d1-local-runtime-2026-10-31)
and the [storage record](../research/2026-10-27-session-storage.md#grouped-checkpoints-and-d1-2026-10-31).
The Rust consumer has 22 passing tests; the actual executable passed nine crash
boundaries and Shell/Python cancellation. Ordinary coding regressions pass. Four
known Rig Windows baseline tests still fail; they are documented, not ignored.
Final implementation verification completed on 2026-11-01; both branches remain
local and the original main checkout is unchanged.
One fresh-context whole-branch review found three Important lifecycle issues;
all three were reproduced with failing tests and fixed in one pass. There were
no Critical or deferred Minor findings. The fix-pass consumer suite passes 22/22:

- Pending controls now bind origin and target generations and are invalidated
  on human stop. Sixteen combinations cover all four actions before/after control
  commit, both live and across restart.
- Ending A by defer, silent finish or normal answer preserves a foreground
  decision turn for already-delivered B, live and after reopen. The existing text
  is not appended again. Silent end records Rig's halt, rather than letting a
  trailing tool result look like interrupted work after restart.
- Cancelling B after Rig inbox acceptance withdraws its input before the next
  model request. The regression inspects the actual captured request content.

Execution rulings:

- Keep grouped encoding in a public opt-in Rig adapter, preserving logical Rig
  logs. This avoids a private Vivy store; an adapter defect would affect all
  selected logs, so conformance and interrupted writes are acceptance tests.
- Preserve conditional-length `Changed` conflicts as retryable at Store level,
  invalidating receipts. The execution Writer still latches failure. Poisoning
  the adapter on an ordinary rejected precondition would break Store conformance.
- Match route by safe provider configuration content, not an ephemeral reload
  counter that resets on restart. Opaque external routing changes remain outside
  this local identity check.
- Reuse and retain the existing ignored `.workspace/d0-d1` ledger/test evidence
  rather than creating/deleting another skill workspace; durable conclusions and
  runnable commands are in these existing docs. Cost is local scratch disk space.
- Continue already-admitted unfinished input after an ordinary task end; this
  completes foreground delivery rather than adding D3 idle scheduling. Human stop
  clears that pending continuation. Regression coverage guards against both lost
  input and unexpected execution after stop.

Review boundaries retained: OS/power/filesystem failure and actual providers are
unverified; D2-D4 and delegation remain excluded; sustained retention/performance
and the four reproduced Windows baseline failures remain later work. These limits
mean this is a local D1 experiment, not the complete always-on assistant or a
fully green cross-platform Rig release.

## Historical first D0 probe delivery

Status: implemented and independently reviewed; findings fixed with regressions.
D0 persistence gate: blocked by
reproduced gaps. Spec: [architecture baseline](../DESIGN.md#preliminary-detailed-architecture-baseline-2026-10-31).

**Owner scope amendment, 2026-10-31:** initial persistence guarantees ordinary
process-crash recovery, not survival of OS/power/filesystem failure. Missing sync
alone is no longer a gate. Complete recoverable bundles, precise checkpoint
acknowledgement, final-result ordering and required-state validation remain
required; existing successful ordinary appends do not prove them. Keep prior probe
reports as historical evidence and rerun the revised acceptance after the missing
seams are implemented. This document amendment does not make D0 pass or change
the diagnostic tests' existing results.

The first persistent agent is a new lifecycle on the shared core, separate from
coding subagents. It excludes persistent delegation, parent/child joins and
coding-session creation/management. Existing coding/subagent regression checks
below protect that separate capability; they are not durable-delegation acceptance.
Reusing the coding face is a validation convenience. Subsequent D2 uses asynchronous
Recap-only publication; existing resource controls are reused and owner/outsider
policy is deferred. See the [owner rulings](../DESIGN.md#requirements-audit-and-owner-rulings-2026-10-31).

The owner chose to upgrade first to the inspected Rig commit
`fc556f982d501eea9d7167398a19386e4884e776`. The shorter, mistyped hash in the
conversation plan is not a second candidate. Keep Rust 1.97.1 and Bevy 0.20.0.

## Deliverables

1. Upgrade every active Rig dependency and the coding recipe together. Verify
   normal coding, provider wires, sessions, subagents, Python lifecycle and the
   Windows terminal. Ordinary restored turns follow Rig's stop default; never
   globally opt them into CarryOn.
2. Add an experimental `vivy-durable` crate with an explicit `d0-probe` feature,
   a minimal saved probe component and an observing Store. Reuse Rig's loop and
   test support. Ordinary append acknowledgement is not a durable receipt.
3. Exercise holds across multiple passes, effective-request binding, matching
   saved revisions, final redaction, restoration, cancellation, incompatible
   saved state and writer ownership. Report supported, gap or unverified with
   evidence, including negative reproductions. Do not ignore failing contracts.
4. Run a synthetic loopback coding task through the actual assembled executable.
   Derive its temporary recipe from the baseline. Use isolated homes and files,
   observe event ordering and actual process-restart recovery; mocked writes
   cannot establish recoverable checkpoints. Stronger sync evidence is separate.
5. Record exact upstream-sized gaps and proposed changes in the existing
   session-storage research, and update DESIGN/HANDOFF with verified limits.

## Boundaries and acceptance

No reference checkout edits, upstream patch implementation, fork, private Store,
second runtime, D1 state machine, persona/memory/wake/evolution work or publishing.
Real-provider acceptance remains deferred. New tests stay offline; a scripted
gateway is not a real provider. Process interruption does not prove power-loss
durability. Keep ordinary recipes free of the experimental plugin.

Delivery is complete when upgrade checks and the reproducible probe/report set
are complete. The D0 persistence gate passes only if all required contracts have
real-path evidence. A reproduced blocker is a successful investigation but a
failed contract: stop at the bounded upstream proposal before D1.

Implementation uses native execution in an isolated worktree, with a final
independent review. Run focused Cargo tests, existing smoke scripts, formatting
and diff checks. Commit the upgrade separately from probes and conclusions.

## Follow-up: session ownership at every entry (2026-10-31)

Status: implemented and verified, following the owner's request to continue after D0.
This closes the observed Vivy ownership gap; synced storage and required saved
state remain separate upstream work. The production dependency pin stays fixed.
Upstream comparison on this date found only catalog updates and the 0.46 release
after our pin, with no ECS/memory/launcher source fix for the reproduced gaps.

Use the existing `vivy-process` host support and Rig's `SessionDir::hold`.
Every generated main acquires a guard before constructing Rig or any plugin.
Direct runs hold the session lease themselves. A child of our actual launcher
checks the parent's run identity and held run/session leases, then acquires an
additional execution lock; this avoids relocking the parent's lease and prevents
duplicate children. Both guards last until execution exits. The parent retains
its existing session lease across startup, execution, switching and cleanup.
Parent identity and OS locks coordinate trusted processes; they are not a sandbox.
Description-only invocations open no session. Fork holds its source and target
leases and copies no process lock files.

Review refinements: an empty description variable does not mean describe mode.
Keep the execution lock outside the session tree and retain its file identity.
Ephemeral cleanup renames the directory while holding that external lock before
recursive deletion; a newly created session at the original path remains safe.
Upstream maintenance writers still use only Rig's lease and are outside the added
launcher-loss protection. No universal Store-writer guarantee is claimed.

Moving the only lease into the child would expose launcher preparation and
post-exit switching to races. Inheriting a platform-specific lock handle would
add more platform plumbing. A checked parent handoff plus a small execution lock
preserves the existing launcher protocol and keeps the change local.

- [x] Reproduce the direct-entry failure in the actual binary and test live-source fork refusal.
- [x] Add the host guard in `crates/vivy-process/src/session.rs`, offline tests
  for lock conflicts, invalid/stale handoffs, duplicate children and release.
- [x] Wire it into generated mains, reserve the host dependency, sanitize
  inherited launcher protocol state, and lock source/target during CLI fork.
- [x] Run root tests, builds, assembled ownership/cancellation/optional-recipe
  checks and the D0 probe; update its result from ownership gap to supported.
- [x] Review and commit the focused change with evidence and remaining limits.

## Fork implementation: saved serialization failure (2026-10-31)

The owner authorizes cloning and modifying `mastwet/rig`. Use a dedicated editable
checkout at `.workspace/rig-durable`, on `feat/durable-checkpoints` based on the
audited `fc556f982d501eea9d7167398a19386e4884e776`. Reference checkouts stay unchanged.
This supersedes the earlier no-upstream-implementation boundary for the bounded
D0 fixes. Publication remains a separate action.

The first independently testable patch stops a Writer after a saved component or
resource cannot serialize. Its pending records must not flush, subsequent frames
must not resume logging, and `SessionLog::failure` must expose the original cause
with the saved key and log identity. Existing write failures and normal saving
retain their behavior. This is an integrity fix, not a synced receipt or an atomic
transaction spanning earlier flushes; it does not automatically stop a normal turn.

Files: Rig `crates/rig-memory/src/writer.rs`, ECS `session_log.rs` and its existing
tests, the crate READMEs; Vivy diagnostic tests, local fork validation helper and
the existing evidence/handoff documents. No production source pin changes before
a reproducible fork revision is available to other checkouts.

- [x] Reproduce dropped saved components/resources with pending conversation
  changes; assert the stored bytes remain unchanged and the error is observable.
- [x] Add a typed-error Writer failure entry point, preserving its first failure,
  and route reflection serialization failures through it before `WriteLogs`.
- [x] Verify both affected Rig packages and the Vivy diagnostic consumer against
  explicit local source overrides. Missing nextest uses package-scoped cargo test.
- [x] Review the concrete diff, fix reproduced issues, record evidence and the
  still-open sync/result/required-state boundaries.

Review focus: previously written records must remain readable; resource failure
must block agent records too; later frames cannot silently restart logging;
original append failures must not be overwritten; changes must compile in Rig's
WASM-compatible core. Keep owned test data isolated and offline.

Delivered locally as Rig `1ed6ee43`; integration evidence and the four unchanged
Windows baseline failures are in the session-storage record. Independent review's
diagnostic-disclosure finding was fixed with a failing then passing settled-outcome
regression. No other findings were deferred. Publication and the other D0 seams
remain separate follow-up work.
