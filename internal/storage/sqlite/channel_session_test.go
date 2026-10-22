package sqlite

import (
	"context"
	"errors"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func TestChannelSessionBindingRoundTripAndUnique(t *testing.T) {
	ctx := context.Background()
	b := openBackend(t)
	binding := &domain.ChannelSessionBinding{Channel: "fake", ChatID: "chat/raw", TopicID: ""}
	if err := b.CreateSession(ctx, domain.Session{ID: "old-opaque", CreatedAt: 1, ChannelBinding: binding}); err != nil {
		t.Fatal(err)
	}
	if id, err := b.FindChannelSession(ctx, *binding); err != nil || id != "old-opaque" {
		t.Fatalf("lookup=%q %v", id, err)
	}
	got, err := b.GetSession(ctx, "old-opaque")
	if err != nil || got.ChannelBinding == nil || *got.ChannelBinding != *binding {
		t.Fatalf("binding roundtrip=%+v %v", got, err)
	}
	if err := b.CreateSession(ctx, domain.Session{ID: "different-id", CreatedAt: 2, ChannelBinding: binding}); err == nil {
		t.Fatal("duplicate tuple admitted")
	}
	other := *binding
	other.TopicID = "topic-2"
	if err := b.CreateSession(ctx, domain.Session{ID: "other-topic", CreatedAt: 2, ChannelBinding: &other}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.FindChannelSession(ctx, domain.ChannelSessionBinding{Channel: "fake", ChatID: "missing"}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("absent=%v", err)
	}
	if err := b.CreateSession(ctx, domain.Session{ID: "invalid", ChannelBinding: &domain.ChannelSessionBinding{Channel: "fake"}}); err == nil {
		t.Fatal("incomplete tuple admitted")
	}
	if _, err := b.db.Exec(`INSERT INTO sessions(id,title,created_at,channel_name) VALUES('partial','bad',1,'fake')`); err == nil {
		t.Fatal("partial tuple admitted by schema")
	}
}
