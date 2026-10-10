package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
)

func TestMemoryLoopActivityFailurePreservesSourceAndArchiveRetry(t *testing.T) {
	probe := genassembly.BuildDefault()
	if !probe.HasCognitiveFactory() {
		t.Skip("DIVA integration overlay required")
	}
	f := newMemoryLoopFixture(t, memoryLoopOptions{ConfigPath: memoryLoopConfig(t), ModelMode: "busy"})
	session := memoryLoopSession(t, f)
	fact := memoryLoopRandomFact(t)
	run := memoryLoopTurn(t, f, session, "[hold-foreground] "+fact)
	deadline := time.Now().Add(5 * time.Second)
	for len(f.ModelRequests()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("actual primary response not held")
		}
		time.Sleep(time.Millisecond)
	}
	// A directory at the owned head path produces an actual filesystem read
	// failure even when the test user can bypass chmod permissions.
	head := filepath.Join(f.dataRoot, "garden", "persona", "actmem", "ACTMEM.MD")
	original, readErr := os.ReadFile(head)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		t.Fatal(readErr)
	}
	if readErr == nil {
		if err := os.Remove(head); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(head, 0700); err != nil {
		t.Fatal(err)
	}
	restored := false
	restore := func() {
		if restored {
			return
		}
		if err := os.Remove(head); err != nil {
			t.Fatal(err)
		}
		if readErr == nil {
			if err := os.WriteFile(head, original, 0600); err != nil {
				t.Fatal(err)
			}
		}
		restored = true
	}
	t.Cleanup(restore)
	f.modelReleaseOnce.Do(func() { close(f.modelRelease) })
	terminal, err := f.Wait(context.Background(), "terminal", run)
	if err != nil || terminal.State != "completed" {
		t.Fatalf("activity failure interrupted actual primary: %+v %v", terminal, err)
	}
	source, err := f.Wait(context.Background(), "canonical", run)
	if err != nil || source.CanonicalCount != 1 || !strings.Contains(source.SourceBody, fact) {
		t.Fatalf("original source not retained during native fault: %+v %v", source, err)
	}
	high, err := f.app.cognitive.Source().HighWatermark(context.Background())
	if err != nil || high != 0 {
		t.Fatalf("unprojected source incorrectly released: high=%d %v", high, err)
	}
	err = f.cognitiveValue(context.Background(), "diva.cognitive.actmem.read", session, map[string]any{"sections": []string{"recap"}, "max_chars": 1200}, nil)
	if err == nil {
		t.Fatal("public ACTMEM read reported success during actual native fault")
	}
	// Use the real Service with an explicit server-side deadline. A client
	// RPC timeout alone cannot prove that the archive operation was cancelled.
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	err = f.app.service.DeleteSession(ctx, domain.SessionID(session))
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("actual archive did not wait for failed native projection: %v", err)
	}
	params, _ := json.Marshal(map[string]any{"session_id": session})
	// The failed delete leaves producer admission sealed, so public history
	// reconciliation refuses this session. Inspect the real durable Store;
	// that refusal must not be mistaken for deletion of its rows.
	if _, err := f.app.backend.GetSession(context.Background(), domain.SessionID(session)); err != nil {
		t.Fatal(err)
	}
	messages, err := f.app.backend.ListMessages(context.Background(), domain.SessionID(session))
	if err != nil {
		t.Fatal(err)
	}
	retained := false
	for _, message := range messages {
		if message.Role == domain.RoleUser && strings.Contains(message.Content, fact) {
			retained = true
		}
	}
	if !retained {
		t.Fatal("failed archive deleted original durable user history")
	}
	saveMemoryLoopDevelopmentEvidence(t, f, session, "native-fault", source)
	restore()
	awaitMemoryLoopActivitySource(t, f, source.CaptureSeq)
	after, err := f.Wait(context.Background(), "canonical", run)
	if err != nil || after.IngestionID != source.IngestionID || after.RecordID != source.RecordID || after.Revision != source.Revision || after.CanonicalCount != 1 || len(f.ModelRequests()) != 1 {
		t.Fatalf("native retry changed source identity or reran the primary: %+v %v", after, err)
	}
	saveMemoryLoopDevelopmentEvidence(t, f, session, "native-restored", source, after)
	if _, err := f.Call(context.Background(), "session/delete", params); err != nil {
		t.Fatalf("actual public archive retry failed: %v", err)
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(head), "capsules", "fold-*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("original activity was not archived after recovery: %v %v", files, err)
	}
	var archive strings.Builder
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		archive.Write(raw)
	}
	if !strings.Contains(archive.String(), fact) || !strings.Contains(archive.String(), session) {
		t.Fatal("recovered capsule lost original fact/session")
	}
}
