package runtime

// The agentic generation's message shape. Every runtime seam reads and builds
// schema.AgenticMessage directly (D2: one generation, no text-message bridge):
// assistant output and reasoning are content blocks, and a tool result is a
// FunctionToolResult block inside a user-role message, which is how Eino's
// typed ChatModelAgent hands tool output back to the model.
//
// These helpers are the single place that knows the block layout. The mapper,
// the context projection, the middlewares and the observers all go through
// them so a block-shape change cannot silently diverge across seams.

import (
	"encoding/json"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/agenticopenai"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
)

// agenticSystemMessage builds the system preamble message.
func agenticSystemMessage(text string) *schema.AgenticMessage {
	return schema.SystemAgenticMessage(text)
}

// agenticUserMessage builds a plain text user message.
func agenticUserMessage(text string) *schema.AgenticMessage {
	return schema.UserAgenticMessage(text)
}

// agenticUserBlocks builds a user message from explicit content blocks.
func agenticUserBlocks(blocks []*schema.ContentBlock) *schema.AgenticMessage {
	return &schema.AgenticMessage{Role: schema.AgenticRoleTypeUser, ContentBlocks: blocks}
}

// agenticTextBlock wraps text as a user-input text block.
func agenticTextBlock(text string) *schema.ContentBlock {
	return schema.NewContentBlock(&schema.UserInputText{Text: text})
}

// agenticAssistantTextMessage builds a final assistant answer message.
func agenticAssistantTextMessage(text string) *schema.AgenticMessage {
	return &schema.AgenticMessage{
		Role:          schema.AgenticRoleTypeAssistant,
		ContentBlocks: []*schema.ContentBlock{schema.NewContentBlock(&schema.AssistantGenText{Text: text})},
	}
}

// agenticAssistantToolCallMessage builds an assistant turn that requests tool
// calls.
func agenticAssistantToolCallMessage(calls []*schema.FunctionToolCall) *schema.AgenticMessage {
	blocks := make([]*schema.ContentBlock, 0, len(calls))
	for _, call := range calls {
		if call == nil {
			continue
		}
		copy := *call
		blocks = append(blocks, schema.NewContentBlock(&copy))
	}
	return &schema.AgenticMessage{Role: schema.AgenticRoleTypeAssistant, ContentBlocks: blocks}
}

// agenticToolResultMessage builds the user-role message that carries one tool
// result back to the model.
func agenticToolResultMessage(callID, name, text string) *schema.AgenticMessage {
	return agenticUserBlocks([]*schema.ContentBlock{
		schema.NewContentBlock(&schema.FunctionToolResult{
			CallID: callID,
			Name:   name,
			Content: []*schema.FunctionToolResultContentBlock{
				{Type: schema.FunctionToolResultContentBlockTypeText, Text: &schema.UserInputText{Text: text}},
			},
		}),
	})
}

// agenticMessageText concatenates every text-bearing block of one message:
// user input text, assistant generated text and function tool result text.
// It is the byte/cost/report projection, never the wire projection.
func agenticMessageText(msg *schema.AgenticMessage) string {
	if msg == nil {
		return ""
	}
	var out strings.Builder
	for _, block := range msg.ContentBlocks {
		if block == nil {
			continue
		}
		switch {
		case block.UserInputText != nil:
			out.WriteString(block.UserInputText.Text)
		case block.AssistantGenText != nil:
			out.WriteString(block.AssistantGenText.Text)
		case block.FunctionToolResult != nil:
			out.WriteString(agenticToolResultText(block.FunctionToolResult))
		}
	}
	return out.String()
}

// agenticAssistantText concatenates only the assistant-generated text blocks.
func agenticAssistantText(msg *schema.AgenticMessage) string {
	if msg == nil {
		return ""
	}
	var out strings.Builder
	for _, block := range msg.ContentBlocks {
		if block == nil {
			continue
		}
		if block.AssistantGenText != nil {
			out.WriteString(block.AssistantGenText.Text)
		}
	}
	return out.String()
}

// agenticReasoningText concatenates the reasoning blocks, in order. Reasoning
// now arrives as a first-class block type; the mapper projects it onto the
// existing model.reasoning_delta domain event.
func agenticReasoningText(msg *schema.AgenticMessage) string {
	if msg == nil {
		return ""
	}
	var out strings.Builder
	for _, block := range msg.ContentBlocks {
		if block == nil {
			continue
		}
		if block.Reasoning != nil {
			out.WriteString(block.Reasoning.Text)
		}
	}
	return out.String()
}

// agenticToolCalls returns the function tool calls of one assistant message.
func agenticToolCalls(msg *schema.AgenticMessage) []*schema.FunctionToolCall {
	if msg == nil {
		return nil
	}
	var calls []*schema.FunctionToolCall
	for _, block := range msg.ContentBlocks {
		if block == nil {
			continue
		}
		if block.FunctionToolCall != nil {
			calls = append(calls, block.FunctionToolCall)
		}
	}
	return calls
}

// agenticToolResults returns the function tool results of one message.
func agenticToolResults(msg *schema.AgenticMessage) []*schema.FunctionToolResult {
	if msg == nil {
		return nil
	}
	var results []*schema.FunctionToolResult
	for _, block := range msg.ContentBlocks {
		if block == nil {
			continue
		}
		if block.FunctionToolResult != nil {
			results = append(results, block.FunctionToolResult)
		}
	}
	return results
}

// agenticIsToolResult reports whether the message carries a tool result, which
// is the agentic generation's replacement for the text generation's Tool role.
func agenticIsToolResult(msg *schema.AgenticMessage) bool {
	return len(agenticToolResults(msg)) > 0
}

// agenticUsage returns the response token usage, if the provider reported one.
func agenticUsage(msg *schema.AgenticMessage) *schema.TokenUsage {
	if msg == nil || msg.ResponseMeta == nil {
		return nil
	}
	return msg.ResponseMeta.TokenUsage
}

// agenticFinishReason normalizes the provider stop signal the pinned agentic
// components expose. There is no unified finish reason on AgenticResponseMeta,
// so the projection reads the per-protocol signal: the chat-completions
// component carries its own response-meta extension, Anthropic exposes
// max_tokens, and the responses component exposes an incomplete reason. An
// empty result means the provider exposed no length signal.
func agenticFinishReason(msg *schema.AgenticMessage) string {
	if msg == nil || msg.ResponseMeta == nil {
		return ""
	}
	if ext := msg.ResponseMeta.ClaudeExtension; ext != nil && ext.StopReason == "max_tokens" {
		return "length"
	}
	if ext := msg.ResponseMeta.OpenAIExtension; ext != nil && ext.IncompleteDetails != nil {
		switch ext.IncompleteDetails.Reason {
		case "max_output_tokens", "length":
			return "length"
		}
	}
	if ext, ok := msg.ResponseMeta.Extension.(*agenticopenai.ChatResponseMetaExtension); ok && ext != nil {
		if ext.FinishReason == "length" || ext.FinishReason == "max_tokens" {
			return "length"
		}
	}
	return ""
}

// agenticToolResultText concatenates the text blocks of one tool result.
func agenticToolResultText(result *schema.FunctionToolResult) string {
	if result == nil {
		return ""
	}
	var out strings.Builder
	for _, block := range result.Content {
		if block == nil || block.Text == nil {
			continue
		}
		out.WriteString(block.Text.Text)
	}
	return out.String()
}

// agenticToolResultParts renders the non-text blocks of one tool result as
// JSON parts, mirroring the previous generation's tool-result part projection.
func agenticToolResultParts(result *schema.FunctionToolResult) []json.RawMessage {
	if result == nil || len(result.Content) == 0 {
		return nil
	}
	var out []json.RawMessage
	for _, block := range result.Content {
		if block == nil || block.Type == schema.FunctionToolResultContentBlockTypeText {
			continue
		}
		if data, err := json.Marshal(block); err == nil {
			out = append(out, data)
		}
	}
	return out
}

// agenticRole projects one agentic message onto the three-role domain
// vocabulary the Journal uses. A message carrying tool results is a tool
// message; system and user keep their roles, and everything else collapses to
// assistant because the domain vocabulary intentionally stays at three roles.
func agenticRole(msg *schema.AgenticMessage) domain.Role {
	if msg == nil {
		return domain.RoleAssistant
	}
	switch msg.Role {
	case schema.AgenticRoleTypeUser:
		if agenticIsToolResult(msg) {
			return domain.RoleTool
		}
		return domain.RoleUser
	default:
		return domain.RoleAssistant
	}
}

// agenticRoleName is the wire-facing role label used by request digests.
func agenticRoleName(msg *schema.AgenticMessage) string {
	if msg == nil {
		return ""
	}
	if agenticIsToolResult(msg) {
		return "tool"
	}
	return string(msg.Role)
}
