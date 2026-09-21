# CH-INT-36 Task 1 acceptance

Date: 2026-09-20
Status: Accepted as the unresolved integration handoff for Tasks 2–7.

This record accepts the input-freeze and merge-opening gate only. It is not
final acceptance of the integrated product and does not claim CI, package
tests, conformance regeneration, review, or a merge commit.

## Gate criteria

| Criterion | Evidence | Result |
|---|---|---|
| Exact target recorded | `HEAD` = `c054a9b5b85a7405d071e33e3fc756114f26e2da` | PASS |
| Exact source recorded and locked | `MERGE_HEAD` and `origin/feat/channel-tier1` both equal `eb8fee3c0996746e93def297d700c70fb65b8540` | PASS |
| Exact merge base recorded | `git merge-base HEAD source` = `fe60b1869a166490da72a07eef8f1d754caa2e3e` | PASS |
| Source size frozen | 69 commits and 114 changed paths from the locked base | PASS |
| Target descendants classified | Only `c054a9b5…`, changing three planning documents after code baseline `0c47eb9…` | PASS |
| Planning merge reproduced | Exit `1`, tree `aef901140178144e8e22db0cd7157216b8908b74`, exactly eight paths | PASS |
| Real merge opened | `MERGE_HEAD` remains set to the locked source | PASS |
| Unresolved set preserved | `git diff --name-only --diff-filter=U` returns the same eight paths | PASS |
| Task boundary preserved | No conflict resolution, staging of a resolution, abort, reset, or merge commit performed | PASS |

## Later-task scope

The next tasks inherit this merge state and the plan's ownership authorities.
They must preserve the source ancestry with one real merge, keep app and RPC
on the generic contract boundaries, leave concrete Channel ownership in
`internal/modules/channel`, use the existing runtime run and approval paths,
preserve public `std/channel@v1` compatibility, and regenerate conformance
evidence from the final tree. CH-P0-3, CH-P0-4, and CH-P0-5 work is outside
this integration gate.

The eight paths are to be resolved according to the planning table, not by a
blanket side selection:

- union current and PR completion records in `docs/COMPLETE.MD`;
- retain `channelcontract.Owned` and adapt app behavior through focused
  dependencies/callbacks;
- keep the app channel test deleted and move only valid capability coverage to
  the Module owner;
- keep Provider binding and capability-target behavior in the Module;
- keep RPC core generic and move Channel behavior into Module contributions;
- regenerate `sdk/internal/assembly/conformance_results.json` from final
  sources; and
- keep the reproduction test's internal digest dynamic and source-derived.

## Stop conditions

Return to the supervisor instead of improvising if:

- the PR source head is no longer the locked SHA;
- a product-code target descendant appears after the plan baseline;
- the merge conflict set expands into another subsystem;
- preserving the PR requires a public `std/channel@v1` break;
- app/RPC would import the concrete Channel Host or create a second runtime,
  settings, delivery, or RPC dispatcher path;
- a storage migration number collides with accepted work;
- PR behavior conflicts with the normative #42 architecture; or
- acceptance would require beginning CH-P0-3, CH-P0-4, or CH-P0-5.

## Local integration closeout (not final acceptance)

The real merge now has no unresolved paths and the locked PR source remains an
ancestor candidate. Canonical ownership scans, gofmt, vet, headless compile,
full main-module tests, full SDK tests, all independent plugin/face tests,
UI/i18n gates, conformance producer, and default Pack/Inspect parity are green
under the documented executor Go toolchain.

This is not final acceptance. The executor has no `just`, no PostgreSQL
service/DSN, and its cloud browser cannot route to executor loopback
(`net::ERR_BLOCKED_BY_CLIENT` at `127.0.0.1:3015`). Literal `just ci`, live
PostgreSQL migration evidence, five browser checks with an RPC trace, and a
green external branch CI run remain required before changing CH-INT-36 to
COMPLETE or CH-P0-3 to READY. No push or aggregate-branch merge is performed
without explicit authorization and those external gates.
