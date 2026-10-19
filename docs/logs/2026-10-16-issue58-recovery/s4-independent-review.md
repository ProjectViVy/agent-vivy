# S4 independent review — fixed candidate 168393b0

Decision: **one Important inherited integration blocker; no additional substantiated Critical, Important or Minor finding in the reviewed S4 change. Do not accept the assembled resident runtime path yet.** Root owns the consolidated author fix and final acceptance.

## Exact scope and environment

- Candidate: `168393b0a70d2406de01c2307091c3c4c67fce3c`; tree `372d9ff3f2e66eadb60310877616a83107d5d3d0`.
- Source review: predecessor `3e13019b` through candidate, including S4 backend `94bff801` and S4 UI. Evidence archives and bulk historical documentation were not treated as implementation.
- Isolated clean clone: `/workspace/scratch/5dda642e718d/agent-vivy-issue58-s4-review`. Final tracked status and diff check are clean. Candidate source/test files were never edited; test-only Go overlays and external Vitest probes are in this review directory. Normal UI staging regenerated ignored projection files.
- Sibling Laputa was directly checked at `bb57e93a61cfb41460524d00a557a3a9f2cce1e7`; Go 1.26.4 and Eino 0.9.20. Parent-reported remote tree-equivalent commit `0a762f25a75bfee1a7cbb3ad7bcda346e6f18f2e` was not independently fetched; its object is absent from this isolated clone.
- Read root/UI AGENTS, S4/C4, recovery index and S3 pending-intent/summary rulings; applied receiving/requesting-code-review, verification-before-completion, vivy-eino, vivy-plugin and testing-vivy-ui guidance. The checkout's `oil-frontend` skill directory is empty; no UI implementation was attempted.
- Every executed suite used the existing credential-name-stripping, offline-dependency, loopback-only deny-proxy runner. Metadata records zero external requests forwarded and zero rejected proxy attempts. All model responses are synthetic localhost HTTP. No tenant data, browser reinstall, deployment, GitHub mutation or subagent was used.

## Important I1 — production App omits the existing tool evidence BlobStore

**Locations:** `internal/app/app.go:938-995` (`runtime.NewService` dependencies, especially adjacent `ShellState: backend.Blobs()` at line 952); `internal/runtime/service.go:496-499`; `internal/runtime/recap_compaction_pending.go:176-184`; `internal/runtime/recap_context.go:179-181`.

`NewWithAssembly` supplies the actual bound Recap owner and trusted main-conversation resolver, but never supplies `ServiceDeps.ToolEvidence`. `NewService` infers `ToolOperations` from the Journal but has no corresponding BlobStore inference. Every resident admission calls `resolvePendingRecaps`, which returns `ErrRecapUnavailable` immediately for the resulting nil evidence store. The same omission also prevents exact native tool-evidence retention and pending-compaction metadata once those paths are reached.

**RED proof:** `probes/app_native_probe_test.go`, test `TestReviewS4RealAssemblyNativeTools`; final output `raw/s4-review-assembly-red.log` and execution metadata of the same stem. It creates the real generated default Assembly with `NewWithAssembly`, real Garden owner, real SQLite backend, a disposable trusted singleton resolver, a published catalog model (`deepseek-flash`, known window 1,000,000), and only a local fake HTTP provider. Before the run, current `AuthorizeRecap` and `ContextStatus` both succeed. `Service.Run` then records `run.started` followed by `run.failed` with `recap_unavailable` **before any HTTP request**. The test fails its completed-run assertion. Its subsequent same-owner edit, C2 seal and host reopen assertions are not reached and are not claimed as passed.

Initial attempts also failed and are preserved as `s4-review-assembly-probe*`; they used the old `deepseek-chat` model identifier. The final RED removes that unrelated capacity ambiguity and records the successful authority/status prechecks. Original attempt source variants are retained alongside the final probe.

**Impact:** the real host cannot execute the advertised resident Recap tool workflow even though lower-level runtime tests pass. Those fixtures explicitly inject `ToolEvidence: backend.Blobs()`, masking the missing production dependency. The omission already exists at predecessor `3e13019b`; this is an inherited Issue58 integration blocker, not a claim that S4 newly introduced it. The S3 implementation report's statement that production App already wires the blobs is incorrect.

**Smallest correction:** supply the existing `backend.Blobs()` to `ToolEvidence` in the production `ServiceDeps` construction. Keep one backend/owner; do not bypass pending checks or relax evidence/seal guards. Promote an actual `NewWithAssembly` native-run regression so it proves deferred search, native update of the same owner, exact tool evidence, one C2 interaction, and normal host close/reopen followed by a successful next admission. Re-run the unchanged reviewer RED after the consolidated author fix. A dependency-only fixture repair is insufficient.

## Verified boundaries and evidence

| Executed check | Result and actual scope |
|---|---|
| Focused candidate Go Recap suites | PASS: 20 top-level tests / 34 passing events across runtime, cognitive Module and App; one explicit opt-in split-browser fixture skip. Includes current authority, CAS, restore/clear, exact UTF-8/closed input and real authenticated control Peer/ActionHost read/edit. |
| Independent runtime native probes, final exact source | PASS: 2 top-level tests / 6 passing events. Successful selection, no match, invalid query type and empty query each retain exact raw arguments, exactly one tool evidence row, seal once, reopen SQLite/owner and admit the next run. Ordinary-session administrative activation cannot make the Recap schema/reminder/search result visible; guessed invocation never calls the protected tool. |
| Ordinary discovery and lifecycle regression selection | PASS: 8 tests, including official Eino dynamic search, no-static-tool search, hidden mounts, reserved name, actual MCP engine replacement and bounded App shutdown. |
| Normal staged UI focused suite | PASS: 22 tests / 3 files. The normal `pnpm --dir ui test` pre-hook ran the SDK staging command, and the authoritative Recap source tests executed. |
| Independent UI asynchronous probes | PASS: 3 tests using an external Vitest config with the same source aliases and normal staging pre-hook. Ambiguous failure reuses operation ID, clear retains the dirty draft/base and requires explicit conflict resolution, and a late save cannot contaminate another session or delete its draft. These are component-host mocks, not browser execution. |
| Real default Assembly resident native tool probe | **FAIL I1**: no provider call; later tool/seal/reopen assertions unexecuted. |
| Final tracked source status / diff check | Clean / PASS. `source-hashes.json` records affected source hashes; `raw/source-integrity.txt` captures SHA/tree/status. |

The backend facade derives subject/domain from the current trusted service authorization and rechecks authorization before conflict reread. Recap tools bind through the generated ToolHost to the same bundle; schemas are deferred, and invocation independently checks persisted primary-run, policy and selected-tool pins. No alternate owner/parser/store was found in S4. The new authorization shim uses `github.com/cloudwego/eino/adk/middlewares/dynamictool/toolsearch.New` and the official `BeforeAgent` / `BeforeModelRewriteState` search and selection logic. Its custom work is the unavailable upstream authorization hook and existing durable operation/evidence bridge, not a second search algorithm.

The UI remains in `vivy/memory`, uses the SDK action client and existing kit, retains BML controls, separates draft/base from committed head, and represents loaded revision, consolidation and sources as unavailable. It does not fabricate source excerpts or measured LIFE token totals. Exact source expansion, actual loaded revision and consolidation status are approved S5 dependencies; their documented unavailable state is not a review finding.

## Limits and handoff

No fresh real split-3015 browser run or screenshots were produced: the pinned Chrome installation had previously failed, and reinstall/downgrade was prohibited. The copied `ui/dist` only satisfied Go embedding, never UI validation. Parent independently reported full fixed-168 UI test/typecheck/build (707 tests / 93 files); that report is separate from this reviewer's directly executed evidence. This review did not rerun full `just ci`, broad/race suites, real PostgreSQL, native Windows, live-model quality, installed #66 producer, or final S6 acceptance.

All original logs and failed attempts are retained. One consolidated author TDD fix pass follows. Root should rerun original probes and required affected gates; this report does not authorize a redundant second review of the same fix diff. Whole-candidate S6 review remains a later, separate integration scope.
