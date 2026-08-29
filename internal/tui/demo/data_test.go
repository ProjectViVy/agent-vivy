package demo

import (
	"testing"

	"agent-vivy/internal/domain"
)

func TestNewStoreScript(t *testing.T) {
	s := NewStore()
	if len(s.Sessions) != 3 {
		t.Fatalf("sessions = %d", len(s.Sessions))
	}
	if s.ActiveID != "sess_approval" {
		t.Fatalf("active = %s", s.ActiveID)
	}
	gate := s.PendingGate()
	if gate == nil || gate.ID != "appr_demo_1" {
		t.Fatalf("gate = %+v", gate)
	}
	if len(s.Messages["sess_empty"]) != 0 {
		t.Fatal("empty session should have no messages")
	}
	if len(s.Messages["sess_overnight"]) < 4 {
		t.Fatalf("overnight messages = %d", len(s.Messages["sess_overnight"]))
	}
}

func TestDecideApprovalAndAppend(t *testing.T) {
	s := NewStore()
	if !s.DecideApproval(domain.ApprovalApproved) {
		t.Fatal("expected approval to apply")
	}
	if s.PendingGate() != nil {
		t.Fatal("gate should clear")
	}
	msgs := s.Messages["sess_approval"]
	last := msgs[len(msgs)-2]
	if last.Tool == nil || last.Tool.Status != "done" {
		t.Fatalf("tool = %+v", last.Tool)
	}

	s.SelectSession("sess_empty")
	s.AppendUser("hello")
	got := s.ActiveMessages()
	if len(got) != 2 || got[0].Content != "hello" || got[1].Content != "（demo：未接控制面）" {
		t.Fatalf("messages = %+v", got)
	}
}

func TestMoveSessionWraps(t *testing.T) {
	s := NewStore()
	s.ActiveID = s.Sessions[0].ID
	s.MoveSession(-1)
	if s.ActiveID != s.Sessions[len(s.Sessions)-1].ID {
		t.Fatalf("wrap up = %s", s.ActiveID)
	}
	s.MoveSession(1)
	if s.ActiveID != s.Sessions[0].ID {
		t.Fatalf("wrap down = %s", s.ActiveID)
	}
}
