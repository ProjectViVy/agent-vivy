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
