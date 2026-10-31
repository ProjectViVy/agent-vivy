# D4 AutoDream AD-2 implementation plan

Status: implemented and verified on `feat/d4-autodream`. Base: `334ed9e` (D3).

## Goal and constraints

Close committed evidence -> persona-free Dream -> candidate -> host-owned
background validation -> automatic publication -> actual use -> new evidence.
Both native Laputa persona and executable pure-Python Skills are required.
Use the existing Rust/Rig core, workbench, initialization and process ownership.
No second engine/database, persistent delegation, inbox, external transport or
periodic reflection loop. New D4 sessions only; preserve D3 executable/logs.
The authoritative spec is DESIGN.md's AD-2 integration contracts, including the
owner's Recap-only and separate-persistent-agent corrections.

## Tasks

1. Extend workbench publication with immutable managed source revisions, retained
   manifests and validation records, shared preparation/publication lock, staged
   preparation, expected-base checking, update-ID reconciliation and rollback.
   Reuse compatible dependency environments without modifying published sources.
2. Pin the Skill publication visible to a model request into its tool operations
   and retries. Replace workers at a call boundary, verify module origins, refresh
   trusted inventory/instructions/commands, and record actual callable usage.
3. Add one host-owned durable validation job per workspace: submit, inspect/wait,
   cancel. Commit admission/dispatch before launch and outcome before delivery.
   Check frozen imports, binding, function assertions and old/new defect cases.
   Recover known results; unknown attempts are not blindly replayed.
4. Add a compile-selected AutoDream coordinator and fixed internal agent context,
   workspace ownership, deduplicated evidence collection, explicit requests and
   D3 one-shot wakes. Dream autonomously authors/abandons candidates; submitting
   a candidate requests validation and automatic adoption. Laputa remains persona
   authority. Foreground work has priority without cancelling independent jobs.
5. Exercise the real Windows executable with a synthetic gateway: executable
   repair, publication, subsequent call, feedback, persona isolation, failure,
   cancellation, crash boundaries, stale bases, concurrent preparation and rollback.
   Run relevant existing regressions, record costs, update design/handoff, obtain
   one fresh whole-branch review and commit the focused deliverable.

## Defaults and acceptance

One active Dream, one coalesced pending request, one validation job per workspace.
Explicit request only; no idle activity or per-turn reflection. Validation timeout
defaults to 60 seconds; staging/output budget is 256 MiB, without automatic deletion
of published/pinned revisions. Unadmitted dependency changes use setup; managed
source updates within admitted scope do not ask per-candidate approval. Live hosts
are required; no detached daemon. Ordinary process crashes only, not power loss.

Acceptance requires: exact code/environment/instruction agreement during publication
and retry; failed/stale/tampered candidates preserving active versions; process-tree
cleanup and waiter-vs-job cancellation; deduplicated evidence and delivery; activation
reconciliation by update ID even after later publications; callable and persona
rollback; old/session-capability refusal without log changes; recipe dependency
evidence; and explicit distinction between synthetic and real-provider acceptance.
Record build time, request counts, foreground latency, job time and storage growth.

## Completion evidence

All five tasks are implemented. See [HANDOFF](../HANDOFF.md#d4-autodream-ad-2-2026-11-01)
for executed commands, counts, timings, limits and the independent review/fix pass.
The final native matrix passes all 15 cases; the ordinary executable separately
passes adoption, actual invocation and absence of diagnostic hooks. Offline
publication tests cover stale/tampered sources, shared preparation ownership and
rollback. D3's feature-set and executable regressions still pass. Generated Cargo
wiring selects `autodream` and its managed-workbench dependencies from the recipe;
the coding recipe remains independent. Real-provider acceptance is still pending,
as specified before D0; synthetic checks do not waive it.
