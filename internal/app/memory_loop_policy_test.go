package app

import (
	"context"
	"strings"
	"testing"
	"time"

	genassembly "agent-vivy/internal/generated/assembly"
)

type memoryLoopCognitionStatus struct {
	Cognition struct {
		ActiveRunID string `json:"active_run_id"`
		Watermark   uint64 `json:"watermark"`
		Phase       string `json:"phase"`
		BlockReason string `json:"block_reason"`
	} `json:"cognition"`
}

func TestMemoryLoopNoChangeAndTriggerPolicy(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	t.Run("disabled", func(t *testing.T) {
		f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "nochange"})
		session := memoryLoopSession(t, f)
		run := memoryLoopTurn(t, f, session, memoryLoopRandomFact(t))
		if _, err := f.Wait(context.Background(), "canonical", run); err != nil {
			t.Fatal(err)
		}
		assertMemoryLoopTriggerReason(t, f, session, "disabled")
		assertMemoryLoopNoExtraRequests(t, f, 1, 6*time.Second)
	})
	t.Run("no-new-input", func(t *testing.T) {
		f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "nochange"})
		session := memoryLoopSession(t, f)
		memoryLoopEnable(t, f, session)
		assertMemoryLoopTriggerReason(t, f, session, "no_new_input")
		assertMemoryLoopNoExtraRequests(t, f, 0, 6*time.Second)
	})
	t.Run("no-change-and-no-self-loop", func(t *testing.T) {
		f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "nochange"})
		session := memoryLoopSession(t, f)
		memoryLoopEnable(t, f, session)
		run := memoryLoopTurn(t, f, session, memoryLoopRandomFact(t))
		source, err := f.Wait(context.Background(), "canonical", run)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		var status memoryLoopCognitionStatus
		for {
			if err := f.cognitiveValue(ctx, "diva.cognitive.status", session, nil, &status); err != nil {
				t.Fatal(err)
			}
			if status.Cognition.Watermark >= source.CaptureSeq && status.Cognition.Phase == "idle" {
				break
			}
			select {
			case <-ctx.Done():
				t.Fatalf("actual no-change never settled: %+v", status)
			case <-ticker.C:
			}
		}
		requests := f.ModelRequests()
		if len(requests) != 3 || !strings.Contains(string(requests[1]), "stage=reconcile") || !strings.Contains(string(requests[2]), "stage=reflect") {
			t.Fatalf("actual model stages: count=%d", len(requests))
		}
		assertMemoryLoopTriggerReason(t, f, session, "no_new_input")
		// The actual App uses a five-second automatic ticker. Observe more
		// than two full intervals, without replacing its clock or scheduler.
		assertMemoryLoopNoExtraRequests(t, f, len(requests), 11*time.Second)
		after, err := f.Wait(context.Background(), "canonical", run)
		if err != nil || after.CanonicalCount != 1 || after.CaptureSeq != source.CaptureSeq {
			t.Fatalf("no-change/self-output added a canonical source: %+v %v", after, err)
		}
		var settled memoryLoopCognitionStatus
		memoryLoopAction(t, f, "diva.cognitive.status", map[string]any{"session_id": session}, &settled)
		if settled != status {
			t.Fatalf("settled window changed without user input: before=%+v after=%+v", status, settled)
		}
	})
}

func assertMemoryLoopTriggerReason(t *testing.T, f *memoryLoopFixture, session, want string) {
	t.Helper()
	for i := 0; i < 3; i++ {
		var response struct {
			Eligibility struct {
				Run    bool   `json:"run"`
				Reason string `json:"reason"`
			} `json:"eligibility"`
		}
		memoryLoopAction(t, f, "diva.cognitive.trigger", map[string]any{"session_id": session}, &response)
		if response.Eligibility.Run || response.Eligibility.Reason != want {
			t.Fatalf("actual trigger: %+v, want %s", response, want)
		}
	}
}

func assertMemoryLoopNoExtraRequests(t *testing.T, f *memoryLoopFixture, count int, window time.Duration) {
	t.Helper()
	deadline := time.Now().Add(window)
	for time.Now().Before(deadline) {
		if got := len(f.ModelRequests()); got != count {
			t.Fatalf("unexpected inference: actual=%d expected=%d", got, count)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
