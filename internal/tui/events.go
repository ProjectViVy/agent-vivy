package tui

import (
	"bytes"
	"encoding/json"
	"fmt"

	"agent-vivy/internal/domain"
)

type streamEvent struct {
	RunID   string
	Seq     int
	Type    domain.EventType
	Payload json.RawMessage
}

type eventNotice struct {
	RunID   string
	Seq     int
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
			Seq     int              `json:"seq"`
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
		Seq:     envelope.Event.Seq,
		Type:    envelope.Event.Type,
		Payload: envelope.Event.Payload,
	}, true
}

func interpret(event streamEvent) eventNotice {
	base := eventNotice{RunID: event.RunID, Seq: event.Seq}
	switch event.Type {
	case domain.EventModelDelta:
		base.Kind = "delta"
		base.Delta = payloadString(event.Payload, "delta")
		return base
	case domain.EventModelReasoningDelta:
		text := payloadString(event.Payload, "delta")
		if text == "" {
			return base
		}
		base.Kind = "reasoning"
		base.Delta = text
		return base
	case domain.EventToolRequested:
		name := payloadString(event.Payload, "tool_name")
		base.Kind = "tool_requested"
		base.Line = payloadObject(event.Payload, "args")
		base.Message = name
		return base
	case domain.EventToolFinished:
		name := payloadString(event.Payload, "tool_name")
		errText := payloadString(event.Payload, "error")
		base.Kind = "tool_finished"
		base.Message = name
		base.Line = displayToolResult(payloadString(event.Payload, "result"))
		if errText != "" {
			base.Failed = true
			base.Line = fmt.Sprintf("tool %s failed: %s", name, errText)
			return base
		}
		if base.Line == "" {
			base.Line = "tool " + name + " done"
		}
		return base
	case domain.EventToolApprovalRequired:
		id := payloadString(event.Payload, "approval_id")
		name := payloadString(event.Payload, "tool_name")
		preview := payloadObject(event.Payload, "args")
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
		return base
	}
}

func displayToolResult(result string) string {
	var mutation struct {
		Path        string `json:"path"`
		Diff        string `json:"diff"`
		Diagnostics string `json:"diagnostics"`
	}
	if json.Unmarshal([]byte(result), &mutation) == nil && mutation.Diff != "" {
		out := mutation.Path
		if out != "" {
			out += "\n"
		}
		out += mutation.Diff
		if mutation.Diagnostics != "" {
			out += "\n\nDiagnostics:\n" + mutation.Diagnostics
		}
		return out
	}
	return result
}

func payloadObject(raw json.RawMessage, key string) string {
	if len(raw) == 0 {
		return ""
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil || len(obj[key]) == 0 {
		return ""
	}
	var compact bytes.Buffer
	if json.Compact(&compact, obj[key]) != nil {
		return ""
	}
	return compact.String()
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
