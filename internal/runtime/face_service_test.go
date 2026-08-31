package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"agent-vivy/internal/domain"
)

// The face lands on the durable run.started payload so a restart can
// recover it; an invalid face is rejected before anything is persisted.
// Both runs suspend on their scripted approval, so the assertions replay
// the journal rather than waiting for a terminal status.
func TestServiceRunStartedCarriesFace(t *testing.T) {
	svc, backend, _ := newApprovalService(t, 5*time.Minute)
	ctx := context.Background()

	runID, err := svc.RunWithOptions(ctx, "sess-face", "note that I need milk", RunOptions{Face: domain.FaceCode})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForApprovalEvent(t, backend, runID)

	events := replayAll(t, backend, runID)
	var started payloadRunStarted
	mustUnmarshal(t, events[0].Payload, &started)
	if started.Face != string(domain.FaceCode) {
		t.Fatalf("run.started face = %q, want code", started.Face)
	}
	if started.Mode != string(domain.RunModeNormal) {
		t.Fatalf("run.started mode = %q, want normal", started.Mode)
	}

	// The web face keeps the payload compact: the field is omitempty. The
	// scripted model replays once per harness, so the web run gets its own.
	webSvc, webBackend, _ := newApprovalService(t, 5*time.Minute)
	webRun, err := webSvc.RunWithOptions(ctx, "sess-face", "note that I need milk", RunOptions{})
	if err != nil {
		t.Fatalf("web run: %v", err)
	}
	waitForApprovalEvent(t, webBackend, webRun)
	webEvents := replayAll(t, webBackend, webRun)
	var webStarted payloadRunStarted
	mustUnmarshal(t, webEvents[0].Payload, &webStarted)
	if webStarted.Face != string(domain.FaceWeb) {
		t.Fatalf("run.started face = %q, want web", webStarted.Face)
	}
}

func TestServiceRunWithOptionsRejectsInvalidFace(t *testing.T) {
	svc, _, _ := newApprovalService(t, 5*time.Minute)
	_, err := svc.RunWithOptions(context.Background(), "sess-face", "hello", RunOptions{Face: domain.Face("shell")})
	if !errors.Is(err, ErrInvalidFace) {
		t.Fatalf("error = %v, want ErrInvalidFace", err)
	}
}
