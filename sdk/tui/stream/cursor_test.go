package stream

import "testing"

func TestCursorRequiresContiguousDurableEvents(t *testing.T) {
	var cursor Cursor
	if ok, gap := cursor.Accept("run_1", Notice{RunID: "run_1", Seq: 1}); !ok || gap || cursor.LastSeq != 1 {
		t.Fatalf("first event ok=%v gap=%v cursor=%+v", ok, gap, cursor)
	}
	if ok, gap := cursor.Accept("run_1", Notice{RunID: "run_1", Seq: 3}); ok || !gap || !cursor.Recovering || cursor.LastSeq != 1 {
		t.Fatalf("gap event ok=%v gap=%v cursor=%+v", ok, gap, cursor)
	}
	if ok, gap := cursor.Accept("run_1", Notice{RunID: "run_1", Seq: 2}); !ok || gap || cursor.LastSeq != 2 || cursor.Recovering {
		t.Fatalf("replay event ok=%v gap=%v cursor=%+v", ok, gap, cursor)
	}
	if ok, gap := cursor.Accept("run_1", Notice{RunID: "run_1", Seq: 3}); !ok || gap || cursor.LastSeq != 3 {
		t.Fatalf("replayed tail ok=%v gap=%v cursor=%+v", ok, gap, cursor)
	}
	if ok, gap := cursor.Accept("run_1", Notice{RunID: "run_1", Seq: 3}); ok || gap {
		t.Fatalf("duplicate accepted: ok=%v gap=%v", ok, gap)
	}
}

func TestCursorAdvancesUnknownNoticesAndFiltersOtherRuns(t *testing.T) {
	var cursor Cursor
	if ok, gap := cursor.Accept("run_1", Notice{RunID: "run_2", Seq: 1}); ok || gap || cursor.LastSeq != 0 {
		t.Fatalf("other run ok=%v gap=%v cursor=%+v", ok, gap, cursor)
	}
	if ok, gap := cursor.Accept("run_1", Notice{RunID: "run_1", Seq: 1}); !ok || gap {
		t.Fatalf("unknown event ok=%v gap=%v", ok, gap)
	}
	if cursor.LastSeq != 1 {
		t.Fatalf("unknown event did not advance cursor: %+v", cursor)
	}
}

func TestCursorRejectsLateDurableEventAfterTerminalFence(t *testing.T) {
	var cursor Cursor
	if ok, gap := cursor.Accept("", Notice{RunID: "run_done", Seq: 1, Kind: "delta", Delta: "late"}); ok || gap || cursor.LastSeq != 0 {
		t.Fatalf("late event ok=%v gap=%v cursor=%+v", ok, gap, cursor)
	}
}
