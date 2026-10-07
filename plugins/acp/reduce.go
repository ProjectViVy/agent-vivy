package acp

import (
	"context"
	"time"
)

// reduce is the per-prompt ordered reducer (spec §7): committed events are
// released strictly by contiguous sequence; duplicates are dropped; a
// persistent gap, overflow or stream error fails the prompt and cancels the
// affected run. It runs until the prompt's terminal event or a failure
// decision; the final response is chosen exactly once by decide.
func (a *agent) reduce(p *promptState, ctx context.Context) {
	nextSeq := int64(1)
	held := make(map[int64]committedEvent)
	proj := newRunProjection(a.nonce, p.scope)

	var gapTimer *time.Timer
	var gapC <-chan time.Time
	resetGap := func() {
		if gapTimer == nil {
			gapTimer = time.NewTimer(missingSeqTimeout)
			gapC = gapTimer.C
			return
		}
		if !gapTimer.Stop() {
			select {
			case <-gapTimer.C:
			default:
			}
		}
		gapTimer.Reset(missingSeqTimeout)
		gapC = gapTimer.C
	}
	stopGap := func() {
		if gapTimer != nil {
			gapTimer.Stop()
		}
		gapC = nil
	}
	defer stopGap()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.overflow:
			a.failStream(p, "event ingress overflow")
			return
		case <-gapC:
			a.failStream(p, "missing event sequence")
			return
		case ev := <-p.inbox:
			if ev.Seq < nextSeq {
				p.releaseEvent(ev)
				continue // duplicate
			}
			held[ev.Seq] = ev
			for {
				cur, ok := held[nextSeq]
				if !ok {
					break
				}
				delete(held, nextSeq)
				if cur.RunID != "" && p.scope.RunID != "" && cur.RunID != p.scope.RunID {
					// A foreign run's event is dropped without consuming
					// the sequence slot (spec §7 step 2).
					p.releaseEvent(cur)
					continue
				}
				nextSeq++
				p.releaseEvent(cur)
				updates, err := proj.apply(cur)
				if err != nil {
					a.failStream(p, "malformed committed event")
					return
				}
				if !a.emitUpdates(ctx, p, updates) {
					return
				}
				if terminal := a.step(p, cur); terminal {
					return
				}
			}
			if len(held) > 0 {
				resetGap()
			} else {
				stopGap()
			}
		}
	}
}

// step applies one ordered committed event. It returns true when the event
// finalized the prompt. Projection of model/tool updates lands in Tasks
// 5-6; this layer owns terminal precedence and the integrity boundary.
func (a *agent) step(p *promptState, ev committedEvent) bool {
	switch ev.Type {
	case "run.completed":
		if p.isClientCancelled() {
			p.decide(promptResult{resp: cancelledResponse()})
			return true
		}
		p.decide(promptResult{resp: endTurnResponse()})
		return true
	case "run.cancelled":
		if p.isClientCancelled() {
			p.decide(promptResult{resp: cancelledResponse()})
			return true
		}
		if p.isInteractionCancelled() {
			p.decide(promptResult{err: rpcError(-32603, "interaction unsupported in this pilot", "INTERNAL_FAILURE")})
			return true
		}
		p.decide(promptResult{err: cancelledRunError()})
		return true
	case "run.failed":
		if p.isClientCancelled() {
			p.decide(promptResult{resp: cancelledResponse()})
			return true
		}
		p.decide(promptResult{err: rpcError(-32603, "run failed", "INTERNAL_FAILURE")})
		return true
	case "tool.approval_required", "user.question_required":
		// ACP-04 owns human interaction. Until then an interaction event
		// cancels the run with an explicit reason — never auto-approve,
		// never wait (spec §12.6 + plan Task 5/4).
		p.mu.Lock()
		p.interactionCancel = true
		p.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), controlCallTimeout)
		_, _ = a.call(ctx, "run/cancel", map[string]string{"run_id": p.scope.RunID})
		cancel()
		return false
	default:
		// Deliberately unprojected event types still advance the sequence
		// (spec §7 step 4).
		return false
	}
}

// failStream reports a stream-integrity failure once and cancels the
// affected run (spec §7 step 5, §12.7).
func (a *agent) failStream(p *promptState, reason string) {
	if p.scope.RunID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), controlCallTimeout)
		_, _ = a.call(ctx, "run/cancel", map[string]string{"run_id": p.scope.RunID})
		cancel()
	}
	p.decide(promptResult{err: rpcError(-32603, reason, "STREAM_INTEGRITY_FAILED")})
}
