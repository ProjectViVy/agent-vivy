# Preserve unresolved cognitive windows

The host previously treated every native completed workflow as a processed source window, even when the owner's typed outcome was partial or recovery_required. Failed workflows could also mint a fresh attempt after a stage had already committed an authority effect. Both paths could lose or repeat effects.

Settlement now checks the typed owner Outcome, its exact admitted window and every receipt status. Unresolved windows preserve the original ActiveRunID and watermark, across both automatic and manual wakes. A failed effectful stage or unavailable workflow inspection is fenced conservatively. Pure pre-effect failures retain the existing bounded retry behavior; trigger is not a recovery operation.

The existing status projection now exposes pending_through, phase and block_reason from that same durable record. A real App/Garden/Mentle rejected-update regression first demonstrated the missing projection, then passed with a visible blocked window and no extra canonical memory. The test model derives its payload solely from actual source requests; no backend is mocked in this composition test.

Cancellation test assertions now distinguish known, durably acknowledged cancellation from uncertain effects by checking the actual unresolved-operation ledger and matching native status. No production cancellation semantics changed.

Eino capability check: orchestration remains on the pinned Eino ADK/native child and INOFY graph; this change only corrects Vivy's durable admission/settlement boundary. No additional model runtime, authority or scheduler was introduced. Same-operation recovery and the six-cut process crash matrix remain open local work; guarding a window does not implement recovery.
