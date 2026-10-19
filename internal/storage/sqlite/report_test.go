package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	nb "agent-vivy/internal/notebookcontract"
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
	annotation, err := b.Notebook().CreateComment(ctx, nb.MutationContext{
		ScopeID: nb.HomeScopeID, Actor: nb.Actor{Kind: nb.ActorHuman, Ref: "local:reviewer"},
		OperationKey: "report-annotation",
	}, nb.CreateCommentRequest{EntryID: rec.EntryID, AnchorRevisionID: rec.RevisionID, Body: "keep this feedback"})
	if err != nil {
		t.Fatalf("annotate report: %v", err)
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
	comments, err := b.Notebook().ListComments(ctx, nb.HomeScopeID, nb.ListCommentsRequest{EntryID: rec.EntryID})
	if err != nil || len(comments.Comments) != 1 {
		t.Fatalf("annotations after regeneration = %+v, error = %v", comments, err)
	}
	comment := comments.Comments[0]
	if comment.ID != annotation.ResourceID || comment.Body != "keep this feedback" || comment.AnchorRevisionID != rec.RevisionID {
		t.Fatalf("regeneration changed the original annotation: %+v", comment)
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

func TestReportSettingsCommitReceiptReplay(t *testing.T) {
	b := openBackend(t)
	ctx := context.Background()

	base, err := b.EnsureReportSettings(ctx, "home", rc.PeriodWeekly, rc.ReportSettings{
		Scope: "home", Period: rc.PeriodWeekly, Timezone: "UTC", SectionID: "section-weekly"})
	if err != nil {
		t.Fatal(err)
	}
	desired := base
	desired.Timezone = "Asia/Shanghai"
	desired.Enabled = true
	desired.ScheduleExpr = "0 7 * * 1"
	mc := nb.MutationContext{ScopeID: "home", Actor: nb.Actor{Kind: nb.ActorHuman, Ref: "peer:test"}, OperationKey: "op-commit-1"}

	committed, receipt, err := b.CommitReportSettings(ctx, mc, base.Revision, desired, 4242)
	if err != nil {
		t.Fatal(err)
	}
	if committed.Revision != base.Revision+1 || receipt.Replayed || receipt.Version != base.Revision+1 {
		t.Fatalf("commit = %+v receipt %+v", committed, receipt)
	}
	got, err := b.GetReportSettings(ctx, "home", rc.PeriodWeekly)
	if err != nil || got.Revision != committed.Revision || got.Timezone != "Asia/Shanghai" || !got.Enabled {
		t.Fatalf("persisted settings = %+v err=%v", got, err)
	}

	// Same operation key + identical payload replays the committed outcome.
	committed2, receipt2, err := b.CommitReportSettings(ctx, mc, base.Revision, desired, 4242)
	if err != nil || !receipt2.Replayed || committed2.Revision != committed.Revision {
		t.Fatalf("replay = %+v receipt %+v err=%v", committed2, receipt2, err)
	}
	// Same operation key + divergent payload is an idempotency conflict.
	divergent := desired
	divergent.Timezone = "UTC"
	if _, _, err := b.CommitReportSettings(ctx, mc, base.Revision, divergent, 4242); !errors.Is(err, nb.ErrIdempotencyConflict) {
		t.Fatalf("divergent replay err = %v", err)
	}
	// A new key carrying the consumed base revision is a revision conflict.
	mc2 := mc
	mc2.OperationKey = "op-commit-2"
	if _, _, err := b.CommitReportSettings(ctx, mc2, base.Revision, desired, 4242); !errors.Is(err, storage.ErrRevisionConflict) {
		t.Fatalf("stale expected err = %v", err)
	}
}
