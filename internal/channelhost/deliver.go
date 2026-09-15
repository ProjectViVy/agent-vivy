package channelhost

import (
	"context"
	"fmt"
	"time"

	"agent-vivy/internal/attachment"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	plugin "agent-vivy/sdk/port/channel"
)

// enqueueDelivery persists the pending state and spawns the delivery
// goroutine, unless shutdown already began — an intent racing StopAll
// stays pending in the durable store and the next start redelivers it.
// Callers pass the attempt count carried by the durable row so retries
// survive restarts.
func (h *Host) enqueueDelivery(target outboundTarget, attempts int) {
	h.mu.Lock()
	if h.draining {
		h.mu.Unlock()
		return
	}
	h.deliveryWG.Add(1)
	h.mu.Unlock()
	go func() {
		defer h.deliveryWG.Done()
		h.deliver(target, attempts)
	}()
}

// deliver sends the run's last assistant message to the chat the turn
// arrived from, with bounded retries recorded in the durable intent row:
// pending while attempts remain, failed once they are exhausted, deleted
// on success. The reply content is re-read from the message log on every
// attempt — the assistant row is already durable, so the intent row only
// has to remember where the reply must land.
func (h *Host) deliver(target outboundTarget, attempts int) {
	if target.ch == nil {
		// A config envelope naming a channel no compiled-in plugin provides
		// has no adapter to deliver through and never will; settle the
		// intent instead of retrying it forever.
		h.logger.Warn("channelhost: dropping delivery for unregistered channel",
			"run", string(target.runID), "chat_id", target.chatID)
		h.settleDelivery(target)
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), outboundDeliveryTimeout)
	defer cancel()
	for {
		var lastErr error
		content, media, settled, err := h.replyContent(ctx, target)
		switch {
		case settled:
			// A completed run with no assistant text and no media is a
			// settled outcome, not a retryable failure — nothing will ever
			// come.
			h.settleDelivery(target)
			return
		case err == nil:
			lastErr = h.sendReply(ctx, target, content, media)
			if lastErr == nil {
				if derr := h.deps.Deliveries.DeleteChannelDelivery(ctx, target.runID); derr != nil {
					h.logger.Warn("channelhost: delivered but the durable intent delete failed",
						"run", string(target.runID), "err", derr)
				}
				h.logger.Info("channelhost: outbound delivered",
					"run", string(target.runID), "channel", target.name(), "chat_id", target.chatID)
				return
			}
		default:
			// A transient message-store failure keeps the intent and burns
			// an attempt; it must never settle the row.
			lastErr = err
		}
		attempts++
		if attempts >= outboundDeliveryAttempts {
			h.failDelivery(target, attempts, lastErr)
			return
		}
		h.trackDeliveryAttempt(target, attempts)
		select {
		case <-ctx.Done():
			// The delivery budget ran out mid-retry; the intent stays
			// pending and the next start redelivers it.
			return
		case <-time.After(outboundDeliveryRetryDelay):
		}
	}
}

// replyContent loads the run's latest text assistant turn and its media
// attachments (§12 outbound media). settled=true means the intent can
// never produce a delivery and must be removed; err reports a transient
// failure that only the retry path may handle.
func (h *Host) replyContent(ctx context.Context, target outboundTarget) (content string, media []domain.Attachment, settled bool, err error) {
	msgs, err := h.deps.Messages.ListMessages(ctx, target.sessionID)
	if err != nil {
		return "", nil, false, fmt.Errorf("list messages: %w", err)
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		// Tool-call rows project as assistant role too; the reply is the
		// latest text assistant turn of this run.
		if m.RunID == target.runID && m.Role == domain.RoleAssistant && m.ToolCallID == "" {
			if m.Content == "" && len(m.Attachments) == 0 {
				h.logger.Info("channelhost: completed run has no assistant text or media; nothing to deliver",
					"run", string(target.runID), "channel", target.name(), "chat_id", target.chatID)
				return "", nil, true, nil
			}
			return m.Content, m.Attachments, false, nil
		}
	}
	h.logger.Info("channelhost: completed run has no assistant text or media; nothing to deliver",
		"run", string(target.runID), "channel", target.name(), "chat_id", target.chatID)
	return "", nil, true, nil
}

// sendReply splits the content by the adapter's outbound bound and sends
// the chunks in order (CH-C4-N1). The first chunk quotes the triggering
// message via ReplyTo (tier-1 reply threading) — later chunks are
// follow-ups in the same thread, not replies of their own. After the text,
// the reply's media attachments go out as one batch through the ear's
// MediaSender when it has one (§12 outbound media); each part is
// re-validated against the shared limits and an invalid one is dropped
// with a log — reject, never truncate. A media failure fails the whole
// attempt, so the ledger retries it like any send failure; an ear without
// a MediaSender logs a warning and the delivery still succeeds. A
// mid-reply failure is reported as an error so the retry pass redelivers
// from the start — adapters own deduplication limits; the host owns the
// retry.
func (h *Host) sendReply(ctx context.Context, target outboundTarget, content string, media []domain.Attachment) error {
	var delivered int
	chunks := splitRunes(content, target.maxRunes)
	for i, chunk := range chunks {
		outbound := plugin.OutboundMessage{
			ChatID:  target.chatID,
			TopicID: target.topicID,
			Parts:   []plugin.Part{{Kind: plugin.PartText, Text: chunk}},
		}
		if i == 0 {
			outbound.ReplyTo = target.msgID
		}
		ids, err := target.ch.Send(ctx, outbound)
		if err != nil {
			h.logger.Error("channelhost: outbound delivery failed",
				"run", string(target.runID), "channel", target.name(),
				"chat_id", target.chatID, "attempt_delivered", delivered, "err", err)
			if delivered > 0 {
				h.logger.Warn("channelhost: outbound delivery stopped mid-reply",
					"run", string(target.runID), "channel", target.name(),
					"chat_id", target.chatID, "delivered", delivered)
			}
			return err
		}
		delivered += len(ids)
	}
	return h.sendMedia(ctx, target, media)
}

// sendMedia delivers one reply's media attachments as a single batch
// (§12). The bytes are re-read from the message log by the caller on every
// attempt, so a retry re-uploads — at-least-once semantics.
func (h *Host) sendMedia(ctx context.Context, target outboundTarget, media []domain.Attachment) error {
	if len(media) == 0 {
		return nil
	}
	sender, ok := capabilityTarget(target.ch).(plugin.MediaSender)
	if !ok {
		h.logger.Warn("channelhost: ear has no media sender; delivering the reply without its media",
			"run", string(target.runID), "channel", target.name(),
			"chat_id", target.chatID, "media_count", len(media))
		return nil
	}
	parts := make([]plugin.Part, 0, len(media))
	for _, att := range media {
		mime, err := attachment.ValidateOne(att.MimeType, att.Data)
		if err != nil {
			h.logger.Warn("channelhost: dropping invalid outbound media part",
				"run", string(target.runID), "channel", target.name(), "err", err)
			continue
		}
		parts = append(parts, plugin.Part{Kind: plugin.PartMedia, Media: plugin.Media{
			Name:     attachment.SanitizeName(att.Name),
			MimeType: mime,
			Data:     att.Data,
		}})
	}
	if len(parts) == 0 {
		return nil
	}
	if _, err := sender.SendMedia(ctx, target.chatID, parts); err != nil {
		h.logger.Error("channelhost: outbound media delivery failed",
			"run", string(target.runID), "channel", target.name(),
			"chat_id", target.chatID, "media_count", len(parts), "err", err)
		return err
	}
	return nil
}

// trackDeliveryAttempt records the attempt count while the delivery stays
// pending. A persistence failure never stops the in-process retry: the
// log carries the truth and the row may lag by one attempt.
func (h *Host) trackDeliveryAttempt(target outboundTarget, attempts int) {
	if err := h.deps.Deliveries.UpsertChannelDelivery(context.Background(), storage.ChannelDelivery{
		RunID: target.runID, SessionID: target.sessionID, Channel: target.name(),
		ChatID: target.chatID, TopicID: target.topicID,
		State: storage.ChannelDeliveryPending, Attempts: attempts,
		CreatedAtMs: target.createdAtMs, UpdatedAtMs: time.Now().UnixMilli(),
	}); err != nil {
		h.logger.Warn("channelhost: delivery attempt tracking failed",
			"run", string(target.runID), "attempts", attempts, "err", err)
	}
}

// failDelivery marks the intent failed after the attempt budget ran out.
// The row is terminal visibility, not a retry source: an operator revives
// it explicitly through RedeliverDelivery.
func (h *Host) failDelivery(target outboundTarget, attempts int, cause error) {
	attrs := []any{
		"run", string(target.runID), "chat_id", target.chatID, "attempts", attempts,
	}
	if target.ch != nil {
		attrs = append(attrs, "channel", target.name())
	}
	if cause != nil {
		attrs = append(attrs, "err", cause)
	}
	h.logger.Error("channelhost: outbound delivery attempts exhausted; intent parked as failed", attrs...)
	if err := h.deps.Deliveries.UpsertChannelDelivery(context.Background(), storage.ChannelDelivery{
		RunID: target.runID, SessionID: target.sessionID, Channel: target.name(),
		ChatID: target.chatID, TopicID: target.topicID,
		State: storage.ChannelDeliveryFailed, Attempts: attempts,
		CreatedAtMs: target.createdAtMs, UpdatedAtMs: time.Now().UnixMilli(),
	}); err != nil {
		h.logger.Warn("channelhost: failed-intent tracking errored", "run", string(target.runID), "err", err)
	}
}

// settleDelivery deletes a settled intent (delivered, nothing to deliver,
// or undeliverable). Deleting an already-deleted row is not an error.
func (h *Host) settleDelivery(target outboundTarget) {
	if err := h.deps.Deliveries.DeleteChannelDelivery(context.Background(), target.runID); err != nil {
		h.logger.Warn("channelhost: settled-intent delete failed", "run", string(target.runID), "err", err)
	}
}

// FailedDeliveries lists the failed delivery intents — the operator-visible
// side of the ledger. Read-only: rows stay failed until an explicit
// RedeliverDelivery re-arms one.
func (h *Host) FailedDeliveries(ctx context.Context) ([]storage.ChannelDelivery, error) {
	return h.deps.Deliveries.ListFailedChannelDeliveries(ctx)
}

// RedeliverDelivery re-arms one failed delivery intent (the delivery
// control surface failDelivery reserved). The accumulated attempt count is
// kept, so with the budget spent a redeliver is exactly one delivery
// attempt: success deletes the row, another failure re-parks it as failed
// with attempts+1. An operator error (unknown or non-failed run, channel
// not running) is reported, never silently swallowed; a race with StopAll
// keeps the row pending for the next start, matching the drain contract.
func (h *Host) RedeliverDelivery(ctx context.Context, runID domain.RunID) error {
	failed, err := h.deps.Deliveries.ListFailedChannelDeliveries(ctx)
	if err != nil {
		return fmt.Errorf("channelhost: list failed deliveries: %w", err)
	}
	var d storage.ChannelDelivery
	found := false
	for _, row := range failed {
		if row.RunID == runID {
			d = row
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("channelhost: no failed delivery intent for run %s", runID)
	}
	h.mu.Lock()
	draining := h.draining
	started := false
	for _, running := range h.started {
		if running.Name() == d.Channel {
			started = true
			break
		}
	}
	note := h.notes[d.Channel]
	h.mu.Unlock()
	if draining {
		return fmt.Errorf("channelhost: host is shutting down; redeliver %s after restart", runID)
	}
	if !started {
		if note != "" {
			return fmt.Errorf("channelhost: channel %q is not running (%s)", d.Channel, note)
		}
		return fmt.Errorf("channelhost: channel %q is not running", d.Channel)
	}
	ch := h.channelByName(d.Channel)
	if ch == nil {
		return fmt.Errorf("channelhost: channel %q not registered", d.Channel)
	}
	target := outboundTarget{
		runID:       d.RunID,
		sessionID:   d.SessionID,
		chatID:      d.ChatID,
		topicID:     d.TopicID,
		channelName: d.Channel,
		ch:          ch,
		maxRunes:    runesLimit(ch),
		createdAtMs: d.CreatedAtMs,
	}
	h.logger.Info("channelhost: operator redelivery requested",
		"run", string(d.RunID), "channel", d.Channel, "chat_id", d.ChatID, "attempts", d.Attempts)
	// Persist pending BEFORE the goroutine spawns (crash after this point
	// redelivers instead of losing) — the same order as restart recovery.
	h.markPending(target, d.Attempts)
	h.enqueueDelivery(target, d.Attempts)
	return nil
}

// recoverDeliveries reconciles the durable intents at startup, after the
// adapters are live and restart recovery has settled the run store:
//   - armed: the run reached no delivery decision before the restart. The
//     run's journal decides — completed delivers now, failed/cancelled
//     settles, a missing terminal is a post-recovery invariant violation
//     and settles with a warning.
//   - pending: the reply was not confirmed sent. Redeliver (at-least-once;
//     a crash between Send success and the row delete can duplicate one
//     reply). Attempts carried by the row bound the redeliveries so a
//     permanently dead channel settles to failed instead of retrying on
//     every boot.
//
// A listing failure skips the reconcile without failing startup: the rows
// stay open and the next start retries them.
func (h *Host) recoverDeliveries(ctx context.Context) {
	open, err := h.deps.Deliveries.ListOpenChannelDeliveries(ctx)
	if err != nil {
		h.logger.Error("channelhost: list open delivery intents failed; restart redelivery skipped", "err", err)
		return
	}
	for _, d := range open {
		ch := h.startedChannelByName(d.Channel)
		if ch == nil {
			h.logger.Info("channelhost: delivery recovery waits for a started channel",
				"run", string(d.RunID), "channel", d.Channel, "state", d.State)
			continue
		}
		target := outboundTarget{
			runID:       d.RunID,
			sessionID:   d.SessionID,
			chatID:      d.ChatID,
			topicID:     d.TopicID,
			channelName: d.Channel,
			ch:          ch,
			maxRunes:    runesLimit(ch),
			createdAtMs: d.CreatedAtMs,
		}
		switch d.State {
		case storage.ChannelDeliveryArmed:
			switch term := h.runTerminalType(ctx, d.RunID); {
			case term == domain.EventRunCompleted:
				h.markPending(target, d.Attempts)
				h.enqueueDelivery(target, d.Attempts)
			case term.Terminal():
				h.logger.Info("channelhost: restart settles armed intent of ended run",
					"run", string(d.RunID), "type", string(term))
				h.settleDelivery(target)
			default:
				h.logger.Warn("channelhost: armed intent has no terminal run after recovery; settling",
					"run", string(d.RunID))
				h.settleDelivery(target)
			}
		case storage.ChannelDeliveryPending:
			if d.Attempts >= outboundDeliveryAttempts {
				h.failDelivery(target, d.Attempts, nil)
				continue
			}
			h.markPending(target, d.Attempts)
			h.enqueueDelivery(target, d.Attempts)
		default:
			h.logger.Warn("channelhost: unknown open intent state; leaving row untouched",
				"run", string(d.RunID), "state", d.State)
		}
	}
}

// runTerminalType replays the run's journal and returns its terminal event
// type, or the empty type when the run has none.
func (h *Host) runTerminalType(ctx context.Context, runID domain.RunID) domain.EventType {
	it, err := h.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		h.logger.Warn("channelhost: replay for delivery reconcile failed", "run", string(runID), "err", err)
		return ""
	}
	defer func() { _ = it.Close() }()
	term := domain.EventType("")
	for it.Next() {
		if t := it.Value().Event.Type; t.Terminal() {
			term = t
		}
	}
	return term
}

// markPending persists the pending state before the delivery goroutine
// spawns, so a crash after this point redelivers instead of losing.
func (h *Host) markPending(target outboundTarget, attempts int) {
	if err := h.deps.Deliveries.UpsertChannelDelivery(context.Background(), storage.ChannelDelivery{
		RunID: target.runID, SessionID: target.sessionID, Channel: target.name(),
		ChatID: target.chatID, TopicID: target.topicID,
		State: storage.ChannelDeliveryPending, Attempts: attempts,
		CreatedAtMs: target.createdAtMs, UpdatedAtMs: time.Now().UnixMilli(),
	}); err != nil {
		h.logger.Warn("channelhost: pending-intent tracking failed",
			"run", string(target.runID), "err", err)
	}
}

// runesLimit returns the adapter's outbound text bound (0 = unbounded).
func runesLimit(ch plugin.Channel) int {
	if rl, ok := ch.(plugin.RunesLimiter); ok {
		return rl.MaxMessageRunes()
	}
	return 0
}
