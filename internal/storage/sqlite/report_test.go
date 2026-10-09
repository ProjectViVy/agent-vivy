package sqlite

import (
	"context"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	rc "agent-vivy/internal/reportcontract"
	"agent-vivy/internal/storage"
)

func TestReportSettingsEnsureReadAndCAS(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()

	got, err := b.EnsureReportSettings(ctx, "home", rc.PeriodDaily, rc.ReportSettings{
		Scope: "home", Period: rc.PeriodDaily, Timezone: "UTC", SectionID: "section-daily"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Revision != 1 || got.Enabled {
		t.Fatalf("default settings = %+v", got)
	}
	// Second ensure replays the same durable row.
	got2, err := b.EnsureReportSettings(ctx, "home", rc.PeriodDaily, rc.ReportSettings{Timezone: "America/New_York"})
	if err != nil || got2.Revision != got.Revision || got2.Timezone != got.Timezone {
		t.Fatalf("ensure replay = %+v err=%v", got2, err)
	}
	// CAS: expected revision moves it forward; stale fails.
	got.Timezone = "Europe/Berlin"
	updated, err := b.WriteReportSettingsCAS(ctx, got.Revision, got)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 || updated.Timezone != "Europe/Berlin" {
		t.Fatalf("CAS write = %+v", updated)
	}
	if _, err := b.WriteReportSettingsCAS(ctx, 1, got); err == nil {
		t.Fatal("stale CAS must fail")
	} else if err != storage.ErrRevisionConflict {
		t.Fatalf("stale CAS error = %v", err)
	}
}

func TestReportGenerationInsertGetList(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	g := storage.ReportGeneration{
		Scope: "home", RunID: "run-1", Period: "daily", SeriesID: "report.daily", WindowID: "2026-10-09",
		EntryID: "ent-1", RevisionID: "rev-1", OutcomeMode: storage.OutcomePromoted, CreatedAt: time.Now().UnixMilli(),
	}
	if err := b.InsertReportGeneration(ctx, g); err != nil {
		t.Fatal(err)
	}
	got, err := b.GetReportGenerationByRun(ctx, "home", "run-1")
	if err != nil || got.EntryID != "ent-1" {
		t.Fatalf("get = %+v err=%v", got, err)
	}
	if _, err := b.GetReportGenerationByRun(ctx, "home", "run-1"); err != nil {
		t.Fatal("second get must succeed")
	}
	if err := b.InsertReportGeneration(ctx, g); err == nil {
		t.Fatal("duplicate run_id must fail")
	}
	rows, err := b.ListReportGenerations(ctx, "home", "report.daily", "2026-10-01", "2026-10-31")
	if err != nil || len(rows) != 1 {
		t.Fatalf("list = %v err=%v", rows, err)
	}
}

func TestReportPublicationFirstPromotedReplayAndCandidate(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()
	_ = now

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
	if rec.Outcome != storage.OutcomePromoted || rec.EntryID == "" || rec.RevisionID == "" || rec.Version != 1 {
		t.Fatalf("first publication = %+v", rec)
	}
	// Replay same op key returns the committed receipt.
	rec2, err := b.CommitReportPublication(ctx, in)
	if err != nil || rec2.EntryID != rec.EntryID || rec2.RevisionID != rec.RevisionID {
		t.Fatalf("replay = %+v err=%v", rec2, err)
	}
	// Head unchanged → promoted.
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
	// Admitted head stale → candidate, head untouched.
	in3 := in
	in3.OperationKey = "op-3"
	in3.Generation.RunID = "run-3"
	in3.AdmittedHead = rec.RevisionID // stale: head is now rec3's
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
	b := openBackend(t)
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "s1", Title: "work", CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	sessions, err := b.ListReportSourceSessions(ctx, "home")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) == 0 {
		t.Fatal("ordinary session must be collected")
	}
	// Hidden purpose sessions excluded.
	if err := b.CreateSession(ctx, domain.Session{ID: "reportctl_x", Title: "hidden", Purpose: domain.SessionPurposeReportControl, CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	sessions, err = b.ListReportSourceSessions(ctx, "home")
	if err != nil {
		t.Fatal(err)
	}
	for _, sess := range sessions {
		if sess.Purpose != "" {
			t.Fatalf("hidden session leaked: %v", sess.ID)
		}
	}
}
