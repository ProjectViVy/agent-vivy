package storage

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"unicode/utf8"

	"agent-vivy/internal/domain"
)

// ErrHistoryNarrowScope reports a capture which would exceed the fixed
// storage snapshot envelope. Callers must narrow an authorized selection;
// the error intentionally reveals no source payload.
var ErrHistoryNarrowScope = errors.New("storage: history scope must be narrowed")

// ErrHistoryMalformed reports retained history metadata which cannot be
// represented without changing a durable identity or cursor key.
var ErrHistoryMalformed = errors.New("storage: malformed history metadata")

const (
	HistoryCutSessionMax      = 100
	HistoryCutRunMax          = 256
	HistoryCandidateRecordMax = 2000
	HistoryCandidateBytesMax  = 4 << 20
	// Metadata is projected through SQL CASE expressions before it reaches a
	// driver. At the 2,001-row lookahead ceiling these per-field limits keep
	// enumeration bounded independently of payload budgets.
	HistoryMetadataIdentityBytesMax = 512
	HistoryMetadataLabelBytesMax    = 128
)

// HistoryStream is the durable source ordering represented by a cursor.
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
	RunID     domain.RunID
	SessionID domain.SessionID
	Seq       domain.EventSeq
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
		if !validHistoryIdentity(string(item.SessionID), false) || item.Position < 0 || (i > 0 && c.Sessions[i-1].SessionID >= item.SessionID) {
			return fmt.Errorf("storage: invalid history session cut")
		}
	}
	for i, item := range c.Runs {
		if !validHistoryIdentity(string(item.RunID), false) || !validHistoryIdentity(string(item.SessionID), false) || item.Seq < 0 || (i > 0 && c.Runs[i-1].RunID >= item.RunID) {
			return fmt.Errorf("storage: invalid history run cut")
		}
		foundSession := false
		for _, session := range c.Sessions {
			if session.SessionID == item.SessionID {
				foundSession = true
				break
			}
		}
		if !foundSession {
			return fmt.Errorf("storage: history run cut is outside session cut")
		}
	}
	return nil
}

func (c HistoryCut) ContainsRunEventPosition(p HistoryPosition) bool {
	if p.IsZero() {
		return true
	}
	if p.Stream != HistoryStreamRunEvent {
		return false
	}
	for _, run := range c.Runs {
		if run.RunID == p.RunID {
			return p.Seq <= run.Seq
		}
	}
	return false
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
	Stream    HistoryStream
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
		if !validHistoryIdentity(string(p.SessionID), false) || p.Position < 0 || p.RunID != "" || p.Seq != 0 { return fmt.Errorf("storage: invalid message history position") }
	case HistoryStreamRunEvent:
		if !validHistoryIdentity(string(p.RunID), false) || p.Seq < 0 || p.SessionID != "" || p.Position != 0 { return fmt.Errorf("storage: invalid run-event history position") }
	default:
		return fmt.Errorf("storage: invalid history stream")
	}
	return nil
}

// HistoryQueryOptions is intentionally storage-internal. T3 owns cursor
// encoding and public filters; this layer owns only finite scan limits.
type HistoryQueryOptions struct {
	Stream HistoryStream
	Limit  int
	Limits domain.ContinuityLimits
}

// Effective validates the storage page request and applies T1's defaults.
func (o HistoryQueryOptions) Effective() (int, domain.ContinuityLimits, error) {
	limits := o.Limits
	defaults := domain.DefaultContinuityLimits()
	if limits == (domain.ContinuityLimits{}) {
		limits = defaults
	}
	if limits.ReadPageDefault == 0 {
		limits.ReadPageDefault = defaults.ReadPageDefault
	}
	if limits.ReadPageMax == 0 {
		limits.ReadPageMax = defaults.ReadPageMax
	}
	if limits.ResultItemBytes == 0 {
		limits.ResultItemBytes = defaults.ResultItemBytes
	}
	if limits.CandidateRecords == 0 {
		limits.CandidateRecords = defaults.CandidateRecords
	}
	if limits.CandidateBytes == 0 {
		limits.CandidateBytes = defaults.CandidateBytes
	}
	if limits.ReadPageDefault < 0 || limits.ReadPageMax < 1 || limits.ResultItemBytes < 1 || limits.CandidateRecords < 1 || limits.CandidateBytes < 1 {
		return 0, limits, fmt.Errorf("storage: history limits must be positive")
	}
	if limits.ReadPageMax > defaults.ReadPageMax {
		limits.ReadPageMax = defaults.ReadPageMax
	}
	if limits.ReadPageDefault > defaults.ReadPageDefault {
		limits.ReadPageDefault = defaults.ReadPageDefault
	}
	if limits.ReadPageDefault > limits.ReadPageMax {
		limits.ReadPageDefault = limits.ReadPageMax
	}
	if limits.ResultItemBytes > defaults.ResultItemBytes {
		limits.ResultItemBytes = defaults.ResultItemBytes
	}
	if limits.CandidateRecords > HistoryCandidateRecordMax {
		limits.CandidateRecords = HistoryCandidateRecordMax
	}
	if limits.CandidateBytes > HistoryCandidateBytesMax {
		limits.CandidateBytes = HistoryCandidateBytesMax
	}
	if o.Stream != "" && o.Stream != HistoryStreamMessage && o.Stream != HistoryStreamRunEvent {
		return 0, limits, fmt.Errorf("storage: invalid history stream %q", o.Stream)
	}
	limit := o.Limit
	if limit == 0 {
		limit = limits.ReadPageDefault
	}
	if limit < 1 || limit > limits.ReadPageMax {
		return 0, limits, fmt.Errorf("storage: invalid history page limit %d", limit)
	}
	return limit, limits, nil
}

// ResolveStream applies the message-stream default and binds subsequent pages
// to the stream encoded in their durable position.
func (o HistoryQueryOptions) ResolveStream(after HistoryPosition) (HistoryStream, error) {
	stream := o.Stream
	if stream == "" {
		if after.IsZero() {
			return HistoryStreamMessage, nil
		}
		return after.Stream, nil
	}
	if !after.IsZero() && after.Stream != stream {
		return "", fmt.Errorf("storage: history cursor stream %q does not match query stream %q", after.Stream, stream)
	}
	return stream, nil
}

// HistoryCandidate is a bounded storage-internal projection ready for T3's
// field-aware sanitization, redaction, and literal matching. Event Text may
// contain a bounded raw payload but an oversized or malformed payload is
// represented only by unavailable metadata. Unavailable and Truncated remain
// independent flags.
type HistoryCandidate struct {
	Ref            domain.SourceRef
	Author         string
	Text           string
	EventType      domain.EventType
	PayloadVersion int
	Unavailable    bool
	Truncated      bool
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
		if !validHistoryIdentity(string(id), false) || (i > 0 && out[i-1] == id) { return nil, fmt.Errorf("storage: invalid history session selection") }
	}
	return out, nil
}

func validHistoryIdentity(value string, allowEmpty bool) bool {
	if value == "" {
		return allowEmpty
	}
	return len(value) <= HistoryMetadataIdentityBytesMax && utf8.ValidString(value)
}
