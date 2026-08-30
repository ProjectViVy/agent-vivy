package runtime

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	// Embedded IANA timezone database: cron schedules carry an explicit
	// tz and the server must resolve them on machines (Windows installs)
	// without a system zoneinfo.
	_ "time/tzdata"

	"agent-vivy/internal/domain"
)

// CronExpr is one parsed 5-field cron expression (minute hour day-of-month
// month day-of-week), the same minute-precision dialect diva normalizes to
// (a 6-field input with seconds must carry "0" seconds and is accepted for
// wire compatibility). This is a dependency-free replacement for a cron
// library: the sandbox cannot fetch new modules and the semantics we need
// are small.
type CronExpr struct {
	minute  bitset
	hour    bitset
	dom     bitset
	month   bitset
	dow     bitset
	domStar bool
	dowStar bool
}

// bitset is a precomputed membership set for one field's allowed values.
type bitset struct {
	min, max int
	set      []bool
}

func newBitset(min, max int) bitset {
	return bitset{min: min, max: max, set: make([]bool, max-min+1)}
}

func (b bitset) has(v int) bool {
	if v < b.min || v > b.max {
		return false
	}
	return b.set[v-b.min]
}

func (b *bitset) add(v int) {
	if v >= b.min && v <= b.max {
		b.set[v-b.min] = true
	}
}

func (b bitset) nonEmpty() error {
	for _, on := range b.set {
		if on {
			return nil
		}
	}
	return fmt.Errorf("no value in range [%d, %d] matches", b.min, b.max)
}

// cronNow is the schedule clock; tests replace it.
var cronNow = func() time.Time { return time.Now() }

// ParseCronExpr parses a 5-field cron expression (or a 6-field one whose
// seconds field is exactly "0", matching diva's "0 <5 fields>" shape).
// Fields accept "*", lists "a,b", ranges "a-b", and steps "*/n", "a-b/n",
// "a/n" (from a to the field max). Numbers only; names are not supported.
func ParseCronExpr(expr string) (CronExpr, error) {
	fields := strings.Fields(expr)
	if len(fields) == 6 {
		if fields[0] != "0" {
			return CronExpr{}, fmt.Errorf("runtime: cron seconds field must be 0, got %q", fields[0])
		}
		fields = fields[1:]
	}
	if len(fields) != 5 {
		return CronExpr{}, fmt.Errorf("runtime: cron expression needs 5 fields, got %d", len(fields))
	}

	specs := []struct {
		field    string
		min, max int
	}{
		{fields[0], 0, 59},
		{fields[1], 0, 23},
		{fields[2], 1, 31},
		{fields[3], 1, 12},
		{fields[4], 0, 7},
	}

	e := CronExpr{domStar: specs[2].field == "*", dowStar: specs[4].field == "*"}
	sets := make([]*bitset, len(specs))
	for i, spec := range specs {
		bs := newBitset(spec.min, spec.max)
		if err := parseCronField(spec.field, spec.min, spec.max, &bs); err != nil {
			return CronExpr{}, fmt.Errorf("runtime: cron field %d %q: %w", i+1, spec.field, err)
		}
		sets[i] = &bs
	}
	e.minute, e.hour, e.dom, e.month, e.dow = *sets[0], *sets[1], *sets[2], *sets[3], *sets[4]
	return e, nil
}

// ValidateCronExpr reports whether expr parses. The RPC layer calls this
// on create/update so invalid schedules never reach the store.
func ValidateCronExpr(expr string) error {
	_, err := ParseCronExpr(expr)
	return err
}

func parseCronField(field string, min, max int, out *bitset) error {
	for _, term := range strings.Split(field, ",") {
		rangePart, stepPart, hasStep := strings.Cut(term, "/")
		step := 1
		if hasStep {
			n, err := strconv.Atoi(stepPart)
			if err != nil || n <= 0 {
				return fmt.Errorf("invalid step %q", stepPart)
			}
			step = n
		}

		lo, hi := min, max
		if rangePart != "*" {
			a, b, isRange := strings.Cut(rangePart, "-")
			v, err := strconv.Atoi(a)
			if err != nil {
				return fmt.Errorf("invalid value %q", a)
			}
			lo, hi = v, v
			if isRange {
				w, err := strconv.Atoi(b)
				if err != nil {
					return fmt.Errorf("invalid range end %q", b)
				}
				hi = w
			}
			if lo < min || hi > max || lo > hi {
				return fmt.Errorf("value out of range [%d, %d]", min, max)
			}
		}

		for v := lo; v <= hi; v += step {
			// Day-of-week uses the cron convention 0-7 where both 0 and 7
			// mean Sunday.
			if max == 7 && v == 7 {
				out.add(0)
			} else {
				out.add(v)
			}
		}
	}
	return out.nonEmpty()
}

// Next returns the first matching time strictly after t, at second 0, or
// the zero time when nothing matches within a year (e.g. "0 0 30 2 *").
// Standard cron semantics: day-of-month and day-of-week combine with OR
// when both are restricted, otherwise the restricted one decides.
func (e CronExpr) Next(after time.Time) time.Time {
	t := after.Truncate(time.Minute)
	limit := t.AddDate(1, 0, 0)
	for t = t.Add(time.Minute); !t.After(limit); t = t.Add(time.Minute) {
		if !e.month.has(int(t.Month())) {
			// Jump to the first minute of the next month; nothing in this
			// month can match. The loop post-statement advances one more
			// minute from the 1st 00:00.
			t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()).AddDate(0, 1, 0)
			continue
		}
		if !e.dayMatches(t) {
			continue
		}
		if !e.hour.has(t.Hour()) || !e.minute.has(t.Minute()) {
			continue
		}
		return t
	}
	return time.Time{}
}

func (e CronExpr) dayMatches(t time.Time) bool {
	domOK := e.dom.has(t.Day())
	dowOK := e.dow.has(int(t.Weekday()))
	switch {
	case e.domStar && e.dowStar:
		return true
	case e.domStar:
		return dowOK
	case e.dowStar:
		return domOK
	default:
		return domOK || dowOK
	}
}

// NextCronAfter computes the job's next fire time after nowMs (unix milli;
// 0 = no next occurrence). 'at' never fires in the past; 'every' steps from
// now; 'cron' resolves in the schedule's timezone, falling back to server
// local time when the tz is empty or unknown.
func NextCronAfter(schedule domain.CronSchedule, nowMs int64) int64 {
	switch schedule.Kind {
	case domain.CronScheduleAt:
		if schedule.AtMs > nowMs {
			return schedule.AtMs
		}
		return 0
	case domain.CronScheduleEvery:
		if schedule.EveryMs <= 0 {
			return 0
		}
		return nowMs + schedule.EveryMs
	case domain.CronScheduleCron:
		expr, err := ParseCronExpr(schedule.Expr)
		if err != nil {
			return 0
		}
		// The 500ms nudge keeps a match on the current minute boundary
		// from being selected again right after a fire.
		next := expr.Next(cronNow().In(cronLocation(schedule.TZ)).Add(500 * time.Millisecond))
		if next.IsZero() {
			return 0
		}
		return next.UnixMilli()
	}
	return 0
}

// ValidateCronSchedule fully validates one schedule so invalid values
// never reach the store or silently never fire.
func ValidateCronSchedule(schedule domain.CronSchedule) error {
	switch schedule.Kind {
	case domain.CronScheduleAt:
		if schedule.AtMs <= 0 {
			return fmt.Errorf("runtime: cron schedule at requires a positive atMs")
		}
	case domain.CronScheduleEvery:
		if schedule.EveryMs <= 0 {
			return fmt.Errorf("runtime: cron schedule every requires a positive everyMs")
		}
	case domain.CronScheduleCron:
		if err := ValidateCronExpr(schedule.Expr); err != nil {
			return err
		}
		if schedule.TZ != "" {
			if _, err := time.LoadLocation(schedule.TZ); err != nil {
				return fmt.Errorf("runtime: cron schedule tz %q: %w", schedule.TZ, err)
			}
		}
	default:
		return fmt.Errorf("runtime: unknown cron schedule kind %q", schedule.Kind)
	}
	return nil
}

func cronLocation(tz string) *time.Location {
	if tz == "" {
		return time.Local
	}
	if loc, err := time.LoadLocation(tz); err == nil {
		return loc
	}
	return time.Local
}
