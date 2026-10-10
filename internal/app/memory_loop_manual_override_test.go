package app

import (
	"context"
	"testing"
	"time"

	genassembly "agent-vivy/internal/generated/assembly"
)

func TestMemoryLoopManualTriggerOverridesBusyAndIntervalGates(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	ctx := context.Background()
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "busy"})
	session := memoryLoopSession(t, f)
	memoryLoopEnable(t, f, session)
	firstRun := memoryLoopTurn(t, f, session, memoryLoopRandomFact(t))
	first, err := f.Wait(ctx, "canonical", firstRun)
	if err != nil {
		t.Fatal(err)
	}
	awaitMemoryLoopWatermark(t, f, session, first.CaptureSeq, 15*time.Second)
	var policy struct {
		Revision uint64 `json:"policy_revision"`
	}
	memoryLoopAction(t, f, "diva.cognitive.policy.get", map[string]any{"session_id": session}, &policy)
	memoryLoopAction(t, f, "diva.cognitive.policy.set", map[string]any{"session_id": session, "enabled": true, "min_interval_ms": 120000, "base_revision": policy.Revision}, nil)
	secondRun := memoryLoopTurn(t, f, session, memoryLoopRandomFact(t))
	second, err := f.Wait(ctx, "canonical", secondRun)
	if err != nil {
		t.Fatal(err)
	}
	awaitMemoryLoopActivitySource(t, f, second.CaptureSeq)
	assertMemoryLoopNoExtraRequests(t, f, 4, 6*time.Second)
	var deferred memoryLoopCognitionStatus
	memoryLoopAction(t, f, "diva.cognitive.status", map[string]any{"session_id": session}, &deferred)
	if deferred.Cognition.Watermark != first.CaptureSeq || deferred.Cognition.ActiveRunID != "" || deferred.Cognition.Phase != "idle" {
		t.Fatalf("interval gate consumed the actual pending window: %+v", deferred)
	}
	thirdRun := memoryLoopTurn(t, f, session, "[hold-foreground] "+memoryLoopRandomFact(t))
	deadline := time.Now().Add(5 * time.Second)
	for len(f.ModelRequests()) != 5 {
		if time.Now().After(deadline) {
			t.Fatalf("third owned primary request did not reach its response gate: %d requests", len(f.ModelRequests()))
		}
		time.Sleep(time.Millisecond)
	}
	var manual struct {
		ActiveRunID string `json:"ActiveRunID"`
		Eligibility struct {
			Run    bool   `json:"run"`
			Reason string `json:"reason"`
		} `json:"eligibility"`
	}
	memoryLoopAction(t, f, "diva.cognitive.trigger", map[string]any{"session_id": session}, &manual)
	if !manual.Eligibility.Run || manual.Eligibility.Reason != "eligible" || manual.ActiveRunID == "" {
		t.Fatalf("explicit manual wake did not bypass real busy/interval gates: %+v", manual)
	}
	watermarked := awaitMemoryLoopWatermark(t, f, session, second.CaptureSeq, 25*time.Second)
	if watermarked.Cognition.ActiveRunID != "" || len(f.ModelRequests()) != 7 {
		t.Fatalf("manual wake duplicated or failed to settle the pending window: %+v requests=%d", watermarked, len(f.ModelRequests()))
	}
	requests := f.ModelRequests()
	reconcile := memoryLoopInferenceWindow(t, requests[5], "reconcile")
	reflect := memoryLoopInferenceWindow(t, requests[6], "reflect")
	if reconcile != reflect || reconcile.After != first.CaptureSeq || reconcile.Through != second.CaptureSeq {
		t.Fatalf("manual override changed the original deferred window: reconcile=%+v reflect=%+v", reconcile, reflect)
	}
	assertMemoryLoopNoExtraRequests(t, f, 7, 6*time.Second)
	f.modelReleaseOnce.Do(func() { close(f.modelRelease) })
	third, err := f.Wait(ctx, "canonical", thirdRun)
	if err != nil || third.CanonicalCount != 3 {
		t.Fatalf("held original primary did not finish once: %+v %v", third, err)
	}
	awaitMemoryLoopActivitySource(t, f, third.CaptureSeq)
	assertMemoryLoopNoExtraRequests(t, f, 7, 6*time.Second)
	var after memoryLoopCognitionStatus
	memoryLoopAction(t, f, "diva.cognitive.status", map[string]any{"session_id": session}, &after)
	if after.Cognition.Watermark != second.CaptureSeq || after.Cognition.ActiveRunID != "" {
		t.Fatalf("automatic interval gate consumed a later new source: %+v", after)
	}
	saveMemoryLoopDevelopmentEvidence(t, f, session, "manual-override", first, second, third)
}
