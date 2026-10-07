package sqlite

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func seedPageRun(t *testing.T, b *Backend, n int) {
	t.Helper()
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "sess_pg", CreatedAt: 1}); err != nil {
		t.Fatalf("session: %v", err)
	}
	if err := b.CreateRun(ctx, domain.Run{ID: "run_pg", SessionID: "sess_pg", Status: domain.RunActive, CreatedAt: 1}); err != nil {
		t.Fatalf("run: %v", err)
	}
	for i := 1; i <= n; i++ {
		if _, err := b.db.ExecContext(ctx,
			`INSERT INTO run_events (run_id, seq, type, created_at, payload_version, payload) VALUES (?, ?, ?, ?, 1, ?)`,
			"run_pg", i, "model.delta", i, `{"i":`+strings.Repeat("x", i%3)+`}`); err != nil {
			t.Fatalf("event %d: %v", i, err)
		}
	}
}

// TestJournalPageWatermark pins §8.1: the page's ThroughSeq is a fixed
// committed watermark, bounds are enforced before allocation, and a page
// never observes appends past H.
func TestJournalPageWatermark(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	seedPageRun(t, b, 300)

	t.Run("first page fixes H; concurrent append past H stays absent", func(t *testing.T) {
		page, err := b.ReadJournalPage(ctx, storage.JournalPageQuery{
			RunID: "run_pg", AfterSeq: 0, MaxEvents: 100, MaxBytes: 1 << 20,
		})
		if err != nil {
			t.Fatalf("page: %v", err)
		}
		if len(page.Events) != 100 || page.ThroughSeq != 300 || !page.HasMore {
			t.Fatalf("page = %+v events", page)
		}
		// Append H+1 after the watermark was fixed: a later page with the
		// same ThroughSeq must not see it.
		if _, err := b.db.ExecContext(ctx,
			`INSERT INTO run_events (run_id, seq, type, created_at, payload_version, payload) VALUES ('run_pg', 301, 'model.delta', 301, 1, '{}')`); err != nil {
			t.Fatalf("append: %v", err)
		}
		page2, err := b.ReadJournalPage(ctx, storage.JournalPageQuery{
			RunID: "run_pg", AfterSeq: 100, ThroughSeq: page.ThroughSeq, MaxEvents: 256, MaxBytes: 1 << 20,
		})
		if err != nil {
			t.Fatalf("page2: %v", err)
		}
		if len(page2.Events) != 200 || page2.Events[199].Seq != 300 || page2.HasMore {
			t.Fatalf("page2 observed post-H append: %+v events", page2)
		}
		// Fresh read captures the new committed maximum.
		page3, err := b.ReadJournalPage(ctx, storage.JournalPageQuery{
			RunID: "run_pg", AfterSeq: 300, MaxEvents: 10, MaxBytes: 1 << 20,
		})
		if err != nil || len(page3.Events) != 1 || page3.ThroughSeq != 301 || page3.HasMore {
			t.Fatalf("page3 = %+v err=%v", page3, err)
		}
	})

	t.Run("bounds enforced", func(t *testing.T) {
		for _, q := range []storage.JournalPageQuery{
			{RunID: "run_pg", MaxEvents: 0, MaxBytes: 100},
			{RunID: "run_pg", MaxEvents: 257, MaxBytes: 100},
			{RunID: "run_pg", MaxEvents: 10, MaxBytes: 0},
			{RunID: "run_pg", MaxEvents: 10, MaxBytes: (1 << 20) + 1},
		} {
			if _, err := b.ReadJournalPage(ctx, q); !errors.Is(err, storage.ErrJournalPageLimit) {
				t.Fatalf("query %+v err=%v, want ErrJournalPageLimit", q, err)
			}
		}
	})

	t.Run("oversized single event is an explicit limit error", func(t *testing.T) {
		if _, err := b.ReadJournalPage(ctx, storage.JournalPageQuery{
			RunID: "run_pg", MaxEvents: 256, MaxBytes: 3,
		}); !errors.Is(err, storage.ErrJournalPageLimit) {
			t.Fatalf("oversized err = %v", err)
		}
	})

	t.Run("empty page never reports HasMore", func(t *testing.T) {
		page, err := b.ReadJournalPage(ctx, storage.JournalPageQuery{
			RunID: "run_pg", AfterSeq: 290, ThroughSeq: 290, MaxEvents: 10, MaxBytes: 100,
		})
		if err != nil || page.HasMore || len(page.Events) != 0 || page.ThroughSeq != 290 {
			t.Fatalf("empty page = %+v err=%v", page, err)
		}
	})
}
