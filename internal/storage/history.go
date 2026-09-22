package storage

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"agent-vivy/internal/domain"
)

// ErrHistoryNarrowScope reports a capture which would exceed the fixed
// storage snapshot envelope. Callers must narrow an authorized selection;
// the error intentionally reveals no source payload.
var ErrHistoryNarrowScope = errors.New("storage: history scope must be narrowed")

const (
	HistoryCutSessionMax = 100
	HistoryCutRunMax     = 256
)

// HistoryStream is the durable source ordering represented by a cursor.
// T2 currently queries message streams. The run-event form is deliberately
// retained as the T4-compatible cursor seam, without implementing admission.
type HistoryStream string

const (
	HistoryStreamMessage  HistoryStream = "message"
	HistoryStreamRunEvent HistoryStream = "run_event"
)

// HistorySessionCut freezes a session's insertion-position ceiling.
type HistorySessionCut struct {
	SessionID domain.SessionID
	Position  int64
}

// HistoryRunCut freezes a run's journal sequence ceiling.
type HistoryRunCut struct {
	RunID domain.RunID
	Seq   domain.EventSeq
}

// HistoryCut is captured atomically before paging. It is an internal storage
// value, not an authorization token or wire cursor.
type HistoryCut struct {
	Sessions []HistorySessionCut
	Runs     []HistoryRunCut
}

func (c HistoryCut) Validate() error {
	if len(c.Sessions) == 0 || len(c.Sessions) > HistoryCutSessionMax {
		return fmt.Errorf("storage: history cut has %d sessions", len(c.Sessions))
	}
	if len(c.Runs) > HistoryCutRunMax {
		return fmt.Errorf("storage: history cut has %d runs", len(c.Runs))
	}
	for i, item := range c.Sessions {
		if item.SessionID == "" || item.Position < 0 || (i > 0 && c.Sessions[i-1].SessionID >= item.SessionID) {
			return fmt.Errorf("storage: invalid history session cut")
		}
	}
	for i, item := range c.Runs {
		if item.RunID == "" || item.Seq < 0 || (i > 0 && c.Runs[i-1].RunID >= item.RunID) {
			return fmt.Errorf("storage: invalid history run cut")
		}
	}
	return nil
}

// ContainsMessagePosition verifies that a decoded cursor remains inside its
// captured session ceiling. Cursor binding to caller and filters is T3's
// responsibility; this prevents a storage caller from silently changing the
// cut's key range.
func (c HistoryCut) ContainsMessagePosition(p HistoryPosition) bool {
	if p.IsZero() { return true }
	if p.Stream != HistoryStreamMessage { return false }
	for _, session := range c.Sessions {
		if session.SessionID == p.SessionID { return p.Position <= session.Position }
	}
	return false
}

// HistoryPosition is an opaque-cursor building block. It tracks the last
// scanned durable key, never a wall-clock timestamp, so an unavailable row
// still advances pagination.
type HistoryPosition struct {
	Stream   HistoryStream
	SessionID domain.SessionID
	RunID     domain.RunID
	Position  int64
	Seq       domain.EventSeq
}

func (p HistoryPosition) IsZero() bool { return p.Stream == "" }

func (p HistoryPosition) Validate() error {
	if p.IsZero() { return nil }
	switch p.Stream {
	case HistoryStreamMessage:
		if p.SessionID == "" || p.Position < 0 || p.RunID != "" || p.Seq != 0 { return fmt.Errorf("storage: invalid message history position") }
	case HistoryStreamRunEvent:
		if p.RunID == "" || p.Seq < 0 || p.SessionID != "" || p.Position != 0 { return fmt.Errorf("storage: invalid run-event history position") }
	default:
		return fmt.Errorf("storage: invalid history stream")
	}
	return nil
}

// HistoryQueryOptions is intentionally storage-internal. T3 owns cursor
// encoding and public filters; this layer owns only finite scan limits.
type HistoryQueryOptions struct {
	Limit  int
	Limits domain.ContinuityLimits
}

// Effective validates the storage page request and applies T1's defaults.
func (o HistoryQueryOptions) Effective() (int, domain.ContinuityLimits, error) {
	limits := o.Limits.Effective(0)
	if o.Limits == (domain.ContinuityLimits{}) { limits = domain.DefaultContinuityLimits() }
	limit := o.Limit
	if limit == 0 { limit = limits.ReadPageDefault }
	if limit < 1 || limit > limits.ReadPageMax { return 0, limits, fmt.Errorf("storage: invalid history page limit %d", limit) }
	return limit, limits, nil
}

// HistoryCandidate is a bounded typed projection ready for T3's redaction
// and literal matching. Text is never a raw serialized event or an oversized
// payload. Unavailable and Truncated deliberately remain independent flags.
type HistoryCandidate struct {
	Ref         domain.SourceRef
	Author      string
	Text        string
	Unavailable bool
	Truncated   bool
}

// HistoryCandidates is one bounded storage page. Next is the last scanned
// key; ScanIncomplete distinguishes an exhausted scan budget from no matches.
type HistoryCandidates struct {
	Records        []HistoryCandidate
	Next           HistoryPosition
	HasMore        bool
	ScanIncomplete bool
	BytesInspected int
}

// HistoryQueryStore is the narrow, backend-owned stable history projection.
// CaptureHistoryCut must use one consistent read transaction. Query pages
// apply current deletion/rewind visibility while refusing rows beyond cut.
type HistoryQueryStore interface {
	CaptureHistoryCut(context.Context, []domain.SessionID) (HistoryCut, error)
	QueryHistoryPage(context.Context, HistoryCut, HistoryPosition, HistoryQueryOptions) (HistoryCandidates, error)
}

// CanonicalHistorySessions validates and canonicalizes an authorized session
// set before it is interpolated into bounded SQL placeholders.
func CanonicalHistorySessions(ids []domain.SessionID) ([]domain.SessionID, error) {
	if len(ids) == 0 || len(ids) > HistoryCutSessionMax { return nil, fmt.Errorf("storage: history session selection has %d entries", len(ids)) }
	out := append([]domain.SessionID(nil), ids...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	for i, id := range out {
		if id == "" || (i > 0 && out[i-1] == id) { return nil, fmt.Errorf("storage: invalid history session selection") }
	}
	return out, nil
}
