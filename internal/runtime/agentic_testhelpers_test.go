package runtime

// Shared builders for the runtime test suites. The product seams speak
// schema.AgenticMessage directly, so tests build scripts and expectations
// through these helpers instead of the text-message constructors.

import (
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino/schema/claude"
)

// testUser builds a plain user message.
func testUser(text string) *schema.AgenticMessage { return agenticUserMessage(text) }

// testSystem builds a system message.
func testSystem(text string) *schema.AgenticMessage { return agenticSystemMessage(text) }

// testAssistant builds a plain assistant answer.
func testAssistant(text string) *schema.AgenticMessage { return agenticAssistantTextMessage(text) }

// testCall builds one function tool call.
func testCall(callID, name, args string) *schema.FunctionToolCall {
	return &schema.FunctionToolCall{CallID: callID, Name: name, Arguments: args}
}

// testCalls collects call values for the general builder.
func testCalls(calls ...*schema.FunctionToolCall) []*schema.FunctionToolCall {
	return calls
}

// testToolCalls builds an assistant tool-call turn.
func testToolCalls(calls ...*schema.FunctionToolCall) *schema.AgenticMessage {
	return agenticAssistantToolCallMessage(calls)
}

// testToolResult builds the user-role tool result message for one call.
func testToolResult(callID, name, text string) *schema.AgenticMessage {
	return agenticToolResultMessage(callID, name, text)
}

// agenticTestMessage builds an arbitrary agentic message from the fields the
// text-generation literals used to carry: role, text, tool calls, token usage
// and extra metadata.
func agenticTestMessage(role schema.AgenticRoleType, text string, calls []*schema.FunctionToolCall, usage *schema.TokenUsage, extra map[string]any) *schema.AgenticMessage {
	msg := &schema.AgenticMessage{Role: role, Extra: extra}
	if text != "" {
		var block *schema.ContentBlock
		if role == schema.AgenticRoleTypeAssistant {
			block = schema.NewContentBlock(&schema.AssistantGenText{Text: text})
		} else {
			block = schema.NewContentBlock(&schema.UserInputText{Text: text})
		}
		msg.ContentBlocks = append(msg.ContentBlocks, block)
	}
	for _, call := range calls {
		if call == nil {
			continue
		}
		copy := *call
		msg.ContentBlocks = append(msg.ContentBlocks, schema.NewContentBlock(&copy))
	}
	if usage != nil {
		msg.ResponseMeta = &schema.AgenticResponseMeta{TokenUsage: usage}
	}
	return msg
}

// testLengthStop builds an assistant message whose provider stop signal is the
// output-length limit (Anthropic max_tokens), optionally carrying partial text.
// The text generation carried this as ResponseMeta.FinishReason; the agentic
// generation exposes it per protocol.
func testLengthStop(text string) *schema.AgenticMessage {
	msg := agenticTestMessage(schema.AgenticRoleTypeAssistant, text, nil, nil, nil)
	msg.ResponseMeta = &schema.AgenticResponseMeta{
		ClaudeExtension: &claude.ResponseMetaExtension{StopReason: "max_tokens"},
	}
	return msg
}

// testReasoning builds a message carrying only a reasoning block, the agentic
// generation's replacement for the text generation's ReasoningContent field.
func testReasoning(text string) *schema.AgenticMessage {
	return &schema.AgenticMessage{
		Role:          schema.AgenticRoleTypeAssistant,
		ContentBlocks: []*schema.ContentBlock{schema.NewContentBlock(&schema.Reasoning{Text: text})},
	}
}

// testReasoningChunk builds one streaming reasoning chunk at block index.
func testReasoningChunk(text string, index int) *schema.AgenticMessage {
	return &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeAssistant,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlockChunk(&schema.Reasoning{Text: text}, &schema.StreamingMeta{Index: index}),
		},
	}
}

// testToolCallChunk builds one streaming tool-call fragment at block index.
func testToolCallChunk(callID, name, args string, index int) *schema.AgenticMessage {
	return &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeAssistant,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlockChunk(
				&schema.FunctionToolCall{CallID: callID, Name: name, Arguments: args},
				&schema.StreamingMeta{Index: index},
			),
		},
	}
}
