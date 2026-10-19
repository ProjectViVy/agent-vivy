package runtime

import (
	"github.com/cloudwego/eino/schema"
)

func digestModelRequest(msgs []*schema.AgenticMessage, selectedTools []string) payloadModelRequest {
	if selectedTools == nil {
		selectedTools = []string{}
	}
	out := payloadModelRequest{
		SelectedTools: append([]string(nil), selectedTools...),
		Messages:      make([]payloadModelRequestMessage, 0, len(msgs)),
	}
	for i, msg := range msgs {
		if msg == nil {
			continue
		}
		content := modelRequestText(msg)
		row := payloadModelRequestMessage{
			Role:          agenticRoleName(msg),
			ContentSHA256: sha256Hex(content),
			ByteLen:       len(content),
		}
		if results := agenticToolResults(msg); len(results) > 0 {
			row.ToolCallID = results[0].CallID
		}
		if calls := agenticToolCalls(msg); len(calls) > 0 {
			row.ToolName = calls[0].Name
			row.ToolCallID = calls[0].CallID
		}
		if i == 0 && msg.Role == schema.AgenticRoleTypeSystem {
			out.PreambleSHA256 = row.ContentSHA256
			out.PreambleBytes = row.ByteLen
		}
		out.Messages = append(out.Messages, row)
	}
	if out.PreambleSHA256 == "" {
		out.PreambleSHA256 = sha256Hex(nil)
	}
	return out
}

// modelRequestText is the byte identity one request row reports: every
// text-bearing block, so a preamble or a tool result keeps the same digest
// contract the text generation had.
func modelRequestText(msg *schema.AgenticMessage) []byte {
	if msg == nil {
		return nil
	}
	return []byte(agenticMessageText(msg))
}
