package runtime

import (
	"github.com/cloudwego/eino/schema"
)

func describeModelRequest(msgs []*schema.AgenticMessage, selectedTools []string) payloadModelRequest {
	return payloadRequestFromShape(describeModelShape(msgs), selectedTools)
}

// payloadRequestFromShape projects one shape traversal onto the v1-v5 message
// rows shared by every request envelope.
func payloadRequestFromShape(shape modelShape, selectedTools []string) payloadModelRequest {
	if selectedTools == nil {
		selectedTools = []string{}
	}
	out := payloadModelRequest{
		SelectedTools: append([]string{}, selectedTools...),
		PreambleBytes: shape.PreambleBytes,
		Messages:      make([]payloadModelRequestMessage, 0, len(shape.Messages)),
	}
	for _, row := range shape.Messages {
		out.Messages = append(out.Messages, payloadModelRequestMessage{
			Role:       row.Role,
			ToolName:   row.ToolName,
			ToolCallID: row.ToolCallID,
			ByteLen:    row.ByteLen,
			Blocks:     row.Blocks,
		})
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
