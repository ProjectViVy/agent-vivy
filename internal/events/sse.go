package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// heartbeatInterval keeps idle SSE connections alive across proxies and
// client read timeouts.
const heartbeatInterval = 15 * time.Second

// envelope is the wire shape of one streamed event, field-for-field the
// A3 contract (schemas/events/run-event.schema.json). Payload travels as
// raw JSON so the journal bytes are not re-encoded.
type envelope struct {
	RunID          domain.RunID     `json:"run_id"`
	Seq            domain.EventSeq  `json:"seq"`
	Type           domain.EventType `json:"type"`
	CreatedAt      int64            `json:"created_at"`
	PayloadVersion int              `json:"payload_version"`
	Payload        json.RawMessage  `json:"payload"`
}

// ServeSSE streams the run's events to w: first the journal replay from
// afterSeq, then live events from the bus, so a reconnecting client
// sees every event exactly once (AS-7). It returns after the terminal
// event is written, when ctx is cancelled, or on a write failure. The
// journal is the source of truth: if the live subscription is dropped,
// the stream re-subscribes and replays from the last sent seq.
func ServeSSE(ctx context.Context, w http.ResponseWriter, j storage.Journal, bus *Bus, runID domain.RunID, afterSeq domain.EventSeq) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.New("events: response writer does not support streaming")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	lastSent := afterSeq
	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()

	for {
		// Subscribe BEFORE replaying so no publish can slip through the
		// gap; seq-based dedupe below absorbs the overlap.
		live, cancel := bus.Subscribe(runID)
		done, err := replay(ctx, w, flusher, j, runID, &lastSent)
		if err != nil {
			cancel()
			return err
		}
		if done {
			cancel()
			return nil
		}

		done, err = consumeLive(ctx, w, flusher, live, heartbeat, &lastSent)
		cancel()
		if err != nil || done {
			return err
		}
		// Channel closed without a terminal event: the subscriber was
		// dropped (or cancelled). Re-sync from the journal and continue.
	}
}

// replay writes the journal events with seq > *lastSent, updating the
// cursor. It reports done when a terminal event was delivered.
func replay(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, j storage.Journal, runID domain.RunID, lastSent *domain.EventSeq) (bool, error) {
	it, err := j.Replay(ctx, runID, *lastSent)
	if err != nil {
		return false, fmt.Errorf("events: replay %s: %w", runID, err)
	}
	defer func() { _ = it.Close() }()

	for it.Next() {
		ev := it.Value().Event
		if err := writeFrame(w, ev); err != nil {
			return false, err
		}
		*lastSent = ev.Seq
		flusher.Flush()
		if ev.Type.Terminal() {
			return true, nil
		}
	}
	return false, it.Err()
}

// consumeLive pumps live bus events until the terminal event (done),
// ctx cancels, a write fails, or the channel closes early (not done,
// caller must re-sync).
func consumeLive(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, live <-chan domain.RunEvent, heartbeat *time.Ticker, lastSent *domain.EventSeq) (bool, error) {
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case ev, ok := <-live:
			if !ok {
				return false, nil
			}
			if ev.Seq <= *lastSent {
				continue // already delivered by replay
			}
			if err := writeFrame(w, ev); err != nil {
				return false, err
			}
			*lastSent = ev.Seq
			flusher.Flush()
			if ev.Type.Terminal() {
				return true, nil
			}
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return false, err
			}
			flusher.Flush()
		}
	}
}

// writeFrame renders one SSE frame: id carries the seq (usable as the
// reconnect cursor), event carries the type, data carries the A3
// envelope JSON.
func writeFrame(w http.ResponseWriter, ev domain.RunEvent) error {
	payload := json.RawMessage(ev.Payload)
	if len(payload) == 0 {
		payload = json.RawMessage("{}")
	}
	data, err := json.Marshal(envelope{
		RunID:          ev.RunID,
		Seq:            ev.Seq,
		Type:           ev.Type,
		CreatedAt:      ev.CreatedAt,
		PayloadVersion: ev.PayloadVersion,
		Payload:        payload,
	})
	if err != nil {
		return fmt.Errorf("events: marshal envelope: %w", err)
	}
	_, err = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", int64(ev.Seq), string(ev.Type), data)
	return err
}
