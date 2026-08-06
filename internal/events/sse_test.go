package events

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
)

func openJournal(t *testing.T) storage.Journal {
	t.Helper()
	b, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

// appendRun stores the events as separate commits so the journal assigns
// seq 1..len, exactly like the runtime service does.
func appendRun(t *testing.T, j storage.Journal, runID domain.RunID, types ...domain.EventType) {
	t.Helper()
	for _, typ := range types {
		ev := domain.RunEvent{Type: typ, CreatedAt: 1, PayloadVersion: 1, Payload: []byte(`{"ok":true}`)}
		if _, err := j.Append(context.Background(), storage.Commit{RunID: runID, Events: []domain.RunEvent{ev}}); err != nil {
			t.Fatalf("append %s: %v", typ, err)
		}
	}
}

type frame struct {
	ID    string
	Event string
	Data  map[string]any
}

// parseFrames decodes the SSE text into ordered frames.
func parseFrames(t *testing.T, body string) []frame {
	t.Helper()
	var out []frame
	for _, block := range strings.Split(strings.TrimRight(body, "\n"), "\n\n") {
		if block == "" || strings.HasPrefix(block, ":") {
			continue
		}
		var f frame
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "id: "):
				f.ID = strings.TrimPrefix(line, "id: ")
			case strings.HasPrefix(line, "event: "):
				f.Event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &f.Data); err != nil {
					t.Fatalf("frame data not JSON: %v", err)
				}
			}
		}
		out = append(out, f)
	}
	return out
}

func assertEnvelope(t *testing.T, f frame, wantSeq int, wantType string) {
	t.Helper()
	if f.ID != strconv.Itoa(wantSeq) {
		t.Fatalf("frame id = %q, want %d", f.ID, wantSeq)
	}
	if f.Event != wantType {
		t.Fatalf("frame event = %q, want %s", f.Event, wantType)
	}
	if f.Data["run_id"] != "run-t" || f.Data["type"] != wantType {
		t.Fatalf("envelope identity wrong: %+v", f.Data)
	}
	if seq, _ := f.Data["seq"].(float64); int(seq) != wantSeq {
		t.Fatalf("envelope seq = %v, want %d", f.Data["seq"], wantSeq)
	}
	if v, _ := f.Data["payload_version"].(float64); v != 1 {
		t.Fatalf("payload_version = %v, want 1", f.Data["payload_version"])
	}
	if p, ok := f.Data["payload"].(map[string]any); !ok || p["ok"] != true {
		t.Fatalf("payload not passed through raw: %+v", f.Data["payload"])
	}
}

func TestSSEReplayPath(t *testing.T) {
	j := openJournal(t)
	appendRun(t, j, "run-t",
		domain.EventRunStarted, domain.EventModelDelta, domain.EventRunCompleted)
	bus := NewBus(8)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rec := httptest.NewRecorder()
	if err := ServeSSE(ctx, rec, j, bus, "run-t", 0); err != nil {
		t.Fatalf("ServeSSE: %v", err)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type = %q", ct)
	}
	frames := parseFrames(t, rec.Body.String())
	if len(frames) != 3 {
		t.Fatalf("frames = %d, want 3", len(frames))
	}
	assertEnvelope(t, frames[0], 1, "run.started")
	assertEnvelope(t, frames[1], 2, "model.delta")
	assertEnvelope(t, frames[2], 3, "run.completed")
}

func TestSSEAfterSeqReconnect(t *testing.T) {
	j := openJournal(t)
	appendRun(t, j, "run-t",
		domain.EventRunStarted, domain.EventModelDelta, domain.EventModelCompleted, domain.EventRunCompleted)
	bus := NewBus(8)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rec := httptest.NewRecorder()
	// Client reconnects having seen seq 1 and 2.
	if err := ServeSSE(ctx, rec, j, bus, "run-t", 2); err != nil {
		t.Fatalf("ServeSSE: %v", err)
	}
	frames := parseFrames(t, rec.Body.String())
	if len(frames) != 2 {
		t.Fatalf("frames = %d, want 2 (no duplicates, no gaps)", len(frames))
	}
	assertEnvelope(t, frames[0], 3, "model.completed")
	assertEnvelope(t, frames[1], 4, "run.completed")
}

func TestSSELiveThenTerminal(t *testing.T) {
	j := openJournal(t)
	// History already durable: started + one delta.
	appendRun(t, j, "run-t", domain.EventRunStarted, domain.EventModelDelta)
	bus := NewBus(8)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rec := httptest.NewRecorder()

	errCh := make(chan error, 1)
	go func() { errCh <- ServeSSE(ctx, rec, j, bus, "run-t", 0) }()

	// Wait until the stream holds a live subscription.
	deadline := time.Now().Add(3 * time.Second)
	for bus.Subscribers("run-t") == 0 {
		if time.Now().After(deadline) {
			t.Fatal("ServeSSE never subscribed")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Durability precedes visibility, exactly like the runtime service:
	// append to the journal first, then publish.
	appendRun(t, j, "run-t", domain.EventModelDelta)
	bus.Publish(domain.RunEvent{RunID: "run-t", Seq: 3, Type: domain.EventModelDelta, CreatedAt: 1, PayloadVersion: 1, Payload: []byte(`{"ok":true}`)})
	appendRun(t, j, "run-t", domain.EventRunCompleted)
	bus.Publish(domain.RunEvent{RunID: "run-t", Seq: 4, Type: domain.EventRunCompleted, CreatedAt: 1, PayloadVersion: 1, Payload: []byte(`{"ok":true}`)})

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("ServeSSE: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ServeSSE did not finish after terminal")
	}

	frames := parseFrames(t, rec.Body.String())
	if len(frames) != 4 {
		t.Fatalf("frames = %d, want 4 with no duplicates across replay/live", len(frames))
	}
	for i, want := range []string{"run.started", "model.delta", "model.delta", "run.completed"} {
		assertEnvelope(t, frames[i], i+1, want)
	}
}
