# Independent review

Reviewer: fresh read-only `d04_final_review_v2`, with no implementation history.
Eino v0.9.21 capability check accepted typed WrapModel/WrapInvokableToolCall and
native ToolsNode/AgenticToolsNode reuse; no parallel executor/result service.

Initial findings were reproduced before correction:

- Distinct malformed JSON arguments canonicalized to the same empty tuple field:
  six distinct native requests falsely stopped the Run. Preserve exact raw
  bytes for that refusal path; cap raw/canonical values without truncation.
- Missing projection bounds: reject arguments/result/error or canonical expansion
  above 1 MiB, including native metadata tools.
- Scheduling append callback took terminal ownership before returning an error to
  consume. Existing SQLite/Postgres guards already reject a second committed
  terminal, but duplicate terminal attempts and context cancellation can obscure
  the cause. Return the original wrapped error through the one append path and
  let consume own the boundary terminal.

`TestNudgeDistinctMalformedArgumentsRemainDistinct`,
`TestNudgeProjectionRejectsOversizedValues`, and
`TestNudgeSchedulingFailureHasOneTerminalOwner` all failed on the reviewed
candidate, then passed after the fixes. The last case checks original cause,
active Run before caller handling and exactly one persisted terminal.

Delta re-review accepted all three fixes and found no unresolved correctness issue at these boundaries. The reviewer performed no edits or tests; runtime evidence is independently recorded in verification.md.
