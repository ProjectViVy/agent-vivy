package stream

import (
	"strings"
	"testing"
)

func TestInboxIsLosslessAndPrependsGap(t *testing.T) {
	var inbox Inbox
	for i := 0; i < 257; i++ {
		inbox.Push(Notice{Delta: "界"})
	}
	pending, _ := inbox.TakeWithOverflow()
	if len(pending) != 257 || inbox.Len() != 0 {
		t.Fatalf("take len=%d queue=%d", len(pending), inbox.Len())
	}
	inbox.Prepend(pending[200:])
	if inbox.Len() != 57 {
		t.Fatalf("prepend len=%d", inbox.Len())
	}
	if got, _ := inbox.TakeWithOverflow(); len(got) != 57 || strings.Repeat("界", len(got)) != strings.Repeat("界", 57) {
		t.Fatalf("retained notices = %d", len(got))
	}
}

func TestOrderPlacesLegacyEventsAfterDurableEvents(t *testing.T) {
	notices := []Notice{{Seq: 0, Delta: "legacy"}, {Seq: 3}, {Seq: 1}, {Seq: 2}}
	Order(notices)
	if notices[0].Seq != 1 || notices[1].Seq != 2 || notices[2].Seq != 3 || notices[3].Seq != 0 {
		t.Fatalf("ordered notices = %+v", notices)
	}
}

func TestInboxCloseRejectsRacingLatePush(t *testing.T) {
	var inbox Inbox
	inbox.Push(Notice{Seq: 1})
	inbox.Close()
	if inbox.Push(Notice{Seq: 2}) != PushClosed || inbox.Len() != 0 {
		t.Fatalf("closed inbox accepted data: len=%d", inbox.Len())
	}
	if pending, _ := inbox.TakeWithOverflow(); len(pending) != 0 {
		t.Fatalf("closed inbox returned data: %d notices", len(pending))
	}
	inbox.Prepend([]Notice{{Seq: 3}})
	if inbox.Len() != 0 {
		t.Fatalf("closed inbox accepted prepend: len=%d", inbox.Len())
	}
}

func TestInboxBoundsMemoryAndReportsReplayRequired(t *testing.T) {
	inbox := NewInbox(3)
	for seq := 1; seq <= 3; seq++ {
		if inbox.Push(Notice{Seq: seq}) != PushAccepted {
			t.Fatalf("seq %d rejected before capacity", seq)
		}
	}
	if inbox.Push(Notice{Seq: 4}) != PushReplayRequired {
		t.Fatal("overflowing notice was retained")
	}
	pending, overflow := inbox.TakeWithOverflow()
	if !overflow || len(pending) != 0 {
		t.Fatalf("pending=%+v overflow=%v", pending, overflow)
	}
	if pending, overflow = inbox.TakeWithOverflow(); overflow || len(pending) != 0 {
		t.Fatalf("overflow fence did not reset: pending=%+v overflow=%v", pending, overflow)
	}
}

func TestInboxPrependKeepsEarliestBoundedReplayPrefix(t *testing.T) {
	inbox := NewInbox(3)
	inbox.Push(Notice{Seq: 5})
	inbox.Prepend([]Notice{{Seq: 2}, {Seq: 3}, {Seq: 4}})
	pending, overflow := inbox.TakeWithOverflow()
	if !overflow || len(pending) != 0 {
		t.Fatalf("pending=%+v overflow=%v", pending, overflow)
	}
}

func TestInboxBoundsUTF8BytesAndDrainWork(t *testing.T) {
	inbox := NewBoundedInbox(10, 8)
	if inbox.Push(Notice{Delta: "四字"}) != PushAccepted { // six UTF-8 bytes
		t.Fatal("first notice rejected")
	}
	if inbox.Push(Notice{Delta: "界"}) != PushReplayRequired {
		t.Fatal("byte overflow did not request replay")
	}
	if pending, replay := inbox.TakeWithOverflow(); !replay || len(pending) != 0 {
		t.Fatalf("pending=%+v replay=%v", pending, replay)
	}

	inbox = NewBoundedInbox(10, 1024)
	for seq := 1; seq <= 5; seq++ {
		inbox.Push(Notice{Seq: seq, Delta: "x"})
	}
	batch, replay := inbox.TakeBatch(2, 1024)
	if replay || len(batch) != 2 || batch[1].Seq != 2 || inbox.Len() != 3 {
		t.Fatalf("batch=%+v replay=%v remaining=%d", batch, replay, inbox.Len())
	}
}

func TestInboxCountsAuthoritativeCompletedContent(t *testing.T) {
	inbox := NewBoundedInbox(10, 20)
	if inbox.Push(Notice{Kind: "model_completed", HasCompleted: true, Completed: "四字", CompletedAuthoritative: true}) != PushReplayRequired {
		t.Fatal("completed content bypassed byte bound")
	}
}

func TestInboxCountsCompleteApprovalGate(t *testing.T) {
	inbox := NewBoundedInbox(10, 64)
	notice := Notice{Kind: "gate", Gate: &GatePrompt{
		Kind: "approval", ID: "a", Action: strings.Repeat("a", 20), Target: strings.Repeat("t", 20),
		PreconditionHash: strings.Repeat("f", 64), Preview: strings.Repeat("p", 20), Risks: []string{strings.Repeat("r", 20)},
	}}
	if inbox.Push(notice) != PushReplayRequired {
		t.Fatal("approval metadata bypassed inbox byte bound")
	}
}
