# Independent S2 provenance reimplementation review

Reviewed source: `0f27910a8f6c18bd9b9f1acad6e5c06be35116a4`, tree `23e7332d8b71f21993b53745f2bf2abde1d35ac7`, against `b91096cca2028a8e8b127f60d41da600d64bb7eb`. Published as tree-equivalent `6694dc150559fa58fb56f1b70098fd29c54f08fe`. Reviewer: independent GPT-6 Astra High, read-only, fresh context. No independent tests were run by the reviewer. No original lost-code GREEN was inherited.

**Disposition: REQUEST CHANGES. No Critical findings; two Important findings.**

## I1 — streamed invalid arguments lose earlier received bytes and identity

`internal/runtime/service.go:3408–3417` journals `toolCallEvents(chunk)` when one native stream fragment contains invalid UTF-8. It records only the current fragment. Earlier valid argument fragments and the initial tool ID/name are not accumulated by `observedModelCall`; `mapper.go:371–384` journals text/reasoning only while streaming. A first chunk with ID/name and a valid argument prefix followed by a same-index invalid-byte suffix without repeated ID/name therefore loses the prefix and identity. Execution stops, but exact received evidence is irrecoverable. The existing native Generate/direct mapper test does not cover this multi-chunk Stream path.

Smallest correction: retain all received per-call tool fragments and identity through the existing observed-call/native ConcatMessages seam; persist exact received partial evidence with explicit unavailability in the existing Journal. Add native multi-chunk Stream + reopen + no execution/seal regression.

## I2 — automatic ingestion ignores explicit argument-unavailability evidence

`internal/storage/input_provenance.go:105–109` reads model.request only. Valid v4 original input followed by an invalid-UTF8 v2 tool.requested marker remains classified available. Refusal before execution leaves no excluded operation for `internal/app/assembly_observers.go:124–126` to detect. Automatic consumers can receive the unavailable run even though original projection/sealing rejects it. Producer supplementary fresh RED `invalid-args-ingest-red.log` independently supports the source-level finding; that test is outside the reviewed commit.

Smallest correction: the existing run-level automatic-consumer guard must reject explicit invalid/unavailable evidence with unknown-first precedence. Verify actual observer delivery is blocked, not only a predicate return.

## Assessment and limits

Substantial alignment: strict present/non-null v4 boolean, grouped unknown precedence, sealable Notebook originals, existing provenance fields, unchanged legacy stored projections, native summary Finalize metadata and Eino native serialization seams. No new engine, transcript store or dependency upgrade. Eino0.9.20 and Laputabb57 pins remain. Spec/data-integrity acceptance is blocked by I1/I2.

Focused producer/race evidence was read as reported; broad package validation was blocked by automatic approval review and has no result. Real PostgreSQL, just ci, installed UI/LIFE, native Windows, real model quality and S3 acceptance remain unverified. Same-turn complete tool output is S3 and was not assessed as delivered. Root retains final acceptance authority.

The implementer is performing one TDD fix pass for both findings; this file is the pre-fix review, not fix acceptance.

## Fix acceptance — same scoped producer, not product acceptance

Final source `8ae1db9d5adae06d10ad0b8cb44b519dd30f66d7`, treea76e8d96. Root inspected the exact I1/I2 source diff and fresh RED→GREEN logs. I1 now retains tool-only fragments, uses native `schema.ConcatMessages`, persists exact partial raw bytes/identity, and verifies SQLite reopen, no operation, no seal and ceiling0. I2 marks explicit invalid/unavailable tool.requested evidence unavailable through the shared run guard; actual ObserverHost delivery is blocked while its positive original control delivers once.

Producer focused suite and focused race pass after both fixes. Root separately executed the two exact regression tests offline and read successful output. No second reviewer round was claimed. The two Important findings are fixed at this scoped gate; broad regression/product/platform gates remain blocked/unrun as above. No Critical or Minor findings were reported.
