# Acceptance

Hold the real Service's approval visibility append after its SQL pending row is
committed. The helper must remain blocked during that window, then return only
the approval whose ID, call ID and tool name match the committed Journal event
on the active Run. It keeps the original five-second deadline and does not wait
for runtime idleness before allowing a decision.

All affected governance fixtures decide once after readiness. Middleware-drift
and approval-binding checks remain fail-closed, and the secret-redaction test
still rejects any event containing its secret canary without printing that
payload in failure output. This is verification setup correction, not a claim
that the separately reviewed runtime registration window is fixed.
