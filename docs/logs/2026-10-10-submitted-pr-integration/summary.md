# Submitted PR integration

Scope frozen at task start: main e67e6ffaf1b6d71b8acb18b7cf304c669e00bb60; #47 notebook at 9520c5644b4df7521cc896d8a158724754951c70; #49 fix/pr46-dependency-durability at 701cc10115fabb181923668a423aabfff7b4002b; #50 chore/remove-embedded-host-c-abi at 393c84e10ac7b2e6292b842dad2ca19ace33635b; draft #52 fix/development-source-gates at aaaea752023b90ee40a5985b9bd89325fc6420ff. Future PRs and other repositories are excluded.

The user authorized sequential conflict resolution and merges before consolidated CI repair. Required protections remain authoritative. No deployment, forced shared-main updates, feature removal beyond #50, or weakened tests.

Order: #49 preserves #48 while supplying runtime durability and catalog metadata fixes; #50 removes deprecated host paths; #47 brings notebook/reports with immutable released migrations and additive upgrade semantics while the #52 freeze handoff is pending; #52 then removes authored development source-hash gates.

#49 resolution retains #48 HNSW pseudo-version and #49 consistent Laputa module identities at 4b2bec2cc2ab. Existing conformance records remain historical producer evidence; no combined-source passing record is fabricated. Consolidated live behavior verification follows #52. #48 resource shutdown, SQLite initialization, Windows tests and DIVA memory-loop gate remain present.

#47 restores both released migration 017 files byte-for-byte; existing additive migration 039 remains the sole cron revision-column addition. Frozen released-byte fixtures reproduce checksum drift and upgrades from heads 16/17/23/35. Missing cron-table repair also exercises 017 then 039. The old repair test now seeds the actual historical head rather than pretending the latest schema was a head-016 database. Annotation preservation is asserted across promoted and candidate regenerations in both backends. Catalog conflict resolution keeps #49 pure metadata imports and #47 notebook/report inventory.
