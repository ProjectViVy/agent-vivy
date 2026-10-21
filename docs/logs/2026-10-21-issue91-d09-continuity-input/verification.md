# Verification

Base93463a1f includes the completed cursor-v2 verification record. Root checkout
and production Journals remain untouched. Paired039 is allocated only after
checking the integrated embedded head038.

Observed RED before corrections:

- Old input_hash remained present in the target schema (migration test).
- Identical reference retry failed once the run's reference budget was full.
- Identical presentation retry failed after its workspace disappeared.
- Existing destination session deletion left private receipt rows behind.

First shared-receipt selector PASS: runtime27.059s, SQLite14.662s and
migrations16.777s. Broader corrected runtime/reference/deliverable/continuity
selector PASS59.305s, domain0.697s, SQLite6.212s and migrations9.605s.
A temporary full ReferenceSelection.Validate call wrongly required the optional
model-preview digest; it was removed before the passing broader suite. This cut
preserves that existing model behavior for the following exact-preview closure.

Final native, independent review and exact-candidate Linux/real-PG results follow
after execution. Missing DSN, skipped3015 smoke and unrun checks are not passing.


Final native selector after session deletion and legacy-NULL defenses:
`go test -tags=vivy_headless ./internal/runtime ./internal/domain
./internal/storage/sqlite ./internal/storage/migrations -run
'TestD09(ReferenceReceipt|PresentationReceipt|Receipt)|TestContinuity|TestReference|TestDeliverable|TestDeleteSession'
-count=1 -timeout10m`: PASS23.119s/0.046s/2.967s/2.737s.
Affected runtime/domain/all storage vet PASS (including PostgreSQL compilation).
The PostgreSQL fixture still requires actual DSN execution and is not counted by
native vet. Conformance now checks receipt replay after reopen, same-size changed
input refusal, and destination-session cleanup in both dialects.


Independent receipt review found three admission gaps. Deterministic RED cases
confirmed all three: live-run retry hit the active-run gate; changed invalid
budget blocked a completed replay; changed mode/file context/attachment/provenance
was omitted from intent; storage replay after source deletion hit source checks.

Corrections keep admission/projection/deletion locks, normalize caller-selected
values, then resolve receipts before new-run gates. The private envelope now also
binds normalized mode/profile/thinking/collaboration/Face, provenance, attachments
and file-context snapshots; generated IDs/provider settings/callbacks/budget are
excluded. Fresh admission retains the existing policy/budget/allocator checks.
Storage replay locks/rechecks destination ownership without requiring live source
sessions. Fresh writes still lock sources/destination in sorted order; a source
missing during lock acquisition triggers a receipt recheck before refusal.
Focused corrected runtime PASS2.163s and SQLite PASS0.170s. Equivalent explicit
defaults are also covered before live-run replay; later results follow below.


After the admission corrections, the broader native selector PASS runtime52.488s,
domain0.071s, SQLite4.686s and migrations3.987s; affected vet PASS. Review then
found an omitted caller profile incorrectly resolved against the current server
default in the receipt. Changed-default regression failed RED. Preserve the
caller's default declaration (empty profile), while fresh admission still
resolves current policy and explicitly changed caller profiles conflict.

Final admission regressions PASS3.106s; affected runtime/domain/storage vet PASS.
Independent reviewer confirmed all admission findings and the default-declaration
correction closed, with no remaining finding in that correction.


First independent Linux9ccfe393 attempt: wiring PASS; kernel failed only three
migration count assertions still expecting38 after the paired039 allocation.
Update the two fixture owners to39; no runtime/SQL correction is required.
Kernel execution stopped the script, so PostgreSQL/Face/Web/race were not run by
that attempt and remain pending. Retain this failed attempt as evidence.

Full native migration suite after count-fixture correction PASS12.087s.
