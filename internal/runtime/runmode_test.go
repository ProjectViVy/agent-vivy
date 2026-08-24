package runtime

import (
	"context"
	"errors"
	"testing"

	"agent-vivy/internal/domain"
)

func TestNormalizeRunModeDefaultsToNormal(t *testing.T) {
	got, err := normalizeRunMode("")
	if err != nil || got != domain.RunModeNormal {
		t.Fatalf("normalize empty = %q, %v; want normal", got, err)
	}
}

func TestNormalizeRunModeRejectsUnknownMode(t *testing.T) {
	_, err := normalizeRunMode("execute")
	if !errors.Is(err, ErrInvalidRunMode) {
		t.Fatalf("error = %v, want ErrInvalidRunMode", err)
	}
}

func TestRunModeContextDefaultsAndOverrides(t *testing.T) {
	if got := runMode(context.Background()); got != domain.RunModeNormal {
		t.Fatalf("default run mode = %q, want normal", got)
	}
	if got := runMode(withRunMode(context.Background(), domain.RunModePlan)); got != domain.RunModePlan {
		t.Fatalf("overridden run mode = %q, want plan", got)
	}
}
