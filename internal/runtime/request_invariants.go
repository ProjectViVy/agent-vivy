package runtime

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// Pre-send invariants. These reject only shapes we can prove the provider
// cannot accept: a call/result pairing the OpenAI wire cannot express, or an
// empty user/assistant turn. The rules stay narrow on purpose — everything
// else is the provider's call to make, and refusing a legitimate feed would
// turn a model-correctable input into a run failure.

// errModelRequestInvalid marks a call refused before it reached the provider.
var errModelRequestInvalid = errors.New("runtime: model request violates the provider wire contract")

func validateModelInput(msgs []*schema.AgenticMessage) error {
	seenCalls := map[string]struct{}{}
	// pending marks each outstanding call id and whether a non-result user
	// message separated it from its result. A provider may reuse an answered
	// id on a later turn, so the entry is recreated after its result; an id
	// may not be reused while its first occurrence is still unanswered.
	pending := map[string]bool{}
	answered := map[string]struct{}{}
	for i, msg := range msgs {
		if msg == nil {
			return fmt.Errorf("message %d is nil", i)
		}
		calls := agenticToolCalls(msg)
		results := agenticToolResults(msg)
		switch msg.Role {
		case schema.AgenticRoleTypeSystem:
			if len(calls) > 0 || len(results) > 0 {
				return fmt.Errorf("message %d: system message carries tool blocks", i)
			}
		case schema.AgenticRoleTypeAssistant:
			if len(results) > 0 {
				return fmt.Errorf("message %d: assistant message carries a tool result", i)
			}
			if len(calls) == 0 && !messageHasWireContent(msg) {
				return fmt.Errorf("message %d: empty assistant message", i)
			}
			for _, call := range calls {
				if call.CallID == "" {
					return fmt.Errorf("message %d: tool call without a call id", i)
				}
				if call.Name == "" {
					return fmt.Errorf("message %d: tool call without a tool name", i)
				}
				if _, outstanding := pending[call.CallID]; outstanding {
					return fmt.Errorf("message %d: tool call id is still pending from an earlier call", i)
				}
				seenCalls[call.CallID] = struct{}{}
				pending[call.CallID] = false
			}
		case schema.AgenticRoleTypeUser:
			if len(results) > 0 {
				if messageHasNonResultBlocks(msg) {
					return fmt.Errorf("message %d: tool results mixed with non-result user blocks", i)
				}
				for _, result := range results {
					if result.CallID == "" {
						return fmt.Errorf("message %d: tool result without a call id", i)
					}
					if _, ok := seenCalls[result.CallID]; !ok {
						return fmt.Errorf("message %d: tool result answers a call that does not precede it", i)
					}
					separated, outstanding := pending[result.CallID]
					if !outstanding {
						if _, twice := answered[result.CallID]; twice {
							return fmt.Errorf("message %d: tool result answers the same call twice", i)
						}
						return fmt.Errorf("message %d: tool result answers a call that is not pending", i)
					}
					if separated {
						return fmt.Errorf("message %d: tool result is separated from its call by a user message", i)
					}
					delete(pending, result.CallID)
					answered[result.CallID] = struct{}{}
				}
				continue
			}
			for id := range pending {
				pending[id] = true
			}
			if !messageHasWireContent(msg) {
				return fmt.Errorf("message %d: empty user message", i)
			}
		default:
			return fmt.Errorf("message %d: unsupported role %q", i, msg.Role)
		}
	}
	return nil
}

// messageHasWireContent reports whether a message would serialize at least
// one non-empty wire part. Reasoning is deliberately excluded: an
// assistant turn carrying only reasoning has empty wire content.
func messageHasWireContent(msg *schema.AgenticMessage) bool {
	if msg == nil {
		return false
	}
	for _, block := range msg.ContentBlocks {
		if block == nil {
			continue
		}
		switch {
		case block.UserInputText != nil:
			if block.UserInputText.Text != "" {
				return true
			}
		case block.AssistantGenText != nil:
			if block.AssistantGenText.Text != "" {
				return true
			}
		case block.UserInputImage != nil, block.UserInputAudio != nil, block.UserInputVideo != nil, block.UserInputFile != nil,
			block.AssistantGenImage != nil, block.AssistantGenAudio != nil, block.AssistantGenVideo != nil,
			block.FunctionToolCall != nil, block.FunctionToolResult != nil,
			block.ToolSearchFunctionToolResult != nil,
			block.ServerToolCall != nil, block.ServerToolResult != nil,
			block.MCPToolCall != nil, block.MCPToolResult != nil, block.MCPListToolsResult != nil,
			block.MCPToolApprovalRequest != nil, block.MCPToolApprovalResponse != nil:
			return true
		}
	}
	return false
}

func messageHasNonResultBlocks(msg *schema.AgenticMessage) bool {
	if msg == nil {
		return false
	}
	for _, block := range msg.ContentBlocks {
		if block == nil {
			continue
		}
		if block.FunctionToolResult == nil {
			return true
		}
	}
	return false
}

// modelRequestFingerprint is a content-free structural identity:
// role, ordered block kinds, ordered call ids and ordered result ids per
// message. Two equal fingerprints mean the retry replayed the same shape.
func modelRequestFingerprint(msgs []*schema.AgenticMessage) string {
	var out strings.Builder
	for i, msg := range msgs {
		if i > 0 {
			out.WriteByte('|')
		}
		if msg == nil {
			out.WriteString("nil")
			continue
		}
		fmt.Fprintf(&out, "%s:%d", agenticRoleName(msg), len(msg.ContentBlocks))
		for _, block := range msg.ContentBlocks {
			out.WriteByte(':')
			if block == nil {
				out.WriteString("nil")
				continue
			}
			out.WriteString(contentBlockKind(block))
		}
		for _, call := range agenticToolCalls(msg) {
			out.WriteString("{call:")
			out.WriteString(call.CallID)
			out.WriteByte('}')
		}
		for _, result := range agenticToolResults(msg) {
			out.WriteString("{result:")
			out.WriteString(result.CallID)
			out.WriteByte('}')
		}
	}
	return out.String()
}

// hasToolTurns reports whether the input carries a tool result — the only
// evidence that a "tool result does not follow tool call" rejection could
// describe this payload at all.
func hasToolTurns(msgs []*schema.AgenticMessage) bool {
	for _, msg := range msgs {
		if agenticIsToolResult(msg) {
			return true
		}
	}
	return false
}

func clampFingerprint(value string) string {
	return truncateText(value, 256)
}

// truncateText bounds an internal diagnostic string without the content
// clamping warning used for payload bodies.
func truncateText(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return strings.ToValidUTF8(value[:limit], "") + "~"
}
