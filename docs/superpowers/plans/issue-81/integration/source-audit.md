# Source-of-truth audit and finding dispositions

Observed 2026-10-18 UTC. Source presence and old author test records are not fresh verification or acceptance. Main was fetched before the plan branch was created; the branch starts exactly at `7b42b2920f72de6f37de8b9e4ff4bf6a110a762f`. Existing Git author identity is human (`Mastwet Cany`); no AI author/co-author trailer is added.

## Existing remote artifacts reused

| Source | Exact inspected identity | Reuse / correction |
| --- | --- | --- |
| [Epic #81](https://github.com/ProjectViVy/agent-vivy/issues/81) | Updated 2026-10-18 08:34:55Z | Current E1-E9 scope; historical audit is subordinate to owner boundary. |
| [#64](https://github.com/ProjectViVy/agent-vivy/issues/64), [#69](https://github.com/ProjectViVy/agent-vivy/pull/69) | `f2be7f614035927547571127bea73c25b516d614`, open/unmerged, reported mergeable | Existing 20-file bridge, revision lifecycle/provenance, restart logic and tests. Source is branch-only, not main. No separate new Skill design found in this branch; #64 and research guide govern. |
| [Laputa #9](https://github.com/ProjectViVy/laputa/pull/9) | `b3a18e16a93c0b593e4a7f296b4e3ef24b040637`, open/unmerged | Existing `skill_proposal` effect and discovery/read contract; consume together with #69. |
| [ACP branch](https://github.com/ProjectViVy/agent-vivy/tree/21c5b8bb49edd1502a033b0ca557737e102b9551) | `21c5b8bb49edd1502a033b0ca557737e102b9551` | Reuse `docs/architecture/ACP-STDIO-FACE.md`, `docs/research/acp-sdk-compatibility.md`, `docs/superpowers/plans/issue1-acp-face/ACP-01.md` through `ACP-05.md` and index. `2026-10-07-acp-pilot/README.md` explicitly redirects to these five Stories. |
| [ACP SDK fork](https://github.com/mastwet/acp/tree/780fb4dd4b98b3936fe572f61ae83b3abc2f918d) | Remote branch `feat/agent-conn-options-sanitize` resolves to full `780fb4dd4b98b3936fe572f61ae83b3abc2f918d` | Actual plugin go.mod already selects the accepted fork. Compatibility report's baseline v0.0.4 failures are historical input, not proof that no correction exists. Re-execute probes in E8-01. |
| [#32](https://github.com/ProjectViVy/agent-vivy/issues/32), [old plan](https://github.com/ProjectViVy/agent-vivy/blob/6045c0a69a74c04a1a6221d93c0d7bb32ae7a6e1/docs/superpowers/plans/issue32-remediation/index.md) | `6045c0a69a74c04a1a6221d93c0d7bb32ae7a6e1` | Reuse P2-P6 designs, implementations, regression names and logs selectively. Old branch status says engineering verified, but is not current-main verification. Never merge its complete tree. |
| [Laputa Mission-gate branch](https://github.com/ProjectViVy/laputa/tree/30fa208e3cded4af43f8cb226d80b225b1910854) | `30fa208e3cded4af43f8cb226d80b225b1910854` | Existing #32 dependency is published and was read: `ApplyAtMissionRevision`, shared owner gate, paired human writes and two regression names. A combined #9/#10/peer repair pin remains unverified. |
| Main #49 | Merge `794c4bb2` | Existing cognitive CAS and mandatory settlement repairs; retain instead of copying older versions. |
| Main #50 | Merge `906ab676` | Removed embedded host/C ABI path; older “retain sdk/host/v1” instruction in #32 is superseded. |
| Main #42 / #40 | Merge `3c90c586` | R2 recursive redaction proposal superseded by owner text-preservation decision. |
| [#79](https://github.com/ProjectViVy/agent-vivy/pull/79), [Laputa #10](https://github.com/ProjectViVy/laputa/pull/10) | VIVY `b4c03754d2798404234ab7cd7f49bbb26e2bec63`; Laputa `27da7b9e2d991857650b9e373627b65119a73eec` | Existing draft recovery candidate; VIVY reported conflicting with main. Only dependency ledger here. |
| [Peer repair branch](https://github.com/songbai-vv/agent-vivy/tree/c158f2eaea29dc88ad6c6f50901c882b12a6ebe4) | `c158f2eaea29dc88ad6c6f50901c882b12a6ebe4`, now remotely visible | New observation supersedes branch-not-found at 08:26Z. Ten commits after #79; fetch/read only. Publication does not mean applied to #79 or accepted. |
| [#80](https://github.com/ProjectViVy/agent-vivy/pull/80) | `001c563740a4ff32e1376f6bbfb2c6caaa7b22b3`, open/unmerged | Existing Provider spike/upgrade/recovery work. CI failure and normal-cancel limit remain from PR evidence; no implementation assigned here. |

Main Laputa source lock: `4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24`. The selected environment's sibling Laputa checkout is `f44ba00eb6ab5aa6ab9a2f5c09868373e36679f5`; it was read only and is **not** the main lock. Future candidate execution must use its own exact locked sibling, not this environmental mismatch.

## ACP reality versus stale status

The ACP branch contains `internal/faceprocess`, selected-Face dispatch, `plugins/acp/{module,transport,agent,control,input,prompt,reduce,projection}.go` and tests. Its source implements the SDK fork controls, canonical session root, per-call digest windows and cancellation latch. It does **not** contain `interactions.go`, permission/question implementation tests or a final ACP Recipe. Existing ACP-04/05 checklists are unfinished. The old index still calls G1 unscheduled even though ACP-02/03 implementation commits exist; recover implemented work and reconcile status instead of treating this as a blank proposal. Main already has `face.Options.In`; do not add it twice.

Canonical owner D2 eliminated the presentation facet and generic display sanitization. Older ACP-04 “sanitizer” references and AC-05 sanitized-output wording conflict with that ruling; E8-01 corrects those references in the existing docs. SDK generic-error/panic hygiene and original-byte digest integrity stay required. The planning request itself does not schedule ACP G1 implementation.

## All 28 original #32 findings mapped to current main

“Fixed” means the named source correction is present; rerun required coverage on the eventual candidate. “Applicable” means the inspected trigger remains in source; no new dynamic reproduction is claimed here.

| ID | Current-main disposition and direct source evidence | Owner / next action |
| --- | --- | --- |
| H1 | External DIVA native-window authorization | DIVA #32 handoff; no VIVY release blocker |
| H2 | External DIVA host CI trigger/aggregate | DIVA workstream; do not revive go-host |
| H3 | External DIVA release acceptance gate | DIVA workstream; no publication here |
| H4 | Removed: `sdk/host/v1/host.go` and `internal/embedded` absent after #50 | Preserve removal; no logger transplant from obsolete host |
| H5 | External DIVA native shutdown deadline/errors | DIVA workstream |
| H6 | External DIVA frontend native listeners | DIVA workstream |
| H7 | External DIVA second-instance handoff | DIVA workstream |
| C1 | Fixed: `cognitive_service.go` carries original version into save and serializes mutations under `cogStateMu` | Keep main implementation and its tests; no duplicate CAS rewrite |
| C2 | Applicable: `cognitiveAttempt` starts workflow before saving ActiveRunID/PendingThrough; deduped terminal increments Attempt | E9-01; reuse old P2.2 exact-window intent |
| C3 | Applicable: fresh Resolve binding is followed by `b.Binding.CheckMissionRevision(current)` | E9-02; validate fresh binding, preserve atomic Mission apply |
| C4 | Applicable: factory `ResolveBinding` hashes mutable `policySnapshot()`; dispatch still has `setPolicy` | E9-02; durable policy argument, remove duplicate authority |
| C5 | Original omitted fields fixed in `CognitiveStatusView` and app projection; app still reads status/policy separately | Retain fixed fields; E9-03 reuses old coherent-snapshot regression, no DIVA UI edit |
| C6 | Applicable: `cognitiveControlPort.Cancel` calls general `CancelRun` with supplied ID | E9-03; exact current strategy validation and terminal-race check |
| C7 | Applicable: cognitive bundle opened before later failure returns; cleanup remains scattered | E9-03; one failure-cleanup owner until App transfer |
| C8 | Original affected UI is DIVA; VIVY backend already reports deferred capability | DIVA workstream; do not treat VIVY provider catalog as the original defect |
| W1 | Applicable: admission tool list checks membership without duplicate rejection | E9-04 / E3 single workflow lane |
| W2 | Applicable: face bridge creates a fresh UUID each start; UI retains no immutable retry intent | E9-04 / E3 |
| W3 | Applicable: storage ETag derived from UpdatedAt/CreatedAt counter that can repeat under same/backward time | E9-04; reuse opaque-token regression in both drivers |
| W4 | Applicable: run cursor is CreatedAt only in both driver list implementations | E9-04; `(created_at, run_id)` keyset |
| W5 | Applicable: WorkflowsPane/RunsPane list calls consume only first page | E9-04; continuations, session/refresh fencing |
| W6 | Applicable: UI terminal vocabulary has succeeded, while native RPC returns completed; engine status omitted in client type | E9-04; native/engine projection separately |
| W7 | Applicable: null create ETag flows to empty unconditional update | E9-04; explicit absent/create or concrete edit token |
| W8 | Applicable: definition cursor `SplitN(..., ":", 2)` breaks colon-containing IDs | E9-04; parse last separator, validate revision |
| W9 | Applicable: production `orchestration.go` still compiles direct Eino proof and legacy resume route | E9-06; test-only proof, no removed host restoration |
| R1 | Fixed: `Diagnostics.Read` accepts same-identity growth and only uses mtime for same-size rewrite | Retain current platform file identity and append regression |
| R2 | Superseded by owner #40 and merged #42 | No generic redaction work; preserve authorized text |
| R3 | Applicable: `ReadBytes` materializes full line before budget check; clip only fields/message, not complete encoded envelope | E9-05; reuse old bounded-reader tests without overwriting main R1/platform fixes |
| R4 | Fixed: `observeGenerate`/`observeStream` join/return `obs.End` errors; main settlement behavior must survive transplants | Keep main and fault tests; #80 changes remain separate |

Coverage: **28 unique findings** = 7 external DIVA, 1 removed H4, 3 fixed C1/R1/R4, 1 superseded R2, 1 partly fixed C5 and 15 applicable VIVY residuals. External IDs are `H1,H2,H3,H5,H6,H7,C8`; applicable IDs are `C2,C3,C4,C6,C7,W1,W2,W3,W4,W5,W6,W7,W8,W9,R3`. No finding is hidden as a generic follow-up.
