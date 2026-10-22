package channelhost

import (
	"context"
	"errors"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func TestChannelSessionTupleIsCollisionFree(t *testing.T) {
	seen := map[domain.SessionID]bool{}
	for _, tuple := range [][3]string{{"a\x00b", "c", ""}, {"a", "b\x00c", ""}, {"a", "b", "c\x00"}, {"a/b", "c.d", "世界"}, {"a", "b", ""}} {
		id := ChannelSessionID(tuple[0], tuple[1], tuple[2])
		if seen[id] {
			t.Fatalf("distinct platform tuples collide: %q", id)
		}
		seen[id] = true
	}
}

func TestEnsureSessionPreservesConvertedOpaqueIdentity(t *testing.T) {
	ctx := context.Background()
	backend := openBackend(t)
	binding := &domain.ChannelSessionBinding{Channel: "fake", ChatID: "chat-1", TopicID: "topic-9"}
	if err := backend.CreateSession(ctx, domain.Session{ID: "sess_ch_0123456789abcdef", Title: "renamed by user", CreatedAt: 1, ChannelBinding: binding}); err != nil {
		t.Fatal(err)
	}
	host := New(Deps{Sessions: backend, Logger: testLogger()})
	for range 2 {
		id, err := host.EnsureSession(ctx, binding.Channel, binding.ChatID, binding.TopicID)
		if err != nil || id != "sess_ch_0123456789abcdef" {
			t.Fatalf("converted session identity=%q, err=%v", id, err)
		}
	}
	sessions, err := backend.ListSessions(ctx)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("session count=%d, err=%v", len(sessions), err)
	}
}

func TestEnsureSessionRefusesUnprovenHistoryBinding(t *testing.T) {
	ctx := context.Background()
	backend := openBackend(t)
	if err := backend.CreateSession(ctx, domain.Session{ID: "sess_ch_0123456789abcdef", Title: "channel/fake/chat-1", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	host := New(Deps{Sessions: backend, Logger: testLogger()})
	if _, err := host.EnsureSession(ctx, "fake", "chat-1", ""); !errors.Is(err, storage.ErrSourceBindingUnavailable) {
		t.Fatalf("unproven session accepted: %v", err)
	}
	sessions, err := backend.ListSessions(ctx)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("history forked: count=%d, err=%v", len(sessions), err)
	}
}
