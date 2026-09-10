package channelhost

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"agent-vivy/internal/channelhost/fake"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	plugin "agent-vivy/sdk/port/channel"
)

type limitedFakeChannel struct {
	*fake.Channel
	limit int
}

func (l *limitedFakeChannel) MaxMessageRunes() int {
	return l.limit
}

func newDeliverTestHost(t *testing.T, ch plugin.Channel, configured bool) (*Host, context.Context) {
	t.Helper()
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "deliver_test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	cfg := map[string]config.ChannelEnvelope{}
	if configured && ch != nil {
		cfg[ch.Name()] = config.ChannelEnvelope{
			Enabled:   true,
			AllowFrom: []string{"alice"},
		}
	}

	host := New(Deps{
		Journal:  backend,
		Messages: backend,
		Sessions: backend,
		Run: func(ctx context.Context, sessionID domain.SessionID, text string, prov *domain.Provenance) (domain.RunID, error) {
			return "run_1", nil
		},
		Channels: []plugin.Channel{ch},
		Config:   cfg,
	})

	return host, ctx
}

func TestHostDeliverSuccess(t *testing.T) {
	fc := fake.New()
	host, ctx := newDeliverTestHost(t, fc, true)

	if err := host.StartAll(ctx); err != nil {
		t.Fatalf("StartAll: %v", err)
	}
	defer host.StopAll(ctx)

	if err := host.Deliver(ctx, "fake", "chat-999", "Cron summary: all systems operational"); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	sent := fc.Snapshot()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	if sent[0].ChatID != "chat-999" {
		t.Errorf("expected ChatID chat-999, got %s", sent[0].ChatID)
	}
	if len(sent[0].Parts) != 1 || sent[0].Parts[0].Text != "Cron summary: all systems operational" {
		t.Errorf("unexpected sent parts: %+v", sent[0].Parts)
	}
}

func TestHostDeliverSplitsRunes(t *testing.T) {
	base := fake.New()
	lc := &limitedFakeChannel{Channel: base, limit: 10}
	host, ctx := newDeliverTestHost(t, lc, true)

	if err := host.StartAll(ctx); err != nil {
		t.Fatalf("StartAll: %v", err)
	}
	defer host.StopAll(ctx)

	longContent := "First line\nSecond line\nThird line"
	if err := host.Deliver(ctx, "fake", "chat-long", longContent); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	sent := base.Snapshot()
	if len(sent) <= 1 {
		t.Fatalf("expected multiple chunks from splitting, got %d", len(sent))
	}
	for _, m := range sent {
		if m.ChatID != "chat-long" {
			t.Errorf("expected ChatID chat-long, got %s", m.ChatID)
		}
	}
}

func TestHostDeliverErrors(t *testing.T) {
	fc := fake.New()
	host, ctx := newDeliverTestHost(t, fc, false) // unconfigured

	// 1. Empty channel
	if err := host.Deliver(ctx, "", "chat-1", "msg"); err == nil {
		t.Error("expected error for empty channel name")
	}

	// 2. Empty chat ID
	if err := host.Deliver(ctx, "fake", "", "msg"); err == nil {
		t.Error("expected error for empty chat ID")
	}

	// 3. Unregistered channel
	if err := host.Deliver(ctx, "unregistered", "chat-1", "msg"); err == nil {
		t.Error("expected error for unregistered channel")
	}

	// 4. Registered but not running
	if err := host.StartAll(ctx); err != nil {
		t.Fatalf("StartAll: %v", err)
	}
	err := host.Deliver(ctx, "fake", "chat-1", "msg")
	if err == nil {
		t.Error("expected error for unstarted channel")
	} else if !errors.Is(err, err) && err.Error() == "" {
		t.Errorf("unexpected error: %v", err)
	}
}
