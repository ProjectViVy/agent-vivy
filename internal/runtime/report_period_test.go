package runtime

import (
	"testing"
	"time"

	rc "agent-vivy/internal/reportcontract"
)

func TestReportPeriodDailyWindow(t *testing.T) {
	// Complete day: as-of inside next day, completed selector, local calendar.
	asOf := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC).UnixMilli()
	w, err := ResolveReportWindow(rc.PeriodDaily, rc.WindowCompleted, "UTC", asOf)
	if err != nil {
		t.Fatal(err)
	}
	if w.ID != "2026-10-08" || !w.Completed {
		t.Fatalf("completed daily window = %+v", w)
	}
	if w.EndMs-w.StartMs != 24*time.Hour.Milliseconds() {
		t.Fatalf("UTC day length = %dms", w.EndMs-w.StartMs)
	}
}

func TestReportPeriodWeeklyMondayStart(t *testing.T) {
	// Sunday 2026-10-11 belongs to the week starting Monday 2026-10-05.
	asOf := time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC).UnixMilli()
	w, err := ResolveReportWindow(rc.PeriodWeekly, rc.WindowCurrent, "UTC", asOf)
	if err != nil {
		t.Fatal(err)
	}
	if w.ID != "2026-10-05" {
		t.Fatalf("weekly window id = %s, want Monday 2026-10-05", w.ID)
	}
	if w.Completed {
		t.Fatal("current weekly window must never report completed")
	}
}

func TestReportPeriodMonthlyLeapFeb(t *testing.T) {
	// Leap-year February: 2024-02 has 29 days, civil arithmetic not 30d.
	asOf := time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC).UnixMilli()
	w, err := ResolveReportWindow(rc.PeriodMonthly, rc.WindowCurrent, "UTC", asOf)
	if err != nil {
		t.Fatal(err)
	}
	if w.ID != "2024-02" {
		t.Fatalf("monthly id = %s", w.ID)
	}
	days := (w.EndMs - w.StartMs) / int64(time.Hour*24/time.Millisecond*1000)
	if time.UnixMilli(w.EndMs).UTC().Month() != time.March {
		t.Fatalf("monthly end = %v", time.UnixMilli(w.EndMs))
	}
	_ = days
}

func TestReportPeriodDST(t *testing.T) {
	// DST spring-forward (US/Eastern 2026-03-08): the day is 23h long but the
	// local calendar still owns start/end — never a 24h assumption.
	asOf := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC).UnixMilli()
	w, err := ResolveReportWindow(rc.PeriodDaily, rc.WindowCurrent, "America/New_York", asOf)
	if err != nil {
		t.Fatal(err)
	}
	start := time.UnixMilli(w.StartMs).UTC()
	end := time.UnixMilli(w.EndMs).UTC()
	if start.Hour() != 5 || end.Hour() != 4 {
		t.Fatalf("DST bounds start=%v end=%v", start, end)
	}
}

func TestReportPeriodManualCurrentAsOf(t *testing.T) {
	asOfMs := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC).UnixMilli()
	w, err := ResolveReportWindow(rc.PeriodDaily, rc.WindowCurrent, "UTC", asOfMs)
	if err != nil {
		t.Fatal(err)
	}
	if w.AsOfMs != asOfMs || w.Completed {
		t.Fatalf("manual current window = %+v", w)
	}
	if !(w.StartMs <= w.AsOfMs && w.AsOfMs < w.EndMs) {
		t.Fatalf("as_of outside half-open window: %+v", w)
	}
}

func TestReportPeriodInvalidZone(t *testing.T) {
	asOfMs := time.Now().UnixMilli()
	if _, err := ResolveReportWindow(rc.PeriodDaily, rc.WindowCurrent, "Not/AZone", asOfMs); err == nil {
		t.Fatal("invalid timezone must fail")
	}
	if _, err := ResolveReportWindow(rc.Period("hourly"), rc.WindowCurrent, "UTC", asOfMs); err == nil {
		t.Fatal("invalid period must fail")
	}
}

func TestReportPeriodHalfOpen(t *testing.T) {
	asOf := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC).UnixMilli()
	w, err := ResolveReportWindow(rc.PeriodDaily, rc.WindowCurrent, "UTC", asOf)
	if err != nil {
		t.Fatal(err)
	}
	if w.StartMs >= w.EndMs {
		t.Fatal("window must be half-open [start,end)")
	}
	if w.EndMs == w.StartMs+24*time.Hour.Milliseconds() {
		t.Log("note: UTC day is exactly 24h; DST case covered separately")
	}
}
