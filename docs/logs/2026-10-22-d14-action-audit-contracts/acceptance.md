# Focused acceptance observations

An admitted control action still produces a `pre` / `started` record before
the Provider effect and a matching completion outcome under one invocation
ID. Denied calls produce a decision record without running the Provider.
Records preserve the owner/action/caller, effect, outcome, safe error class,
bounded input/output sizes and duration.

The focused tests exercise success, authorization denial, invalid output,
provider failure and oversized input. JSON and slog projections contain none
of the private input/output canaries. Diagnostic sizes clamp to the existing
limit and negative sizes clamp to zero. Sink failure or panic at admission
prevents the effect; at completion it withholds a successful result and
returns the existing safe audit-unavailable error. Sink contexts retain a
deadline. No rollback or retry of an already-completed effect is claimed.

Review the five architecture documents to confirm they require actual
schema/content or existing semantic identity rather than recreating digest
metadata. The session-export proposal is explicitly separate from the shipped
run capture and durable watermark owners. Product-wide D14 acceptance remains
with the integrated candidate and its required external gates.
