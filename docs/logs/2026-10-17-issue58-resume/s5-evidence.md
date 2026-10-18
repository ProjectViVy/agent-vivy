# S5 implementation evidence and remaining acceptance gates

Candidate: e50b74ea2819cd925c738045fa67b71b29f00215; Laputa 27da7b9e2d991857650b9e373627b65119a73eec. Official Linux just ci EXIT0; browser2 PASS13.2s; targeted race runtime32.804s/App6.664s PASS; Windows runtime/App CROSS-COMPILE PASS only. Product S6 acceptance remains open.

## Task1b: historical source originals

- Runtime resolves the exact historical owner revision/certificate, not latest-head sources. Automatic ranges verify retained core digest/sequence/binding; restored revisions follow only the durable decreasing restore chain. Manual edits/clear do not invent certificates.
- Historical corrections expand only the receipt-selected view/message subset; changed correction/range digests and revoked source grants fail closed. No BML index or model call is used to recover range originals.
- App/action/UI expose originals with exact provenance, immutable attachment/file/reference snapshots and unavailable reasons. Browser historical revision2 expands retained original + file contents after real derived BML tombstone.
- Concrete tests: TestIssue58RecapSourcesExactHistorical, TestIssue58RecapSourcesHistoricalCorrectionSubset, TestIssue58RecapSourcesRejectChangedOriginalDigest; dispatcher authorization tests; ui/e2e/issue58-recap.spec.ts. Browser at e50: 2 pass13.2s, no skips; previous frozen007b runtime CI/race passed. Current full CI passes these packages.

## Task2: native consumption, projection, old-path exit and loaded truth

- App construction binds one subject sink; duplicate IDs fail. Terminal C2 originals are sealed once; deterministic operation identity and native payload receipt permit safe reply-loss retry. Delivery cursor completes only on matching canonical success plus ready/not-required index. Pending and failed index receipts never masquerade as success.
- Existing legacy cognitive/BML observers explicitly skip persisted LIFE runs; ordinary Coding paths remain. The native v3 old-writer refusal is retained, with ordinary raw original capture still working.
- Native memory_search returns authorized bounded cards; memory_get requires exact revision and expands only current subject scope. Actual deferred tool_search -> search -> get model-leg tests exclude an unqualified machine-home row and wrong owner.
- Legacy memory_list/write schemas are explicitly unavailable under LIFE. No claim that all legacy mutations were migrated. Default selections/production activation were not changed.
- Loaded Recap tracks the admitted main model.request, independently of later head edits; foreign sessions, terminal runs and absent/restarted evidence do not report a fabricated loaded revision.
- Concrete tests: TestIssue58SubjectProjectionNativeRetry (actual native mutation, response loss + injected pending/failed index receipts), TestIssue58SingleProjectionSink (actual App, qualified tools, duplicate delivery and ordinary Coding after v3), duplicate-provider composition test, TestIssue58LoadedRecapTracksAdmittedModelNotNewHead.
- Limits: index-state fault injection is not a claim of actual delayed SQLite index behavior. A trusted test singleton is not the installed external #66 producer. LIFE remains disabled without it.

## Original archive-failure regression preserved

TestMemoryLoopActivityFailurePreservesSourceAndArchiveRetry has two independently asserted guards:
1. unreadable-head: ACTMEM.MD is a directory; exact recap_unavailable from the early policy/owner read.
2. unclassified-legacy-head: valid readable nonempty legacy head; canonical original exists and activity watermark remains0; actual public session/delete reaches ArchiveSession -> projectActivity -> AppendSnapshot, returning actmem_unclassified_head with session activity archive wrapper.
Both retain original messages, restore the fixture, await real activity, preserve ingestion/record/revision and exactly one primary provider request, retry the public archive action, and inspect the original fact/session in native capsules. No broad error acceptance, removed test, weakened timeout or mocked archive success. Rejected capsule-directory experiment is preserved as failed diagnostic evidence, not acceptance.
Actual SDK DIVA targeted two-subtest PASS at its recorded prior source commit; whole frozen007b DIVA suite PASS776.199s; e50 whole DIVA suite PASS773.544s.

## Remaining / blocked

- Native Windows execution: pending, Linux environment only. Cross-compilation passed at e50b74ea and cannot close owner-lock/crash-cut/restart or Windows mission regressions.
- Real PostgreSQL: pending; postgres/initdb/psql absent, no configured test service supplied. Cached package checks are not live parity.
- Real provider repeated-fold quality and actual owner-led DIVA/Mentle installed acceptance: not performed; deterministic localhost HTTP fixtures are not substitutes.
- External #66 installed singleton/entry: missing external producer, no unauthorized sidebar/product redesign added.
- S6 consolidated installed acceptance corpus and independent whole-candidate review: pending. Constituent adversarial tests and self-review do not satisfy those gates.
- Artifact persistence: local bundles/patches verified, but external backup failed before upload preparation (network); no file ID. All local copies share this temporary environment and can be lost with it. No further upload attempts/new environment/push.
