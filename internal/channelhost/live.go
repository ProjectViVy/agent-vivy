package channelhost

// live.go owns the placeholder + reaction half of the live surface (the
// typing half lives in typing.go; contract §1/§12, 2026-09-15). The shape
// is the typing precedent, one entry per accepted run on the target:
// a goroutine sends the faces best-effort, parks until the turn settles or
// the TTL fires, and cleans the platform state up exactly once. Nothing
// here ever touches the Journal or the delivery ledger — the bookkeeping
// holds platform message ids in process memory only.

import (
	"context"
	"sync"
	"time"

	plugin "agent-vivy/sdk/port/channel"
)

// liveTTL caps how long a placeholder or ack reaction may outlive its run:
// a run that never reaches a terminal (lost event, hung runtime) must not
// leak a platform message forever. It comfortably covers the 5-minute
// default approval expiry, so a pending approval keeps its placeholder
// (contract §12). A var so lifecycle tests can shrink it.
var liveTTL = 10 * time.Minute

// liveCallTimeout bounds one platform live-surface call. The calls run on
// their own goroutine, so a slow platform stalls nothing — the bound keeps
// a wedged adapter from holding the goroutine past its usefulness.
const liveCallTimeout = 10 * time.Second

// liveSurface is the per-run live-surface state beyond typing: the
// placeholder message id and the ack reaction id, filled in by the send
// goroutine and consumed by exactly one cleanup.
type liveSurface struct {
	// settled closes once when the turn ends (terminal handler or StopAll).
	// The send goroutine wakes on it and performs the cleanup itself, so
	// the terminal path never blocks on platform calls.
	settled chan struct{}
	// sentDone closes when the placeholder/reaction sends have finished
	// (each bounded by liveCallTimeout); cleanup waits on it so a terminal
	// racing a slow send cannot read empty ids.
	sentDone chan struct{}
	// cleanOnce makes the cleanup idempotent across the settle, TTL, and
	// StopAll paths.
	cleanOnce sync.Once

	mu            sync.Mutex
	placeholderID string
	reactionID    string
}

func newLiveSurface() *liveSurface {
	return &liveSurface{settled: make(chan struct{}), sentDone: make(chan struct{})}
}

// placeholderFor resolves the adapter's placeholder face through the
// capability seam. A channel without it never placeholders.
func placeholderFor(ch plugin.Channel) plugin.Placeholder {
	target := capabilityTarget(ch)
	if target == nil {
		return nil
	}
	ph, _ := target.(plugin.Placeholder)
	return ph
}

// deleterFor resolves the adapter's message-delete face (the placeholder
// removal path).
func deleterFor(ch plugin.Channel) plugin.MessageDeleter {
	target := capabilityTarget(ch)
	if target == nil {
		return nil
	}
	d, _ := target.(plugin.MessageDeleter)
	return d
}

// reactionFaces resolves the adapter's ack pair. Both faces are required —
// the ack contract includes the withdrawal — matching what Discover
// advertises as Reaction.
func reactionFaces(ch plugin.Channel) (plugin.ReactionSender, plugin.ReactionRemover) {
	target := capabilityTarget(ch)
	if target == nil {
		return nil, nil
	}
	react, _ := target.(plugin.ReactionSender)
	remove, _ := target.(plugin.ReactionRemover)
	return react, remove
}

// newLiveTarget reports whether an accepted turn needs a live entry: any
// of the placeholder / (reaction + withdrawal) faces present. Typing has
// its own stop channel and is not consulted here.
func newLiveTarget(ch plugin.Channel) bool {
	if ch == nil {
		return false
	}
	if placeholderFor(ch) != nil {
		return true
	}
	react, remove := reactionFaces(ch)
	return react != nil && remove != nil
}

// startLiveSurface launches the per-run live-surface goroutine for one
// accepted turn. Nothing here can fail the dispatch: every platform call
// is best-effort with the outcome in the debug log.
func (h *Host) startLiveSurface(target outboundTarget) {
	if target.live == nil {
		return
	}
	h.mu.Lock()
	if h.draining {
		// Shutdown began between dispatch and spawn: nothing will consume
		// the surface through a terminal, and StopAll already swept. Do
		// not start a goroutine that outlives the adapters.
		h.mu.Unlock()
		return
	}
	h.deliveryWG.Add(1)
	h.mu.Unlock()
	go func() {
		defer h.deliveryWG.Done()
		h.runLiveSurface(target)
	}()
}

// runLiveSurface sends the placeholder and the ack reaction best-effort,
// then parks until the turn settles or the TTL fires, and cleans up
// exactly once.
func (h *Host) runLiveSurface(target outboundTarget) {
	live := target.live
	// closeSent ends the send phase. It runs early — before the wait —
	// because cleanupLive (same goroutine, below) waits on it: a deferred
	// close would deadlock the cleanup against its own function exit.
	closeSent := sync.OnceFunc(func() { close(live.sentDone) })
	defer closeSent()

	// A terminal or StopAll that landed before the spawn wins: no placeholder
	// goes out at all — cheaper than send-then-delete during shutdown.
	select {
	case <-live.settled:
		return
	default:
	}

	if ph := placeholderFor(target.ch); ph != nil {
		ctx, cancel := context.WithTimeout(context.Background(), liveCallTimeout)
		id, err := ph.Placeholder(ctx, target.chatID)
		cancel()
		switch {
		case err != nil:
			h.logger.Debug("channelhost: placeholder send failed; the turn runs bare",
				"channel", target.ch.Name(), "chat_id", target.chatID, "err", err)
		case id == "":
			h.logger.Debug("channelhost: placeholder send returned no message id",
				"channel", target.ch.Name(), "chat_id", target.chatID)
		default:
			live.mu.Lock()
			live.placeholderID = id
			live.mu.Unlock()
		}
	}
	if react, _ := reactionFaces(target.ch); react != nil && target.msgID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), liveCallTimeout)
		id, err := react.React(ctx, target.chatID, target.msgID, "")
		cancel()
		switch {
		case err != nil:
			h.logger.Debug("channelhost: ack reaction failed",
				"channel", target.ch.Name(), "chat_id", target.chatID, "err", err)
		case id == "":
			h.logger.Debug("channelhost: ack reaction returned no reaction id; nothing to withdraw",
				"channel", target.ch.Name(), "chat_id", target.chatID)
		default:
			live.mu.Lock()
			live.reactionID = id
			live.mu.Unlock()
		}
	}
	closeSent()

	select {
	case <-live.settled:
		h.cleanupLive(target)
	case <-time.After(liveTTL):
		// The leak backstop: a run that never reached a terminal still
		// gets its placeholder deleted and its ack withdrawn. The delivery
		// target is untouched — a late terminal still delivers.
		h.logger.Info("channelhost: live surface TTL fired; cleaning up a run that never settled",
			"run", string(target.runID), "chat_id", target.chatID)
		h.cleanupLive(target)
	}
}

// closeLive marks the turn's live surface settled. Safe to call from the
// terminal handler and StopAll (idempotent); the platform cleanup happens
// on the surface goroutine, off the runtime event path.
func closeLive(target outboundTarget) {
	if target.live == nil {
		return
	}
	select {
	case <-target.live.settled:
	default:
		close(target.live.settled)
	}
}

// cleanupLive deletes the placeholder and withdraws the ack reaction,
// exactly once per surface. Called from the surface goroutine only, so
// the sentDone wait is already satisfied in practice; it guards the race
// where a TTL fired while a slow send was still in flight.
func (h *Host) cleanupLive(target outboundTarget) {
	live := target.live
	if live == nil {
		return
	}
	live.cleanOnce.Do(func() {
		<-live.sentDone
		live.mu.Lock()
		placeholderID, reactionID := live.placeholderID, live.reactionID
		live.mu.Unlock()

		if placeholderID != "" {
			if d := deleterFor(target.ch); d != nil {
				ctx, cancel := context.WithTimeout(context.Background(), liveCallTimeout)
				if err := d.DeleteMessage(ctx, target.chatID, placeholderID); err != nil {
					h.logger.Warn("channelhost: placeholder delete failed",
						"channel", target.ch.Name(), "chat_id", target.chatID, "err", err)
				}
				cancel()
			}
		}
		if reactionID != "" {
			if _, remove := reactionFaces(target.ch); remove != nil {
				ctx, cancel := context.WithTimeout(context.Background(), liveCallTimeout)
				if err := remove.RemoveReaction(ctx, target.chatID, target.msgID, reactionID); err != nil {
					h.logger.Warn("channelhost: ack reaction withdraw failed",
						"channel", target.ch.Name(), "chat_id", target.chatID, "err", err)
				}
				cancel()
			}
		}
	})
}
