package app

import (
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestIssue58AutomaticIngestProvenance(t *testing.T) {
	for _, tc := range []struct {
		name, request         string
		version               int
		excluded, unavailable bool
	}{
		{"original", `{"input_provenance":{"content_origin":"original","exclude_automatic_ingest":false}}`, 4, false, false},
		{"notebook", `{"input_provenance":{"content_origin":"notebook","exclude_automatic_ingest":true}}`, 4, true, false},
		{"unknown", `{"input_provenance":{"content_origin":"unknown","exclude_automatic_ingest":false}}`, 4, false, true},
		{"missing-bool", `{"input_provenance":{"content_origin":"notebook"}}`, 4, false, true},
		{"legacy", `{}`, 3, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			b, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "ingest.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			if err := b.CreateSession(ctx, domain.Session{ID: "s", CreatedAt: 1}); err != nil {
				t.Fatal(err)
			}
			if err := b.CreateRun(ctx, domain.Run{ID: "r", SessionID: "s", Status: domain.RunActive, CreatedAt: 1}); err != nil {
				t.Fatal(err)
			}
			if _, err := b.Append(ctx, storage.Commit{RunID: "r", Events: []domain.RunEvent{{Type: domain.EventModelRequest, PayloadVersion: tc.version, Payload: []byte(tc.request)}}}); err != nil {
				t.Fatal(err)
			}
			excluded, err := ingestExclusionPredicate(b)(ctx, "r")
			if tc.unavailable {
				if !errors.Is(err, storage.ErrSubjectEvidenceIncomplete) {
					t.Fatalf("unknown automatic ingest did not fail closed: %t %v", excluded, err)
				}
			} else if err != nil || excluded != tc.excluded {
				t.Fatalf("ingest decision %t %v", excluded, err)
			}
		})
	}
}
