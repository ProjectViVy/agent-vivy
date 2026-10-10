package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	genassembly "agent-vivy/internal/generated/assembly"
)

func TestMemoryLoopCaptureTerminalMatrix(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	for _, tc := range []struct{ mode, terminal string }{
		{"ack", "completed"}, {"failed", "failed"}, {"wait-cancel", "cancelled"},
	} {
		t.Run(tc.terminal, func(t *testing.T) {
			f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: tc.mode})
			session := memoryLoopSession(t, f)
			fact := memoryLoopRandomFact(t)
			run := memoryLoopTurn(t, f, session, fact)
			if tc.mode == "wait-cancel" {
				deadline := time.Now().Add(10 * time.Second)
				for len(f.ModelRequests()) == 0 {
					if time.Now().After(deadline) {
						t.Fatal("actual provider request was not admitted")
					}
					time.Sleep(10 * time.Millisecond)
				}
				params, _ := json.Marshal(map[string]any{"run_id": run})
				if _, err := f.Call(context.Background(), "run/cancel", params); err != nil {
					t.Fatal(err)
				}
			}
			terminal, err := f.Wait(context.Background(), "terminal", run)
			if err != nil || terminal.State != tc.terminal || terminal.EventSeq == 0 {
				t.Fatalf("actual terminal: %+v %v", terminal, err)
			}
			assertMemoryLoopCapturedFact(t, f, run, fact, terminal.EventSeq)
		})
	}
}

func TestMemoryLoopCapturesLongUserSource(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "ack"})
	fact := memoryLoopRandomFact(t) + strings.Repeat("长", 4000)
	if len(fact) <= 8192 {
		t.Fatal("fixture does not cross 8192 UTF-8 bytes")
	}
	run := memoryLoopTurn(t, f, memoryLoopSession(t, f), fact)
	terminal, err := f.Wait(context.Background(), "terminal", run)
	if err != nil || terminal.State != "completed" {
		t.Fatalf("long source terminal: %+v %v", terminal, err)
	}
	assertMemoryLoopCapturedFact(t, f, run, fact, terminal.EventSeq)
}

func TestMemoryLoopLiteralProfilePath(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	name := "profile#fragment%25"
	if runtime.GOOS != "windows" {
		name += "?literal-query"
	}
	root := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfigAtRoot(t, root), ModelMode: "reflection"})
	session := memoryLoopSession(t, f)
	memoryLoopEnable(t, f, session)
	fact := memoryLoopRandomFact(t)
	run := memoryLoopTurn(t, f, session, fact)
	snap, err := f.Wait(context.Background(), "reflected", run)
	if err != nil || snap.CanonicalBody != fact || len(snap.CanonicalSources) != 1 {
		t.Fatalf("literal profile reflection: %+v %v", snap, err)
	}
	for _, rel := range []string{"vivy-test.db", "garden/garden.db", "garden/palace/palace.db/canonical.sqlite3"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("literal database identity %s: %v", rel, err)
		}
	}
}

func memoryLoopSession(t *testing.T, f *memoryLoopFixture) string {
	t.Helper()
	raw, err := f.Call(context.Background(), "session/create", json.RawMessage(`{"title":"source matrix"}`))
	var session struct {
		ID string `json:"id"`
	}
	if err != nil || json.Unmarshal(raw, &session) != nil || session.ID == "" {
		t.Fatalf("actual session: %s %v", raw, err)
	}
	return session.ID
}

func memoryLoopRandomFact(t *testing.T) string {
	t.Helper()
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	return "synthetic user-only fact " + hex.EncodeToString(nonce[:])
}

func assertMemoryLoopCapturedFact(t *testing.T, f *memoryLoopFixture, run, fact string, seq uint64) {
	t.Helper()
	snap, err := f.Wait(context.Background(), "canonical", run)
	if err != nil || snap.EventSeq != seq || snap.CaptureSeq == 0 || snap.RecordID == "" || snap.CanonicalCount != 1 {
		t.Fatalf("actual capture: %+v %v", snap, err)
	}
	var source struct {
		RunID    string `json:"run_id"`
		Messages []struct {
			Role     string `json:"role"`
			Content  string `json:"content"`
			Complete bool   `json:"complete"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(snap.CanonicalBody), &source); err != nil || source.RunID != run {
		t.Fatalf("canonical envelope: %v", err)
	}
	users := 0
	for _, message := range source.Messages {
		if message.Role == "user" {
			users++
			if message.Content != fact || !message.Complete {
				t.Fatal("admitted user source was truncated or mislabeled")
			}
		} else if strings.Contains(message.Content, fact) {
			t.Fatal("user-only fact appeared in assistant projection")
		}
	}
	if users != 1 {
		t.Fatalf("actual user sources=%d", users)
	}
	t.Logf("canonical run=%s terminal_event=%d capture=%d record=%s bytes=%d", run, seq, snap.CaptureSeq, snap.RecordID, len(fact))
}
