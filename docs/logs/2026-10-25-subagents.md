# Native subagent delivery

Date: 2026-10-25. Repository: vivy-test. Branch: feat/subagents, base 5477eee.
Scope: owner-approved [implementation plan](../plans/2026-10-25-subagents.md).
Implementation commit c94f87f was fast-forward merged into local main on
2026-10-25 with owner approval. Collaborative Plan was subsequently integrated on
2026-10-26; combined verification is recorded in HANDOFF.md. No push was performed.

## Implementation

The selected Vivy component wraps Rig 55cd481a's native SubagentsPlugin,
SpawnSubagent, reports, ToolAccess, WorkDir, UsagePlugin and saved sessions.
Owned product code adds role discovery/snapshots, bounded admissions, finite
parallel/chain result aggregation and terminal rendering. Existing YAML parsing
is shared. There is no second agent loop, transcript store, model connector,
graph scheduler, SDK or daemon.

Roles come from built-ins, home agents, then trusted project agents. Task/cwd and
tool validation occurs before launch. Native model resolution and depth checks
remain authoritative. Default depth is one; the session admits four running
children including follow-ups. Root requests wait; nested spawns at capacity
fail visibly instead of creating descendant-slot deadlocks. Parallel calls have
1–8 tasks; chains stop on failure and substitute the previous bounded answer.

Command/ask handling, report collection and admission have explicit native stage
ordering, covered by the harness conflict audit. Workflow reports do not steer the
parent into duplicate turns. Each inline answer including its child reference is
at most 50 KiB; native child history remains complete. Stopping cancels active and
queued work. Resume stops interrupted work without replay and preserves completed
children for follow-up. There are no automatic retries of delegated side effects.

Children share the process and project, with agent-local cwd. Roles constrain
available tools; shell/Python and compiled plugins are not sandboxes. Give
parallel writers distinct files. Automatic worktrees, subprocess isolation,
cross-project context loading, Python delegation and apixis remain excluded.

## Verification

Windows x64, Rust 1.97.1, isolated CARGO_TARGET_DIR=C:/vt-subagents. Root and
plugin workspaces are separate; root cargo test does not test plugins.

| Command | Observed result |
| --- | --- |
| cargo test --workspace --locked | 20 passed |
| cargo test --manifest-path plugins/vivy-basics/Cargo.toml --locked --features tui | 46 passed |
| cargo run -p vivy -- build recipes/coding-agent.yaml | Actual baseline species built |
| python scripts/smoke.py C:/vt-subagents/debug/vivy.exe | Gateway/model, shell, sessions, Python state/reset, JSON, error exit, secret filtering, image/effort/compaction passed |
| python scripts/subagents_smoke.py C:/vt-subagents/debug/vivy.exe --tui | Single, parallel, file-editing chain, partial failure, limits, role trust including roles created after startup under --no-approve, raw ID/access/secrets, actual terminal roles/cards/child views/usage, four active + two queued and shell-descendant cancellation passed |
| python scripts/subagents_optional.py C:/vt-subagents/debug/vivy.exe | Headless species built and passed delegation smoke; normal graph excludes rig-tui. No-component species built and offered none of the five delegation tools |

The TUI smoke uses test-only pyte 0.8.2, wcwidth 0.2.13 and Windows pywinpty
3.1.0 under ignored scratch. Set PYTHONPATH to that installation to run it.
No product dependency was added for testing. Synthetic slash-command typing
waits for the editor and dismisses completion before submission. The stop test
uses the advertised Esc action, verifies actual owned process PIDs exit and
checks queued work never starts. Ctrl+C is not Rig's turn-stop binding.

The legacy sessions_tui.py declares Unix-only support. Its attempted Windows run
failed early reply/rewind/session assertions and later reached a closed PTY.
It is not claimed to pass on Windows. This delivery's terminal paths use the
targeted ConPTY smoke; existing headless session smoke passed. Linux/macOS and
a new side-by-side Pi run were not performed for this Windows addition.

Real-gateway smoke was attempted with the previously authorized Desktop key file,
an isolated temporary home/project and a two-scout file-reading prompt. Both
attempts failed at the first HTTP model request after native retries, with
`HttpError: error sending request`; reported model calls were zero. No completed
real-provider delegation is claimed. Credentials were absent from captured
output/session data, and temporary sessions were discarded. The root failure
does not establish whether this is endpoint availability, TLS or local networking;
that diagnosis is outside this subagent delivery. Scripted evidence remains valid.

The no-default-features basics cargo check passed. Formatting checks passed for
root and basics manifests; Git whitespace checks passed. Clippy completed, with
style warnings for existing shell nesting, ECS query types and function arity;
it is not a warning-free lint claim.

Local-main integration checks: workspace tests (20) and tui-enabled basics tests
(46) passed again with CARGO_TARGET_DIR=C:/vt and CARGO_PROFILE_DEV_DEBUG=0.
The baseline recipe rebuilt successfully from the main checkout. Both
scripts/subagents_smoke.py and scripts/smoke.py passed against C:/vt/debug/vivy.exe.
The existing start.cmd now launches that integrated build; it was not modified.

## Independent review and corrections

One fresh-context whole-branch review found two Important defects, both reproduced
by the reviewer with the actual binary and by focused failing regression tests:

- With an explicit depth override, a child may settle a turn while waiting for a
  descendant. Its interim answer must not complete a workflow. A settled turn now
  supplies only a candidate; the correlated native Report to the owning parent
  commits it. The blocked-grandchild test verifies no early parent request and
  only the final child answer in the aggregate, without a late extra parent turn.
- An empty initial protected-resource list previously implied project trust,
  including under an explicit refusal. Trust now records only an affirmative
  decision, independently of discovery. The deterministic empty-list test and
  actual late-created-role scenario verify the correction. No startup question is
  added when there is nothing to approve.

The reviewer also noted the headless question handler/admission stage warning.
An actual-binary regression reproduced it; explicit ordering after native
PrintSystems removes the warning. There is no second question/approval path.
The complete basics suite passes after the fix pass. No Critical or deferred
Minor findings remain from that review.

Review boundaries were retained: process isolation/worktrees/Python/cross-project
loading are excluded; shell/Python are not sandboxes; Linux/macOS parity, the
legacy Unix terminal script on Windows and actual provider delegation remain
unverified. These are disclosed limits, not new implementation prerequisites.

## Manual acceptance

For local main, use CARGO_TARGET_DIR=C:/vt and rebuild the baseline recipe;
start.cmd uses that build and loads the existing Desktop gateway configuration.
The earlier isolated C:/vt-subagents build remains separate.

1. Run /roles, then ask for two scout investigations in parallel. Open /agents
   to inspect a child and return to the parent; /usage includes both children.
2. Ask for a scout-worker-reviewer chain using {previous}. Verify the worker's
   file exists and the reviewer reads it. Use a disposable project for writes.
3. Ask for six long-running tasks. Confirm four run and the others wait; Esc
   stops them and queued work does not begin. Already completed writes persist.
4. Resume a completed session and ask a child a follow-up. Interrupted work
   should be described as interrupted, without rerunning its earlier effects.

The mirrored research record preserves decisions in both repositories. Only
vivy-test contains runtime changes; original Vivy repair code/data is untouched.
