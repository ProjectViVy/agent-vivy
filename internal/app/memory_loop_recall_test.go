package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	genassembly "agent-vivy/internal/generated/assembly"
)

// First S08 observation: no seeded answer, public-memory UI read, BML or
// persona substitute. The fact must arrive through the ordinary ContextHost.
func TestMemoryLoopRecallAfterProcessRestart(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	for i := 0; i < 3; i++ {
		t.Run(fmt.Sprintf("profile-%d", i+1), func(t *testing.T) {
			f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "recall"})
			a := memoryLoopSession(t, f)
			memoryLoopEnable(t, f, a)
			fact := memoryLoopRandomFact(t)
			runA := memoryLoopTurn(t, f, a, fact)
			written, err := f.Wait(context.Background(), "reflected", runA)
			if err != nil {
				t.Fatal(err)
			}
			var policy struct {
				Revision uint64 `json:"policy_revision"`
			}
			memoryLoopAction(t, f, "diva.cognitive.policy.get", map[string]any{"session_id": a}, &policy)
			memoryLoopAction(t, f, "diva.cognitive.policy.set", map[string]any{"session_id": a, "enabled": false, "min_interval_ms": 0, "base_revision": policy.Revision}, nil)
			if err := f.Restart(context.Background()); err != nil {
				t.Fatal(err)
			}
			b := memoryLoopSession(t, f)
			if b == a {
				t.Fatal("new session reused source history")
			}
			params, _ := json.Marshal(map[string]any{"session_id": b})
			history, err := f.Call(context.Background(), "session/messages", params)
			if err != nil || strings.Contains(string(history), fact) {
				t.Fatalf("new session history contains source fact: %s %v", history, err)
			}
			runB := memoryLoopTurn(t, f, b, "What synthetic user-only fact do you remember from previous conversations?")
			if terminal, err := f.Wait(context.Background(), "terminal", runB); err != nil || terminal.State != "completed" {
				t.Fatalf("actual recall turn: %+v %v", terminal, err)
			}
			requests := f.ModelRequests()
			if len(requests) != 1 {
				t.Fatalf("recall request count=%d", len(requests))
			}
			marker := fmt.Sprintf("[context: vivy.memory.mentle/%s version=%d provenance=", written.RecordID, written.Revision)
			if !strings.Contains(string(requests[0]), fact) || !strings.Contains(string(requests[0]), marker) {
				t.Fatalf("MEM-S08-01: committed ordinary record %s revision%d absent from fresh-process model request; queries=%+v; actual request=%s", written.RecordID, written.Revision, f.RecallQueries(), requests[0])
			}
			queries := f.RecallQueries()
			if len(queries) != 1 || queries[0].Error != "" || queries[0].Disabled || queries[0].Request.SessionID != b {
				t.Fatalf("actual native recall trace: %+v", queries)
			}
			var request struct {
				Messages []memoryLoopWireMessage `json:"messages"`
			}
			if err := json.Unmarshal(requests[0], &request); err != nil {
				t.Fatal(err)
			}
			matched := false
			for _, candidate := range queries[0].Page.Candidates {
				if candidate.ContentID != written.RecordID {
					continue
				}
				if !strings.Contains(candidate.Content, fact) {
					t.Fatal("actual native evidence lost source fact")
				}
				for _, message := range request.Messages {
					if message.Role == "system" && strings.Contains(message.Content, fact) {
						t.Fatal("ordinary memory became system authority")
					}
					if message.Role == "user" && strings.Contains(message.Content, candidate.Content) {
						matched = true
					}
				}
			}
			if !matched {
				t.Fatal("actual native evidence was not projected as untrusted user data")
			}
			assertMemoryLoopAssistantAnswer(t, f, b, fact)
		})
	}
}

func TestMemoryLoopRecallNegativeControls(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	for _, disabled := range []bool{false, true} {
		for i := 0; i < 3; i++ {
			name := "empty-profile"
			if disabled {
				name = "recall-disabled"
			}
			t.Run(fmt.Sprintf("%s-%d", name, i+1), func(t *testing.T) {
				f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "recall", RecallDisabled: disabled})
				fact := memoryLoopRandomFact(t)
				var original memoryLoopSnapshot
				var sourceRun string
				var sourceSession string
				if disabled {
					a := memoryLoopSession(t, f)
					sourceSession = a
					memoryLoopEnable(t, f, a)
					run := memoryLoopTurn(t, f, a, fact)
					sourceRun = run
					var err error
					original, err = f.Wait(context.Background(), "reflected", run)
					if err != nil {
						t.Fatal(err)
					}
					var policy struct {
						Revision uint64 `json:"policy_revision"`
					}
					memoryLoopAction(t, f, "diva.cognitive.policy.get", map[string]any{"session_id": a}, &policy)
					memoryLoopAction(t, f, "diva.cognitive.policy.set", map[string]any{"session_id": a, "enabled": false, "min_interval_ms": 0, "base_revision": policy.Revision}, nil)
				}
				if err := f.Restart(context.Background()); err != nil {
					t.Fatal(err)
				}
				if disabled {
					params, _ := json.Marshal(map[string]any{"session_id": sourceSession})
					if _, err := f.Call(context.Background(), "session/get", params); err != nil {
						t.Fatal(err)
					}
					persisted, err := f.Wait(context.Background(), "reflected", sourceRun)
					if err != nil || persisted.RecordID != original.RecordID || persisted.Revision != original.Revision || persisted.CanonicalBody != original.CanonicalBody || !strings.Contains(persisted.CanonicalBody, fact) {
						t.Fatalf("disabled control lost actual ordinary commit: %+v %v", persisted, err)
					}
				}
				b := memoryLoopSession(t, f)
				run := memoryLoopTurn(t, f, b, "What synthetic user-only fact do you remember from previous conversations?")
				terminal, err := f.Wait(context.Background(), "terminal", run)
				if err != nil || terminal.State != "completed" {
					t.Fatalf("control run: %+v %v", terminal, err)
				}
				requests := f.ModelRequests()
				if len(requests) != 1 || strings.Contains(string(requests[0]), fact) || strings.Contains(string(requests[0]), "[context: vivy.memory.mentle/") {
					t.Fatalf("control model request gained memory: %s", requests)
				}
				queries := f.RecallQueries()
				if len(queries) != 1 || queries[0].Error != "" || len(queries[0].Page.Candidates) != 0 || queries[0].Disabled != disabled {
					t.Fatalf("control reachability differed: %+v", queries)
				}
				assertMemoryLoopAssistantAnswer(t, f, b, "未知")
			})
		}
	}
}

func assertMemoryLoopAssistantAnswer(t *testing.T, f *memoryLoopFixture, session, expected string) {
	t.Helper()
	params, _ := json.Marshal(map[string]any{"session_id": session})
	raw, err := f.Call(context.Background(), "session/messages", params)
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatal(err)
	}
	for _, message := range page.Messages {
		if message.Role == "assistant" && message.Content == expected {
			return
		}
	}
	t.Fatalf("actual assistant response missing %q: %s", expected, raw)
}

// Instrumented cold native inference may exceed the unchanged ContextHost
// deadline. Late results must not become model authority or outlive Close.
func TestMemoryLoopRecallDeadlineDegradesSafely(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "recall"})
	a := memoryLoopSession(t, f)
	memoryLoopEnable(t, f, a)
	fact := memoryLoopRandomFact(t)
	run := memoryLoopTurn(t, f, a, fact)
	written, err := f.Wait(context.Background(), "reflected", run)
	if err != nil {
		t.Fatal(err)
	}
	var policy struct {
		Revision uint64 `json:"policy_revision"`
	}
	memoryLoopAction(t, f, "diva.cognitive.policy.get", map[string]any{"session_id": a}, &policy)
	memoryLoopAction(t, f, "diva.cognitive.policy.set", map[string]any{"session_id": a, "enabled": false, "min_interval_ms": 0, "base_revision": policy.Revision}, nil)
	if err := f.Restart(context.Background()); err != nil {
		t.Fatal(err)
	}
	b := memoryLoopSession(t, f)
	turn := memoryLoopTurn(t, f, b, "What synthetic user-only fact do you remember from previous conversations?")
	terminal, err := f.Wait(context.Background(), "terminal", turn)
	if err != nil || terminal.State != "completed" {
		t.Fatalf("degraded run: %+v %v", terminal, err)
	}
	requests := f.ModelRequests()
	if len(requests) != 1 {
		t.Fatalf("primary requests=%d", len(requests))
	}
	marker := fmt.Sprintf("[context: vivy.memory.mentle/%s version=%d provenance=", written.RecordID, written.Revision)
	admitted := strings.Contains(string(requests[0]), marker)
	if admitted {
		if !strings.Contains(string(requests[0]), fact) {
			t.Fatal("admitted evidence lost actual fact")
		}
		assertMemoryLoopAssistantAnswer(t, f, b, fact)
	} else {
		if strings.Contains(string(requests[0]), fact) {
			t.Fatal("unadmitted source fact reached model")
		}
		assertMemoryLoopAssistantAnswer(t, f, b, "未知")
	}
	deadline := time.Now().Add(15 * time.Second)
	for len(f.RecallQueries()) == 0 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	queries := f.RecallQueries()
	if len(queries) != 1 || queries[0].Disabled {
		t.Fatalf("native Source not actually called: %+v", queries)
	}
	if !admitted {
		t.Logf("actual bounded Source degraded; late native query error=%q candidates=%d", queries[0].Error, len(queries[0].Page.Candidates))
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := f.Close(closeCtx); err != nil {
		t.Fatalf("owned Close after native query: %v", err)
	}
}
