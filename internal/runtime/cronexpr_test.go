package runtime

import (
	"testing"
	"time"

	"agent-vivy/internal/domain"
)

func TestCronExprParseValid(t *testing.T) {
	valid := []string{
		"* * * * *",
		"*/5 * * * *",
		"0 9 * * *",
		"30 8 1,15 * 1-5",
		"0 0 1 1 *",
		"0 9 * * 7",   // 7 == Sunday
		"0 0 9 * * *", // 6-field diva normalization
		"0 */6,*/9 * * *",
	}
	for _, expr := range valid {
		if _, err := ParseCronExpr(expr); err != nil {
			t.Errorf("ParseCronExpr(%q) = %v, want nil", expr, err)
		}
	}
}

func TestCronExprParseInvalid(t *testing.T) {
	invalid := []string{
		"* * * *",      // 4 fields
		"61 * * * *",   // minute out of range
		"* 24 * * *",   // hour out of range
		"* * 0 * *",    // day-of-month out of range
		"* * * 13 *",   // month out of range
		"*/0 * * * *",  // zero step
		"1-0 * * * *",  // reversed range
		"30 0 9 * * *", // 6-field with nonzero seconds
		"meh * * * *",  // non-numeric
		"",             // empty
	}
	for _, expr := range invalid {
		if _, err := ParseCronExpr(expr); err == nil {
			t.Errorf("ParseCronExpr(%q) = nil, want error", expr)
		}
	}
}

func TestCronExprNext(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load tz: %v", err)
	}
	// 2026-08-30 is a Sunday.
	after := time.Date(2026, 8, 30, 10, 0, 0, 0, loc)

	cases := []struct {
		expr string
		want time.Time
	}{
		{"* * * * *", time.Date(2026, 8, 30, 10, 1, 0, 0, loc)},
		{"0 9 * * *", time.Date(2026, 8, 31, 9, 0, 0, 0, loc)},  // next day
		{"0 9 * * 0", time.Date(2026, 9, 6, 9, 0, 0, 0, loc)},   // next Sunday
		{"0 9 * * 7", time.Date(2026, 9, 6, 9, 0, 0, 0, loc)},   // 7 == Sunday
		{"0 0 13 * 1", time.Date(2026, 8, 31, 0, 0, 0, 0, loc)}, // dom/dow OR: Monday beats the 13th
		{"30 8 1 * *", time.Date(2026, 9, 1, 8, 30, 0, 0, loc)}, // next month
		{"0 9 1 3 *", time.Date(2027, 3, 1, 9, 0, 0, 0, loc)},   // month jump
	}
	for _, tc := range cases {
		expr, err := ParseCronExpr(tc.expr)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.expr, err)
		}
		got := expr.Next(after)
		if !got.Equal(tc.want) {
			t.Errorf("Next(%q from %s) = %s, want %s", tc.expr, after, got, tc.want)
		}
	}
}

func TestCronExprNextUnreachable(t *testing.T) {
	expr, err := ParseCronExpr("0 0 30 2 *") // February 30th never exists
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := expr.Next(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); !got.IsZero() {
		t.Errorf("unreachable schedule returned %s, want zero", got)
	}
}

func TestNextCronAfterKinds(t *testing.T) {
	if got := NextCronAfter(domain.CronSchedule{Kind: domain.CronScheduleEvery, EveryMs: 1000}, 5000); got != 6000 {
		t.Errorf("every next = %d, want 6000", got)
	}
	if got := NextCronAfter(domain.CronSchedule{Kind: domain.CronScheduleEvery, EveryMs: 0}, 5000); got != 0 {
		t.Errorf("zero every next = %d, want 0", got)
	}
	if got := NextCronAfter(domain.CronSchedule{Kind: domain.CronScheduleAt, AtMs: 9000}, 5000); got != 9000 {
		t.Errorf("future at next = %d, want 9000", got)
	}
	if got := NextCronAfter(domain.CronSchedule{Kind: domain.CronScheduleAt, AtMs: 1000}, 5000); got != 0 {
		t.Errorf("past at next = %d, want 0", got)
	}

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load tz: %v", err)
	}
	fixed := time.Date(2026, 8, 30, 8, 0, 0, 0, loc)
	restore := cronNow
	cronNow = func() time.Time { return fixed }
	t.Cleanup(func() { cronNow = restore })

	want := time.Date(2026, 8, 30, 9, 0, 0, 0, loc).UnixMilli() // 08:00 now → today 09:00
	got := NextCronAfter(domain.CronSchedule{Kind: domain.CronScheduleCron, Expr: "0 9 * * *", TZ: "Asia/Shanghai"}, fixed.UnixMilli())
	if got != want {
		t.Errorf("cron next = %d, want %d", got, want)
	}

	// After 09:00 the next occurrence is tomorrow.
	later := time.Date(2026, 8, 30, 9, 30, 0, 0, loc)
	cronNow = func() time.Time { return later }
	wantTomorrow := time.Date(2026, 8, 31, 9, 0, 0, 0, loc).UnixMilli()
	if got := NextCronAfter(domain.CronSchedule{Kind: domain.CronScheduleCron, Expr: "0 9 * * *", TZ: "Asia/Shanghai"}, later.UnixMilli()); got != wantTomorrow {
		t.Errorf("cron next after 09:30 = %d, want %d", got, wantTomorrow)
	}
}

func TestValidateCronSchedule(t *testing.T) {
	if err := ValidateCronSchedule(domain.CronSchedule{Kind: domain.CronScheduleAt, AtMs: 123}); err != nil {
		t.Errorf("valid at rejected: %v", err)
	}
	if err := ValidateCronSchedule(domain.CronSchedule{Kind: domain.CronScheduleAt, AtMs: 0}); err == nil {
		t.Error("at without atMs accepted")
	}
	if err := ValidateCronSchedule(domain.CronSchedule{Kind: domain.CronScheduleEvery, EveryMs: 5}); err != nil {
		t.Errorf("valid every rejected: %v", err)
	}
	if err := ValidateCronSchedule(domain.CronSchedule{Kind: domain.CronScheduleCron, Expr: "nope"}); err == nil {
		t.Error("bad expr accepted")
	}
	if err := ValidateCronSchedule(domain.CronSchedule{Kind: domain.CronScheduleCron, Expr: "0 9 * * *", TZ: "Mars/Olympus"}); err == nil {
		t.Error("unknown tz accepted")
	}
	if err := ValidateCronSchedule(domain.CronSchedule{Kind: "weekly"}); err == nil {
		t.Error("unknown kind accepted")
	}
}
