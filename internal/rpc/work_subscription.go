package rpc

import (
	"context"
	"errors"

	"agent-vivy/internal/domain"
)

const workReplayPageSize = 256

type workSubscribeParams struct {
	SessionID string `json:"session_id"`
	AfterSeq  int64  `json:"after_seq,omitempty"`
}

func (h *controlHandler) subscribeWork(ctx context.Context, peer *Peer, request Request) (any, *Error) {
	if h.deps.Work == nil || h.deps.WorkBus == nil {
		return nil, &Error{Code: MethodNotFound, Message: "work subscription is not configured"}
	}
	if peer == nil {
		return nil, &Error{Code: InternalError, Message: "subscription requires a connected peer"}
	}
	var params workSubscribeParams
	if err := decodeParams(request, &params); err != nil {
		return nil, err
	}
	if params.AfterSeq < 0 {
		return nil, &Error{Code: InvalidParams, Message: "after_seq must be non-negative"}
	}
	sessionID, rpcErr := h.authorizeWorkSession(ctx, params.SessionID)
	if rpcErr != nil {
		return nil, rpcErr
	}
	// Attach the live buffer before reading the durable watermark. A commit
	// after this read is either buffered here or recovered by sequence replay.
	ch, stopLive := h.deps.WorkBus.Subscribe(sessionID)
	work, err := h.deps.Work.ReadWork(ctx, sessionID)
	if err != nil {
		stopLive()
		return nil, workError(err)
	}
	subscriptionID := newControlID("work_sub_")
	streamCtx, cancel := context.WithCancel(ctx)
	h.mu.Lock()
	h.subscriptions[subscriptionID] = cancel
	h.mu.Unlock()
	cleanup := func() {
		h.mu.Lock()
		delete(h.subscriptions, subscriptionID)
		h.mu.Unlock()
		cancel()
		stopLive()
	}
	go func() {
		select {
		case <-peer.done:
		case <-streamCtx.Done():
		}
		cleanup()
	}()
	peer.AfterResponse(request.ID, func() {
		defer cleanup()
		h.streamWorkBuffered(streamCtx, peer, subscriptionID, sessionID, domain.WorkVersion(params.AfterSeq), work.Version, ch, stopLive)
	})
	return map[string]any{
		"subscription_id": subscriptionID,
		"session_id":      string(sessionID),
		"after_seq":       params.AfterSeq,
		"watermark_seq":   int64(work.Version),
		"process_epoch":   h.processEpoch,
	}, nil
}

func (h *controlHandler) streamWorkBuffered(ctx context.Context, peer *Peer, subscriptionID string, sessionID domain.SessionID, after, watermark domain.WorkVersion, ch <-chan domain.WorkEvent, cancel func()) {
	defer func() { cancel() }()
	last := domain.WorkSeq(after)
	cursor := domain.WorkState{SessionID: sessionID}
	send := func(event domain.WorkEvent) bool {
		if event.Seq <= last {
			return true
		}
		if err := peer.NotifyContext(ctx, "session/work/event", map[string]any{
			"subscription_id": subscriptionID,
			"event":           workEventView(event),
			"process_epoch":   h.processEpoch,
			"work_version":    int64(event.Seq),
		}); err != nil {
			return false
		}
		last = event.Seq
		return true
	}
	replay := func(through domain.WorkVersion) error {
		for cursor.Version < through {
			limit := workReplayPageSize
			if remaining := int(through - cursor.Version); remaining < limit {
				limit = remaining
			}
			events, next, err := h.deps.Work.ReplayWork(ctx, sessionID, cursor, limit)
			if err != nil {
				return err
			}
			if len(events) == 0 {
				return domain.ErrNonContiguousWorkSeq
			}
			cursor = next
			for _, event := range events {
				if !send(event) {
					return context.Canceled
				}
			}
		}
		return nil
	}
	reportReplayError := func(err error) {
		if !errors.Is(err, context.Canceled) {
			_ = peer.NotifyContext(ctx, "session/work/stream_error", map[string]any{
				"subscription_id": subscriptionID,
				"message":         "work event replay failed",
			})
		}
	}
	if err := replay(watermark); err != nil {
		reportReplayError(err)
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-peer.done:
			return
		case event, ok := <-ch:
			if !ok {
				// Subscribe before replay so events committed while the durable
				// gap is repaired remain buffered for the resumed stream.
				cancel()
				ch, cancel = h.deps.WorkBus.Subscribe(sessionID)
				work, err := h.deps.Work.ReadWork(ctx, sessionID)
				if err != nil {
					reportReplayError(err)
					return
				}
				if err := replay(work.Version); err != nil {
					reportReplayError(err)
					return
				}
				continue
			}
			if event.Seq > last && event.Seq-last > 1 {
				work, err := h.deps.Work.ReadWork(ctx, sessionID)
				if err != nil {
					reportReplayError(err)
					return
				}
				if err := replay(work.Version); err != nil {
					reportReplayError(err)
					return
				}
				if event.Seq > last && event.Seq-last > 1 {
					reportReplayError(domain.ErrNonContiguousWorkSeq)
					return
				}
			}
			if !send(event) {
				return
			}
		}
	}
}
