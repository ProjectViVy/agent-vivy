package tui

import (
	"encoding/json"

	"agent-vivy/sdk/tui/stream"
)

const (
	roleUser      = "user"
	roleAssistant = "assistant"
)

const (
	decisionApproved = "approved"
	decisionDenied   = "denied"
)

type streamEvent struct {
	RunID   string
	Seq     int
	Type    string
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
		Type:    event.Type,
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
