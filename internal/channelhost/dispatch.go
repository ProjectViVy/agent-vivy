package channelhost

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	plugin "agent-vivy/sdk/port/channel"
)

// channelInboundPayload is the channel.inbound event payload. Its JSON
// tags match schemas/events/payloads/channel.inbound.json exactly:
// channel, chat_id, sender, message_id, session_id are required; run_id
// is optional and stays omitted this slice (no run is known at journal
// time). No other property may appear (additionalProperties: false).
type channelInboundPayload struct {
	Channel   string `json:"channel"`
	ChatID    string `json:"chat_id"`
	Sender    string `json:"sender"`
	MessageID string `json:"message_id"`
	SessionID string `json:"session_id"`
	RunID     string `json:"run_id,omitempty"`
}

// publishInbound is the dispatch pipeline behind ChannelEnv.PublishInbound
// (VIVY-CHANNEL-PACK.md §7): resolve the envelope, enforce the sender
// allow-list, map the chat to a session, journal channel.inbound, then
// open the run. Dropping is policy, not an error: unknown channels,
// disabled channels, and non-allow-listed senders return nil after a
// structured log that carries identifiers only — never message content.
func (h *Host) publishInbound(ctx context.Context, msg plugin.InboundMessage) error {
	if msg.Channel == "" || msg.ChatID == "" || msg.Sender == "" || msg.MessageID == "" {
		// The channel.inbound payload schema requires all four ids; a
		// malformed envelope never reaches the journal.
		h.logger.Warn("channelhost: dropping inbound with missing channel/chat/sender/message id",
			"channel", msg.Channel, "chat_id", msg.ChatID, "message_id", msg.MessageID)
		return nil
	}
	envelope, ok := h.deps.Config[msg.Channel]
	if !ok {
		h.logger.Warn("channelhost: dropping inbound for unconfigured channel", "channel", msg.Channel)
		return nil
	}
	if !envelope.Enabled {
		h.logger.Warn("channelhost: dropping inbound for disabled channel", "channel", msg.Channel)
		return nil
	}
	// Fail closed even though Start already refused an empty allow_from:
	// the list is the policy for every inbound message, not a start-time
	// decoration. The match is exact — no patterns, no "*".
	allowed := false
	for _, allow := range envelope.AllowFrom {
		if msg.Sender == allow {
			allowed = true
			break
		}
	}
	if !allowed {
		h.logger.Warn("channelhost: dropping inbound from sender outside allow_from",
			"channel", msg.Channel, "chat_id", msg.ChatID, "sender", msg.Sender,
			"message_id", msg.MessageID, "allow_from_count", len(envelope.AllowFrom))
		return nil
	}

	sessionID, err := h.EnsureSession(ctx, msg.Channel, msg.ChatID, msg.TopicID)
	if err != nil {
		return fmt.Errorf("channelhost: ensure session: %w", err)
	}
	if err := h.journalInbound(ctx, msg, sessionID); err != nil {
		return fmt.Errorf("channelhost: journal channel.inbound: %w", err)
	}

	// v1 carries text only; media-ref and structured parts are ignored
	// with a log so a silent content loss stays visible in the gateway log.
	var texts []string
	for _, part := range msg.Parts {
		switch part.Kind {
		case plugin.PartText:
			texts = append(texts, part.Text)
		default:
			h.logger.Warn("channelhost: ignoring non-text inbound part this slice",
				"channel", msg.Channel, "message_id", msg.MessageID, "part_kind", string(part.Kind))
		}
	}
	text := strings.Join(texts, "\n")

	runID, err := h.deps.Run(ctx, sessionID, text, &domain.Provenance{
		Source:           "channel",
		Channel:          msg.Channel,
		ChatID:           msg.ChatID,
		ChannelMessageID: msg.MessageID,
	})
	if err != nil {
		return fmt.Errorf("channelhost: start run for channel %s: %w", msg.Channel, err)
	}
	ch := h.channelByName(msg.Channel)
	maxRunes := 0
	if rl, ok := ch.(plugin.RunesLimiter); ok {
		maxRunes = rl.MaxMessageRunes()
	}
	h.mu.Lock()
	h.targets[runID] = outboundTarget{
		sessionID: sessionID,
		chatID:    msg.ChatID,
		topicID:   msg.TopicID,
		ch:        ch,
		maxRunes:  maxRunes,
	}
	h.mu.Unlock()
	return nil
}

// journalInbound appends one channel.inbound event under a per-message
// pseudo run scope (chanin_<16hex>). This resolves TODO CH-C1-N1 within
// the existing journal machinery: storage.Journal already rejects empty
// run ids and enforces one-terminal-per-run, and no terminal event is
// ever appended to the pseudo run, so the run contract stays intact
// without a new storage table.
func (h *Host) journalInbound(ctx context.Context, msg plugin.InboundMessage, sessionID domain.SessionID) error {
	chaninID, err := newChannelInboundID()
	if err != nil {
		return fmt.Errorf("mint pseudo run id: %w", err)
	}
	data, err := json.Marshal(channelInboundPayload{
		Channel:   msg.Channel,
		ChatID:    msg.ChatID,
		Sender:    msg.Sender,
		MessageID: msg.MessageID,
		SessionID: string(sessionID),
	})
	if err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}
	event := domain.RunEvent{
		RunID:          chaninID,
		Type:           domain.EventChannelInbound,
		CreatedAt:      time.Now().UnixMilli(),
		PayloadVersion: 1,
		Payload:        data,
	}
	if _, err := h.deps.Journal.Append(ctx, storage.Commit{RunID: chaninID, Events: []domain.RunEvent{event}}); err != nil {
		return err
	}
	return nil
}

// OnRunEvent observes durable run events (runtime.RunHook satisfied
// structurally). Only tracked channel runs are acted on: on run.completed
// the run's last assistant message is delivered back to the originating
// chat; run.failed / run.cancelled are logged and deliver nothing this
// slice. The tracking entry is removed either way, so a terminal closes
// exactly one delivery.
func (h *Host) OnRunEvent(ctx context.Context, ev domain.RunEvent) {
	if !ev.Type.Terminal() {
		return
	}
	h.mu.Lock()
	target, tracked := h.targets[ev.RunID]
	delete(h.targets, ev.RunID)
	h.mu.Unlock()
	if !tracked {
		return
	}
	if target.ch == nil {
		// channelByName missed (a config envelope naming a channel no
		// compiled-in plugin provides): there is no adapter to deliver
		// through and no channel name to log. Drop with a warning — a
		// method call on the nil interface would panic the goroutine.
		h.logger.Warn("channelhost: dropping delivery for unregistered channel",
			"run", string(ev.RunID), "chat_id", target.chatID)
		return
	}
	if ev.Type != domain.EventRunCompleted {
		h.logger.Info("channelhost: channel run ended without delivery",
			"run", string(ev.RunID), "type", string(ev.Type),
			"channel", target.ch.Name(), "chat_id", target.chatID)
		return
	}
	// Delivery hops off the runtime goroutine: platform Send latency must
	// never stall event mapping. The fresh context survives the run's own
	// cancellation (an inbound turn must be answered even if its caller
	// is gone) and is bounded by outboundDeliveryTimeout.
	go h.deliverCompleted(ev.RunID, target)
}

// deliverCompleted sends the run's last assistant message to the chat the
// turn arrived from.
func (h *Host) deliverCompleted(runID domain.RunID, target outboundTarget) {
	if target.ch == nil {
		// Defense at the goroutine boundary (the only unguarded hop): a
		// nil-channel target is dropped with a warning, never dereferenced.
		h.logger.Warn("channelhost: dropping delivery for unregistered channel",
			"run", string(runID), "chat_id", target.chatID)
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), outboundDeliveryTimeout)
	defer cancel()
	msgs, err := h.deps.Messages.ListMessages(ctx, target.sessionID)
	if err != nil {
		h.logger.Error("channelhost: list messages for channel delivery failed",
			"run", string(runID), "channel", target.ch.Name(), "err", err)
		return
	}
	content := ""
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		// Tool-call rows project as assistant role too; the reply is the
		// latest text assistant turn of this run.
		if m.RunID == runID && m.Role == domain.RoleAssistant && m.ToolCallID == "" {
			content = m.Content
			break
		}
	}
	if content == "" {
		h.logger.Info("channelhost: completed run has no assistant text; nothing to deliver",
			"run", string(runID), "channel", target.ch.Name(), "chat_id", target.chatID)
		return
	}
	// CH-C4-N1: an adapter-declared outbound bound splits the reply into
	// several sends instead of one delivery the platform would reject (the
	// telegram failure that motivated the row). Chunk boundaries prefer a
	// newline inside the window; a mid-word hard break is the fallback.
	// Runes approximate platform character limits; an astral-heavy text
	// may still edge past a UTF-16-counting ceiling, but never by the
	// order of magnitude that caused the original total loss.
	var delivered int
	for _, chunk := range splitRunes(content, target.maxRunes) {
		ids, err := target.ch.Send(ctx, plugin.OutboundMessage{
			ChatID:  target.chatID,
			TopicID: target.topicID,
			Parts:   []plugin.Part{{Kind: plugin.PartText, Text: chunk}},
		})
		if err != nil {
			h.logger.Error("channelhost: outbound delivery failed",
				"run", string(runID), "channel", target.ch.Name(), "chat_id", target.chatID, "err", err)
			if delivered > 0 {
				h.logger.Warn("channelhost: outbound delivery stopped mid-reply",
					"run", string(runID), "channel", target.ch.Name(),
					"chat_id", target.chatID, "delivered", delivered)
			}
			return
		}
		delivered += len(ids)
	}
	h.logger.Info("channelhost: outbound delivered",
		"run", string(runID), "channel", target.ch.Name(), "chat_id", target.chatID, "ids", delivered)
}

// splitRunes chunks content into pieces of at most limit runes. limit <= 0
// returns the content unchanged. Each chunk breaks at the last newline
// inside the window when one exists (keeps paragraph shapes readable);
// otherwise it hard-breaks. No empty chunk escapes.
func splitRunes(content string, limit int) []string {
	runes := []rune(content)
	if limit <= 0 || len(runes) <= limit {
		return []string{content}
	}
	var out []string
	for start := 0; start < len(runes); {
		end := start + limit
		if end >= len(runes) {
			out = append(out, string(runes[start:]))
			break
		}
		cut := end
		if idx := lastNewline(runes[start:end]); idx > 0 {
			cut = start + idx + 1
		}
		out = append(out, string(runes[start:cut]))
		start = cut
	}
	return out
}

// lastNewline returns the index of the last '\n' in r, or -1.
func lastNewline(r []rune) int {
	for i := len(r) - 1; i >= 0; i-- {
		if r[i] == '\n' {
			return i
		}
	}
	return -1
}

// channelByName resolves the tracking channel by name. A miss (nil
// interface) is reachable, not hypothetical: StartAll ignores a config
// envelope naming a channel no compiled-in plugin provides, but the
// dispatch pipeline accepts that envelope's inbound and keys the delivery
// target by the envelope name. Both delivery branches therefore guard the
// nil target and drop with a warning instead of panicking on it.
func (h *Host) channelByName(name string) plugin.Channel {
	for _, ch := range h.deps.Channels {
		if ch != nil && ch.Name() == name {
			return ch
		}
	}
	return nil
}

// newChannelInboundID mints chanin_<16hex> from crypto/rand; identity
// grade randomness, same discipline as run ids.
func newChannelInboundID() (domain.RunID, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand unavailable: %w", err)
	}
	return domain.RunID("chanin_" + hex.EncodeToString(b)), nil
}
