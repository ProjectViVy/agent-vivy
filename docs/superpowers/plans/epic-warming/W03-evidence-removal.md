# W03: Remove hash-only warming evidence and close the residue

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Preserve the user's execution choice; use one active write lane and fresh independent reviewers for final acceptance.

**Goal:** Leave the extracted warming path with no Vivy-owned prefix hash or hash-only diagnostic, remove every core-owned warming evidence/state residue that the cutover made dead, and add a guard that fails if a hash-only prefix diagnostic is ever reintroduced. At Baseline A the named 10-18 hash symbols do not exist, so the Story starts by proving that absence and then owns the removal if and when the gated post-baseline batch track re-introduces the newer warmer body.

**Architecture:** Warming's only evidence is the existing `cache.warmed` diagnostic with provider/model/mode/status/reason and provider-reported token counts, plus the `MaintenanceResult`/`MaintenanceDiagnostic` hand-off. There is no prefix digest, hash-only JSON encoding, `prefix_digest_failed` status, prefix-hash field, schema/UI property, fixture or test. The removal is textual and complete: no disabled branch, alias, renamed hash, no-op verifier or compatibility reader is kept.

**Tech Stack:** Go 1.26.4; repository tests only; no new dependency, no schema file, no Eino change.

**Spec:** [Complete deletion design C11, §10.1a, §11.5](../../../plans/2026-10-19-complete-deletion-plan.md). Epic index: [index.md](index.md). Predecessor: [W02](W02-adapter-settings-recovery.md).

**Story:** W03 · **Epic:** E-WARMING · **Requirements:** R1, R5, R6.

**Immediate predecessors:** W02.

**Readiness gate:** W02 accepted: serving comes from the package, the core policy body is deleted and the `cache.warmed` producer runs on the Host. Current request authorizes planning only.

## Global Constraints

- Remove only hash-only prefix diagnostics and the dead core-owned warming evidence/state the cutover created. Do not remove real behavior: prefix construction, the provider maintenance request, the 60 s timeout, the in-flight guard, cancellation, the reported tokens, or the real statuses `warmed`/`skipped`/`failed`.
- No hash is reintroduced: no digest-named field, no hash-only JSON, no prefix fingerprint, no renamed equivalent, no disabled verifier.
- No production Journal access, no database reset, no schema addition, no commit/push by the implementing agent.
- Do not edit issue-91 docs, the research re-anchor file, other epic packages or `docs/TODO.md`.
- Do not claim a no-op removal: the Story must record the exact absence evidence and the exact conditional removal path.

## Review Focus

- Absence vs removal: a reviewer must distinguish "verified absent at Baseline A" from "removed by this Story" and require evidence for both. A missing file is not proof; the scan and the installed symbols are.
- Conditional removal correctness: when the newer body is present, the diff must delete the whole evidence chain (producer field, DTO/JSON property, reason/status, tests, fixtures, UI/schema property) and keep the paid request. A partial removal that leaves a hashing helper is a rejection.
- Residue scope: only warming-owned files are scanned for hash identifiers; `model.request`'s unrelated `digestModelRequest` and other domains must not be touched or falsely flagged.
- Event contract: the `cache.warmed` JSON keys are exactly the documented set; no prefix/digest property; historical rendering stays passive and cannot instantiate warming.
- Dead code: no helper, comment or test remains that only the deleted core policy used; no second `cache.warmed` producer.

## Baseline A mapping (202aac38, 2026-10-07)

Verified in this tree on 2026-10-19:

| 10-18-plan claim / symbol | Baseline A reality | Disposition |
| --- | --- | --- |
| `PrefixSHA256` on the warmer/payload | **absent** — `internal/runtime/cachewarmer.go` has no hash and `payloadCacheWarmed` (`internal/runtime/payloads.go:191-203`) has no prefix field | W03 records absence; guard added |
| hash-only JSON prefix encoding | **absent** — the warm request uses real `schema.SystemMessage`/`UserMessage` and `schema.ToolInfo` (`cachewarmer.go:183-191`) | no change |
| `prefix_digest_failed` status/reason | **absent** — only `warmed`/`skipped`/`failed` and reasons `below_min_savings`/`below_token_floor` exist | no change; status/reason vocabulary stays |
| `schemas/events/payloads/cache.warmed.json` | **absent** — the schema directory has no warming file, while a subset of events do | out of scope (Q5); no schema added, no hash property exists |
| warming UI/settings consumers of a digest | **absent** — zero warming references in `ui/src` | no change |
| engine fields kept "for the cache warmer" comment | `engine.go:138-144` says `instruction`/`toolInfos` exist so a warm request can carry the prefix | comment updated to the Host's `ReusablePrefix` ownership; fields stay (they belong to the agent/prompt path) |
| maintenance unobserved comment | `model_stream_observer.go:89-96` documents `withoutModelCallObserver` | keep; wording updated only if the Host name changed |
| core warmer policy body | deleted by W02 (cutover) | W03 verifies no residue remains |

If the gated post-baseline batch track has re-introduced the newer `cachewarmer.go` body before W03 executes, the symbols above exist and the removal task below applies in full; the Story then records which commit/batch introduced them and removes them in this same change.

## Files and ownership

- Create: `internal/app/warming_evidence_test.go` (hash residue guard over an explicit warming file list, plus the `cache.warmed` key-set assertion).
- Modify only if residue exists: `internal/runtime/cachewarmer.go`, `internal/runtime/extensions/warming/adapter.go`, `internal/runtime/payloads.go`, `plugins/exp/warming/policy.go`, `sdk/port/loopexp/warming.go`, and the tests/fixtures that asserted the removed evidence.
- Modify: `internal/runtime/engine.go` and `internal/runtime/model_stream_observer.go` comments only (no behavior).
- Do not touch files outside the warming path; do not add a schema, UI property or migration.

## Tasks

- [ ] 1. Produce the Baseline A absence evidence and record it in the handoff: run `rg -n "PrefixSHA256|prefix_digest|prefixDigest|prefix_hash|prefixHash|fingerprint" internal/runtime internal/runtime/extensions plugins/exp sdk/port/loopexp schemas ui/src` and `rg -n "sha256|SHA256|digest" internal/runtime/cachewarmer.go internal/runtime/payloads.go plugins/exp/warming sdk/port/loopexp/warming.go`, and attach the exact zero/whatever output. Distinguish unrelated matches (e.g. `digestModelRequest`, `HashSourceTree`) and state why each is out of scope.
- [ ] 2. Removal (conditional on task 1 finding the newer body): delete `PrefixSHA256` (and any prefix-hash field) from the warmer, the payload/DTO, the JSON encoding, any event schema/UI/fixture property and any test asserting it; delete the hash-only JSON prefix encoding and construct the request from the real messages/tools; delete `prefix_digest_failed` and any status/reason that exists only to describe a hashing failure. Keep the real provider request, timeout, in-flight guard, cancellation, token reporting and `warmed`/`skipped`/`failed`. Leave no alias or disabled branch.
- [ ] 3. Residue sweep after W02: read `internal/runtime/cachewarmer.go` and confirm it contains only Host/lifecycle code (prefix derivation, policy-input assembly, maintenance request, diagnostic, timer/guard). Delete any helper, constant, comment or test that only the deleted core policy used; update the `engine.go` comment to say the Host derives the prefix from the observed invocation; update the `model_stream_observer.go` comment only if it names the old warmer type.
- [ ] 4. Verify a single producer and consumer story: `rg -n "EventCacheWarmed" .` must show exactly one producer (the Host) plus the domain registry and tests; no second diagnostic producer, no warming consumer that would activate the feature in an unselected build.
- [ ] 5. Add `internal/app/warming_evidence_test.go`:
  - hash residue guard over an explicit list — `internal/runtime/cachewarmer.go`, `internal/runtime/extensions/warming/*.go`, `plugins/exp/warming/*.go`, `sdk/port/loopexp/warming.go` — failing on `PrefixSHA256`, `prefix_digest_failed`, and hash-only prefix identifiers (`(?i)prefix[_-]?(hash|sha|digest|fingerprint)`), with comments admissible only when they describe absence (exclude the guard file itself);
  - `cache.warmed` key-set assertion: marshal `payloadCacheWarmed` and assert the exact key set `{call_id, provider, model, mode, status, reason, prompt_tokens, cache_write_tokens}` with no prefix/hash/digest property.
- [ ] 6. Keep the passive historical-event contract test honest: an unselected build's event rendering (RPC replay/consumer) must accept a historical `cache.warmed` event without constructing warmer state or enabling a control. If a suitable test exists in `internal/rpc` or `ui`, extend it with a historical `cache.warmed` fixture; otherwise document the existing generic rendering path and add no new surface.
- [ ] 7. Update only the docs this Story makes true (comment-level); do not edit the epic index's status or other planning packages. Return the residue report to the Lead.
- [ ] 8. Final scan: `rg` over the warming path for the patterns above plus `cache.warmed` key names, and `git diff --check`; record the exact output.

## Verification commands and expected read

Implementation-time (toolchain absent in this planning environment):

- `go test ./internal/app -run 'Warming|CacheWarm|EinoImports|PluginWindow' -count=1` → exit 0.
- `go test ./internal/runtime ./plugins/exp/warming ./sdk/port/loopexp` → exit 0.
- `rg -n "PrefixSHA256|prefix_digest|prefixDigest|prefix_hash|prefixHash" internal plugins sdk schemas` → no match in the warming path (record unrelated matches with disposition).
- `rg -n "EventCacheWarmed" .` → one producer, the domain registry, tests.
- `git diff --check` → exit 0.
- `just ci` (implementation time) → record the exact result.

## Acceptance, rollback and handoff

The warming path has zero hash-only prefix diagnostics: either they were verified absent at Baseline A (with the recorded evidence) or removed completely, with no alias/disabled path; no dead core policy/evidence residue remains; the event contract has no prefix/hash property and exactly one producer; the guard test fails if any of these is reintroduced. Return the absence/removal evidence, the changed paths, the residue scan output, the guard and key-set test names, and the CI result.

Roll back with the Story commit; never restore a hash to satisfy a historical test, and never delete the real provider request, timeout or token evidence to make a residue scan quiet. Hand off to the Lead for the epic acceptance review: the epic's claim is that warming is a selectable real package with hash-free evidence, not that the whole `#91` deletion is complete.
