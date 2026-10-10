package app

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	genassembly "agent-vivy/internal/generated/assembly"
	"github.com/ProjectViVy/laputa/garden/agentapi"
)

func TestMemoryLoopModelCannotEscalatePersonaAuthority(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	for _, claim := range []string{"mission", "dream", "actor"} {
		t.Run(claim, func(t *testing.T) {
			f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "persona-restricted"})
			session := memoryLoopSession(t, f)
			before := readMemoryLoopPersonaAuthorities(t, f, session)
			memoryLoopEnable(t, f, session)
			primary := memoryLoopTurn(t, f, session, "[forbid="+claim+"] "+memoryLoopRandomFact(t))
			source, err := f.Wait(context.Background(), "canonical", primary)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			var state memoryLoopCognitionStatus
			for {
				if err := f.cognitiveValue(ctx, "diva.cognitive.status", session, nil, &state); err != nil {
					t.Fatal(err)
				}
				if state.Cognition.Phase == "blocked" {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatalf("restricted model output not rejected: %+v", state)
				case <-time.After(20 * time.Millisecond):
				}
			}
			if state.Cognition.Watermark != 0 || state.Cognition.ActiveRunID == "" || state.Cognition.BlockReason != "unknown_outcome" {
				t.Fatalf("invalid persona output was settled/retried: %+v", state)
			}
			if !reflect.DeepEqual(before, readMemoryLoopPersonaAuthorities(t, f, session)) {
				t.Fatal("restricted model changed persona authority")
			}
			var reviews agentapi.ReviewPage
			memoryLoopAction(t, f, "diva.cognitive.persona.reviews.list", map[string]any{"session_id": session, "limit": 100}, &reviews)
			if len(reviews.Items) != 0 {
				t.Fatalf("forbidden kind/forged actor created review: %+v", reviews)
			}
			assertMemoryLoopNoExtraRequests(t, f, 3, 11*time.Second)
			after, err := f.Wait(context.Background(), "canonical", primary)
			if err != nil || after.CanonicalCount != 1 || after.RecordID != source.RecordID {
				t.Fatalf("invalid proposal changed ordinary canonical memory: %+v %v", after, err)
			}
			saveMemoryLoopDevelopmentEvidence(t, f, session, "denied", source, after)
		})
	}
}

func readMemoryLoopPersonaAuthorities(t *testing.T, f *memoryLoopFixture, session string) map[string]agentapi.PersonaDocument {
	t.Helper()
	docs := map[string]agentapi.PersonaDocument{}
	for _, kind := range []string{"identity", "mission", "dream"} {
		var doc agentapi.PersonaDocument
		memoryLoopAction(t, f, "diva.cognitive.persona.read", map[string]any{"session_id": session, "kind": kind}, &doc)
		docs[kind] = doc
	}
	return docs
}

func TestMemoryLoopReviewRejectsCallerActorClaim(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "persona"})
	session := memoryLoopSession(t, f)
	before := readMemoryLoopPersonaAuthorities(t, f, session)
	memoryLoopEnable(t, f, session)
	primary := memoryLoopTurn(t, f, session, memoryLoopRandomFact(t))
	source, err := f.Wait(context.Background(), "canonical", primary)
	if err != nil {
		t.Fatal(err)
	}
	request := waitMemoryLoopPersonaRequest(t, f, session, source.CaptureSeq)
	pendingAuthority := readMemoryLoopPersonaAuthorities(t, f, session)
	for _, actor := range []string{"user", "agent"} {
		args, _ := json.Marshal(map[string]any{"module_id": "vivy/diva-cognitive", "action_id": "diva.cognitive.persona.review.decide", "input": map[string]any{"session_id": session, "review_id": request.ID, "decision": "accept", "actor": actor}})
		raw, err := f.Call(context.Background(), "module.action.invoke", args)
		if err == nil {
			var result struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal(raw, &result); err != nil {
				t.Fatal(err)
			}
			if result.Status == "ok" {
				t.Fatalf("caller actor claim accepted: %s", raw)
			}
		}
		current := readMemoryLoopPersonaAuthorities(t, f, session)
		if !reflect.DeepEqual(pendingAuthority, current) {
			t.Fatalf("caller actor claim changed pending authority: actor=%s reply=%s error=%v before=%+v after=%+v", actor, raw, err, pendingAuthority, current)
		}
		stillPending := waitMemoryLoopPersonaRequest(t, f, session, source.CaptureSeq)
		if stillPending.ID != request.ID {
			t.Fatal("caller claim replaced original pending review")
		}
	}
	// The ordinary trusted human decision remains usable after denial.
	memoryLoopAction(t, f, "diva.cognitive.persona.review.decide", map[string]any{"session_id": session, "review_id": request.ID, "decision": "reject"}, nil)
	var page agentapi.ReviewPage
	memoryLoopAction(t, f, "diva.cognitive.persona.reviews.list", map[string]any{"session_id": session, "state": "pending", "limit": 100}, &page)
	if len(page.Items) != 0 || !reflect.DeepEqual(before, readMemoryLoopPersonaAuthorities(t, f, session)) {
		t.Fatal("ordinary trusted rejection failed after denied claims")
	}
	saveMemoryLoopDevelopmentEvidence(t, f, session, "rejected", source)
}
