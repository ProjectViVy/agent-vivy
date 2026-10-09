package runtime

import (
	"errors"
	"time"

	rc "agent-vivy/internal/reportcontract"
)

// ResolveReportWindow computes the local-calendar window for a period and
// selector. Manual "current" generation never claims the period is complete:
// asOf stays inside the half-open interval. Arithmetic always goes through
// civil dates — never fixed 24h assumptions — so DST, month lengths and
// leap years resolve correctly.
func ResolveReportWindow(period rc.Period, selector rc.WindowSelector, tz string, asOfMs int64) (rc.Window, error) {
	if !period.Valid() {
		return rc.Window{}, errors.New("invalid period")
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return rc.Window{}, err
	}
	asOf := time.UnixMilli(asOfMs).In(loc)
	y, m, d := asOf.Date()
	var start, end time.Time
	var id string
	switch period {
	case rc.PeriodDaily:
		start = time.Date(y, m, d, 0, 0, 0, 0, loc)
		end = start.AddDate(0, 0, 1)
		id = start.Format("2006-01-02")
	case rc.PeriodWeekly:
		// ISO week: Monday start.
		day := startOfDay(asOf, loc)
		delta := (int(day.Weekday()) + 6) % 7
		start = day.AddDate(0, 0, -delta)
		end = start.AddDate(0, 0, 7)
		id = start.Format("2006-01-02")
	case rc.PeriodMonthly:
		start = time.Date(y, m, 1, 0, 0, 0, 0, loc)
		end = start.AddDate(0, 1, 0)
		id = start.Format("2006-01")
	}
	if selector == rc.WindowCompleted {
		// The fully completed sibling period.
		end = start
		start = addPeriod(start, period, -1, loc)
		id = windowID(period, start, loc)
	}
	completed := !end.After(asOf)
	return rc.Window{
		Period:    period,
		ID:        id,
		StartMs:   start.UnixMilli(),
		EndMs:     end.UnixMilli(),
		AsOfMs:    asOfMs,
		Completed: completed,
	}, nil
}

func startOfDay(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

func addPeriod(t time.Time, p rc.Period, n int, loc *time.Location) time.Time {
	switch p {
	case rc.PeriodDaily:
		return t.AddDate(0, 0, n)
	case rc.PeriodWeekly:
		return t.AddDate(0, 0, 7*n)
	case rc.PeriodMonthly:
		return t.AddDate(0, n, 0)
	}
	return t
}

func windowID(period rc.Period, start time.Time, loc *time.Location) string {
	if period == rc.PeriodMonthly {
		return start.In(loc).Format("2006-01")
	}
	return start.In(loc).Format("2006-01-02")
}

// reportSeriesID maps a period onto its durable notebook series identity.
func reportSeriesID(period rc.Period) string {
	return "report." + string(period)
}

// coveredLocalDays lists each local calendar date (YYYY-MM-DD) inside the
// half-open window — weekly/monthly collect iterates these for daily reuse.
func coveredLocalDays(w rc.Window, loc *time.Location) []string {
	var out []string
	for d := time.UnixMilli(w.StartMs).In(loc); d.Before(time.UnixMilli(w.EndMs).In(loc)); d = d.AddDate(0, 0, 1) {
		out = append(out, d.Format("2006-01-02"))
	}
	return out
}

// reportRunInput is the admission-pinned run input the sealed program's
// collect node reads from `input`. Every downstream node re-carries it as
// `pins` so replayed nodes never re-resolve mutable state.
type reportRunInput struct {
	Scope          string            `json:"scope"`
	Period         rc.Period         `json:"period"`
	Window         rc.WindowSelector `json:"window"`
	Target         *rc.TargetRef     `json:"target,omitempty"`
	OperationKey   string            `json:"operation_key"`
	RequestDigest  string            `json:"request_digest"`
	SeriesID       string            `json:"series_id"`
	WindowID       string            `json:"window_id"`
	WindowStartMs  int64             `json:"window_start_ms"`
	WindowEndMs    int64             `json:"window_end_ms"`
	AsOfMs         int64             `json:"as_of_ms"`
	Completed      bool              `json:"completed"`
	Timezone       string            `json:"timezone"`
	SectionID      string            `json:"section_id"`
	ConfigRevision int64             `json:"config_revision"`
	Provider       string            `json:"provider,omitempty"`
	ModelID        string            `json:"model_id,omitempty"`
}

func (in reportRunInput) rcWindow() rc.Window {
	return rc.Window{Period: in.Period, ID: in.WindowID, StartMs: in.WindowStartMs,
		EndMs: in.WindowEndMs, AsOfMs: in.AsOfMs, Completed: in.Completed}
}
