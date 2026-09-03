package tui

import (
	"encoding/json"

	"agent-vivy/internal/domain"
	"agent-vivy/sdk/tui/stream"
)

// streamEvent is the kernel transport adapter. The protocol-independent
// event/notice/reducer implementation lives in sdk/tui/stream.
type streamEvent struct {
	RunID   string
	Seq     int
	Type    domain.EventType
	Payload json.RawMessage
}

type eventNotice = stream.Notice
type gatePrompt = stream.GatePrompt

func decodeStreamEvent(params json.RawMessage) (streamEvent, bool) {
	event, ok := stream.Decode(params)
	if !ok {
		return streamEvent{}, false
	}
	return streamEvent{
		RunID:   event.RunID,
		Seq:     event.Seq,
		Type:    domain.EventType(event.Type),
		Payload: event.Payload,
	}, true
}

func interpret(event streamEvent) eventNotice {
	return stream.Interpret(stream.Event{
		RunID:   event.RunID,
		Seq:     event.Seq,
		Type:    string(event.Type),
		Payload: event.Payload,
	})
}

func payloadString(raw json.RawMessage, key string) string {
	return stream.PayloadString(raw, key)
}
