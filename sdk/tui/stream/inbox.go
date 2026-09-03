package stream

import (
	"sort"
	"sync"
)

// Inbox is a lossless FIFO between a transport notification callback and the
// UI update loop. Take transfers ownership of the pending slice; Prepend puts
// an unrendered suffix back after a sequence gap is detected.
type Inbox struct {
	mu      sync.Mutex
	pending []Notice
	closed  bool
}

// Push appends one notice without applying backpressure or dropping events.
func (q *Inbox) Push(notice Notice) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return false
	}
	q.pending = append(q.pending, notice)
	return true
}

// Take removes and returns all currently pending notices.
func (q *Inbox) Take() []Notice {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return nil
	}
	pending := q.pending
	q.pending = nil
	q.mu.Unlock()
	return pending
}

// Prepend restores notices before any concurrently-arrived notices.
func (q *Inbox) Prepend(notices []Notice) {
	if len(notices) == 0 {
		return
	}
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	merged := make([]Notice, 0, len(notices)+len(q.pending))
	merged = append(merged, notices...)
	merged = append(merged, q.pending...)
	q.pending = merged
	q.mu.Unlock()
}

// Clear releases all queued notices during face shutdown.
func (q *Inbox) Clear() {
	q.mu.Lock()
	q.pending = nil
	q.mu.Unlock()
}

// Close permanently rejects future notifications and releases queued data.
// This fences callbacks that raced face shutdown after its context was
// cancelled but before they reached Push.
func (q *Inbox) Close() {
	q.mu.Lock()
	q.closed = true
	q.pending = nil
	q.mu.Unlock()
}

// Len reports the current queue length.
func (q *Inbox) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending)
}

// Order sorts durable events by sequence while keeping legacy sequence-zero
// notices after numbered events. Stable ordering preserves FIFO for ties.
func Order(notices []Notice) {
	sort.SliceStable(notices, func(i, j int) bool {
		if notices[i].Seq <= 0 {
			return false
		}
		if notices[j].Seq <= 0 {
			return true
		}
		return notices[i].Seq < notices[j].Seq
	})
}
