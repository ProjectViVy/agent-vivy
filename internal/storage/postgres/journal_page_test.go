package postgres

import (
	"context"
	"errors"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func TestJournalPageWatermarkPG(t *testing.T) {
	b := openChannelTaskBackend(t)
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "sess_jp", CreatedAt: 1}); err != nil {
		t.Fatalf("session: %v", err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "run_jp", SessionID: "sess_jp", Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatalf("run: %v", err)
	}
	for i := 1; i <= 300; i++ {
		if _, err := b.db.ExecContext(ctx,
			`INSERT INTO run_events (run_id, seq, type, created_at, payload_version, payload) VALUES (?, ?, ?, ?, 1, '{}')`,
			"run_jp", i, "model.delta", i); err != nil {
			t.Fatalf("event %d: %v", i, err)
		}
	}

	page, err := b.ReadJournalPage(ctx, storage.JournalPageQuery{
		RunID: "run_jp", MaxEvents: 100, MaxBytes: 1 << 20,
	})
	if err != nil || len(page.Events) != 100 || page.ThroughSeq != 300 || !page.HasMore {
		t.Fatalf("page = %+v err=%v", page, err)
	}
	// Post-watermark append invisible to pages bound to the fixed ceiling.
	if _, err := b.db.ExecContext(ctx,
		`INSERT INTO run_events (run_id, seq, type, created_at, payload_version, payload) VALUES ('run_jp', 301, 'model.delta', 301, 1, '{}')`); err != nil {
		t.Fatalf("append: %v", err)
	}
	page2, err := b.ReadJournalPage(ctx, storage.JournalPageQuery{
		RunID: "run_jp", AfterSeq: 100, ThroughSeq: page.ThroughSeq, MaxEvents: 256, MaxBytes: 1 << 20,
	})
	if err != nil || len(page2.Events) != 200 || page2.Events[199].Seq != 300 || page2.HasMore {
		t.Fatalf("page2 = %+v err=%v", page2, err)
	}
	if _, err := b.ReadJournalPage(ctx, storage.JournalPageQuery{
		RunID: "run_jp", MaxEvents: 0, MaxBytes: 10,
	}); !errors.Is(err, storage.ErrJournalPageLimit) {
		t.Fatalf("bounds err = %v", err)
	}
}
