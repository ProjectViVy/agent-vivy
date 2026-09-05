package live

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
	SubscriptionID string
	RunID          string
	Seq            int
	Type           string
	PayloadVersion int
	Payload        json.RawMessage
}

type eventNotice = stream.Notice
type gatePrompt = stream.GatePrompt

func decodeStreamEvent(params json.RawMessage) (streamEvent, bool) {
	event, ok := stream.Decode(params)
	if !ok {
		return streamEvent{}, false
	}
	return streamEvent{
		SubscriptionID: event.SubscriptionID,
		RunID:          event.RunID,
		Seq:            event.Seq,
		Type:           event.Type,
		PayloadVersion: event.PayloadVersion,
		Payload:        event.Payload,
	}, true
}

func interpret(event streamEvent) eventNotice {
	return stream.Interpret(stream.Event{
		SubscriptionID: event.SubscriptionID,
		RunID:          event.RunID,
		Seq:            event.Seq,
		Type:           string(event.Type),
		PayloadVersion: event.PayloadVersion,
		Payload:        event.Payload,
	})
}

func payloadString(raw json.RawMessage, key string) string {
	return stream.PayloadString(raw, key)
}
