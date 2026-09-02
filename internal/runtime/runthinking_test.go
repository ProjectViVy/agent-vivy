package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/testsupport"
)

type thinkingCaptureModel struct {
	inner domain.ChatModel

	modes chan domain.ThinkingMode
}

func (m *thinkingCaptureModel) Stream(ctx context.Context, input []*domain.Message) (domain.Stream[*domain.Message], error) {
	select {
	case m.modes <- domain.ThinkingModeFromContext(ctx):
	default:
	}
	return m.inner.Stream(ctx, input)
}

func newThinkingCaptureService(t *testing.T) (*Service, *thinkingCaptureModel, *sqlite.Backend) {
	t.Helper()
	capture := &thinkingCaptureModel{inner: testsupport.NewEchoModel(), modes: make(chan domain.ThinkingMode, 16)}
	svc, backend, _ := newTestService(t, capture)
	return svc, capture, backend
}

func TestNormalizeThinkingMode(t *testing.T) {
	cases := []struct {
		name  string
		input domain.ThinkingMode
		want  domain.ThinkingMode
	}{
		{"empty becomes auto", "", domain.ThinkingModeAuto},
		{"auto", domain.ThinkingModeAuto, domain.ThinkingModeAuto},
		{"on", domain.ThinkingModeOn, domain.ThinkingModeOn},
		{"off", domain.ThinkingModeOff, domain.ThinkingModeOff},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := normalizeThinkingMode(testCase.input)
			if err != nil || got != testCase.want {
				t.Fatalf("normalizeThinkingMode(%q) = %q, %v; want %q, nil", testCase.input, got, err, testCase.want)
			}
		})
	}
	if _, err := normalizeThinkingMode(domain.ThinkingMode("execute")); !errors.Is(err, ErrInvalidThinkingMode) {
		t.Fatalf("err = %v, want ErrInvalidThinkingMode", err)
	}
}

// TestRunWithOptionsRejectsInvalidThinkingMode pins the boundary: an
// unknown thinking preference fails before anything is persisted.
func TestRunWithOptionsRejectsInvalidThinkingMode(t *testing.T) {
	svc, _, _ := newThinkingCaptureService(t)
	_, err := svc.RunWithOptions(context.Background(), "sess-think", "hi", RunOptions{Thinking: domain.ThinkingMode("execute")})
	if !errors.Is(err, ErrInvalidThinkingMode) {
		t.Fatalf("err = %v, want ErrInvalidThinkingMode", err)
	}
}

// TestRunWithOptionsCarriesThinkingModeToTheModel drives real runs and
// asserts the run-scoped context value reaches the model seam: an explicit
// "on" rides the run context, and the default run reads back "auto".
func TestRunWithOptionsCarriesThinkingModeToTheModel(t *testing.T) {
	svc, capture, backend := newThinkingCaptureService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	assertNextMode := func(t *testing.T, want domain.ThinkingMode) {
		t.Helper()
		select {
		case got := <-capture.modes:
			if got != want {
				t.Fatalf("model saw thinking mode %q, want %q", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("model was never invoked")
		}
	}

	runID, err := svc.RunWithOptions(ctx, "sess-think-on", "hi", RunOptions{Thinking: domain.ThinkingModeOn})
	if err != nil {
		t.Fatalf("run with thinking: %v", err)
	}
	assertNextMode(t, domain.ThinkingModeOn)
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	runID, err = svc.RunWithOptions(ctx, "sess-think-auto", "hi", RunOptions{})
	if err != nil {
		t.Fatalf("default run: %v", err)
	}
	assertNextMode(t, domain.ThinkingModeAuto)
	waitForRunStatus(t, backend, runID, domain.RunCompleted)
}
