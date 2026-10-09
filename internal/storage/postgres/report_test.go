package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	rc "agent-vivy/internal/reportcontract"
	"agent-vivy/internal/storage"
)

func openReportBackend(t *testing.T) *Backend {
	t.Helper()
	dsn := os.Getenv("VIVY_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("VIVY_POSTGRES_TEST_DSN not set")
	}
	schema := fmt.Sprintf("rpt_%d", time.Now().UnixNano())
	b, err := OpenSchema(context.Background(), dsn, schema)
	if err != nil {
		t.Fatalf("OpenSchema: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

func TestReportSettingsEnsureReadAndCAS(t *testing.T) {
	b := openReportBackend(t)
	ctx := context.Background()
	got, err := b.EnsureReportSettings(ctx, "home", rc.PeriodDaily, rc.ReportSettings{
		Scope: "home", Period: rc.PeriodDaily, Timezone: "UTC", SectionID: "section-daily"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Revision != 1 || got.Enabled {
		t.Fatalf("default settings = %+v", got)
	}
	got2, err := b.EnsureReportSettings(ctx, "home", rc.PeriodDaily, rc.ReportSettings{Timezone: "America/New_York"})
	if err != nil || got2.Revision != got.Revision || got2.Timezone != got.Timezone {
		t.Fatalf("ensure replay = %+v err=%v", got2, err)
	}
	got.Timezone = "Europe/Berlin"
	updated, err := b.WriteReportSettingsCAS(ctx, got.Revision, got)
	if err != nil || updated.Revision != 2 || updated.Timezone != "Europe/Berlin" {
		t.Fatalf("CAS write = %+v err=%v", updated, err)
	}
	if _, err := b.WriteReportSettingsCAS(ctx, 1, got); err != storage.ErrRevisionConflict {
		t.Fatalf("stale CAS error = %v", err)
	}
}

func TestReportPublicationFirstPromotedReplayAndCandidate(t *testing.T) {
	b := openReportBackend(t)
	ctx := context.Background()
	in := storage.ReportPublicationInput{
		Scope: "home", OperationKey: "op-1", RequestDigest: "d1",
		SectionID: "section-daily", SeriesID: "report.daily", WindowID: "2026-10-09",
		Title: "Daily report — 2026-10-09", Markdown: "# report\n", Actor: "run:r1",
		Generation: storage.ReportGeneration{Scope: "home", RunID: "run-1", Period: "daily", SeriesID: "report.daily", WindowID: "2026-10-09"},
	}
	rec, err := b.CommitReportPublication(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Outcome != storage.OutcomePromoted || rec.Version != 1 {
		t.Fatalf("first publication = %+v", rec)
	}
	rec2, err := b.CommitReportPublication(ctx, in)
	if err != nil || rec2.EntryID != rec.EntryID || rec2.RevisionID != rec.RevisionID {
		t.Fatalf("replay = %+v err=%v", rec2, err)
	}
	in2 := in
	in2.OperationKey = "op-2"
	in2.Generation.RunID = "run-2"
	in2.AdmittedHead = rec.RevisionID
	in2.AdmittedVersion = rec.Version
	in2.Title = "v2"
	rec3, err := b.CommitReportPublication(ctx, in2)
	if err != nil || rec3.Outcome != storage.OutcomePromoted || rec3.Version != 2 {
		t.Fatalf("promoted second = %+v err=%v", rec3, err)
	}
	in3 := in
	in3.OperationKey = "op-3"
	in3.Generation.RunID = "run-3"
	in3.AdmittedHead = rec.RevisionID
	in3.AdmittedVersion = rec.Version
	rec4, err := b.CommitReportPublication(ctx, in3)
	if err != nil || rec4.Outcome != storage.OutcomeCandidate {
		t.Fatalf("candidate race = %+v err=%v", rec4, err)
	}
	head, err := b.GetReportEntryHead(ctx, "home", rec.EntryID)
	if err != nil || head.HeadRevisionID != rec3.RevisionID {
		t.Fatalf("head after candidate = %+v", head)
	}
}

func TestReportSourcesBounded(t *testing.T) {
	b := openReportBackend(t)
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "s1", Title: "work", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateSession(ctx, domain.Session{ID: "reportctl_x", Title: "hidden", Purpose: domain.SessionPurposeReportControl, CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	sessions, err := b.ListReportSourceSessions(ctx, "home")
	if err != nil {
		t.Fatal(err)
	}
	for _, sess := range sessions {
		if sess.Purpose != "" {
			t.Fatalf("hidden session leaked: %v", sess.ID)
		}
	}
}
