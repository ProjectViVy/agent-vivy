package runtime

import (
	"errors"
	"fmt"

	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
)

// ContextPolicy bounds the transient context sent to one model run. It is
// deliberately about the current session transcript only; it is not a
// long-term memory policy.
type ContextPolicy struct {
	// MaxBytes is an approximate UTF-8 byte budget for message content and
	// small role/envelope overhead. Zero means unbounded for direct runtime
	// tests; config.Default supplies a bounded production value.
	MaxBytes int
	// MaxHistoryMessages limits retained user/assistant history rows. Zero
	// means unbounded for direct runtime tests; the current user message is
	// always retained separately.
	MaxHistoryMessages int
}

// ContextStats describes how the policy shaped one run's context.
type ContextStats struct {
	OriginalHistoryMessages int
	IncludedHistoryMessages int
	DroppedHistoryMessages  int
	Bytes                   int
}

// ErrContextBudgetExceeded means the fixed preamble and current user request
// cannot fit within the configured budget. The service classifies it as a
// bounded, user-visible run failure rather than silently expanding context.
var ErrContextBudgetExceeded = errors.New("runtime: context budget exceeded")

const contextMessageOverhead = 16

// buildRunContext creates the exact message list sent to the engine. The
// preamble and current user request are mandatory; older transcript rows are
// retained from newest to oldest until the message and byte budgets are
// reached. Tool-call / tool-result pairs stay in the feed (ADR-010).
func buildRunContext(policy ContextPolicy, preamble string, stored []domain.Message, currentUserText string) ([]*schema.Message, ContextStats, error) {
	if policy.MaxBytes < 0 || policy.MaxHistoryMessages < 0 {
		return nil, ContextStats{}, fmt.Errorf("%w: policy values must not be negative", ErrContextBudgetExceeded)
	}

	transcript := feedableMessages(stored)
	// Run persists the current user message before drive starts. Keep the
	// helper correct for direct callers too, without duplicating that row.
	if len(transcript) == 0 || transcript[len(transcript)-1].Role != domain.RoleUser ||
		transcript[len(transcript)-1].Content != currentUserText {
		transcript = append(transcript, domain.Message{Role: domain.RoleUser, Content: currentUserText})
	}
	current := transcript[len(transcript)-1]
	if current.Role != domain.RoleUser {
		return nil, ContextStats{}, fmt.Errorf("%w: current message must be a user message", ErrContextBudgetExceeded)
	}
	history := transcript[:len(transcript)-1]

	stats := ContextStats{OriginalHistoryMessages: len(history)}
	baseBytes := messageCost(preamble, "system") + messageCost(current.Content, string(domain.RoleUser))
	if policy.MaxBytes > 0 && baseBytes > policy.MaxBytes {
		return nil, stats, fmt.Errorf("%w: preamble and current request require %d bytes; budget is %d", ErrContextBudgetExceeded, baseBytes, policy.MaxBytes)
	}

	maxHistory := len(history)
	if policy.MaxHistoryMessages > 0 && maxHistory > policy.MaxHistoryMessages {
		maxHistory = policy.MaxHistoryMessages
	}
	selected := make([]domain.Message, 0, maxHistory)
	usedBytes := baseBytes
	for i := len(history) - 1; i >= 0 && len(selected) < maxHistory; i-- {
		cost := messageCost(history[i].Content, string(history[i].Role)) + len(history[i].ToolArgs) + len(history[i].ToolCallID)
		if policy.MaxBytes > 0 && usedBytes+cost > policy.MaxBytes {
			break
		}
		selected = append(selected, history[i])
		usedBytes += cost
	}
	for left, right := 0, len(selected)-1; left < right; left, right = left+1, right-1 {
		selected[left], selected[right] = selected[right], selected[left]
	}

	selected = pairToolTurns(selected)
	stats.IncludedHistoryMessages = len(selected)
	stats.DroppedHistoryMessages = len(history) - stats.IncludedHistoryMessages
	stats.Bytes = usedBytes

	msgs := make([]*schema.Message, 0, len(selected)+2)
	msgs = append(msgs, schema.SystemMessage(preamble))
	msgs = append(msgs, projectFeed(selected)...)
	msgs = append(msgs, schema.UserMessage(current.Content))
	return msgs, stats, nil
}

func feedableMessages(stored []domain.Message) []domain.Message {
	out := make([]domain.Message, 0, len(stored))
	for _, msg := range stored {
		switch msg.Role {
		case domain.RoleUser, domain.RoleAssistant, domain.RoleTool:
			out = append(out, msg)
		}
	}
	return out
}

// pairToolTurns drops assistant tool-call rows without a matching tool
// result in the same window, and tool results without a matching call.
func pairToolTurns(in []domain.Message) []domain.Message {
	haveResult := make(map[string]bool)
	haveCall := make(map[string]bool)
	for _, msg := range in {
		if msg.ToolCallID == "" {
			continue
		}
		switch msg.Role {
		case domain.RoleTool:
			haveResult[msg.ToolCallID] = true
		case domain.RoleAssistant:
			haveCall[msg.ToolCallID] = true
		}
	}
	out := make([]domain.Message, 0, len(in))
	for _, msg := range in {
		if msg.ToolCallID == "" {
			out = append(out, msg)
			continue
		}
		if msg.Role == domain.RoleAssistant && !haveResult[msg.ToolCallID] {
			continue
		}
		if msg.Role == domain.RoleTool && !haveCall[msg.ToolCallID] {
			continue
		}
		out = append(out, msg)
	}
	return out
}

func projectFeed(msgs []domain.Message) []*schema.Message {
	out := make([]*schema.Message, 0, len(msgs))
	for i := 0; i < len(msgs); {
		msg := msgs[i]
		switch {
		case msg.Role == domain.RoleUser:
			out = append(out, schema.UserMessage(msg.Content))
			i++
		case msg.Role == domain.RoleTool:
			out = append(out, schema.ToolMessage(msg.Content, msg.ToolCallID))
			i++
		case msg.Role == domain.RoleAssistant && msg.ToolCallID != "":
			var calls []schema.ToolCall
			for i < len(msgs) && msgs[i].Role == domain.RoleAssistant && msgs[i].ToolCallID != "" {
				calls = append(calls, schema.ToolCall{
					ID: msgs[i].ToolCallID,
					Function: schema.FunctionCall{
						Name:      msgs[i].ToolName,
						Arguments: string(msgs[i].ToolArgs),
					},
				})
				i++
			}
			out = append(out, schema.AssistantMessage("", calls))
		default:
			out = append(out, schema.AssistantMessage(msg.Content, nil))
			i++
		}
	}
	return out
}

func messageCost(content, role string) int {
	return len(content) + len(role) + contextMessageOverhead
}
