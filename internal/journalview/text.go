// Package journalview holds the pure committed-Journal reductions shared
// by the native transcript projector and the channel task projection.
// Nothing here touches storage, services or sockets.
package journalview

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

// TextSegment is one completed assistant text output with its
// deterministic native identity (msgp_<run>_<seq>_<slot>).
type TextSegment struct {
	ID   string
	Text string
}

// TextReducer is the pure assistant-text reducer extracted from the native
// message projector (design §7.1): it resets at model.request, accumulates
// committed model.delta, flushes pre-tool text at the tool.requested
// boundary, and validates the remaining text against model.completed v2
// byte length and SHA-256. It owns no side effects and no tool rows.
type TextReducer struct {
	runID   domain.RunID
	pending strings.Builder
	noop    bool
}

func NewTextReducer(runID domain.RunID) *TextReducer {
	return &TextReducer{runID: runID}
}

// Apply folds one committed event into the reducer and returns any
// completed segment it closes. Corrupt digests, unknown completed versions
// and malformed payloads are explicit errors, never silent truncation.
func (r *TextReducer) Apply(ev domain.RunEvent) ([]TextSegment, error) {
	if r.noop {
		return nil, nil
	}
	switch ev.Type {
	case domain.EventRunStarted:
		var p struct {
			NoContext bool `json:"no_context"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return nil, fmt.Errorf("decode run.started seq %d: %w", ev.Seq, err)
		}
		// `!!` no-context shell runs journal for the transcript only;
		// their rows never enter any text projection.
		r.noop = p.NoContext
		return nil, nil
	case domain.EventModelRequest:
		r.pending.Reset()
		return nil, nil
	case domain.EventModelDelta:
		var p struct {
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return nil, fmt.Errorf("decode model.delta seq %d: %w", ev.Seq, err)
		}
		r.pending.WriteString(p.Delta)
		return nil, nil
	case domain.EventToolRequested:
		if r.pending.Len() == 0 {
			return nil, nil
		}
		segment := TextSegment{ID: TextMessageID(r.runID, ev.Seq, "0"), Text: r.pending.String()}
		r.pending.Reset()
		return []TextSegment{segment}, nil
	case domain.EventModelCompleted:
		content, err := CompletedProjectionContent(ev, r.pending.String())
		if err != nil {
			return nil, err
		}
		r.pending.Reset()
		if content == "" {
			return nil, nil
		}
		return []TextSegment{{ID: TextMessageID(r.runID, ev.Seq, "0"), Text: content}}, nil
	default:
		return nil, nil
	}
}

// TextMessageID is the shared deterministic projection identity:
// msgp_<run>_<seq>_<slot>. It must byte-equal the native transcript IDs.
func TextMessageID(runID domain.RunID, seq domain.EventSeq, slot string) string {
	return fmt.Sprintf("msgp_%s_%020d_%s", runID, seq, slot)
}

// ErrUnsupportedCompletedVersion marks model.completed events that predate
// the v2 hash-commit shape.
var _ = ErrUnsupportedCompletedVersion

// ErrUnsupportedCompletedVersion is the shared sentinel for pre-v2
// model.completed payloads.
var ErrUnsupportedCompletedVersion = errors.New("unsupported model.completed payload version")

// CompletedProjectionContent validates a model.completed payload against
// the accumulated committed deltas and returns that text. It is the same
// rule the native projector applies — a checksum is never decoded as text.
func CompletedProjectionContent(ev domain.RunEvent, deltas string) (string, error) {
	if ev.PayloadVersion != 2 {
		return "", fmt.Errorf("%w %d at seq %d", ErrUnsupportedCompletedVersion, ev.PayloadVersion, ev.Seq)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(ev.Payload, &fields); err != nil {
		return "", fmt.Errorf("decode model.completed seq %d: %w", ev.Seq, err)
	}
	if _, hasContent := fields["content"]; hasContent {
		return "", fmt.Errorf("model.completed seq %d v2 must not contain content", ev.Seq)
	}
	if len(fields) != 2 {
		return "", fmt.Errorf("model.completed seq %d v2 has unknown or missing fields", ev.Seq)
	}
	var p struct {
		ByteLen       int    `json:"byte_len"`
		ContentSHA256 string `json:"content_sha256"`
	}
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return "", fmt.Errorf("decode model.completed metadata seq %d: %w", ev.Seq, err)
	}
	if p.ByteLen != len([]byte(deltas)) {
		return "", fmt.Errorf("model.completed seq %d byte length mismatch", ev.Seq)
	}
	if len(p.ContentSHA256) != sha256.Size*2 || strings.ToLower(p.ContentSHA256) != p.ContentSHA256 {
		return "", fmt.Errorf("model.completed seq %d invalid content digest", ev.Seq)
	}
	sum := sha256.Sum256([]byte(deltas))
	if _, err := hex.DecodeString(p.ContentSHA256); err != nil || p.ContentSHA256 != hex.EncodeToString(sum[:]) {
		return "", fmt.Errorf("model.completed seq %d content digest mismatch", ev.Seq)
	}
	return deltas, nil
}
