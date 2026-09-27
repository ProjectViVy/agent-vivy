# Acceptance — Issue 39 ORCH-01

## Gate decision

**G0 is BLOCKED.** The D15 recovery implementation and bounded Service/Eino proof pass focused post-fix Go, provider-conformance, vet and UI checks. A full `go test ./...` passed before the final policy fix; the final rerun was blocked by automatic review because a test attempted an unapproved request to `api.deepseek.com`. The repository `just ci` recipe and real PostgreSQL DSN-backed conformance are also unavailable. Do not treat any of these missing gates as a pass.

## What the evidence establishes

- A stable `(Run ID, operation ID)` identifies each durable tool operation. Admission binds the request and effective arguments; SQL state and Journal lifecycle transitions commit atomically.
- The D15 `tool.operation` lifecycle event carries hashes and lifecycle only; the SQL operation row retains invocation bytes for recovery. Existing `tool.requested` and approval-required event payloads keep their pre-existing contracts and redaction/projection behavior.
- Approval resume re-runs current pre-tool middleware against the original middleware input and fails closed if its effective arguments differ from the admitted operation. The stored operation cannot bypass a changed approval binding.
- A current `PolicyDeny` remains authoritative even when a caller presents approval: it rejects initial broker execution and completed-operation replay. Approval can satisfy `PolicyPrompt`, not override denial.
- The native Eino Workflow proof exercises parallel A/B work, explicit join outputs, approval pause/resume, sibling progress, cancellation, checkpoint/prompt identity checks and completed-node reuse through Service.
- The four-boundary process crash matrix proves completed outcomes can be reused and uncertain claimed outcomes do not replay the external effect.

## Scope and release posture

This is a bounded feasibility and recovery proof, not the public child/DAG product. ORCH-02–08 remain gated; no public product surface is released. PostgreSQL schema acceptance remains pending real backend execution. The previous historical NO-GO record remains unchanged; this attempt does not claim universal exactly-once external effects or guaranteed automatic continuation.

The complete command record and operation IDs, Journal sequences, effect counts and recovery outcomes are in [verification](verification.md). No commit was created, in accordance with repository `AGENTS.md`.
