package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	genassembly "agent-vivy/internal/generated/assembly"
)

func TestMemoryLoopRejectedEffectRemainsVisibleAndFenced(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "rejected"})
	sessionID := createCognitiveSession(t, f.peer)
	memoryLoopEnable(t, f, sessionID)
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	fact := "rejected-memory-fact=" + hex.EncodeToString(nonce)
	runID := memoryLoopTurn(t, f, sessionID, fact)
	canonical, err := f.Wait(context.Background(), "canonical", runID)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	var receiptID string
	for receiptID == "" {
		var page struct {
			Items []struct {
				ID     string `json:"operation_id"`
				Kind   string `json:"kind"`
				Status string `json:"status"`
			} `json:"items"`
		}
		if err := f.cognitiveValue(ctx, "diva.cognitive.results.list", sessionID, map[string]any{"limit": 100}, &page); err != nil {
			t.Fatal(err)
		}
		for _, item := range page.Items {
			if item.Kind == "memory_mutation" && item.Status == "rejected" {
				receiptID = item.ID
			}
		}
		if receiptID == "" {
			select {
			case <-ctx.Done():
				t.Fatalf("real backend rejection receipt missing: %+v", page)
			case <-ticker.C:
			}
		}
	}
	var trigger struct {
		Eligibility struct {
			Reason string `json:"reason"`
		} `json:"eligibility"`
	}
	for !strings.HasPrefix(trigger.Eligibility.Reason, "blocked:") {
		memoryLoopAction(t, f, "diva.cognitive.trigger", map[string]any{"session_id": sessionID}, &trigger)
		select {
		case <-ctx.Done():
			t.Fatalf("unresolved run was not fenced: %+v", trigger)
		case <-ticker.C:
		}
	}
	var status struct {
		Cognition struct {
			ActiveRunID    string `json:"active_run_id"`
			Watermark      uint64 `json:"watermark"`
			PendingThrough uint64 `json:"pending_through"`
			Phase          string `json:"phase"`
			BlockReason    string `json:"block_reason"`
		} `json:"cognition"`
	}
	memoryLoopAction(t, f, "diva.cognitive.status", map[string]any{"session_id": sessionID}, &status)
	if status.Cognition.ActiveRunID == "" || status.Cognition.Watermark != 0 || status.Cognition.PendingThrough < canonical.CaptureSeq || status.Cognition.Phase != "blocked" || status.Cognition.BlockReason == "" {
		t.Fatalf("public status hid the durable unresolved window: %+v", status.Cognition)
	}
	before := status.Cognition
	for i := 0; i < 3; i++ {
		memoryLoopAction(t, f, "diva.cognitive.trigger", map[string]any{"session_id": sessionID}, nil)
	}
	memoryLoopAction(t, f, "diva.cognitive.status", map[string]any{"session_id": sessionID}, &status)
	if status.Cognition != before {
		t.Fatalf("manual wake changed unresolved identity: before=%+v after=%+v", before, status.Cognition)
	}
	after, err := f.Wait(context.Background(), "canonical", runID)
	if err != nil || after.CanonicalCount != 1 || !strings.Contains(after.CanonicalBody, fact) {
		t.Fatalf("rejected update changed canonical source authority: %+v %v", after, err)
	}
}
