package loopdetection

import (
	"errors"
	"testing"
)

func TestLoopWindowCountsAndEvicts(t *testing.T) {
	argsA := "{\"text\":\"a\"}"
	argsB := "{\"text\":\"b\"}"

	var w loopWindow
	if _, err := w.record("echo_info", argsA, "a", ""); err != nil {
		t.Fatalf("first record: %v", err)
	}
	// Window of 10 alternating signatures never exceeds 5 repeats of
	// either, and evicts older entries.
	for i := 0; i < 9; i++ {
		if i%2 == 0 {
			if _, err := w.record("echo_info", argsB, "b", ""); err != nil {
				t.Fatalf("record %d: %v", i+2, err)
			}
			continue
		}
		if _, err := w.record("echo_info", argsA, "a", ""); err != nil {
			t.Fatalf("record %d: %v", i+2, err)
		}
	}
	// The 11th record keeps five of each signature in the window; the
	// 12th makes six of the first: the guardrail must trip.
	if _, err := w.record("echo_info", argsA, "a", ""); err != nil {
		t.Fatalf("record 11: %v", err)
	}
	if _, err := w.record("echo_info", argsA, "a", ""); !errors.Is(err, errLoopDetected) {
		t.Fatalf("err = %v, want errLoopDetected", err)
	}
	// A different tool name with the same payload is a different call.
	var w2 loopWindow
	for i := 0; i < loopRepeatLimit; i++ {
		if _, err := w2.record("echo_info", argsA, "a", ""); err != nil {
			t.Fatalf("record %d: %v", i+1, err)
		}
	}
	if _, err := w2.record("other_tool", argsA, "a", ""); err != nil {
		t.Fatalf("distinct tool flagged: %v", err)
	}
	// Tool errors participate: identical failing calls loop too.
	var w3 loopWindow
	for i := 0; i < loopRepeatLimit; i++ {
		if _, err := w3.record("echo_info", argsA, "", "boom"); err != nil {
			t.Fatalf("record %d: %v", i+1, err)
		}
	}
	if _, err := w3.record("echo_info", argsA, "", "boom"); !errors.Is(err, errLoopDetected) {
		t.Fatalf("err = %v, want errLoopDetected", err)
	}
}
