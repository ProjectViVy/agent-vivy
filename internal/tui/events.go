package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

type streamEvent struct {
	RunID   string
	Type    domain.EventType
	Payload json.RawMessage
}

type eventNotice struct {
	RunID   string
	Kind    string
	Line    string
	Delta   string
	Gate    *gatePrompt
	Done    bool
	Failed  bool
	Message string
}

type gatePrompt struct {
	Kind  string // approval | question
	ID    string
	Title string
	Body  string
}

func decodeStreamEvent(params json.RawMessage) (streamEvent, bool) {
	var envelope struct {
		Event struct {
			RunID   string           `json:"run_id"`
			Type    domain.EventType `json:"type"`
			Payload json.RawMessage  `json:"payload"`
		} `json:"event"`
	}
	if err := json.Unmarshal(params, &envelope); err != nil {
		return streamEvent{}, false
	}
	if envelope.Event.Type == "" {
		return streamEvent{}, false
	}
	return streamEvent{
		RunID:   envelope.Event.RunID,
		Type:    envelope.Event.Type,
		Payload: envelope.Event.Payload,
	}, true
}

func interpret(event streamEvent) eventNotice {
	base := eventNotice{RunID: event.RunID}
	switch event.Type {
	case domain.EventModelDelta:
		base.Kind = "delta"
		base.Delta = payloadString(event.Payload, "delta")
		return base
	case domain.EventModelReasoningDelta:
		text := payloadString(event.Payload, "delta")
		if text == "" {
			return eventNotice{}
		}
		base.Kind = "line"
		base.Line = "thinking: " + strings.TrimSpace(text)
		return base
	case domain.EventToolRequested:
		name := payloadString(event.Payload, "tool_name")
		base.Kind = "tool_requested"
		base.Line = "tool " + name
		base.Message = name
		return base
	case domain.EventToolFinished:
		name := payloadString(event.Payload, "tool_name")
		errText := payloadString(event.Payload, "error")
		base.Kind = "tool_finished"
		base.Message = name
		if errText != "" {
			base.Failed = true
			base.Line = fmt.Sprintf("tool %s failed: %s", name, errText)
			return base
		}
		base.Line = "tool " + name + " done"
		return base
	case domain.EventToolApprovalRequired:
		id := payloadString(event.Payload, "approval_id")
		name := payloadString(event.Payload, "tool_name")
		preview := payloadString(event.Payload, "preview")
		body := name
		if preview != "" {
			body = name + "\n" + preview
		}
		base.Kind = "gate"
		base.Line = "approval required: " + name + "  (y/n)"
		base.Gate = &gatePrompt{Kind: "approval", ID: id, Title: name, Body: body}
		return base
	case domain.EventUserQuestionRequired:
		id := payloadString(event.Payload, "question_id")
		prompt := payloadString(event.Payload, "prompt")
		base.Kind = "gate"
		base.Line = "question: " + prompt
		base.Gate = &gatePrompt{Kind: "question", ID: id, Title: "question", Body: prompt}
		return base
	case domain.EventRunCompleted:
		base.Kind = "done"
		base.Done = true
		return base
	case domain.EventRunFailed:
		base.Kind = "done"
		base.Done = true
		base.Failed = true
		base.Message = payloadString(event.Payload, "message")
		return base
	case domain.EventRunCancelled:
		base.Kind = "done"
		base.Done = true
		base.Failed = true
		base.Message = "cancelled"
		return base
	default:
		return eventNotice{}
	}
}

func payloadString(raw json.RawMessage, key string) string {
	if len(raw) == 0 {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	value, _ := obj[key].(string)
	return value
}
