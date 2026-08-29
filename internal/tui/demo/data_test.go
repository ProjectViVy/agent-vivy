package demo

import (
	"testing"

	"agent-vivy/internal/domain"
)

func TestNewStoreScript(t *testing.T) {
	s := NewStore()
	if len(s.Sessions()) != 3 {
		t.Fatalf("sessions = %d", len(s.Sessions()))
	}
	if s.Active().ID != "sess_approval" {
		t.Fatalf("active = %s", s.Active().ID)
	}
	gate := s.PendingGate()
	if gate == nil || gate.ID != "appr_demo_1" {
		t.Fatalf("gate = %+v", gate)
	}
	if len(s.ActiveMessages()) == 0 {
		// approval session has messages; empty is another id
	}
	s.SelectSession("sess_empty")
	if len(s.ActiveMessages()) != 0 {
		t.Fatal("empty session should have no messages")
	}
	s.SelectSession("sess_overnight")
	if len(s.ActiveMessages()) < 4 {
		t.Fatalf("overnight messages = %d", len(s.ActiveMessages()))
	}
}

func TestDecideApprovalAndAppend(t *testing.T) {
	s := NewStore()
	s.DecideApproval(domain.ApprovalApproved)
	if s.PendingGate() != nil {
		t.Fatal("gate should clear")
	}
	msgs := s.ActiveMessages()
	var toolDone bool
	for _, m := range msgs {
		if m.Tool != nil && m.Tool.Status == "done" {
			toolDone = true
		}
	}
	if !toolDone {
		t.Fatalf("tool not done: %+v", msgs)
	}

	s.SelectSession("sess_empty")
	s.Send("hello")
	got := s.ActiveMessages()
	if len(got) != 2 || got[0].Content != "hello" || got[1].Content != "（demo：未接控制面）" {
		t.Fatalf("messages = %+v", got)
	}
}

func TestMoveSessionWraps(t *testing.T) {
	s := NewStore()
	first := s.Sessions()[0].ID
	last := s.Sessions()[len(s.Sessions())-1].ID
	s.SelectSession(first)
	s.MoveSession(-1)
	if s.Active().ID != last {
		t.Fatalf("wrap up = %s", s.Active().ID)
	}
	s.MoveSession(1)
	if s.Active().ID != first {
		t.Fatalf("wrap down = %s", s.Active().ID)
	}
}
