package runtime

import (
	"context"
	"sync"
)

// orderedToolCall is one tool call recorded in model-request order.
type orderedToolCall struct {
	id   string
	name string
}

// toolBatchOrder tracks, per run, every model-turn tool batch in request
// order plus a done signal per call. Parallel dispatch uses it to rebuild the
// sequential contract on top of the RW gate: a call positioned after a
// model-work call in the same batch waits for that work call to finish
// (commit included) before it reaches its own fence check, and a model-work
// call waits for every earlier sibling so a later commit cannot precede an
// earlier call's effect. Waits always point from later calls to earlier ones,
// so the wait graph cannot cycle.
type toolBatchOrder struct {
	mu      sync.Mutex
	batches [][]orderedToolCall
	ready   map[string]chan struct{}
	done    map[string]chan struct{}
	settled map[string]struct{}
}

func newToolBatchOrder() *toolBatchOrder {
	return &toolBatchOrder{
		ready:   make(map[string]chan struct{}),
		done:    make(map[string]chan struct{}),
		settled: make(map[string]struct{}),
	}
}

// noteBatch records one model turn's calls in request order and unblocks
// dispatchers waiting on their batch membership. Consume registers batches
// while tools may already be dispatching, so membership is a signal, not a
// point-in-time read.
func (t *toolBatchOrder) noteBatch(calls []orderedToolCall) {
	if len(calls) == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.batches = append(t.batches, calls)
	for _, c := range calls {
		if ch, ok := t.ready[c.id]; ok {
			close(ch)
			delete(t.ready, c.id)
		}
	}
}

// signalDone marks a call's dispatch as settled — committed, refused, or
// unwound by interrupt — and releases every sibling waiting on it. It is
// idempotent and also fires for calls whose finish was only ever observed in
// the journal (replayed or rebuilt legs never re-dispatch them).
func (t *toolBatchOrder) signalDone(id string) {
	if id == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.settled[id]; ok {
		return
	}
	t.settled[id] = struct{}{}
	if ch, ok := t.done[id]; ok {
		close(ch)
		delete(t.done, id)
	}
}

// awaitEarlier blocks until the batch containing id is registered, then
// returns the calls positioned before id in request order.
func (t *toolBatchOrder) awaitEarlier(ctx context.Context, id string) ([]orderedToolCall, error) {
	t.mu.Lock()
	earlier := t.earlierLocked(id)
	if earlier != nil {
		t.mu.Unlock()
		return earlier, nil
	}
	ch, ok := t.ready[id]
	if !ok {
		ch = make(chan struct{})
		t.ready[id] = ch
	}
	t.mu.Unlock()
	select {
	case <-ch:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.earlierLocked(id), nil
}

// earlierLocked returns the calls before id inside the batch that contains
// it, or nil when no registered batch contains id.
func (t *toolBatchOrder) earlierLocked(id string) []orderedToolCall {
	for _, batch := range t.batches {
		for i, c := range batch {
			if c.id == id {
				out := make([]orderedToolCall, i)
				copy(out, batch[:i])
				return out
			}
		}
	}
	return nil
}

// awaitDone blocks until the call id settles, or ctx ends.
func (t *toolBatchOrder) awaitDone(ctx context.Context, id string) error {
	t.mu.Lock()
	if _, ok := t.settled[id]; ok {
		t.mu.Unlock()
		return nil
	}
	ch, ok := t.done[id]
	if !ok {
		ch = make(chan struct{})
		t.done[id] = ch
	}
	t.mu.Unlock()
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
