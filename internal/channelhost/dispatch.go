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

	// HITL command surface (contract §12): an exact /approve, /deny, or
	// /pending token is answered in place — journaled like any inbound
	// message, but it opens no run, tracks no target, and records no
	// delivery intent. Non-command text (including other "/" tokens) falls
	// through to the ordinary turn path untouched.
	if cmd, arg, ok := parseApprovalCommand(text); ok {
		sessionID, err := h.EnsureSession(ctx, msg.Channel, msg.ChatID, msg.TopicID)
		if err != nil {
			return fmt.Errorf("channelhost: ensure session: %w", err)
		}
		if err := h.journalInbound(ctx, msg, sessionID); err != nil {
			return fmt.Errorf("channelhost: journal channel.inbound: %w", err)
		}
		ch := h.channelByName(msg.Channel)
		if ch == nil {
			h.logger.Warn("channelhost: dropping approval command for unregistered channel",
				"channel", msg.Channel, "chat_id", msg.ChatID)
			return nil
		}
		reply := channelReply{
			ch: ch, chatID: msg.ChatID, topicID: msg.TopicID,
			sessionID: sessionID, senderID: msg.Sender, maxRunes: runesLimit(ch),
		}
		h.handleApprovalCommand(ctx, reply, cmd, arg)
		return nil
	}

	sessionID, err := h.EnsureSession(ctx, msg.Channel, msg.ChatID, msg.TopicID)
	if err != nil {
		return fmt.Errorf("channelhost: ensure session: %w", err)
	}
	if err := h.journalInbound(ctx, msg, sessionID); err != nil {
		return fmt.Errorf("channelhost: journal channel.inbound: %w", err)
	}

	runID, err := h.deps.Run(ctx, sessionID, text, &domain.Provenance{
		Source:           domain.SourceChannel,
		Channel:          msg.Channel,
		ChatID:           msg.ChatID,
		ChannelMessageID: msg.MessageID,
	})
	if err != nil {
		return fmt.Errorf("channelhost: start run for channel %s: %w", msg.Channel, err)
	}
	ch := h.channelByName(msg.Channel)
	target := outboundTarget{
		runID:       runID,
		sessionID:   sessionID,
		chatID:      msg.ChatID,
		topicID:     msg.TopicID,
		ch:          ch,
		maxRunes:    runesLimit(ch),
		createdAtMs: time.Now().UnixMilli(),
	}
	// The durable reply intent (CH-C3-N1): from this point a restart can
	// finish or settle the delivery. A persistence failure never stops the
	// in-process delivery — it only degrades this one reply back to the
	// pre-hardening best effort, with the error in the log.
	if err := h.deps.Deliveries.UpsertChannelDelivery(ctx, storage.ChannelDelivery{
		RunID: runID, SessionID: sessionID, Channel: msg.Channel,
		ChatID: msg.ChatID, TopicID: msg.TopicID,
		State:       storage.ChannelDeliveryArmed,
		CreatedAtMs: target.createdAtMs, UpdatedAtMs: target.createdAtMs,
	}); err != nil {
		h.logger.Error("channelhost: durable delivery intent recording failed",
			"run", string(runID), "channel", msg.Channel, "err", err)
	}
	h.mu.Lock()
	h.targets[runID] = target
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
// structurally). Non-terminal interception: a channel run suspended for
// approval notifies its originating chat (contract §12) — the tracked
// target is read, never consumed, so the terminal still delivers or settles
// exactly one reply. Terminal handling: only tracked channel runs are acted
// on; on run.completed the durable intent flips to pending and the run's
// last assistant message is delivered back to the originating chat;
// run.failed / run.cancelled settle the intent (the journal already records
// why). Either way a terminal closes exactly one delivery.
func (h *Host) OnRunEvent(ctx context.Context, ev domain.RunEvent) {
	if ev.Type == domain.EventToolApprovalRequired {
		h.notifyApprovalRequired(ctx, ev)
		return
	}
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
		// through and no channel name to log. Settle the intent with a
		// warning — a method call on the nil interface would panic the
		// goroutine.
		h.logger.Warn("channelhost: dropping delivery for unregistered channel",
			"run", string(ev.RunID), "chat_id", target.chatID)
		h.settleDelivery(target)
		return
	}
	if ev.Type != domain.EventRunCompleted {
		h.logger.Info("channelhost: channel run ended without delivery",
			"run", string(ev.RunID), "type", string(ev.Type),
			"channel", target.ch.Name(), "chat_id", target.chatID)
		h.settleDelivery(target)
		return
	}
	// Delivery hops off the runtime goroutine: platform Send latency must
	// never stall event mapping. The durable intent flips to pending BEFORE
	// the goroutine spawns, so a crash after this point redelivers instead
	// of losing the reply.
	h.markPending(target, 0)
	h.enqueueDelivery(target, 0)
}

// splitRunes chunks content into pieces of at most limit runes. limit <= 0
// returns the content unchanged. Each chunk breaks at the last newline
// inside the window when one exists (keeps paragraph shapes readable);
// otherwise it hard-breaks. No empty chunk escapes.
//
// Fenced code blocks are kept renderable: when the chosen cut would land
// inside an open ``` fence, the chunk either extends to the block's real
// closing fence (when it fits) or ends with a synthesized ``` closer while
// the remainder reopens with the original fence line, so every delivered
// chunk stands alone. Content without an open fence at the cut splits
// exactly as before.
func splitRunes(content string, limit int) []string {
	runes := []rune(content)
	if limit <= 0 || len(runes) <= limit {
		return []string{content}
	}
	var out []string
	for start := 0; start < len(runes); {
		end := start + limit
		if end >= len(runes) {
			// The tail travels as-is; when the source itself never closed a
			// fence, the tail stays open the same way.
			out = append(out, string(runes[start:]))
			break
		}
		cut := end
		if idx := lastNewline(runes[start:end]); idx > 0 {
			cut = start + idx + 1
		}
		open := lastUnclosedFence(runes[start:cut])
		if open < 0 || limit < fenceMinLimit {
			out = append(out, string(runes[start:cut]))
			start = cut
			continue
		}
		opener := start + open
		// Prefer ending the chunk at the block's real closing fence.
		if closer := nextFence(runes, opener+len(fenceMarker)); closer >= 0 && closer+len(fenceMarker)-start <= limit {
			out = append(out, string(runes[start:closer+len(fenceMarker)]))
			start = closer + len(fenceMarker)
			continue
		}
		// The block does not fit in one chunk: close the fence here and
		// reopen the remainder with the original fence line (info string
		// included), so both halves render standalone.
		header := fenceHeader(runes, opener)
		bodyStart := opener + len(header)
		inner := start + limit - len(fenceMarker) - 1 // keep room for "\n```"
		if inner <= bodyStart {
			// No body inside the window: let the block travel whole to the
			// next chunk by ending just before its opener. When the opener
			// owns the chunk start there is nowhere before it — hard-cut
			// and reopen with a bare fence.
			if opener > start {
				out = append(out, string(runes[start:opener]))
				start = opener
				continue
			}
			out = append(out, string(runes[start:inner])+"\n"+fenceMarker)
			runes = append([]rune("```\n"), runes[inner:]...)
			start = 0
			continue
		}
		if idx := lastNewline(runes[bodyStart:inner]); idx >= 0 {
			inner = bodyStart + idx + 1
		}
		chunk := string(runes[start:inner])
		if !strings.HasSuffix(chunk, "\n") {
			chunk += "\n"
		}
		out = append(out, chunk+fenceMarker)
		runes = append(append([]rune{}, header...), runes[inner:]...)
		start = 0
	}
	return out
}

// fenceMarker is the fenced code block delimiter the splitter protects.
const fenceMarker = "```"

// fenceMinLimit is the smallest limit where fence-aware splitting stays
// sane: room for an opener line, some body, and a "\n```" closer. Below it
// the splitter falls back to plain cutting instead of mangling markers.
const fenceMinLimit = 16

// lastUnclosedFence returns the index of the last unmatched ``` opener in
// r, or -1 when the range leaves no fence open. The model is deliberately
// simple, matching the markdown most models emit: every ``` toggles fence
// state — info strings are not parsed and line anchoring is not required.
func lastUnclosedFence(r []rune) int {
	inFence := false
	last := -1
	for i := 0; i+2 < len(r); i++ {
		if r[i] == '`' && r[i+1] == '`' && r[i+2] == '`' {
			if !inFence {
				last = i
			}
			inFence = !inFence
			i += 2
		}
	}
	if !inFence {
		return -1
	}
	return last
}

// nextFence returns the index of the first ``` in r at or after from, or
// -1 when none follows.
func nextFence(r []rune, from int) int {
	for i := from; i+2 < len(r); i++ {
		if r[i] == '`' && r[i+1] == '`' && r[i+2] == '`' {
			return i
		}
	}
	return -1
}

// fenceHeader returns the fence line starting at opener (the ``` plus its
// info string) including the trailing newline; an opener line that never
// ends before EOF gets one synthesized so the reopened chunk renders.
func fenceHeader(r []rune, opener int) []rune {
	for i := opener; i < len(r); i++ {
		if r[i] == '\n' {
			return r[opener : i+1]
		}
	}
	return append(r[opener:], '\n')
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
