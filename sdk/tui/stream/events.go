// Package stream contains the protocol-independent stream core used by both
// Vivy TUI faces. Transport adapters convert their wire event enum to Event;
// projection and sequence handling stay here so the faces cannot drift.
package stream

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Event is the normalized run/event envelope. Type deliberately remains a
// string: the core does not import the kernel domain package or a plugin face.
type Event struct {
	SubscriptionID string
	RunID          string
	Seq            int
	Type           string
	Payload        json.RawMessage
}

// Notice is the normalized, render-oriented event consumed by Projection.
// Unknown events retain RunID and Seq so the sequence cursor can advance even
// when a face does not render that event yet.
type Notice struct {
	SubscriptionID string
	RunID          string
	Seq            int
	Kind           string
	Line           string
	Delta          string
	Gate           *GatePrompt
	Done           bool
	Failed         bool
	Message        string
}

// GatePrompt is the normalized interaction overlay attached to a notice.
type GatePrompt struct {
	Kind  string // approval | question
	ID    string
	Title string
	Body  string
}

// StreamError is the control message emitted when durable replay itself
// fails. It is keyed by subscription so stale failures cannot poison a newer
// recovery stream.
type StreamError struct {
	SubscriptionID string
	Message        string
}

// DecodeStreamError validates a run/stream_error notification.
func DecodeStreamError(params json.RawMessage) (StreamError, bool) {
	var envelope struct {
		SubscriptionID string `json:"subscription_id"`
		Message        string `json:"message"`
	}
	if err := json.Unmarshal(params, &envelope); err != nil || envelope.SubscriptionID == "" {
		return StreamError{}, false
	}
	return StreamError{SubscriptionID: envelope.SubscriptionID, Message: envelope.Message}, true
}

// Decode validates and decodes the control-plane run/event envelope.
func Decode(params json.RawMessage) (Event, bool) {
	var envelope struct {
		SubscriptionID string `json:"subscription_id"`
		Event          struct {
			RunID   string          `json:"run_id"`
			Seq     int             `json:"seq"`
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		} `json:"event"`
	}
	if err := json.Unmarshal(params, &envelope); err != nil {
		return Event{}, false
	}
	// Durable RunEvent sequence numbers are strictly positive by schema.
	// Rejecting legacy/unsequenced wire events keeps overflow replay sound:
	// every accepted notification can be reconstructed from the Journal.
	if envelope.Event.Type == "" || envelope.Event.Seq <= 0 {
		return Event{}, false
	}
	return Event{
		SubscriptionID: envelope.SubscriptionID,
		RunID:          envelope.Event.RunID,
		Seq:            envelope.Event.Seq,
		Type:           envelope.Event.Type,
		Payload:        envelope.Event.Payload,
	}, true
}

// Interpret converts a normalized event to a render-oriented notice.
func Interpret(event Event) Notice {
	base := Notice{SubscriptionID: event.SubscriptionID, RunID: event.RunID, Seq: event.Seq}
	switch event.Type {
	case "model.delta":
		base.Kind = "delta"
		base.Delta = PayloadString(event.Payload, "delta")
		return base
	case "model.reasoning_delta":
		text := PayloadString(event.Payload, "delta")
		if text == "" {
			return base
		}
		base.Kind = "reasoning"
		base.Delta = text
		return base
	case "tool.requested":
		name := PayloadString(event.Payload, "tool_name")
		base.Kind = "tool_requested"
		base.Line = PayloadObject(event.Payload, "args")
		base.Message = name
		return base
	case "tool.finished":
		name := PayloadString(event.Payload, "tool_name")
		errText := PayloadString(event.Payload, "error")
		base.Kind = "tool_finished"
		base.Message = name
		base.Line = DisplayToolResult(PayloadString(event.Payload, "result"))
		if errText != "" {
			base.Failed = true
			base.Line = fmt.Sprintf("tool %s failed: %s", name, errText)
			return base
		}
		if base.Line == "" {
			base.Line = "tool " + name + " done"
		}
		return base
	case "tool.approval_required":
		id := PayloadString(event.Payload, "approval_id")
		name := PayloadString(event.Payload, "tool_name")
		preview := PayloadObject(event.Payload, "args")
		body := name
		if preview != "" {
			body = name + "\n" + preview
		}
		base.Kind = "gate"
		base.Line = "approval required: " + name + "  (y/n)"
		base.Gate = &GatePrompt{Kind: "approval", ID: id, Title: name, Body: body}
		return base
	case "user.question_required":
		id := PayloadString(event.Payload, "question_id")
		prompt := PayloadString(event.Payload, "prompt")
		base.Kind = "gate"
		base.Line = "question: " + prompt
		base.Gate = &GatePrompt{Kind: "question", ID: id, Title: "question", Body: prompt}
		return base
	case "run.completed":
		base.Kind = "done"
		base.Done = true
		return base
	case "run.failed":
		base.Kind = "done"
		base.Done = true
		base.Failed = true
		base.Message = PayloadString(event.Payload, "message")
		return base
	case "run.cancelled":
		base.Kind = "done"
		base.Done = true
		base.Failed = true
		base.Message = "cancelled"
		return base
	default:
		return base
	}
}

// DisplayToolResult renders the structured mutation result without coupling
// the stream core to a tool implementation package.
func DisplayToolResult(result string) string {
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

// PayloadObject returns a compact JSON object member for tool previews.
func PayloadObject(raw json.RawMessage, key string) string {
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

// PayloadString returns a string member from a JSON payload.
func PayloadString(raw json.RawMessage, key string) string {
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
