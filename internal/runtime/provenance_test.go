package runtime

import (
	"context"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/provider"
)

// TestRunWithChannelProvenance pins the channel world entry: the user
// message row carries Source=channel plus the three provenance fields,
// and the run.started payload stays provenance-free (contract §12).
func TestRunWithChannelProvenance(t *testing.T) {
	svc, backend, _ := newTestService(t, provider.NewMock())
	runID, err := svc.RunWithOptions(context.Background(), "sess-ch-1", "hello vivy", RunOptions{
		Provenance: &domain.Provenance{Source: "channel", Channel: "fake", ChatID: "chat-1", ChannelMessageID: "m-1"},
	})
	if err != nil {
		t.Fatalf("run with provenance: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	msgs, err := backend.ListMessages(context.Background(), "sess-ch-1")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	var user *domain.Message
	for i := range msgs {
		if msgs[i].Role == domain.RoleUser {
			user = &msgs[i]
			break
		}
	}
	if user == nil {
		t.Fatalf("no user message persisted for session sess-ch-1: %+v", msgs)
	}
	if user.Source != "channel" {
		t.Fatalf("user message Source = %q, want channel", user.Source)
	}
	if user.Channel != "fake" || user.ChatID != "chat-1" || user.ChannelMessageID != "m-1" {
		t.Fatalf("user message provenance = channel=%q chat=%q message=%q, want fake/chat-1/m-1",
			user.Channel, user.ChatID, user.ChannelMessageID)
	}
	if user.RunID != runID {
		t.Fatalf("user message RunID = %q, want %q", user.RunID, runID)
	}

	// Provenance stays out of the run.started payload (contract §12).
	events := replayAll(t, backend, runID)
	if len(events) == 0 || events[0].Type != domain.EventRunStarted {
		t.Fatalf("first event = %+v, want run.started", events)
	}
	started := string(events[0].Payload)
	for _, leak := range []string{"provenance", "channel", "chat_id", "fake", "chat-1", "m-1"} {
		if strings.Contains(started, leak) {
			t.Fatalf("run.started payload leaks provenance (%q found): %s", leak, started)
		}
	}
}

// TestRunWithoutProvenanceKeepsUISource pins today's UI semantics: a nil
// Provenance stamps Source "ui" and leaves the channel fields empty.
func TestRunWithoutProvenanceKeepsUISource(t *testing.T) {
	svc, backend, _ := newTestService(t, provider.NewMock())
	runID, err := svc.Run(context.Background(), "sess-ui-1", "hello vivy")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	waitForRunStatus(t, backend, runID, domain.RunCompleted)

	msgs, err := backend.ListMessages(context.Background(), "sess-ui-1")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	var user *domain.Message
	for i := range msgs {
		if msgs[i].Role == domain.RoleUser {
			user = &msgs[i]
			break
		}
	}
	if user == nil {
		t.Fatalf("no user message persisted for session sess-ui-1: %+v", msgs)
	}
	if user.Source != "ui" {
		t.Fatalf("user message Source = %q, want ui", user.Source)
	}
	if user.Channel != "" || user.ChatID != "" || user.ChannelMessageID != "" {
		t.Fatalf("ui user message carries channel provenance: %+v", user)
	}
}

// TestRunWithEmptyProvenanceSourceRejected covers the failure path: a
// non-nil Provenance with an empty Source is rejected before anything is
// persisted.
func TestRunWithEmptyProvenanceSourceRejected(t *testing.T) {
	svc, backend, _ := newTestService(t, provider.NewMock())
	_, err := svc.RunWithOptions(context.Background(), "sess-bad-1", "hello vivy", RunOptions{
		Provenance: &domain.Provenance{},
	})
	if err == nil {
		t.Fatal("run with empty provenance source: want error, got nil")
	}
	msgs, listErr := backend.ListMessages(context.Background(), "sess-bad-1")
	if listErr != nil {
		t.Fatalf("list messages: %v", listErr)
	}
	if len(msgs) != 0 {
		t.Fatalf("rejected run left messages behind: %+v", msgs)
	}
	runs, runsErr := backend.ListActiveRuns(context.Background())
	if runsErr != nil {
		t.Fatalf("list active runs: %v", runsErr)
	}
	if len(runs) != 0 {
		t.Fatalf("rejected run left run rows behind: %+v", runs)
	}
}
