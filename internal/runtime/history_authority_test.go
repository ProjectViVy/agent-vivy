package runtime

import (
	"testing"

	"agent-vivy/internal/domain"
)

func TestHistoryAttachmentDoesNotGrantScope(t *testing.T) {
	got, err := ResolveHistoryScope("B", nil, []domain.SessionID{"A", "B"})
	if err != nil || len(got) != 1 || got[0] != "B" {
		t.Fatalf("default leaked: %v %v", got, err)
	}
	_, err = ResolveHistoryScope("B", []domain.SessionID{"A"}, []domain.SessionID{"B"})
	if err == nil {
		t.Fatal("explicit selection bypassed deny")
	}
}

func TestContinuityResolveHistoryScopeCanonicalizesAndBounds(t *testing.T) {
	got, err := ResolveHistoryScope("B", []domain.SessionID{"C", "A"}, []domain.SessionID{"C", "B", "A"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(got) != 3 || got[0] != "A" || got[1] != "B" || got[2] != "C" {
		t.Fatalf("non-canonical scope: %v", got)
	}
	limits := domain.DefaultContinuityLimits()
	selected := make([]domain.SessionID, limits.ExplicitSessions+1)
	for i := range selected {
		selected[i] = domain.SessionID(string(rune('a' + i)))
	}
	if _, err := ResolveHistoryScope("B", selected, append(selected, "B")); err == nil {
		t.Fatal("accepted too many explicit sessions")
	}
}

func TestContinuityRunStartedPayloadCompatibility(t *testing.T) {
	legacy := payloadRunStarted{Provider: "provider", Model: "model", Mode: "normal"}
	if legacy.HistoryScope != nil {
		t.Fatal("legacy payload unexpectedly has history scope")
	}
	withScope := payloadRunStarted{HistoryScope: &domain.AcceptedHistoryScope{DestinationSessionID: "session", SourceSessionIDs: []domain.SessionID{"session"}, ScopeHash: "digest"}}
	if withScope.HistoryScope == nil || withScope.HistoryScope.ScopeHash != "digest" {
		t.Fatalf("typed accepted scope missing: %#v", withScope)
	}
}
