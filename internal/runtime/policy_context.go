package runtime

import (
	"context"

	"agent-vivy/internal/domain"
)

type runIDContextKey struct{}
type sessionIDContextKey struct{}
type workspaceIDContextKey struct{}
type policyProfileContextKey struct{}
type policySnapshotContextKey struct{}
type sandboxModeContextKey struct{}
type approvalPolicyContextKey struct{}
type approvedToolArgumentsHashContextKey struct{}

// directShellContextKey marks the runtime-owned direct shell lifecycle. It
// is deliberately process-local context metadata: the marker tells the
// command backend that this request must remain foreground-only. Journal and
// message projections stay safe because their payloads are already redacted.
type directShellContextKey struct{}

func withRunID(ctx context.Context, runID domain.RunID) context.Context {
	return context.WithValue(ctx, runIDContextKey{}, runID)
}

func contextRunID(ctx context.Context) domain.RunID {
	runID, _ := ctx.Value(runIDContextKey{}).(domain.RunID)
	return runID
}

func withSessionID(ctx context.Context, sessionID domain.SessionID) context.Context {
	return context.WithValue(ctx, sessionIDContextKey{}, sessionID)
}

func contextSessionID(ctx context.Context) domain.SessionID {
	sessionID, _ := ctx.Value(sessionIDContextKey{}).(domain.SessionID)
	return sessionID
}

// withWorkspaceID carries the allocator-issued workspace identity through
// the live Engine context. It is intentionally distinct from RunID: a
// workspace may be shared by a code-face run or have a stable allocator ID
// that is not the durable run identifier.
func withWorkspaceID(ctx context.Context, workspaceID string) context.Context {
	return context.WithValue(ctx, workspaceIDContextKey{}, workspaceID)
}

func contextWorkspaceID(ctx context.Context) string {
	workspaceID, _ := ctx.Value(workspaceIDContextKey{}).(string)
	return workspaceID
}

func withPolicyProfile(ctx context.Context, profile domain.PolicyProfile) context.Context {
	return context.WithValue(ctx, policyProfileContextKey{}, profile)
}

func policyProfile(ctx context.Context) domain.PolicyProfile {
	profile, ok := ctx.Value(policyProfileContextKey{}).(domain.PolicyProfile)
	if !ok || !profile.Valid() {
		if runMode(ctx) == domain.RunModePlan {
			return domain.PolicyProfilePlan
		}
		return domain.PolicyProfileDefault
	}
	return profile
}

func withPolicySnapshot(ctx context.Context, snapshot domain.PolicySnapshot) context.Context {
	return context.WithValue(ctx, policySnapshotContextKey{}, snapshot)
}

func policySnapshot(ctx context.Context) domain.PolicySnapshot {
	snapshot, _ := ctx.Value(policySnapshotContextKey{}).(domain.PolicySnapshot)
	return snapshot
}

func withSandboxMode(ctx context.Context, mode domain.SandboxMode) context.Context {
	return context.WithValue(ctx, sandboxModeContextKey{}, mode)
}

func sandboxMode(ctx context.Context) domain.SandboxMode {
	mode, ok := ctx.Value(sandboxModeContextKey{}).(domain.SandboxMode)
	if ok && mode.Valid() {
		return mode
	}
	return ""
}

func withApprovalPolicy(ctx context.Context, policy domain.ApprovalPolicy) context.Context {
	return context.WithValue(ctx, approvalPolicyContextKey{}, policy)
}

func approvalPolicy(ctx context.Context) domain.ApprovalPolicy {
	policy, _ := ctx.Value(approvalPolicyContextKey{}).(domain.ApprovalPolicy)
	if policy.Valid() {
		return policy
	}
	return domain.ApprovalPolicyAsk
}

func withApprovedToolArgumentsHash(ctx context.Context, hash string) context.Context {
	return context.WithValue(ctx, approvedToolArgumentsHashContextKey{}, hash)
}

func approvedToolArgumentsHash(ctx context.Context) string {
	hash, _ := ctx.Value(approvedToolArgumentsHashContextKey{}).(string)
	return hash
}

func withSessionSandbox(ctx context.Context, mode domain.SandboxMode, policy domain.ApprovalPolicy) context.Context {
	return withApprovalPolicy(withSandboxMode(ctx, mode), policy)
}

func withDirectShell(ctx context.Context) context.Context {
	return context.WithValue(ctx, directShellContextKey{}, true)
}

func isDirectShell(ctx context.Context) bool {
	marked, _ := ctx.Value(directShellContextKey{}).(bool)
	return marked
}
