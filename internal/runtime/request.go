package runtime

import (
	"github.com/cloudwego/eino/schema"
)

func describeModelRequest(msgs []*schema.AgenticMessage, selectedTools []string) payloadModelRequest {
	if selectedTools == nil {
		selectedTools = []string{}
	}
	out := payloadModelRequest{
		SelectedTools: append([]string{}, selectedTools...),
		Messages:      make([]payloadModelRequestMessage, 0, len(msgs)),
	}
	for i, msg := range msgs {
		if msg == nil {
			continue
		}
		content := modelRequestText(msg)
		row := payloadModelRequestMessage{
			Role:    agenticRoleName(msg),
			ByteLen: len(content),
		}
		if results := agenticToolResults(msg); len(results) > 0 {
			row.ToolCallID = results[0].CallID
		}
		if calls := agenticToolCalls(msg); len(calls) > 0 {
			row.ToolName = calls[0].Name
			row.ToolCallID = calls[0].CallID
		}
		if i == 0 && msg.Role == schema.AgenticRoleTypeSystem {
			out.PreambleBytes = row.ByteLen
		}
		out.Messages = append(out.Messages, row)
	}
	return out
}

// modelRequestText counts every text-bearing block without publishing its content.
func modelRequestText(msg *schema.AgenticMessage) []byte {
	if msg == nil {
		return nil
	}
	return []byte(agenticMessageText(msg))
}
