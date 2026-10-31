# Session storage and durable state: reference comparison

## Grouped checkpoints and D1 (2026-10-31)

This implementation supersedes the older missing-seam findings **only for the
local Rig fork**. Ordinary pinned-upstream probe results below remain historical
characterizations. Reproduction commands and checkout locations are in
[HANDOFF](../HANDOFF.md#d0d1-local-runtime-2026-10-31).

Rig owns the opt-in `CheckpointStore`, `Writer::open_checked`, required saved
keys/versions, and `PersistOutcomes`/`CommitBarrier`/`FinalOutcome` in `Finalize`.
Vivy selects `DurablePlugin` with `runtime`; it owns admission identities, task
generations, frozen intents, exact receipt matching, retry restoration, explicit
unknown effects and task controls. Both use Rig's existing conversation folding,
model/tool execution, holds, cancellation, storage and inbox. There is one
execution path and no separate persistence database. The local build verifies a
single Rig dependency source and isolates development locks/targets.

Executed Windows evidence (synthetic offline/loopback fixtures):

| Check | Result |
| --- | --- |
| Root workspace tests | 34 passed |
| Vivy basics library regression | 62 passed with default features |
| Local D0 checkpoint + D1 runtime consumer | 22 passed, including three review regressions |
| Previous fork acceptance | 12 passed |
| Rig memory, all features | 62 unit, 11 checkpoint, 2 migration and 16 doc tests passed |
| Rig ECS, all features | 49 unit, 12 other integration, 70 stage and 7 doc tests passed; 4 pre-existing Windows stage failures remain |
| Scoped Rig library Clippy | Passed with warnings denied |
| Rig memory/ECS WASM libraries | `wasm32-unknown-unknown` compilation passed; no browser runtime claim |
| Actual D0 executable | Intent/result ordering; absent plugin, malformed required state, corrupt group, fork refusal; logs/files preserved on refusal |
| Actual D1 process crashes | Nine boundaries passed; saved replies/results not resent; uncertain tool not replayed; competing writer refused |
| Human stop | Real Shell descendant and Python workbench worker stopped; cancellation persisted; no request after restart |
| Ordinary coding, provider, native subagent and ownership smokes | Passed |

The nine boundaries are before/after model intent, model reply before result
commit, after model result, before/after tool intent, external effect before result
commit, after tool result, and after task completion. Each reopens the actual
executable on a disposable session. The last completed/no-input reopen exits 2,
as Rig normally does without a prompt. The uncertain cases end suspended with
evidence retained, not with a fabricated successful result.

Observed costs on this machine: the first recorded full persistent executable
build after warm dependency compilation took 58.37 seconds; an incremental matrix
build took 24.20 seconds; the final persistent recipe rebuild took 10.51 seconds.
The final one-tool crash fixtures produced 13–16 groups and
365,033–654,462 bytes per log. Reopen plus completion took 0.191–2.817 seconds,
including configured retry delay where applicable. These are fixture observations,
not a cold-build benchmark or a performance improvement claim. Grouped appends
avoid rewriting the entire physical log, but the required state currently repeats
accumulated operation snapshots. Retention/compaction remains future work.

Known ECS failures reproduced on the unchanged baseline: Unix footprint-path
expectations in `a_footprint_a_plugin_declared_on_a_call_stands`,
`a_held_call_holds_back_only_the_calls_that_touch_what_it_writes`,
`gate_sees_footprint_paths_normalized`, plus
`a_stopped_call_says_whether_it_ran_and_whether_it_may_have_changed_something`.
No full-suite-green claim is made. Raw logs/reports are retained in the ignored
`.workspace/d0-d1` and `.workspace/d0/fork-probe` directories.

This proves the bounded ordinary-process-crash contract, not OS/kernel/power-loss
survival or a real provider's behavior. D1 uses ordinary conversation context for
execution validation; D2 still must supply persona and asynchronous Recap-only
publication. There is no persistent delegation, scheduled wake or AutoDream here.

## Original research and later decision history

Date: 2026-10-27. This is source research and a provisional direction, not an
approved implementation plan or a measured performance claim. The owner asked
how Prime and Pi store sessions and what the two Vivy harnesses should reuse.
The current project is a Windows coding experiment; the durable harness and
shared memory integration are not yet implemented.

Decision update, 2026-10-31: the owner selects
[Plan B](../DESIGN.md#plan-b-durable-execution-within-the-existing-rig-chain-owner-2026-10-31),
adding durable execution within the existing Rust/Rig chain and validating it
through the coding face. The comparisons below retain their original source
scope and dates. They inform the detailed storage contract; they do not select
a separate engine or establish newly tested recovery guarantees. Consult HANDOFF
for later workbench/native-memory implementation status.

The owner's subsequent synthesis is covered by the
[preliminary detailed baseline](../DESIGN.md#preliminary-detailed-architecture-baseline-2026-10-31).
It specifies receipt-gated Rig execution, with a framed Store preserving logical
Rig logs only as a conditional fallback after D0. The baseline's commit/recovery
contract and explicitly unproven integration seams take precedence over older
implementation suggestions below; this research is not runtime validation.

**Owner scope amendment, 2026-10-31:** the
[accepted audit rulings](../DESIGN.md#requirements-audit-and-owner-rulings-2026-10-31)
select ordinary process-crash recovery for the first persistent agent. Stronger
synced durability is a later option. The executed D0 tables retain the exact
observations under the original contract; missing sync is no longer by itself a
first-phase blocker. Complete checkpoint/result ordering and required-state
validation still need implementation and acceptance. Coding subagents and coding
sessions are separate facilities, not part of first-phase persistent delegation.
No runtime or test-result change accompanies this amendment.

## Rig fork: saved serialization failure (2026-10-31)

After the local ownership fix, the owner authorizes source changes in
[`mastwet/rig`](https://github.com/mastwet/rig). A dedicated local checkout, branch
`feat/durable-checkpoints`, starts at the audited `fc556f9` revision. Production Vivy
dependencies remain pinned until a fork revision is available to other checkouts.

The first patch closes the serialization omission in the local fork: ECS calls
`Writer::fail` when a saved component or resource cannot serialize. The shared
Writer latch retains the first error and refuses all pending and subsequent flushes;
earlier stored records are not rolled back. Regression tests cover component and
resource failures, readable prior bytes, multiple pending logs, later frames,
attempted restarts/forks, and preserving an earlier append error.

Independent review found a new disclosure route: the stage's save-failure tool
result interpolated the serializer's arbitrary diagnostic. An executed regression
reproduced the text in a settled outcome. The tool result now gives a generic
storage failure while `SessionLog::failure` and local notices retain the cause.
This does not claim disk sync, cross-flush/blob atomicity, required-state validation,
rollback, automatic turn cancellation or the D1 runtime.

`scripts/rig_fork_probe.py` generates an ignored standalone manifest/lock, preserving
the Vivy plugin source but selecting Rig's local packages transitively. It records
the checkout revision and dirty-diff hash and runs 12 offline diagnostic tests.
The same acceptance test fails on the stock pin with an actually started tool and
passes on the patched source. The helper changes no normal recipe, Cargo pin or
checked-in lock. This is local integration evidence, not a published dependency.

The package-level Windows run exposed four `rig-ecs/tests/stages.rs` failures.
All four reproduce on an unmodified detached `fc556f9` worktree:

- `a_footprint_a_plugin_declared_on_a_call_stands`
- `a_held_call_holds_back_only_the_calls_that_touch_what_it_writes`
- `a_stopped_call_says_whether_it_ran_and_whether_it_may_have_changed_something`
- `gate_sees_footprint_paths_normalized`

The footprint fixtures use Unix-style paths, which become unrestricted scopes on
Windows; the stopped-call fixture times out. The patch does not change these paths.
Keep these failures visible rather than claiming a green full Rig suite. A separate
Windows compilation failure in non-Unix `rig-tools::process::kill_group` was fixed
by documenting that public function, without weakening `missing_docs` or changing
runtime behavior. Nextest is unavailable locally, so checks use package-scoped
`cargo test --locked`. Detailed run logs remain under ignored `.workspace/d0`.

Final local Rig commit: `1ed6ee437e772ea120f2efadf9aa6001e5db67b5`, with a clean
checkout recorded by the repeated consumer run. No push or PR was performed.

| Executed check | Result |
| --- | --- |
| Rig `cargo test --locked -p rig-memory --all-features` | 80 passed, including 16 doctests |
| Rig `cargo test --locked -p rig-ecs --no-fail-fast` | 129 passed including 7 doctests; the 4 named baseline stage failures remain |
| Rig `cargo clippy --locked -p rig-memory -p rig-ecs --all-targets --all-features` | Passed |
| Rig `cargo check --locked -p rig-memory -p rig-ecs --target wasm32-unknown-unknown` | Compiled; no WASM runtime test claimed |
| Vivy `python scripts/rig_fork_probe.py --rig <checkout>` | 12 passed against the clean local commit |
| Vivy pinned `cargo test --manifest-path plugins/vivy-durable/Cargo.toml --features d0-probe --locked` | Original 12 characterizations passed |
| Python D0 checks | 5 passed |

All checks used `CARGO_TARGET_DIR=C:/vtd0`, `CARGO_PROFILE_DEV_DEBUG=0` on Windows.
The unmodified baseline comparison is in `fork-baseline-stages.log`; the patched
suite is in `fork-ecs-final.log`. The privacy regression first failed with the raw
serializer diagnostic in a settled tool outcome (`fork-review-red.log`) and then
passed in the final ECS suite. During fixture construction a settled entity was
collected before inspection; the final test observes settlement directly instead.
No whole-workspace, hosted CI, real-provider or power-loss acceptance is claimed.

## D0 follow-up: local ownership (2026-10-31)

The continued implementation closes the host ownership gap reproduced below.
`vivy-process::session` now supplies a guard to the launcher and every generated
main, before app construction. Direct entry holds `SessionDir::hold` plus an
execution lock. A launcher retains the former across preparation/run/switching
and a checked child holds the latter. The child verifies the actual parent PID,
canonical run directory and held parent run/session leases. The second lock
prevents duplicate children and reentry during cleanup after a launcher dies.
It resides outside the session tree at `RIG_HOME/sessions.vivy-locks/<session-id>`;
small lock files remain in place so unlinking cannot create a second lock identity.
This is coordination of trusted executables, not authentication of hostile code.

Fork takes both source/target guards and copies no process lock files; a live
source is refused before creating a target. A regression also reproduced that
ephemeral cleanup could delete a session another owner had acquired. Cleanup now
takes both guards and reports retention instead of deleting an active session.
Under the external execution lock it releases the internal lease, atomically
renames the session to a unique `.vivy-deleted-*` sibling, then deletes that path.
A new owner at the original path is unaffected. Failed removal reports the retired
path for cleanup. An internal execution lock prevented renaming on Windows;
the external namespace avoids that limitation without an unlocked deletion window.
Description-only runs acquire no session only for a nonempty `RIG_DESCRIBE`,
matching Rig. An empty value cannot bypass ownership. Nested binaries inheriting only a
launcher path remain ordinary direct runs. Runtime/credential selection and YAML
optional plugin selection retain their existing roles. Regenerate/rebuild old
binaries to gain this protection; arbitrary callers of DirStore are not covered.
Upstream maintenance writers (`edit_stopped`, `rewrite_stopped`, `DeleteSession`)
still consult only the Rig lease, not the external lock. Their behavior after
launcher loss but before child cleanup remains a coverage limitation; this change
does not claim universal writer exclusion or alter those upstream paths.

`scripts/session_ownership.py` uses actual processes and a local scripted gateway.
It verifies that refusals precede requests and logical log changes, that no failed
live-source fork leaves a target, and that normal exit and killed processes free
the locks. It checks inherited launcher-control isolation, nested direct execution
and ephemeral deletion too. `scripts/durable_probe.py` now requires direct-entry
refusal and reports ownership as supported. Sync, final-result and required-state
gaps remain; this change does not deliver D1 or durable fork/rewind lineage.
Independent review reproduced an empty-description bypass and a recursive-delete
race; both were fixed with regressions. A deterministic test retires a directory,
opens a new owner at its old path, and verifies deletion preserves the new state.
The root suite passes 29 tests; the actual ownership fixture and recipe build pass
on Windows. Generated wiring bridges SessionDir by path rather than sharing a type
across the host's fixed Rig source and the recipe's selected Rig source; this is
not evidence that arbitrary new Rig revisions are compatible.

An upstream recheck found two commits after the audited pin: a catalog/provider
update and the 0.46.0 release, ending at `6e039cd24994bc1e7d963af6f6278df73a81b130`.
The [exact comparison](https://github.com/0xPlaygrounds/rig/compare/fc556f982d501eea9d7167398a19386e4884e776...6e039cd24994bc1e7d963af6f6278df73a81b130)
contains no ECS, memory or host implementation changes for these gaps. The current
pin stays unchanged; this is source comparison, not runtime validation of 0.46.0.

## D0 execution evidence and bounded proposals (2026-10-31)

The [D0 plan](../plans/2026-10-31-d0-rig-probes.md) is implemented against
`fc556f982d501eea9d7167398a19386e4884e776`, with Rust 1.97.1 and Bevy 0.20.0.
This section records executed synthetic tests separately from source inspection.
**Investigation delivery is complete; the persistence acceptance gate is blocked.**
The owner selected reproductions and proposals when a contract fails, not an
automatic fork, private Store or upstream patch implementation. D1 has not begun.

`plugins/vivy-durable` exports only an explicitly compiled `d0-probe` plugin.
It observes the existing Store and holds native Rig effects until an exact saved
checkpoint appears in a successful ordinary append. It neither replaces storage
nor supplies a production task state machine. Ordinary recipes do not compile it.
The bounded trace records identities, generations, revisions and request/result
digests, not raw requests or credentials. Every event says `durable: false`.

| Contract / experiment | Evidence and result at this pin |
| --- | --- |
| Hold before dispatch across repeated passes | Rust probe: 20 AgentPass runs do not start a tool before its checkpoint is appended. The next frame releases it; a quiet loop needs an explicit wake/release after Last. Supported for ordinary writes. |
| Exact acknowledgement | Other-agent and stale-revision writes do not release a call. Changed arguments, generation or routed provider/model identity require another saved revision, even when the completion payload is unchanged. Injected append failure leaves the call held. Supported for the diagnostic gate. |
| State and conversation in one write | Probe observes both records in one agent-log append. This is not proof of atomic recovery of a partly written append or a transaction across logs. |
| Real assembled coding path | Loopback gateway, one file read and two model requests: four gated starts (including input), exact observed checkpoints, no disk receipts. Trace size varies with frames. The checker rejects stale releases and duplicate dispatch. |
| Final result boundary | Review digest differs from Settled digest after kernel secret redaction. The trace observes settlement without a result commit gate. Capturing Review is insufficient; observing Settled is too late to prevent continuation. Gap. |
| Saved component failure | An injected serialization error omits a component while conversation records still write and SessionLog reports no write failure. Gap for required state. |
| Restore policy and tools | Actual mock-model/Rig execution: settled tool result retained, uncertain tool not repeated; zero successor requests with default stop, one with explicit CarryOn. Stop plus a late outcome starts no successor model request. Supported within these cases. |
| Unsupported Rig format | Both launcher and generated executable refuse `rig-session/999`, send no request and leave logical logs/files byte-identical. Supported; not a Vivy durable-format guard. |
| Missing or malformed required state | Rust restore and assembled entry tests accept an unknown probe component or invalid value; the baseline launcher also resumes/forks such a session. The warning says subsequent saves replace it. Fail-closed required state is absent. |
| Writer ownership | Two DirStores reject stale lengths but allow sequential writes by both owners. A second real launcher is refused before a model request. Direct species execution with an explicit shared RIG_SESSION nevertheless reaches the gateway while the first launcher owns the session. The launcher lease is useful; every supported entry must honor it. |
| Disk sync | Source inspection: DirStore append uses write_all without sync, and no opt-in durable receipt was exercised. Replace uses a synced replacement. No power-loss guarantee is inferred from successful appends or process exits. |

Reproduce on Windows from this checkout (the isolated target path is illustrative):

```powershell
$env:CARGO_TARGET_DIR='C:/vtd0'
$env:CARGO_PROFILE_DEV_DEBUG='0'
$env:PYTHONUTF8='1'
cargo build --locked
C:/vtd0/debug/vivy.exe build recipes/coding-agent.yaml
python -m unittest discover -s scripts -p test_durable_probe.py
python scripts/durable_probe.py C:/vtd0/debug/vivy.exe
```

The probe script runs 12 offline Rust characterizations, derives and builds a
temporary recipe, checks the baseline dependency graph, and writes
`.workspace/d0/report.json`, `coding-trace.json`, `coding.stderr` and
`rust-probes.log`. **Exit 2 means the known persistence gaps were reproduced**;
an unexpected assertion/build/process failure is an error, not a passed D0 gate.
Reports begin as `running` so an aborted invocation cannot reuse an old conclusion.
The five Python checker/environment tests are also offline. Logs, synthetic homes and traces
are not committed. Cargo may fetch pinned dependencies; no external model is used.

Every subprocess uses an environment stripped of inherited Rig control and secret
variables. The assembled run injects poisoned launcher variables pointing to an
owned sentinel directory outside its fixture and verifies those files stay
unchanged and notices do not reach the model. Independent review found and led to
fixes for this environment isolation and the missing routed-model fingerprint;
both regressions were reproduced before fixing them. Model fingerprints include
the public provider/model identity and adapted payload. An opaque handler can
still change endpoint/credential behavior under the same identity; this probe
does not attest to that change. Production freezing needs a host-owned transport
configuration revision, without serializing credential values.

### Proposed changes and ownership

These are bounded proposals, not APIs that exist or patches already submitted.
The receipt proposal below reflects the subsequent owner-selected process-crash
target; the executed evidence above is unchanged. Keep ordinary coding defaults
intact and negotiate the generic seams with Rig.

1. **Opt-in committed saved bundle and receipt (Rig ECS + memory).** Add a
   fallible checkpoint operation that serializes the required component set and
   associated conversation changes before any of that bundle is released. Bind
   the acknowledgement to store/session, agent, operation, request digest,
   generation and revision; an unrelated flush or has_written is insufficient.
   An opt-in checkpoint commit must acknowledge the complete recoverable bundle,
   propagate storage errors and make a partial bundle uncommitted on recovery.
   A process-local buffer or unrelated append is insufficient. Preserve existing
   ordinary-session behavior. First acceptance: serialization/append failure and
   interrupted writes never release an uncommitted checkpoint; a complete bundle
   is released once and recovered after process death. Stronger sync is later
   opt-in work; cross-log atomicity and power-loss survival are not promised.
2. **Final redacted outcome gate (Rig ECS).** Expose an ordered phase after the
   kernel's final redaction and before Settled/transcript publication or successor
   dispatch. A result held there must remain frozen or invalidate its receipt
   when changed. Reuse public holds/decisions and the existing loop. Include a
   supported way to reinsert a saved model result and stable tool call IDs on
   restoration, without another provider/tool invocation. Test redaction, late
   cancellation, interrupted streams and saved-result-before-transcript recovery.
   The existing Interrupted::Settled tool path is reuse evidence, not proof of
   completed model-result reinjection.
3. **Required saved keys (Rig harness/ECS).** Let a build declare required saved
   components/versioned schema for a marked session. Missing plugin, decode
   failure and required serialization failure must refuse or suspend before
   dispatch and leave the authoritative state unchanged. Keep optional plugin
   compatibility for ordinary sessions. Test the launcher, generated executable,
   fork and rewrite paths; a type name or stable key alone is not a requirement.
4. **Host ownership and durable format/lineage guard (Vivy, with Rig protocol
   reuse).** Reuse SessionDir::hold; cover direct execution and resume before
   opening writable state, with an explicit launcher-to-child ownership handoff
   rather than taking the same lock twice. A durable marker must reject builds
   lacking its capabilities. Fork/rewind need fresh execution lineage and no
   copied live obligations. The existing baseline CLI fork copies unknown probe
   state; no production durable lineage or rewind guarantee is claimed here.
   **Follow-up:** the local entry, CLI fork and ephemeral cleanup ownership part
   is implemented above. Durable format/lineage and upstream maintenance-writer
   coverage remain open.

Precise source starting points at the audited commit:
[`dir.rs`](https://github.com/0xPlaygrounds/rig/blob/fc556f982d501eea9d7167398a19386e4884e776/crates/rig-memory/src/dir.rs)
(`append_log` / `replace_log`),
[`session_log.rs`](https://github.com/0xPlaygrounds/rig/blob/fc556f982d501eea9d7167398a19386e4884e776/crates/rig-ecs/src/session_log.rs)
(`SaveSystems`, private flush and serialization),
[`stage.rs`](https://github.com/0xPlaygrounds/rig/blob/fc556f982d501eea9d7167398a19386e4884e776/crates/rig-ecs/src/stage.rs)
(`redact_results` in private Enter(Commit), public holds), and
[`restore.rs`](https://github.com/0xPlaygrounds/rig/blob/fc556f982d501eea9d7167398a19386e4884e776/crates/rig-ecs/src/restore.rs)
(skipped saved values, CarryOn and Interrupted). The Review documentation at
this pin describes redaction earlier than the registered schedule; the executed
redaction probe is the relevant evidence. Vivy's lease entry is
`crates/vivy/src/launch.rs::run_sessions`; its raw-copy fork also needs the future
durable guard. No source/reference checkout was modified and no issue/PR posted.

Unverified: power loss, sync-failure recovery (no receipt API yet), the full D1
crash matrix, production cancellation generations, durable fork/rewind lineage,
completed saved model-result reinjection and real-provider acceptance. The probes
only mutate a generation in diagnostic state; they do not deliver production
cancellation persistence. Do not substitute these tests for those contracts.
Re-run D0 at the eventual upstream patch revision before authorizing D1. A private
Store remains conditional; these findings do not select it automatically.

## Current upstream adoption assessment (2026-10-31)

The owner asks whether using Rig directly and contributing generic fixes can
avoid a long-lived downstream fork. This assessment recommends that approach
within Plan B. It is source/API research, not an upgraded dependency or a
successful integration test.

GitHub's default branch resolves to
`fc556f982d501eea9d7167398a19386e4884e776` (2026-10-30 UTC); the
[comparison with our recipe pin](https://github.com/0xPlaygrounds/rig/compare/e1cd47d57f0bccb50ddffd6f45de2ac3365b271c...fc556f982d501eea9d7167398a19386e4884e776)
reports 12 commits ahead, none behind. The latest published release is
[v0.45.0](https://github.com/0xPlaygrounds/rig/releases/tag/v0.45.0), published
2026-10-19. Our current 2026-10-27 pin is already later than that release;
upgrading to a tested Git revision and switching to the latest release are not
the same operation. Neither the recipe pin nor the reference checkout changed
during this assessment.

| Upstream evidence | Meaning for Vivy |
| --- | --- |
| [Discussion #1924](https://github.com/0xPlaygrounds/rig/discussions/1924), [issue #2244](https://github.com/0xPlaygrounds/rig/issues/2244), [PR #2762](https://github.com/0xPlaygrounds/rig/pull/2762) | Serializable `AgentRun` and resume through the shared runner have an established upstream history. These are the classic `rig-agent` path; our harness uses `rig-ecs`, so these APIs are not a drop-in replacement for the current loop. |
| [PR #2514](https://github.com/0xPlaygrounds/rig/pull/2514), followed by [#2761](https://github.com/0xPlaygrounds/rig/pull/2761) and [#2771](https://github.com/0xPlaygrounds/rig/pull/2771) | The old ECS tool-turn checkpoint API was followed by deletion of that ECS and a new implementation. The old `ToolTurnCommit` / `RigSet::Checkpoint` names are absent from our pin; do not design against their historical announcement. |
| [PR #2943](https://github.com/0xPlaygrounds/rig/pull/2943), merged 2026-10-29 | Latest harness defaults stop unfinished work after exit/restart. A plugin explicitly opts a turn into continuation with public `CarryOn`, re-armed from its saved policy on restoration. This supersedes the old default automatic-continuation behavior described in #2841/#2844. |
| [Latest restore source](https://github.com/0xPlaygrounds/rig/blob/fc556f982d501eea9d7167398a19386e4884e776/crates/rig-ecs/src/restore.rs) and [plugin guide](https://github.com/0xPlaygrounds/rig/blob/fc556f982d501eea9d7167398a19386e4884e776/crates/rig-harness/PLUGINS.md) | `CarryOn`, `Restored`, `Interrupted`, saved reflected components, holds and `WakeAt` are existing building blocks. `CarryOn` is not itself saved and is consumed during restoration; the host must reconstruct its policy. |
| [crash_resume](https://github.com/0xPlaygrounds/rig/blob/fc556f982d501eea9d7167398a19386e4884e776/corpus/sn/src/crash_resume.rs), [task_queue](https://github.com/0xPlaygrounds/rig/blob/fc556f982d501eea9d7167398a19386e4884e776/corpus/sn/src/task_queue.rs), [batch_provider](https://github.com/0xPlaygrounds/rig/blob/fc556f982d501eea9d7167398a19386e4884e776/corpus/fh/src/batch_provider.rs) | Upstream examples demonstrate recovery choices, a saved queue and a saved external batch wait re-armed after restart. They are corpus examples, not a separately released persistent-assistant package. The queue marks interrupted tasks for retry; it does not establish exact phase recovery. |
| [PR #2890](https://github.com/0xPlaygrounds/rig/pull/2890), merged 2026-10-23 | SQLite/Postgres session backends exist as `AsyncStore`. The PR explicitly leaves the synchronous harness `Store` unchanged, so replacing the current harness storage with these is not a configuration-only switch. |

The durability gaps recorded in
[PR #2844](https://github.com/0xPlaygrounds/rig/pull/2844) are not all solved by
the newer continuation interface. At the inspected latest revision,
[`stage.rs`](https://github.com/0xPlaygrounds/rig/blob/fc556f982d501eea9d7167398a19386e4884e776/crates/rig-ecs/src/stage.rs)
still describes its pre-effect log write as not disk-synced, and
[`DirStore::append_log`](https://github.com/0xPlaygrounds/rig/blob/fc556f982d501eea9d7167398a19386e4884e776/crates/rig-memory/src/dir.rs)
uses `write_all` without syncing. `replace_log` does use a synced replacement;
do not describe the whole store as having no sync anywhere. The writer still
appends logs separately; no atomic parent-report/request-clear operation was
established by this inspection. `SessionLog::flush` remains crate-private.

PR #2943 also lists unresolved cases for a carried turn whose tool is active at
quit, and for a carried child whose parent stopped. These are promising bounded
reproduction/contribution targets, not proof that arbitrary continuation is safe.
Public searches for open PRs containing `durable`, `checkpoint` or `fsync` found
no dedicated pending completion of the deferred stronger-durability work. This
is a bounded search, not a claim about private plans or an upstream delivery date.

Recommended integration order:

1. Audit D0 against a deliberately selected current Rig revision, including the
   new stop/`CarryOn` contract and relevant TUI/plugin migration. Reuse Rig's
   saved state, recovery markers, queue/timer mechanisms and logical storage.
2. Implement only Vivy policy locally: which tasks survive, when to hand off,
   deadlines/budgets, context/persona, memory and evolution. Keep the existing
   YAML compile-time assembly and coding face.
3. Reproduce missing generic guarantees in upstream-sized tests. Prefer fixes
   or small opt-in APIs in Rig for commit/sync acknowledgement, continuation
   correctness and parent/child result consistency. Do not change interactive
   coding's default stop behavior merely to support Vivy's unattended policy.
4. Keep a private Store/runtime replacement conditional on a demonstrated gap
   and the upstream design response. A temporary patch needs a bounded scope
   and removal target; an unmerged upstream contribution is not a guarantee of
   future acceptance. Pin upgrades and run downstream acceptance even after
   a fix lands, since public APIs and default semantics are still changing.

This tightens the earlier design's implementation ownership: a Vivy-owned framed
`DurableStore` is a fallback proposal, not the first implementation task. The
accepted recovery guarantees remain; direct upstream reuse does not waive them.
No issue/PR was posted and no runtime verification was performed in this audit.

## Reference scope

Historical comparison below: source checkouts inspected read-only. The
2026-10-31 refresh follows this list; it does not redate earlier test evidence.

- Prime `c4c238ca025e382d96fb5f37e6328ca77fb82ef3`:
  `crates/pa-core/src/session/{manager,window.rs,window_cache.rs}` and
  `crates/pa-daemon/src/session_store/`.
- Pi `f321e5bdd26b3624bfb2ab182a334d0a5edb698a`:
  `packages/coding-agent/{docs/session-format.md,src/core/session-manager.ts}`
  and `packages/durable/{README.md,src/storage,src/session}`.
- Rig `55cd481a2421fd5da468b5bd6afc6ac876883599`:
  `crates/{rig-memory,rig-ecs,rig-harness,rig-sqlite}/src`.
- vivy-test: `recipes/coding-agent.yaml`, `plugins/vivy-basics/src/prompt.rs`,
  `docs/PI-BASELINE.md`, and the Python workbench documentation.

These are pinned local source findings, not new runtime tests. No user sessions,
databases, credentials, or production data were inspected.

## Durable reference refresh (2026-10-31)

The owner starts persistent-assistant design preparation and reaffirms pi-durable
as its anchor. The local Pi reference now matches upstream `main` at
`a14cdf90f0a76532f3e20bc3c01872837cef24ba` (2026-10-30). Its durable package still
declares itself experimental. Reviewed material includes its
[README](https://github.com/earendil-works/pi/blob/a14cdf90f0a76532f3e20bc3c01872837cef24ba/packages/durable/README.md),
changelog and the relevant tool, generation, live-progress and submission code.
This is a reference refresh, not a TypeScript runtime dependency or a completed
Rust implementation. No upstream or Vivy runtime tests were run for this review.

- **Keep the lifecycle anchor.** Admission deduplication, committed phases and
  sleep deadlines, explicit abort and recovery remain relevant. Cancelling a
  wait does not abort work. A sleeping task still needs a running host or a host
  alarm to resume; a stored deadline cannot start a stopped desktop process.
- **Recovery is conditional.** The tool implementation commits intent before
  execution, defaults to unsafe replay, and reruns an interrupted tool only when
  both its saved and current replay policies are safe. Otherwise it reports
  interruption and possible partial execution. Arbitrary Python cells, MCP
  actions and remote messages need effect-specific treatment.
- **Separate progress from durable decisions.** Streamed partials/output now use
  deferred document commits: views update immediately, while persistence occurs
  with dependent work or after the default approximately 1,000 ms deadline.
  Task states, entries and tool intent still force persistence. The old blanket
  description that nothing is shown before storage must not be applied to live
  progress at this revision. Adopting this optimization in Vivy is undecided.
- **Do not copy the steering boundary.** Pi places steering after the current
  tool round; follow-ups wait for an answer. Vivy's accepted ordinary handoff is
  after a single tool finishes, with the agent deciding what comes next and the
  human retaining immediate interruption. Multiple calls in one model response
  therefore need explicit scheduling and result handling in the later design.
- **Keep evolution scheduling explicit.** New `afterRun` hooks create durable
  child tasks after an answer; the answer settles first, but the conversation
  stays busy until those tasks finish and new input waits. This is useful
  lifecycle evidence, not a selected AutoDream trigger or a reason to block
  conversation on evolution. The separately deferred official inbox is also
  distinct from Pi's internal submission queue.
- **Keep the storage decision bounded.** Storage now accepts grouped atomic
  commits with per-commit receipts; conversation views use bounded transcript
  windows. JSONL still keeps full history in memory, and storage still needs a
  single process owner. A bounded UI view does not fix backend history growth.
  The owner's JSON-file preference stands; format, sync, locking and recovery
  remain open. No grouping or cache machinery is required without a measured
  need in the selected Rust implementation.

The owner confirmed "myllm" means mysoul. The Desktop mysoul checkout is now at
`3a7cbde7`, newer than the previously referenced worktree. Its
`src/mysoul/service.py`, `laputa.py`, integration tests, runbook and validation
records support persona plus committed Recap and current input/activity state.
The inspected new-turn composer does not search memory and sets recall status to
`not_requested`; its optional memory tools only run after an explicit agent call.
Those tools and subject-scheduled wakeups are separate reference capabilities,
not required parts of Recap-only. See the later
[assistant context clarification](../DESIGN.md#assistant-context-clarification-2026-10-31).
This inspection did not rerun those integrations. Existing real-provider wake-tool
validation records include gateway failures; the owner's basic-loop evidence
does not establish every integration or the new Rust recovery contract.

Laputa's new persona authority and AutoDream's automatic adoption remain the
accepted direction. Exact durable context policy, task orchestration and the
first acceptance scenario are still for discussion. Entering design preparation
does not mark the deferred Vivy workbench provider gate passed or advance the
official inbox implementation schedule.

## Historical implementation comparison (2026-10-27)

The owner's follow-up correctly notes that **Go Vivy uses SQLite**. The Rig
file-store row below describes the current **vivy-test experiment**, not the
older Go product. The earlier source-only inventory also identifies an optional
PostgreSQL backend. At Go revision `9cd2801f8090723a652d3e835f72d83715b82121`,
the SQLite backend uses a single database connection, applies migrations on
open, and has session, message, run, journal, snapshot, lease and other product
stores. Its journal appends an event batch and applicable user messages in one
transaction; tool-operation admission stores the operation and its journal event
in one transaction. See the pinned Go source for
[SQLite setup](https://github.com/ProjectViVy/agent-vivy/blob/9cd2801f8090723a652d3e835f72d83715b82121/internal/storage/sqlite/sqlite.go),
[sessions](https://github.com/ProjectViVy/agent-vivy/blob/9cd2801f8090723a652d3e835f72d83715b82121/internal/storage/sqlite/sessions.go),
[journal](https://github.com/ProjectViVy/agent-vivy/blob/9cd2801f8090723a652d3e835f72d83715b82121/internal/storage/sqlite/journal.go)
and [tool operations](https://github.com/ProjectViVy/agent-vivy/blob/9cd2801f8090723a652d3e835f72d83715b82121/internal/storage/sqlite/tool_operations.go).
These are useful design references, not a reason to import the Go backend,
reproduce every product store, or migrate legacy data.

The Go [V0 architecture](https://github.com/ProjectViVy/agent-vivy/blob/9cd2801f8090723a652d3e835f72d83715b82121/docs/AGENT-VIVY-ARCHITECTURE-V0.md)
records that the pure-Go SQLite driver supported a single Windows executable,
while the desired storage semantics included atomic journal commits, snapshots,
leases and restart recovery. D-026 explicitly says the database engine was not
the architectural invariant; D-031 made SQLite the V0 reference and deferred a
filesystem journal probe. This explains the recorded choice, not a proven
performance advantage over files.

## What the references actually keep

| System | Authoritative representation | Lifecycle and recovery | Long history and concurrency |
| --- | --- | --- | --- |
| Pi coding | One JSONL file per session; header plus entries with `id` and `parentId`. A branch is a path through the tree; compaction appends a summary and retains old entries. | Setup entries stay in memory until a user or assistant message exists; subsequent records append synchronously. The loader skips malformed lines and handles an unterminated last line. This is conversation history, not a durable task scheduler. | The ordinary session loader reads the file into entries. No fsync-per-message or multi-process task ownership guarantee was established. |
| Prime | The session JSONL is authoritative; an active leaf and parent links describe the branch. The manager appends records and can hydrate the full history when required. | `append_cached` flushes and calls the platform fsync helper. Suspected torn or zero-filled damage triggers a repair scan and replacement; full rewrites use a temporary file and sync before replacement. Incremental cache certification requires the existing session lease. | A disposable generation-certified window sidecar lets a warm open read the header and retained suffix; metadata sidecars support session listing. Cold or invalid cases fall back to source reads. The optimized window explicitly returns no result on non-Unix systems because the file-identity certificate is insufficient; Windows behavior must not be inferred from the Unix path. |
| pi-durable | A durable session contains conversations, immutable entries, submissions, task states and scoped documents. It offers SQLite, JSONL and other backends. | Submissions and phase transitions commit before being exposed; a sleeping task stores deadline and checkpoint and must be woken by a live scheduler or host alarm. An interrupted phase restarts from its beginning. A tool marked replay-safe can rerun; an unsafe interrupted tool reports interruption. | SQLite stores indexed records in one database; the documented Node adapter uses WAL and `synchronous=NORMAL`, so the latest commits may be lost on power/host failure. The JSONL backend optionally fsyncs before commit markers but keeps its full history in memory; the README recommends SQLite for long-running agents. One process owns the storage; there is no cross-process ownership lock. |
| Current Rig coding path | `DirStore` keeps `logs/<agent>.jsonl`, a session log, `files/` and image blobs in a session directory; `rig-session/4` records conversation changes and saved state. | The session log queues and flushes changes during the agent loop, including before tools that may change things. A torn final line is set aside when opening for writing; unreadable or incompatible content is refused. Ordinary append uses `write_all`, with no per-append sync in that path; durable replacement syncs a temporary file. | A directory lock serializes cooperating changes across processes and an expected-length check refuses a stale log write. This protects storage writes; it does not provide exclusive task ownership or durable external effects. `read_log` reads a complete log. |

Pi's `/tree` branches within one file, while current Rig agents have separate
logs within a session. Existing resume/fork/clone behavior was exercised in the
project's scripted comparison; this is not proof of identical branching or
crash semantics. A chat snapshot, an ACTMEM summary, a filesystem rollback,
and a scheduled task checkpoint are different records and must not be conflated.

## Existing infrastructure worth reusing

The selected coding recipe already includes Rig sessions and the prompt plugin
installs `SessionLogPlugin`; the default `StoreOpener` opens `DirStore`.
Rig supports resume, saved state, session listing metadata, image blobs,
compaction and stale-write detection. Its session writer records changes
incrementally. Start with those working paths and verify observed limitations
before building a second coding storage layer.

Rig also has `rig-sqlite`'s `SqliteSessionStore` behind its `memory` feature:
it stores logs as chunks and checks each log append in an SQLite transaction.
Its `AsyncStore` read assembles the complete log. This is useful reference or a
possible future backend, **not** an already-selected backend in this recipe:
the default host `StoreOpener` expects a synchronous `Store`. Its per-log
transaction alone does not atomically commit a durable task phase and all
conversation/effect records; integration and transaction boundaries need proof.
Nor does switching to it automatically provide indexed message-level queries.

Prime's generalizable ideas are source-of-truth plus disposable, validated
indexes, append-before-effect where appropriate, and recovery that does not
silently treat a broken file as a complete session. Its exact cache certificate
is not a ready Windows optimization. Pi-durable supplies the stronger conceptual
model for task admission, checkpoint, sleep, resume, replay and cancellation;
the TypeScript implementation is a reference, not a Rust dependency.

## Proposed direction for Vivy

The owner's subsequent preference is **JSON-based files rather than SQLite**.
Start with Rig's existing file store for coding and evaluate a Rust-owned
file-backed durable store using pi-durable's JSONL backend as a reference.
The exact format, transaction protocol and recovery semantics remain to be
designed and tested; this is a direction, not a claim that they are implemented.
The owner further clarified that an additional Vivy-owned SQLite database is
not desired alongside MemPalace: the intended memory investment is an ingestion
and recall flow into MemPalace's existing storage. This is not a ban on upstream
storage internals: the inspected MemPalace source includes a Chroma backend
with SQLite files. Single-binary deployment is not an owner goal.
Keep the two harnesses' session and
execution state distinct even when they use the same workspace. The Python
workbench supplies capabilities; it must not be the authoritative owner of the
Rust runtime's session or task lifecycle.

For a durable session, publish a task transition and the conversation records
it depends on as one authoritative commit. A minimal model would identify
the session, task, phase/status, revision, checkpoint and wake deadline, plus
request/effect identifiers where needed. A durable submission is admitted once;
resume loads committed state; a sleep commits its checkpoint and deadline before
the runtime relinquishes control. An actual running scheduler or host wakeup is
still required. A persisted deadline alone cannot start a stopped application.
Implement only the task kinds and records needed by the first durable use case.

This proposal does not require a generic workflow platform. Keep one
authoritative representation per session; UI projections, search indexes and
caches are derived data. Do not make separate files independently authoritative
for the same commit without a recovery protocol: a crash between writes would
leave ambiguous state. One candidate is a single JSONL batch per commit; another
is pi-durable's multiple sidecars with a main commit marker. The latter is more
involved and should not be implemented preemptively. Define and test commit
publication, truncated writes, sync policy and single-owner behavior on Windows.
Python/MCP effects still require their own recovery rules.

The owner has expressed a preference, not approved a concrete on-disk format.
Readable JSON is suitable for small snapshots; append-only JSONL avoids rewriting
a growing conversation on every event. Neither is automatically faster for
every query. Pi-durable's present JSONL backend holds full history in memory;
do not adopt that limitation as a Vivy requirement. Large-history loading,
pagination, indexes, and crash guarantees need measurement and a bounded design.
No compatibility with old Vivy data or its deployment architecture is required.

## Effects, memory and the shared workspace

- A transaction cannot encompass an arbitrary Python/MCP call, remote service,
  workspace file write, or MemPalace operation. Record intent and completion
  where needed; use stable operation IDs and idempotency only where the receiver
  supports it. If a crash leaves an external effect uncertain, expose or
  reconcile that state instead of claiming exactly-once execution or blindly
  retrying a non-replay-safe operation.
- The shared workspace provides shared files, not shared task ownership or
  automatic conflict-free file edits. Concurrent coding and durable work needs
  an explicit policy for overlapping writes. Session branching and restoring
  task state do not rewind user files.
- Laputa remains the persona source. ACTMEM/Recap is durable working context,
  with its source and commit boundary retained; MemPalace is domain-partitioned
  long-term memory accessed through the Python workbench. A palace/wing/room/
  drawer is an organization and retrieval boundary, not an access-control
  guarantee. Do not automatically copy every transcript into long-term memory.
- MemPalace, Obsidian and their synchronization remain a later design. They
  should not become a second authoritative task log. Any future asynchronous
  memory write needs an identifiable source, retry/failure policy and explicit
  visibility semantics; no cross-store atomicity is assumed.
- A derived search index or Prime-style sidecar must be rebuildable, versioned,
  validated against its source and allowed to fail without losing the source.
  Include caches, exports and backups in privacy/deletion decisions; redaction
  is not a substitute for a defined retention policy.

## What to measure before optimizing

Use isolated synthetic sessions and the actual selected implementation. Compare
short and long histories (for example 1,000 and 100,000 entries) and record cold
and warm open, active-context assembly, paged history/listing, append/commit
latency, disk growth and peak memory on Windows. Capture the sync policy,
hardware, data shape and p50/p95; benchmark before adding an index or cache.
Test interruption during append and checkpoint, corrupted tail, disk-full/error
propagation, two competing owners, restart during a replay-safe and unsafe tool,
overdue wakeup, and memory-write failure. Separate process-kill tests from
power-loss claims. These are future acceptance experiments, not tests run here.

The Python worker currently loses its namespace on worker restart/session
switch and cancels cell-spawned asynchronous tasks; its state must not be
mistaken for a durable checkpoint. Real provider, MCP, multi-process recovery,
SQLite integration and performance have not been validated by this research.
