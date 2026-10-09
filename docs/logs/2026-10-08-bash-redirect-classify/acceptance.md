# Acceptance — bash redirect classification

Status: **implementation evidence complete; owner acceptance pending**.

Verified in this session: classifier tiers for quoted/heredoc/fd redirects,
fail-closed behavior for dynamic and out-of-workspace targets, and real
native-bash writes landing in the run workspace with correct bytes
(synthetic `CommandBackend` workspace, `SandboxModeWorkspaceWrite`).

Bounded: acceptance per the handoff did not require a real model API round
trip; no live-provider run was exercised. The tiered-approval path that
consumes `ClassifyInvocation` (auto policy skipping approval for safe
invocations) is covered by the classifier and backend tests, not by an
end-to-end LLM session.

Owner-controlled gates not performed or claimed: merge, release, and
verification on Windows hosts (the embedded `powershell.exe` path in the
justfile is host-specific; this session verified on Linux).
