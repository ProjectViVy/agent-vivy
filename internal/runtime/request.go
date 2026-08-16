package runtime

import (
	"github.com/cloudwego/eino/schema"
)

func digestModelRequest(msgs []*schema.Message, selectedTools []string) payloadModelRequest {
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
		row := payloadModelRequestMessage{
			Role:          string(msg.Role),
			ContentSHA256: sha256Hex([]byte(msg.Content)),
			ByteLen:       len(msg.Content),
		}
		if msg.Role == schema.Tool {
			row.ToolCallID = msg.ToolCallID
		}
		if len(msg.ToolCalls) == 1 {
			row.ToolName = msg.ToolCalls[0].Function.Name
			row.ToolCallID = msg.ToolCalls[0].ID
		} else if len(msg.ToolCalls) > 1 {
			row.ToolName = msg.ToolCalls[0].Function.Name
		}
		if i == 0 && msg.Role == schema.System {
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
