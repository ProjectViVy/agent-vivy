// Package compaction provides context compression, tool-result pruning,
// and conversation summarization for Vivy. It is designed to keep the
// model's context window within safe bounds while preserving essential
// information.
package compaction

import (
	"agent-vivy/internal/domain"
)

// ContextBreakdown describes how much of the model's context budget each
// layer consumes. All sizes are in bytes; callers convert to tokens using
// an approximate ratio or a provider-specific tokenizer.
type ContextBreakdown struct {
	// SystemPrompt is the byte size of the system instruction, including
	// any notebook digest injected by the preamble composer.
	SystemPrompt int
	// HistoryMessages is the byte size of all retained user/assistant/tool
	// messages from prior turns.
	HistoryMessages int
	// ToolResults is the byte size specifically attributable to tool result
	// content within history messages.
	ToolResults int
	// CurrentRequest is the byte size of the new user message about to be
	// sent.
	CurrentRequest int
	// Total is the sum of all layers above.
	Total int
	// ModelLimit is the context window capacity in tokens. Zero means
	// unknown; callers use conservative defaults.
	ModelLimit int
	// EstimatedTokens is an approximate token count derived from Total
	// using a simple byte-to-token heuristic.
	EstimatedTokens int
	// UsagePercent is the percentage of ModelLimit consumed by
	// EstimatedTokens. When ModelLimit is zero, this is also zero.
	UsagePercent float64
}

// Measure computes a ContextBreakdown for one run's assembled context. The
// caller provides the raw message list that will be sent to the engine,
// plus the model's context window limit. This function is pure: it does not
// mutate its inputs or depend on external state.
func Measure(messages []*domain.Message, systemPrompt string, modelLimitTokens int) ContextBreakdown {
	systemBytes := len(systemPrompt)
	historyBytes := 0
	toolResultBytes := 0
	currentBytes := 0

	if len(messages) == 0 {
		estTokens := estimateTokens(systemBytes)
		usagePct := 0.0
		if modelLimitTokens > 0 {
			usagePct = float64(estTokens) / float64(modelLimitTokens) * 100.0
		}
		return ContextBreakdown{
			SystemPrompt:    systemBytes,
			Total:           systemBytes,
			ModelLimit:      modelLimitTokens,
			EstimatedTokens: estTokens,
			UsagePercent:    usagePct,
		}
	}

	// The last message is always the current request; everything before it
	// is history.
	lastIdx := len(messages) - 1
	for _, msg := range messages[:lastIdx] {
		msgCost := messageCost(msg)
		historyBytes += msgCost
		if msg.Role == domain.RoleTool {
			toolResultBytes += msgCost
		}
	}
	if lastIdx >= 0 {
		currentBytes = messageCost(messages[lastIdx])
	}

	total := systemBytes + historyBytes + currentBytes
	estTokens := estimateTokens(total)
	usagePct := 0.0
	if modelLimitTokens > 0 {
		usagePct = float64(estTokens) / float64(modelLimitTokens) * 100.0
	}

	return ContextBreakdown{
		SystemPrompt:    systemBytes,
		HistoryMessages: historyBytes,
		ToolResults:     toolResultBytes,
		CurrentRequest:  currentBytes,
		Total:           total,
		ModelLimit:      modelLimitTokens,
		EstimatedTokens: estTokens,
		UsagePercent:    usagePct,
	}
}

// messageCost estimates the byte footprint of one message, including role
// and envelope overhead. This mirrors the logic in runtime/context.go but
// adds tool-argument accounting.
func messageCost(msg *domain.Message) int {
	const overhead = 16 // approximate role/envelope cost
	cost := len(msg.Content) + overhead
	for _, file := range msg.FileContexts {
		cost += len(file.Path) + len(file.Name) + len(file.Content) + overhead
	}
	if msg.ToolCallID != "" {
		cost += len(msg.ToolCallID) + len(msg.ToolName)
	}
	if len(msg.ToolArgs) > 0 {
		cost += len(msg.ToolArgs)
	}
	return cost
}

// estimateTokens converts bytes to an approximate token count using a
// conservative heuristic: 1 token ≈ 4 bytes of UTF-8 text. This is less
// accurate than a real tokenizer but sufficient for pressure detection
// without external dependencies.
func estimateTokens(bytes int) int {
	if bytes <= 0 {
		return 0
	}
	return bytes / 4
}

// IsOverBudget reports whether the estimated token usage exceeds the given
// fraction of the model's context window. For example, thresholdRatio=0.8
// returns true when usage exceeds 80% of the window. When ModelLimit is
// zero (unknown), this always returns false.
func (b ContextBreakdown) IsOverBudget(thresholdRatio float64) bool {
	if b.ModelLimit <= 0 || thresholdRatio <= 0 {
		return false
	}
	return float64(b.EstimatedTokens) > float64(b.ModelLimit)*thresholdRatio
}

// Headroom returns the remaining token budget. Negative values mean the
// context is already over the limit.
func (b ContextBreakdown) Headroom() int {
	return b.ModelLimit - b.EstimatedTokens
}
