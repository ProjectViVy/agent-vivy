package journalview

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"agent-vivy/internal/domain"
)

func ev(runID domain.RunID, seq domain.EventSeq, t domain.EventType, version int, payload string) domain.RunEvent {
	return domain.RunEvent{RunID: runID, Seq: seq, Type: t, CreatedAt: int64(seq), PayloadVersion: version, Payload: []byte(payload)}
}

func completedV2(text string) string {
	sum := sha256.Sum256([]byte(text))
	raw, _ := json.Marshal(map[string]any{
		"byte_len":       len([]byte(text)),
		"content_sha256": hex.EncodeToString(sum[:]),
	})
	return string(raw)
}

// TestTaskTextProjectionEquivalence pins the §7.1 rules: delta accumulation
// with a pre-tool flush and a checksum-validated completed segment, in the
// exact native msgp_ identity format.
func TestTaskTextProjectionEquivalence(t *testing.T) {
	runID := domain.RunID("run_equiv")
	r := NewTextReducer(runID)

	// model.request -> delta("before tool") -> tool.requested:
	// one completed segment at the tool boundary, seq of that event.
	for _, e := range []domain.RunEvent{
		ev(runID, 1, domain.EventRunStarted, 1, `{"no_context":false}`),
		ev(runID, 2, domain.EventModelRequest, 1, `{}`),
		ev(runID, 3, domain.EventModelDelta, 1, `{"delta":"before tool "}`),
		ev(runID, 4, domain.EventModelDelta, 1, `{"delta":"reply"}`),
	} {
		segs, err := r.Apply(e)
		if err != nil || len(segs) != 0 {
			t.Fatalf("seq %d: segs=%+v err=%v", e.Seq, segs, err)
		}
	}
	segs, err := r.Apply(ev(runID, 5, domain.EventToolRequested, 1, `{"tool_call_id":"c1","tool_name":"t","args":{}}`))
	if err != nil || len(segs) != 1 {
		t.Fatalf("tool.requested: segs=%+v err=%v", segs, err)
	}
	wantID := fmt.Sprintf("msgp_%s_%020d_%s", runID, 5, "0")
	if segs[0].ID != wantID || segs[0].Text != "before tool reply" {
		t.Fatalf("segment = %+v, want id %q", segs[0], wantID)
	}

	// Later completed v2: checksum/byte_len validated; the hash is never
	// decoded as text.
	finished := "done ✅"
	for _, e := range []domain.RunEvent{
		ev(runID, 6, domain.EventModelRequest, 1, `{}`),
		ev(runID, 7, domain.EventModelDelta, 1, `{"delta":"done ✅"}`),
	} {
		if _, err := r.Apply(e); err != nil {
			t.Fatalf("seq %d: %v", e.Seq, err)
		}
	}
	segs, err = r.Apply(ev(runID, 8, domain.EventModelCompleted, 2, completedV2(finished)))
	if err != nil || len(segs) != 1 || segs[0].Text != finished {
		t.Fatalf("completed: segs=%+v err=%v", segs, err)
	}
	if segs[0].ID != fmt.Sprintf("msgp_%s_%020d_%s", runID, 8, "0") {
		t.Fatalf("completed id = %q", segs[0].ID)
	}

	// Corrupt digest: explicit error, no segment.
	r2 := NewTextReducer(runID)
	if _, err := r2.Apply(ev(runID, 1, domain.EventRunStarted, 1, `{}`)); err != nil {
		t.Fatalf("started: %v", err)
	}
	if _, err := r2.Apply(ev(runID, 2, domain.EventModelDelta, 1, `{"delta":"x"}`)); err != nil {
		t.Fatalf("delta: %v", err)
	}
	ysum := sha256.Sum256([]byte("y"))
	bad := `{"byte_len":1,"content_sha256":"` + hex.EncodeToString(ysum[:]) + `"}`
	if _, err := r2.Apply(ev(runID, 3, domain.EventModelCompleted, 2, bad)); err == nil {
		t.Fatal("corrupt digest accepted")
	}

	// v1 completed: explicit unsupported error, not silent text.
	if _, err := r2.Apply(ev(runID, 3, domain.EventModelCompleted, 1, `{"content":"hi"}`)); err == nil {
		t.Fatal("v1 completed accepted")
	}

	// Missing completion (run ends mid-segment): nothing emitted — an
	// incomplete segment is never advertised.
	r3 := NewTextReducer(runID)
	if _, err := r3.Apply(ev(runID, 1, domain.EventRunStarted, 1, `{}`)); err != nil {
		t.Fatalf("started: %v", err)
	}
	if _, err := r3.Apply(ev(runID, 2, domain.EventModelDelta, 1, `{"delta":"half"}`)); err != nil {
		t.Fatalf("delta: %v", err)
	}
	segs, err = r3.Apply(ev(runID, 3, domain.EventRunCompleted, 2, `{"outcome":"completed"}`))
	if err != nil || len(segs) != 0 {
		t.Fatalf("dangling segment emitted: %+v err=%v", segs, err)
	}
}
