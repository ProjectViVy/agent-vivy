package app

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	genassembly "agent-vivy/internal/generated/assembly"
	"github.com/dashimaki/garden/agentapi"
	laputaevolution "github.com/dashimaki/laputa/evolution"
	"github.com/dashimaki/laputa/persona"
)

func TestMemoryLoopPersonaReviewAndFrozenSessions(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	for _, decision := range []string{"reject", "accept"} {
		t.Run(decision, func(t *testing.T) {
			f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "persona"})
			session := memoryLoopSession(t, f)
			var original agentapi.PersonaDocument
			memoryLoopAction(t, f, "diva.cognitive.persona.read", map[string]any{"session_id": session, "kind": "identity"}, &original)
			memoryLoopEnable(t, f, session)
			fact := memoryLoopRandomFact(t)
			run := memoryLoopTurn(t, f, session, fact)
			source, err := f.Wait(context.Background(), "canonical", run)
			if err != nil {
				t.Fatal(err)
			}
			var frozen laputaevolution.FrozenCoreV2
			memoryLoopAction(t, f, "diva.cognitive.frozen.read", map[string]any{"session_id": session}, &frozen)
			request := waitMemoryLoopPersonaRequest(t, f, session, source.CaptureSeq)
			if request.Kind != persona.KindIdentity || request.State != persona.RequestPending || request.BaseRevision != original.Revision || !strings.Contains(request.ProposedMarkdown, fact) {
				t.Fatalf("actual proposal/source mismatch: %+v", request)
			}
			var unchanged agentapi.PersonaDocument
			memoryLoopAction(t, f, "diva.cognitive.persona.read", map[string]any{"session_id": session, "kind": "identity"}, &unchanged)
			if unchanged.Content != original.Content || unchanged.Revision != original.Revision {
				t.Fatal("submitted review applied authority before human decision")
			}
			var policy struct {
				Revision uint64 `json:"policy_revision"`
			}
			memoryLoopAction(t, f, "diva.cognitive.policy.get", map[string]any{"session_id": session}, &policy)
			memoryLoopAction(t, f, "diva.cognitive.policy.set", map[string]any{"session_id": session, "enabled": false, "min_interval_ms": 0, "base_revision": policy.Revision}, nil)
			memoryLoopAction(t, f, "diva.cognitive.persona.review.decide", map[string]any{"session_id": session, "review_id": request.ID, "decision": decision}, nil)
			var current agentapi.PersonaDocument
			memoryLoopAction(t, f, "diva.cognitive.persona.read", map[string]any{"session_id": session, "kind": "identity"}, &current)
			if decision == "accept" {
				if current.Content != request.ProposedMarkdown || current.Revision <= original.Revision {
					t.Fatal("accepted review did not change native authority")
				}
			} else if current.Content != original.Content || current.Revision != original.Revision {
				t.Fatal("rejected review changed native authority")
			}
			if err := f.Restart(context.Background()); err != nil {
				t.Fatal(err)
			}
			params, _ := json.Marshal(map[string]any{"session_id": session})
			if _, err := f.Call(context.Background(), "session/get", params); err != nil {
				t.Fatal(err)
			}
			var reopened laputaevolution.FrozenCoreV2
			memoryLoopAction(t, f, "diva.cognitive.frozen.read", map[string]any{"session_id": session}, &reopened)
			if !reflect.DeepEqual(frozen, reopened) {
				t.Fatal("process restart changed old session Frozen Core")
			}
			oldRun := memoryLoopTurn(t, f, session, "continue old frozen session")
			if _, err := f.Wait(context.Background(), "canonical", oldRun); err != nil {
				t.Fatal(err)
			}
			fresh := memoryLoopSession(t, f)
			newRun := memoryLoopTurn(t, f, fresh, "start fresh frozen session")
			if _, err := f.Wait(context.Background(), "canonical", newRun); err != nil {
				t.Fatal(err)
			}
			var newCore laputaevolution.FrozenCoreV2
			memoryLoopAction(t, f, "diva.cognitive.frozen.read", map[string]any{"session_id": fresh}, &newCore)
			oldIdentity, oldOK := reopened.Section(laputaevolution.FrozenKindIdentity)
			newIdentity, newOK := newCore.Section(laputaevolution.FrozenKindIdentity)
			if !oldOK || !newOK || oldIdentity.SourceRevision != original.Revision || newIdentity.SourceRevision != current.Revision || newIdentity.Content != current.Content {
				t.Fatalf("session authority revisions: old=%+v new=%+v", oldIdentity, newIdentity)
			}
			requests := f.ModelRequests()
			if len(requests) != 2 || !memoryLoopSystemContains(requests[0], oldIdentity.Content) || !memoryLoopSystemContains(requests[1], newIdentity.Content) {
				t.Fatal("actual new process provider system input did not use each session's Frozen Core")
			}
			if decision == "accept" && memoryLoopSystemContains(requests[0], request.ProposedMarkdown) {
				t.Fatal("approved identity leaked into old session system input")
			}
		})
	}
}

func waitMemoryLoopPersonaRequest(t *testing.T, f *memoryLoopFixture, session string, through uint64) persona.ChangeRequest {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		var page agentapi.ReviewPage
		if err := f.cognitiveValue(ctx, "diva.cognitive.persona.reviews.list", session, map[string]any{"kind": "identity", "state": "pending", "limit": 100}, &page); err != nil {
			t.Fatal(err)
		}
		var status memoryLoopCognitionStatus
		if err := f.cognitiveValue(ctx, "diva.cognitive.status", session, nil, &status); err != nil {
			t.Fatal(err)
		}
		if len(page.Items) == 1 && status.Cognition.Watermark >= through && status.Cognition.Phase == "idle" {
			return page.Items[0]
		}
		select {
		case <-ctx.Done():
			t.Fatalf("actual persona review never settled: %+v %+v", page, status)
		case <-ticker.C:
		}
	}
}

func memoryLoopSystemContains(raw []byte, text string) bool {
	var req struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if json.Unmarshal(raw, &req) != nil {
		return false
	}
	for _, m := range req.Messages {
		if m.Role == "system" && strings.Contains(m.Content, text) {
			return true
		}
	}
	return false
}
