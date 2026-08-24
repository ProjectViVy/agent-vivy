package runtime

import (
	"context"

	"agent-vivy/internal/domain"
)

type runIDContextKey struct{}
type sessionIDContextKey struct{}
type policyProfileContextKey struct{}
type policySnapshotContextKey struct{}

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
