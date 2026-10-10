package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	nb "agent-vivy/internal/notebookcontract"
	rc "agent-vivy/internal/reportcontract"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/testsupport"
)

func reportSettingsWrite(period rc.Period, rev int64, opKey string, mutate func(*rc.ReportSettingsWrite)) nb.OperationKeyed[rc.ReportSettingsWrite] {
	w := rc.ReportSettingsWrite{
		Period: period, ExpectedRevision: rev, Timezone: "UTC",
		SectionID: "section-" + string(period), Enabled: false, ScheduleExpr: "0 9 * * *",
	}
	if mutate != nil {
		mutate(&w)
	}
	return nb.OperationKeyed[rc.ReportSettingsWrite]{OperationKey: opKey, Request: w}
}

// TestReportSettingsWrite covers the typed settings lane: validation,
// revision CAS, idempotency replay, and conflict on key reuse.
func TestReportSettingsWrite(t *testing.T) {
	ctx := context.Background()
	svc, _ := inofyExecService(t, testsupport.NewEchoModel())
	ac := reportAdmissionContext()
	ac.Actor.Ref = "peer:test"

	base, err := svc.ReadReportSettings(ctx, ac, rc.PeriodDaily)
	if err != nil {
		t.Fatal(err)
	}

	res, err := svc.WriteReportSettings(ctx, ac, reportSettingsWrite(rc.PeriodDaily, base.Revision, "op-w1", func(w *rc.ReportSettingsWrite) {
		w.Enabled = true
		w.Timezone = "Asia/Shanghai"
	}))
	if err != nil {
		t.Fatal(err)
	}
	if res.Replayed || res.Settings.Revision != base.Revision+1 || !res.Settings.Enabled || res.Settings.Timezone != "Asia/Shanghai" {
		t.Fatalf("write = %+v", res)
	}

	// Identical retry with the same operation key replays the committed row.
	replay, err := svc.WriteReportSettings(ctx, ac, reportSettingsWrite(rc.PeriodDaily, base.Revision, "op-w1", func(w *rc.ReportSettingsWrite) {
		w.Enabled = true
		w.Timezone = "Asia/Shanghai"
	}))
	if err != nil || !replay.Replayed || replay.Settings.Revision != res.Settings.Revision {
		t.Fatalf("replay = %+v err=%v", replay, err)
	}

	// Stale expected revision declines without writing.
	if _, err := svc.WriteReportSettings(ctx, ac, reportSettingsWrite(rc.PeriodDaily, base.Revision, "op-w2", nil)); err == nil {
		t.Fatal("stale revision should conflict")
	} else if re, ok := err.(*rc.Error); !ok || re.Code != rc.CodeIdempotencyConflict {
		t.Fatalf("stale err = %v", err)
	}

	// Same key, different payload: idempotency conflict.
	if _, err := svc.WriteReportSettings(ctx, ac, reportSettingsWrite(rc.PeriodDaily, res.Settings.Revision, "op-w1", nil)); err == nil {
		t.Fatal("key reuse with different payload should conflict")
	}

	// Validation surfaces.
	for name, mutate := range map[string]func(*rc.ReportSettingsWrite){
		"bad timezone":     func(w *rc.ReportSettingsWrite) { w.Timezone = "Mars/Olympus" },
		"bad schedule":     func(w *rc.ReportSettingsWrite) { w.ScheduleExpr = "every day" },
		"foreign section":  func(w *rc.ReportSettingsWrite) { w.SectionID = "nope" },
		"deleted section":  func(w *rc.ReportSettingsWrite) { w.SectionID = "notes" },
		"unknown provider": func(w *rc.ReportSettingsWrite) { w.Provider = "acme-models" },
	} {
		if _, err := svc.WriteReportSettings(ctx, ac, reportSettingsWrite(rc.PeriodDaily, res.Settings.Revision, "op-"+name, mutate)); err == nil {
			t.Fatalf("%s should be rejected", name)
		}
	}
}

// TestReportCronFireDeduplicates admits one enabled report job through the
// scheduler and proves a duplicated fire of the same instant converges on
// the single committed Run.
func TestReportCronFireDeduplicates(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	svc.deps.Crons = backend
	ac := reportAdmissionContext()
	ac.Actor.Ref = "peer:test"

	base, err := svc.ReadReportSettings(ctx, ac, rc.PeriodDaily)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.WriteReportSettings(ctx, ac, reportSettingsWrite(rc.PeriodDaily, base.Revision, "op-enable", func(w *rc.ReportSettingsWrite) {
		w.Enabled = true
	})); err != nil {
		t.Fatal(err)
	}
	jobID := storage.ReportSettingsJobID("home", rc.PeriodDaily)
	job, err := backend.GetCronJob(ctx, jobID)
	if err != nil || !job.Enabled || job.Payload.Kind != domain.CronPayloadKindReport {
		t.Fatalf("settings job = %+v err=%v", job, err)
	}
	fireAt := job.State.NextRunAtMs
	if fireAt <= 0 {
		t.Fatal("enabled settings row carries no next fire")
	}

	if _, err := svc.TriggerCron(ctx, jobID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	var g *storage.ReportGeneration
	for time.Now().Before(deadline) {
		rows, lerr := backend.ListReportGenerations(ctx, "home", "report.daily", "", "9999")
		if lerr == nil && len(rows) > 0 {
			g = &rows[0]
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if g == nil {
		job, _ := backend.GetCronJob(ctx, jobID)
		t.Fatalf("scheduled fire produced no generation; last=%q err=%q", job.State.LastStatus, job.State.LastError)
	}
	firstRun := g.RunID

	// Wait for the watcher to settle the first fire before re-triggering.
	deadline = time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		fresh, _ := backend.GetCronJob(ctx, jobID)
		if fresh.State.LastStatus == "ok" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	// Force the same fire instant again: identical op key replays to the
	// same committed Run instead of opening a second one.
	job, _ = backend.GetCronJob(ctx, jobID)
	job.State.NextRunAtMs = fireAt
	if err := backend.UpdateCronJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TriggerCron(ctx, jobID); err != nil {
		t.Fatal(err)
	}
	// Let the watcher settle, then assert still exactly one generation/run.
	deadline = time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		fresh, _ := backend.GetCronJob(ctx, jobID)
		if fresh.State.LastStatus == "ok" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	rows, err := backend.ListReportGenerations(ctx, "home", "report.daily", "", "9999")
	if err != nil || len(rows) != 1 || rows[0].RunID != firstRun {
		t.Fatalf("generations after duplicate fire = %+v err=%v", rows, err)
	}
}

// TestReportCronDisabledDoesNotFire: the scheduled loop only fires enabled
// rows; a disabled settings row produces no admission.
func TestReportCronDisabledDoesNotFire(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	svc.deps.Crons = backend

	if _, err := backend.EnsureReportSettings(ctx, "home", rc.PeriodWeekly, rc.ReportSettings{
		Timezone: "UTC", SectionID: "section-weekly", Enabled: false, ScheduleExpr: "0 9 * * 1", Revision: 1,
	}); err != nil {
		t.Fatal(err)
	}
	jobID := storage.ReportSettingsJobID("home", rc.PeriodWeekly)
	job, err := backend.GetCronJob(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}
	job.State.NextRunAtMs = time.Now().UnixMilli() - 1 // overdue but disabled
	if err := backend.UpdateCronJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	svc.ensureCronState(CronSchedulerOptions{})
	svc.fireDueCronJobs(ctx)
	rows, err := backend.ListReportGenerations(ctx, "home", "report.weekly", "", "9999")
	if err != nil || len(rows) != 0 {
		t.Fatalf("disabled schedule admitted: %+v", rows)
	}
}

// TestReportCronOmittedCapability fences a report fire when the capability
// is not bound, and fences recovery of an in-flight report run instead of
// handing it to a generic executor.
func TestReportCronOmittedCapability(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	svc.deps.Crons = backend
	ac := reportAdmissionContext()
	ac.Actor.Ref = "peer:test"

	// New admissions fail closed while the capability is unbound.
	svc.deps.Report = nil
	if _, err := svc.StartReport(ctx, ac, reportRequest("op-omit")); err == nil {
		t.Fatal("admission should fail without the capability")
	}
	svc.deps.Report = backend
	admission, err := svc.StartReport(ctx, ac, reportRequest("op-omit"))
	if err != nil {
		t.Fatal(err)
	}
	run := waitReportRun(t, svc, admission.RunID, domain.RunCompleted)

	// Simulate a restarted process without the capability while the run is
	// still in flight: recovery fences it instead of executing generically.
	if err := backend.SetRunStatus(ctx, run.ID, domain.RunActive); err != nil {
		t.Fatal(err)
	}
	run.Status = domain.RunActive
	svc.deps.Report = nil
	if err := svc.recoverWorkflowRun(ctx, run, ""); !errors.Is(err, ErrWorkflowRecoveryRequired) {
		t.Fatalf("omitted recovery = %v", err)
	}
	svc.deps.Report = backend
	if err := backend.SetRunStatus(ctx, run.ID, domain.RunCompleted); err != nil {
		t.Fatal(err)
	}
}

// TestReportSkippedWindows proves bounded catch-up enumerates calendar
// windows between the last admitted period and the fired one.
func TestReportSkippedWindows(t *testing.T) {
	got := skippedWindowsBetween(rc.PeriodDaily, "2026-03-08", "2026-03-12", "America/New_York", 24)
	if len(got) != 3 || got[0] != "2026-03-09" || got[2] != "2026-03-11" {
		t.Fatalf("daily skipped = %v", got)
	}
	got = skippedWindowsBetween(rc.PeriodMonthly, "2026-01", "2026-04", "UTC", 24)
	if len(got) != 2 || got[0] != "2026-02" || got[1] != "2026-03" {
		t.Fatalf("monthly skipped = %v", got)
	}
	if got := skippedWindowsBetween(rc.PeriodDaily, "2026-03-12", "2026-03-12", "UTC", 24); len(got) != 0 {
		t.Fatalf("same window skipped = %v", got)
	}
	if got := skippedWindowsBetween(rc.PeriodDaily, "2026-03-08", "2026-03-12", "Invalid/Zone", 24); got != nil {
		t.Fatalf("bad zone skipped = %v", got)
	}
}

// TestReportGetSurfacesSkippedWindows: a scheduled admission records the
// skipped range in the committed input and Get reports it.
func TestReportGetSurfacesSkippedWindows(t *testing.T) {
	ctx := context.Background()
	svc, backend := inofyExecService(t, testsupport.NewEchoModel())
	svc.deps.Crons = backend
	ac := reportAdmissionContext()
	ac.Actor.Ref = "peer:test"

	// Two scheduled-style admissions pinned to different as-of instants:
	// the first admits the completed window for 2026-03-07, the second the
	// one for 2026-03-10 — bounded catch-up skips 03-08 and 03-09.
	day := func(d string) int64 {
		tm, err := time.Parse("2006-01-02 15:04", d)
		if err != nil {
			t.Fatal(err)
		}
		return tm.UnixMilli()
	}
	r1 := reportRequest("op-sk1")
	r1.AsOfMs = day("2026-03-08 12:00")
	a1, err := svc.StartReport(ctx, ac, r1)
	if err != nil {
		t.Fatal(err)
	}
	waitReportRun(t, svc, a1.RunID, domain.RunCompleted)
	r2 := reportRequest("op-sk2")
	r2.AsOfMs = day("2026-03-11 12:00")
	admission, err := svc.StartReport(ctx, ac, r2)
	if err != nil {
		t.Fatal(err)
	}
	waitReportRun(t, svc, admission.RunID, domain.RunCompleted)
	res, err := svc.GetReport(ctx, ac, admission.RunID)
	if err != nil {
		t.Fatal(err)
	}
	want := "2026-03-08,2026-03-09"
	if strings.Join(res.SkippedWindows, ",") != want {
		t.Fatalf("skipped projection = %v", res.SkippedWindows)
	}
	if res.Generation == nil || strings.Join(res.Generation.Skipped, ",") != want {
		t.Fatalf("generation skipped = %+v", res.Generation)
	}
	_ = backend
}
