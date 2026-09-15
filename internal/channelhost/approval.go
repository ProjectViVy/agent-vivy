package channelhost

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-vivy/internal/domain"
	plugin "agent-vivy/sdk/port/channel"
)

// The HITL channel surface (contract §12): a run suspended for approval
// notifies the originating chat, and an allow-listed sender may list or
// decide that chat's pending approvals with slash commands. Red lines:
//   - the kernel owns every decision; the channel only forwards an
//     attributable decision through the same Service path as the local UI,
//   - the authorization boundary is allow_from (the same sender policy as
//     every inbound message) scoped to the originating session — a sender
//     can never decide another chat's approvals,
//   - notifications and replies are plain adapter Sends: they never touch
//     the delivery ledger or its attempt budget.

// approvalNotifyPayload is the subset of the tool.approval_required event
// payload the notification needs. The payload schema is kernel-owned; this
// local projection keeps the package from importing internal/runtime.
type approvalNotifyPayload struct {
	ApprovalID string `json:"approval_id"`
	ToolName   string `json:"tool_name"`
}

// approvalIDShortLen bounds the approval id echoed into notification and
// list texts so a chat can type it back. Prefix matching in the decide
// command accepts any unambiguous prefix.
const approvalIDShortLen = 12

// channelReply is the direct-send context of one chat: where a notification
// or a command reply lands. It carries no ledger row and no retry budget.
type channelReply struct {
	ch        plugin.Channel
	chatID    string
	topicID   string
	sessionID domain.SessionID
	senderID  string
	maxRunes  int
}

// notifyApprovalRequired sends one "pending approval" text to the chat a
// suspended channel run came from. It must never block or fail the event
// pipeline: the Send hops off the runtime goroutine behind the same
// draining gate as reply deliveries, and a failed notification is only a
// log line (the kernel's local HITL surface stays authoritative).
func (h *Host) notifyApprovalRequired(ctx context.Context, ev domain.RunEvent) {
	h.mu.Lock()
	target, tracked := h.targets[ev.RunID]
	h.mu.Unlock()
	if !tracked {
		// Not a channel run (local UI, headless, cron): nothing to notify.
		return
	}
	if target.ch == nil {
		return
	}
	var payload approvalNotifyPayload
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		h.logger.Warn("channelhost: approval notification skipped; undecodable payload",
			"run", string(ev.RunID), "err", err)
		return
	}
	short := shortApprovalID(payload.ApprovalID)
	text := fmt.Sprintf("⏸ 等待人工审批:%s(会话 %s)。回复 /approve %s 批准、/deny %s 拒绝;也可在本地客户端处理。",
		payload.ToolName, target.sessionID, short, short)
	reply := channelReply{
		ch: target.ch, chatID: target.chatID, topicID: target.topicID,
		sessionID: target.sessionID, maxRunes: target.maxRunes,
	}
	h.sendBoundedText(reply, text, "approval notification")
}

// sendBoundedText fires one direct adapter Send on a gated goroutine — the
// delivery-ledger path without a ledger row: the draining gate keeps StopAll
// race-free, the WaitGroup joins it, and the outbound timeout bounds it.
// Failure is logged, never retried and never surfaced to the pipeline.
func (h *Host) sendBoundedText(reply channelReply, text, what string) {
	h.mu.Lock()
	if h.draining {
		h.mu.Unlock()
		return
	}
	h.deliveryWG.Add(1)
	h.mu.Unlock()
	go func() {
		defer h.deliveryWG.Done()
		ctx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), outboundDeliveryTimeout)
		defer cancel()
		for _, chunk := range splitRunes(text, reply.maxRunes) {
			if _, err := reply.ch.Send(ctx, plugin.OutboundMessage{
				ChatID:  reply.chatID,
				TopicID: reply.topicID,
				Parts:   []plugin.Part{{Kind: plugin.PartText, Text: chunk}},
			}); err != nil {
				h.logger.Warn("channelhost: "+what+" failed",
					"channel", reply.ch.Name(), "chat_id", reply.chatID, "err", err)
				return
			}
		}
	}()
}

// approvalCommands are the exact inbound tokens the command surface answers.
// Everything else — including other "/" prefixed text — is an ordinary
// message and opens a run as usual.
const (
	cmdApprove = "/approve"
	cmdDeny    = "/deny"
	cmdPending = "/pending"
)

// shortApprovalID bounds an approval id to its leading slice.
func shortApprovalID(id string) string {
	if len(id) <= approvalIDShortLen {
		return id
	}
	return id[:approvalIDShortLen]
}

// parseApprovalCommand reports the command and its argument for an inbound
// text, or ok=false when the text is not one of the three exact commands.
func parseApprovalCommand(text string) (cmd string, arg string, ok bool) {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "", "", false
	}
	switch fields[0] {
	case cmdApprove, cmdDeny:
		arg := ""
		if len(fields) > 1 {
			arg = fields[1]
		}
		return fields[0], arg, true
	case cmdPending:
		return cmdPending, "", true
	default:
		return "", "", false
	}
}

// approvalFeatureEnabled reports whether the HITL channel surface is wired.
func (h *Host) approvalFeatureEnabled() bool {
	return h.deps.Approvals != nil && h.deps.Runs != nil && h.deps.DecideApproval != nil
}

// handleApprovalCommand serves one allow-listed slash command. The caller
// has already journaled the inbound message; this only decides the reply.
func (h *Host) handleApprovalCommand(ctx context.Context, reply channelReply, cmd, arg string) {
	if !h.approvalFeatureEnabled() {
		h.sendBoundedText(reply, "审批功能未启用。", "approval command reply")
		return
	}
	switch cmd {
	case cmdPending:
		h.replyPendingApprovals(ctx, reply)
	case cmdApprove:
		h.replyDecision(ctx, reply, arg, domain.ApprovalApproved)
	case cmdDeny:
		h.replyDecision(ctx, reply, arg, domain.ApprovalDenied)
	}
}

// sessionPendingApprovals lists the pending approvals that belong to the
// chat's session. A pending approval maps to its run's session through the
// durable run row; unreadable rows are skipped — the local UI remains the
// fallback for anything the channel surface cannot resolve.
func (h *Host) sessionPendingApprovals(ctx context.Context, reply channelReply) []domain.Approval {
	pending, err := h.deps.Approvals.ListPendingApprovals(ctx)
	if err != nil {
		h.logger.Warn("channelhost: list pending approvals failed",
			"chat_id", reply.chatID, "err", err)
		return nil
	}
	var out []domain.Approval
	for _, approval := range pending {
		run, err := h.deps.Runs.GetRun(ctx, approval.RunID)
		if err != nil {
			continue
		}
		if run.SessionID == reply.sessionID {
			out = append(out, approval)
		}
	}
	return out
}

// replyPendingApprovals answers /pending with this chat's session-scoped
// queue, soonest expiry first (the store's own order).
func (h *Host) replyPendingApprovals(ctx context.Context, reply channelReply) {
	pending := h.sessionPendingApprovals(ctx, reply)
	if len(pending) == 0 {
		h.sendBoundedText(reply, "没有待审批的事项。", "approval command reply")
		return
	}
	var b strings.Builder
	b.WriteString("待审批:")
	for _, approval := range pending {
		remaining := time.Until(time.UnixMilli(approval.ExpiresAt)).Round(time.Minute)
		fmt.Fprintf(&b, "\n• %s %s(约 %s 后过期)", shortApprovalID(approval.ID), approval.ToolName, remaining)
	}
	b.WriteString("\n回复 /approve <id> 或 /deny <id>。")
	h.sendBoundedText(reply, b.String(), "approval command reply")
}

// replyDecision answers /approve and /deny. Without an argument the most
// urgent pending approval of this session is decided; with an argument the
// prefix must match exactly one of them — a cross-chat approval is invisible
// here and can never be decided (the session is the authorization scope).
func (h *Host) replyDecision(ctx context.Context, reply channelReply, arg, decision string) {
	pending := h.sessionPendingApprovals(ctx, reply)
	if len(pending) == 0 {
		h.sendBoundedText(reply, "没有待审批的事项。", "approval command reply")
		return
	}
	var chosen *domain.Approval
	if arg == "" {
		chosen = &pending[0]
	} else {
		for i := range pending {
			if strings.HasPrefix(pending[i].ID, arg) {
				if chosen != nil {
					h.sendBoundedText(reply, "id 前缀不唯一,请多写几位。", "approval command reply")
					return
				}
				chosen = &pending[i]
			}
		}
		if chosen == nil {
			h.sendBoundedText(reply, "本会话没有匹配的待审批:"+arg, "approval command reply")
			return
		}
	}
	actor := "channel:" + reply.ch.Name() + ":" + reply.senderID
	if err := h.deps.DecideApproval(ctx, chosen.ID, decision, actor); err != nil {
		// First-writer-wins losses, expiry, and unknown ids all surface as
		// a bounded failure text; the journal owns the authoritative truth.
		h.logger.Warn("channelhost: channel approval decision failed",
			"approval", chosen.ID, "decision", decision, "chat_id", reply.chatID, "err", err)
		h.sendBoundedText(reply, "审批失败:"+err.Error(), "approval command reply")
		return
	}
	if decision == domain.ApprovalApproved {
		h.sendBoundedText(reply, fmt.Sprintf("✅ 已批准 %s(%s)。", chosen.ToolName, shortApprovalID(chosen.ID)), "approval command reply")
		return
	}
	h.sendBoundedText(reply, fmt.Sprintf("❌ 已拒绝 %s(%s)。", chosen.ToolName, shortApprovalID(chosen.ID)), "approval command reply")
}
