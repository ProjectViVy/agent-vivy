# D09 shared continuity intended input

Task admission, reference attachment and deliverable presentation now bind the
existing receipt identity to actual normalized intended input. The single private
receipt field is excluded from JSON projections. Same request/input replays the
original Journal result; unequal input under an existing ID conflicts, including
same-length byte changes. No second receipt store or migration runner is added.

Paired039_continuity_intended_input follows integrated038. It removes the retired
input_hash column and adds nullable intended_input bytes. Historical receipt IDs,
run/event positions and Journal outcomes are preserved; missing exact input is
unavailable for replay and conflicts rather than being guessed from a hash.
Released001..038 SQL is unchanged. This is a lane allocation, not D14's finalT.

The shared mutation no longer duplicates input beside its receipt. The unused
source-expectation digest is removed; existence and truncation revisions remain
checked inside the admission transaction. Session deletion now removes private
receipt rows in its existing transaction. Rewind keeps durable receipts/fences
with their retained Journal outcomes; it does not authorize new source reads.

Admission receipts resolve before active-run/new-admission/budget checks, with
normalized caller-selected options included in the private input. Storage replay
requires destination ownership, while first admission still locks all sources.
Existing reference/presentation receipts resolve before mutable source reads,
workspace lookup and per-run budgets, so lost-response retries return their
already committed destination-owned result. Concurrent first commits still use
the existing transaction; terminal checks, unknown-commit fences and event
publication remain. Commit errors preserve the original cause as well as the
uncertain-outcome sentinel.

The existing Eino/native ownership path is unchanged; pinned API inspection is
in D09. Reference exact-preview binding and immutable durable presentation/transfer
bytes remain subsequent D09 cuts. Broader D10/D11 prerequisites and D14 final
artifact/release acceptance are unchanged. Standard3015 smoke is owner-deferred.
