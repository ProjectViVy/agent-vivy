package provider

// Shared builders for the provider test suites. The agentic generation speaks
// schema.AgenticMessage, so tests build inputs and expectations through these
// helpers instead of the retired text-message constructors.

import (
	"github.com/cloudwego/eino/schema"
)

// testUser builds a plain user message.
func testUser(text string) *schema.AgenticMessage { return schema.UserAgenticMessage(text) }

// testSystem builds a system message.
func testSystem(text string) *schema.AgenticMessage { return schema.SystemAgenticMessage(text) }

// testAssistant builds a plain assistant answer.
func testAssistant(text string) *schema.AgenticMessage {
	return &schema.AgenticMessage{
		Role:          schema.AgenticRoleTypeAssistant,
		ContentBlocks: []*schema.ContentBlock{schema.NewContentBlock(&schema.AssistantGenText{Text: text})},
	}
}

// agenticMessageText concatenates the text-bearing blocks of one message.
func agenticMessageText(msg *schema.AgenticMessage) string {
	if msg == nil {
		return ""
	}
	var out string
	for _, block := range msg.ContentBlocks {
		if block == nil {
			continue
		}
		if block.AssistantGenText != nil {
			out += block.AssistantGenText.Text
		}
		if block.UserInputText != nil {
			out += block.UserInputText.Text
		}
	}
	return out
}

// agenticReasoningText concatenates the reasoning blocks of one message.
func agenticReasoningText(msg *schema.AgenticMessage) string {
	if msg == nil {
		return ""
	}
	var out string
	for _, block := range msg.ContentBlocks {
		if block == nil || block.Reasoning == nil {
			continue
		}
		out += block.Reasoning.Text
	}
	return out
}
