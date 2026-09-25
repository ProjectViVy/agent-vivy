package events

import (
	"log/slog"
	"sync"

	"agent-vivy/internal/domain"
)

// WorkBus fans out committed session work events to live subscribers. The
// durable session_work_events stream remains the source of truth; a dropped
// subscriber replays from its last session sequence.
type WorkBus struct {
	buffer int

	mu   sync.Mutex
	subs map[domain.SessionID][]*workSubscription
}

type workSubscription struct {
	ch        chan domain.WorkEvent
	closeOnce sync.Once
}

func (s *workSubscription) close() {
	s.closeOnce.Do(func() { close(s.ch) })
}

// NewWorkBus builds a work-event bus whose channels buffer buffer events.
func NewWorkBus(buffer int) *WorkBus {
	if buffer <= 0 {
		buffer = 1
	}
	return &WorkBus{buffer: buffer, subs: make(map[domain.SessionID][]*workSubscription)}
}

// Publish hands a committed work event to every subscriber of its session.
// Slow subscribers are dropped; their next connection replays from durable
// storage and therefore cannot lose the session's history.
func (b *WorkBus) Publish(ev domain.WorkEvent) {
	if b == nil || ev.SessionID == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.subs[ev.SessionID]
	kept := subs[:0]
	for _, sub := range subs {
		select {
		case sub.ch <- ev:
			kept = append(kept, sub)
		default:
			slog.Warn("work subscriber dropped; it will re-sync from durable work events",
				"session", string(ev.SessionID), "seq", int64(ev.Seq))
			sub.close()
		}
	}
	if len(kept) == 0 {
		delete(b.subs, ev.SessionID)
	} else {
		b.subs[ev.SessionID] = kept
	}
}

// Subscribe registers a live session work stream. The returned cancel is
// idempotent and closes the channel exactly once.
func (b *WorkBus) Subscribe(sessionID domain.SessionID) (<-chan domain.WorkEvent, func()) {
	if b == nil {
		ch := make(chan domain.WorkEvent)
		close(ch)
		return ch, func() {}
	}
	sub := &workSubscription{ch: make(chan domain.WorkEvent, b.buffer)}
	b.mu.Lock()
	b.subs[sessionID] = append(b.subs[sessionID], sub)
	b.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			all := b.subs[sessionID]
			for i, current := range all {
				if current == sub {
					all = append(all[:i], all[i+1:]...)
					sub.close()
					break
				}
			}
			if len(all) == 0 {
				delete(b.subs, sessionID)
			} else {
				b.subs[sessionID] = all
			}
		})
	}
	return sub.ch, cancel
}

// Subscribers reports the live subscriber count for a session.
func (b *WorkBus) Subscribers(sessionID domain.SessionID) int {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs[sessionID])
}
