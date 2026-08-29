package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

type streamEvent struct {
	Type    domain.EventType
	Payload json.RawMessage
}

type eventNotice struct {
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
	return streamEvent{Type: envelope.Event.Type, Payload: envelope.Event.Payload}, true
}

func interpret(event streamEvent) eventNotice {
	switch event.Type {
	case domain.EventModelDelta:
		return eventNotice{Kind: "delta", Delta: payloadString(event.Payload, "delta")}
	case domain.EventModelReasoningDelta:
		text := payloadString(event.Payload, "delta")
		if text == "" {
			return eventNotice{}
		}
		return eventNotice{Kind: "line", Line: "thinking: " + strings.TrimSpace(text)}
	case domain.EventToolRequested:
		name := payloadString(event.Payload, "tool_name")
		return eventNotice{Kind: "line", Line: "tool " + name}
	case domain.EventToolFinished:
		name := payloadString(event.Payload, "tool_name")
		errText := payloadString(event.Payload, "error")
		if errText != "" {
			return eventNotice{Kind: "line", Line: fmt.Sprintf("tool %s failed: %s", name, errText)}
		}
		return eventNotice{Kind: "line", Line: "tool " + name + " done"}
	case domain.EventToolApprovalRequired:
		id := payloadString(event.Payload, "approval_id")
		name := payloadString(event.Payload, "tool_name")
		preview := payloadString(event.Payload, "preview")
		body := name
		if preview != "" {
			body = name + "\n" + preview
		}
		return eventNotice{
			Kind: "gate",
			Line: "approval required: " + name + "  (y/n)",
			Gate: &gatePrompt{Kind: "approval", ID: id, Title: name, Body: body},
		}
	case domain.EventUserQuestionRequired:
		id := payloadString(event.Payload, "question_id")
		prompt := payloadString(event.Payload, "prompt")
		return eventNotice{
			Kind: "gate",
			Line: "question: " + prompt,
			Gate: &gatePrompt{Kind: "question", ID: id, Title: "question", Body: prompt},
		}
	case domain.EventRunCompleted:
		return eventNotice{Kind: "done", Done: true}
	case domain.EventRunFailed:
		return eventNotice{Kind: "done", Done: true, Failed: true, Message: payloadString(event.Payload, "message")}
	case domain.EventRunCancelled:
		return eventNotice{Kind: "done", Done: true, Failed: true, Message: "cancelled"}
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
