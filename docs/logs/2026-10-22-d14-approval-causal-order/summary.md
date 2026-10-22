# D14 approval test causal ordering

The native full-CI run at frozen candidate
`1b8da8cca150c7a4a7734c6e480fa41e24b230cc` exposed a strict test-order assumption:
the next main `model.request` can precede reconstructed `tool.started/finished`.
The fixture has no warmable catalog, so this is not maintenance interleaving.
Native Eino's async iterator lets its producer continue after the durable tool
operation has committed while the consumer still projects the tool result.

Only `internal/runtime/approval_test.go` changed. The test retains the strict
approval/policy/claim/completion prefix and now validates approval identity,
operation identity/state, and completed-operation sequence before the next main
request. The derived tool projection can interleave with that request, while
`tool.started < tool.finished < first model.delta` remains mandatory. Call/result
bindings, final finish/completion/terminal suffix, exactly one terminal, and the
assistant reply remain strict.

A small test-only model wrapper uses the existing fixture's model injection to
check the actual provider input: its approved tool call ID/name and result must
match the already-completed successful durable operation before Stream proceeds.
Its evidence channel is nonblocking and adds no runtime synchronization.

This matches shared deletion design section 11.4. Inspected pinned Eino v0.9.21
`adk/utils.go` AsyncIterator/AsyncGenerator and its unbounded channel; no Eino
implementation or runtime behavior changed. No consumer acknowledgment barrier,
warming toggle, new dependency, production-state access or release was added.
