# Acceptance

Approve the real fixture's pending write_note invocation. Before the resumed
provider receives its tool result, the exact operation must already be completed
successfully and its persisted result must equal the actual provider input.

The Journal must contain the approved decision, bound policy checks, exact claim
and completion states before the resumed main model request. The projected tool
start must precede its finish, which must precede the first assistant delta.
The approved tool executes once with the expected result, the final model call
keeps its call ID, the assistant reply is preserved, and exactly one Run terminal
is present. Request admission may precede tool-status projection after the
mandatory operation commit, as permitted by design section 11.4.

No product behavior or warming setting changes as a result of this test fix.
