package events

import (
	"log/slog"
	"sync"

	"agent-vivy/internal/domain"
)

// Bus fans out persisted RunEvents to live subscribers, keyed by run.
// The journal remains the single source of truth (D-017): the bus is a
// pure optimization that never owns history. Subscribers that fall
// behind are dropped (their channel closes); recovery is to re-read the
// journal from the last seen seq (the SSE path does exactly this), so a
// slow consumer can never wedge a run or lose events.
type Bus struct {
	buffer int

	mu   sync.Mutex
	subs map[domain.RunID][]*subscription
}

type subscription struct {
	ch chan domain.RunEvent
	// closeOnce guards the channel: a terminal publish, a drop, and an
	// explicit cancel may race, and only the first may close.
	closeOnce sync.Once
}

func (s *subscription) close() {
	s.closeOnce.Do(func() { close(s.ch) })
}

// NewBus builds a bus whose subscription channels buffer buffer events.
func NewBus(buffer int) *Bus {
	if buffer <= 0 {
		buffer = 1
	}
	return &Bus{buffer: buffer, subs: make(map[domain.RunID][]*subscription)}
}

// Publish hands one event to every subscriber of its run. Publishing a
// terminal event closes and removes all subscriptions for that run: the
// stream is over, and late subscribers get their history from the
// journal replay anyway.
func (b *Bus) Publish(ev domain.RunEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.subs[ev.RunID]
	if len(subs) == 0 {
		return
	}
	if ev.Type.Terminal() {
		for _, s := range subs {
			s.close()
		}
		delete(b.subs, ev.RunID)
		return
	}
	kept := subs[:0]
	for _, s := range subs {
		select {
		case s.ch <- ev:
			kept = append(kept, s)
		default:
			// Subscriber fell behind: drop it. It re-syncs from the
			// journal; nothing is lost (AS-7).
			slog.Warn("event subscriber dropped; it will re-sync from the journal",
				"run", string(ev.RunID), "seq", int64(ev.Seq))
			s.close()
		}
	}
	b.subs[ev.RunID] = kept
}

// Subscribe registers a live subscription for runID. The returned cancel
// unregisters it; the channel is closed either by cancel, by a terminal
// publish, or when the subscriber falls behind (in which case the caller
// must re-subscribe and replay from the journal).
func (b *Bus) Subscribe(runID domain.RunID) (<-chan domain.RunEvent, func()) {
	s := &subscription{ch: make(chan domain.RunEvent, b.buffer)}
	b.mu.Lock()
	b.subs[runID] = append(b.subs[runID], s)
	b.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			all := b.subs[runID]
			for i, cur := range all {
				if cur == s {
					b.subs[runID] = append(all[:i], all[i+1:]...)
					s.close()
					break
				}
			}
			if len(b.subs[runID]) == 0 {
				delete(b.subs, runID)
			}
		})
	}
	return s.ch, cancel
}

// Subscribers reports the live subscription count for runID (tests and
// diagnostics).
func (b *Bus) Subscribers(runID domain.RunID) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs[runID])
}
