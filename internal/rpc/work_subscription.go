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
		h.streamWork(streamCtx, peer, subscriptionID, sessionID, domain.WorkVersion(params.AfterSeq))
	})
	return map[string]any{
		"subscription_id": subscriptionID,
		"session_id":      string(sessionID),
		"after_seq":       params.AfterSeq,
	}, nil
}

func (h *controlHandler) streamWork(ctx context.Context, peer *Peer, subscriptionID string, sessionID domain.SessionID, after domain.WorkVersion) {
	ch, cancel := h.deps.WorkBus.Subscribe(sessionID)
	defer cancel()
	last := after
	send := func(event domain.WorkEvent) bool {
		if event.Seq <= last {
			return true
		}
		if err := peer.NotifyContext(ctx, "session/work/event", map[string]any{
			"subscription_id": subscriptionID,
			"event":           workEventView(event),
		}); err != nil {
			return false
		}
		last = event.Seq
		return true
	}
	replay := func() error {
		for {
			events, err := h.deps.Work.ReplayWork(ctx, sessionID, last, workReplayPageSize)
			if err != nil {
				return err
			}
			for _, event := range events {
				if !send(event) {
					return context.Canceled
				}
			}
			if len(events) < workReplayPageSize {
				return nil
			}
		}
	}
	if err := replay(); err != nil {
		if !errors.Is(err, context.Canceled) {
			_ = peer.NotifyContext(ctx, "session/work/stream_error", map[string]any{
				"subscription_id": subscriptionID,
				"message":         "work event replay failed",
			})
		}
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
				// A dropped subscriber must re-enter durable replay before
				// listening again; this preserves the no-gap contract.
				if err := replay(); err != nil {
					return
				}
				ch, cancel = h.deps.WorkBus.Subscribe(sessionID)
				defer cancel()
				continue
			}
			if !send(event) {
				return
			}
		}
	}
}
