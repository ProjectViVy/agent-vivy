package stream

import (
	"sort"
	"sync"
)

const (
	DefaultInboxCapacity = 512
	DefaultInboxBytes    = 8 << 20
	DefaultDrainItems    = 128
	DefaultDrainBytes    = 2 << 20
)

type PushResult uint8

const (
	PushAccepted PushResult = iota
	PushReplayRequired
	PushClosed
)

// Inbox is a bounded, non-blocking FIFO between transport callbacks and the
// UI loop. On overflow it drops the whole unrendered batch and raises a sticky
// replay fence: durable events are recovered from the caller's last cursor.
type Inbox struct {
	mu             sync.Mutex
	pending        []Notice
	pendingBytes   int
	maxItems       int
	maxBytes       int
	replayRequired bool
	closed         bool
}

func NewInbox(capacity int) Inbox { return NewBoundedInbox(capacity, DefaultInboxBytes) }

func NewBoundedInbox(maxItems, maxBytes int) Inbox {
	return Inbox{maxItems: maxItems, maxBytes: maxBytes}
}

func (q *Inbox) limitsLocked() (int, int) {
	items, bytes := q.maxItems, q.maxBytes
	if items <= 0 {
		items = DefaultInboxCapacity
	}
	if bytes <= 0 {
		bytes = DefaultInboxBytes
	}
	return items, bytes
}

func noticeBytes(notice Notice) int {
	size := len(notice.SubscriptionID) + len(notice.RunID) + len(notice.Kind) + len(notice.ToolCallID) + len(notice.Line) + len(notice.Delta) + len(notice.Completed) + len(notice.Message)
	if notice.Gate != nil {
		size += len(notice.Gate.Kind) + len(notice.Gate.ID) + len(notice.Gate.ToolCallID) + len(notice.Gate.Title) + len(notice.Gate.Body)
		size += len(notice.Gate.Action) + len(notice.Gate.Target) + len(notice.Gate.PreconditionHash) + len(notice.Gate.Preview)
		for _, risk := range notice.Gate.Risks {
			size += len(risk)
		}
	}
	return size
}

// Push never blocks. PushReplayRequired means the unrendered batch was
// discarded and the caller must resubscribe from its last applied sequence.
func (q *Inbox) Push(notice Notice) PushResult {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return PushClosed
	}
	if q.replayRequired {
		return PushReplayRequired
	}
	maxItems, maxBytes := q.limitsLocked()
	size := noticeBytes(notice)
	if len(q.pending) >= maxItems || size > maxBytes || q.pendingBytes+size > maxBytes {
		q.pending = nil
		q.pendingBytes = 0
		q.replayRequired = true
		return PushReplayRequired
	}
	q.pending = append(q.pending, notice)
	q.pendingBytes += size
	return PushAccepted
}

func (q *Inbox) Take() []Notice {
	pending, _ := q.TakeWithOverflow()
	return pending
}

func (q *Inbox) TakeWithOverflow() ([]Notice, bool) { return q.TakeBatch(0, 0) }

// TakeBatch bounds one Bubble Tea update. Zero limits transfer all pending
// notices. A sticky replay fence is returned even when the batch is empty.
func (q *Inbox) TakeBatch(maxItems, maxBytes int) ([]Notice, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil, false
	}
	replay := q.replayRequired
	q.replayRequired = false
	if len(q.pending) == 0 {
		return nil, replay
	}
	if maxItems <= 0 {
		maxItems = len(q.pending)
	}
	if maxBytes <= 0 {
		maxBytes = q.pendingBytes
	}
	count, bytes := 0, 0
	for count < len(q.pending) && count < maxItems {
		size := noticeBytes(q.pending[count])
		if count > 0 && bytes+size > maxBytes {
			break
		}
		bytes += size
		count++
	}
	if count == 0 {
		count = 1
		bytes = noticeBytes(q.pending[0])
	}
	batch := append([]Notice(nil), q.pending[:count]...)
	q.pending = append([]Notice(nil), q.pending[count:]...)
	q.pendingBytes -= bytes
	return batch, replay
}

// Prepend restores a gap suffix before concurrently arrived notices. If the
// merged queue exceeds either bound, the entire batch becomes replay.
func (q *Inbox) Prepend(notices []Notice) {
	if len(notices) == 0 {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed || q.replayRequired {
		return
	}
	maxItems, maxBytes := q.limitsLocked()
	bytes := q.pendingBytes
	for _, notice := range notices {
		bytes += noticeBytes(notice)
	}
	if len(notices)+len(q.pending) > maxItems || bytes > maxBytes {
		q.pending = nil
		q.pendingBytes = 0
		q.replayRequired = true
		return
	}
	merged := make([]Notice, 0, len(notices)+len(q.pending))
	merged = append(merged, notices...)
	merged = append(merged, q.pending...)
	q.pending = merged
	q.pendingBytes = bytes
}

func (q *Inbox) Clear() {
	q.mu.Lock()
	q.pending = nil
	q.pendingBytes = 0
	q.replayRequired = false
	q.mu.Unlock()
}

func (q *Inbox) Close() {
	q.mu.Lock()
	q.closed = true
	q.pending = nil
	q.pendingBytes = 0
	q.replayRequired = false
	q.mu.Unlock()
}

func (q *Inbox) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending)
}

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
