package channelhost

import (
	"context"
	"time"

	plugin "agent-vivy/sdk/port/channel"
)

// typingInterval is the resend cadence of the typing indicator. Telegram's
// chat action expires after ~5s, Discord's after ~8s, QQ's InputNotify
// after the declared input_second — one 4s cadence covers all three with
// margin, collapsing picoclaw's per-ear loops into the host. A var so
// lifecycle tests can shrink it.
var typingInterval = 4 * time.Second

// typingMaxDuration caps one typing loop: a run must never type forever
// even if its terminal event were lost (picoclaw's 5-minute cap). A var so
// lifecycle tests can shrink it.
var typingMaxDuration = 5 * time.Minute

// typingPingTimeout bounds one platform Typing call. The loop runs on its
// own goroutine, so a slow platform call stalls nothing — the bound keeps
// a wedged adapter from holding the goroutine past its usefulness.
const typingPingTimeout = 5 * time.Second

// typingFor resolves the live adapter's typing face through the capability
// seam. A channel without it simply never types.
func typingFor(ch plugin.Channel) plugin.Typing {
	target := capabilityTarget(ch)
	if target == nil {
		return nil
	}
	tp, _ := target.(plugin.Typing)
	return tp
}

// startTyping launches the live-surface typing loop for one accepted run
// (contract §7: typing never enters the Journal, adds no event types, has
// no config knob). The target's stop channel is closed by the terminal
// handler or StopAll; the 5-minute cap backstops both. A channel without a
// typing face starts nothing.
func (h *Host) startTyping(target outboundTarget) {
	tp := typingFor(target.ch)
	if tp == nil || target.stopTyping == nil {
		return
	}
	go h.typingLoop(target, tp)
}

// typingLoop sends one ping immediately and resends on the ticker until
// the stop channel closes or the cap fires. The first failure ends the
// loop: typing is best-effort, and hammering a degraded ear for the rest
// of the cap serves nobody.
func (h *Host) typingLoop(target outboundTarget, tp plugin.Typing) {
	timer := time.NewTimer(typingMaxDuration)
	defer timer.Stop()
	ticker := time.NewTicker(typingInterval)
	defer ticker.Stop()
	if !h.typingPing(target, tp) {
		return
	}
	for {
		select {
		case <-target.stopTyping:
			return
		case <-timer.C:
			return
		case <-ticker.C:
			if !h.typingPing(target, tp) {
				return
			}
		}
	}
}

// typingPing sends one indicator ping, reporting failures at debug. It
// never touches the Journal and is skipped once the stop channel has
// closed, so a terminal that races the first ping cannot outlive it.
func (h *Host) typingPing(target outboundTarget, tp plugin.Typing) bool {
	select {
	case <-target.stopTyping:
		return false
	default:
	}
	ctx, cancel := context.WithTimeout(context.Background(), typingPingTimeout)
	defer cancel()
	if err := tp.Typing(ctx, target.chatID); err != nil {
		h.logger.Debug("channelhost: typing ping failed; ending the indicator",
			"channel", target.ch.Name(), "chat_id", target.chatID, "err", err)
		return false
	}
	return true
}

// closeTyping ends a target's typing loop. mu must be held; the
// closed-channel check makes the terminal path and StopAll idempotent.
func closeTyping(target outboundTarget) {
	if target.stopTyping == nil {
		return
	}
	select {
	case <-target.stopTyping:
	default:
		close(target.stopTyping)
	}
}
