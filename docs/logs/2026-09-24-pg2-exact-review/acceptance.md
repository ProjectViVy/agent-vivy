# Acceptance

With `approval_policy: never` and a permissive ordinary tool profile, invoking `create_goal` must first surface the existing human approval interrupt. Before a human decision there is no durable Goal creation/admission and no next Goal round. Rejecting leaves no Goal; approving creates the Goal and starts its bounded round.

For model-originated work, retrying an identical request with the same Eino tool-call ID replays the original durable result. An identical payload with a different call ID is distinct work. Reusing an ID with changed arguments is rejected by the durable WorkStore as a conflict.

This iteration does not establish live PostgreSQL acceptance or release readiness; those remain external acceptance gates.
