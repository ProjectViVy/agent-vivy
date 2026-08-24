package runtime

import (
	"context"
	"errors"

	"agent-vivy/internal/domain"
)

// ErrInvalidRunMode is returned before a run is persisted when a caller
// requests a mode outside Vivy's explicit policy vocabulary.
var ErrInvalidRunMode = errors.New("runtime: invalid run mode")

type runModeContextKey struct{}

func normalizeRunMode(mode domain.RunMode) (domain.RunMode, error) {
	if mode == "" {
		return domain.RunModeNormal, nil
	}
	if !mode.Valid() {
		return "", errors.Join(ErrInvalidRunMode, errors.New("mode must be normal or plan"))
	}
	return mode, nil
}

func normalizeRunPolicy(mode domain.RunMode, profile domain.PolicyProfile) (domain.RunMode, domain.PolicyProfile, error) {
	normalizedMode, err := normalizeRunMode(mode)
	if err != nil {
		return "", "", err
	}
	if profile == "" {
		profile = domain.PolicyProfileDefault
	}
	if !profile.Valid() {
		return "", "", ErrInvalidPolicyProfile
	}
	// Plan Mode is a physical safety boundary. A caller cannot combine it
	// with full_auto and regain effectful execution.
	if normalizedMode == domain.RunModePlan {
		profile = domain.PolicyProfilePlan
	}
	return normalizedMode, profile, nil
}

func recoveredProfile(mode domain.RunMode, profile string) domain.PolicyProfile {
	if profile != "" && domain.PolicyProfile(profile).Valid() {
		return domain.PolicyProfile(profile)
	}
	if mode == domain.RunModePlan {
		return domain.PolicyProfilePlan
	}
	return domain.PolicyProfileDefault
}

func withRunMode(ctx context.Context, mode domain.RunMode) context.Context {
	return context.WithValue(ctx, runModeContextKey{}, mode)
}

func runMode(ctx context.Context) domain.RunMode {
	mode, ok := ctx.Value(runModeContextKey{}).(domain.RunMode)
	if !ok || mode == "" {
		return domain.RunModeNormal
	}
	return mode
}
