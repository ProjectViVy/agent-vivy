package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
)

// errRunCancelled is the mapper's sentinel for engine-level cancellation;
// the service turns it into a run.cancelled terminal event.
var errRunCancelled = errors.New("runtime: run cancelled")

// eventMapper converts Eino AgentEvents into Vivy domain.RunEvents
// (IMPLEMENTATION-PLAN section 6 mapping table, A1 deviations included:
// run.started and tool.requested are synthesized by Vivy, not the engine).
// Seq is left zero: the journal assigns it on append.
type eventMapper struct {
	runID      domain.RunID
	maxPayload int

	// pendingText accumulates the in-flight assistant turn so that
	// model.completed can carry the reassembled text even when the engine
	// never emits a final whole-message event.
	pendingText strings.Builder
	hasPending  bool

	// openCalls tracks tool calls requested by the model whose results
	// have not arrived yet, in request order.
	openCalls []openToolCall
}

type openToolCall struct {
	id   string
	name string
}

func newEventMapper(runID domain.RunID, maxPayload int) *eventMapper {
	return &eventMapper{runID: runID, maxPayload: maxPayload}
}

// onEvent maps one engine event. A non-nil error means the run cannot
// continue; the service emits the terminal event.
func (m *eventMapper) onEvent(ev *adk.AgentEvent) ([]domain.RunEvent, error) {
	if ev.Err != nil {
		var ce *adk.CancelError
		if errors.As(ev.Err, &ce) {
			return nil, errRunCancelled
		}
		return nil, fmt.Errorf("engine event error: %w", ev.Err)
	}
	// Interrupt actions are C6 territory; C4 ignores them.
	if ev.Action != nil && ev.Action.Interrupted != nil {
		return nil, nil
	}
	if ev.Output == nil || ev.Output.MessageOutput == nil {
		return nil, nil
	}
	mv := ev.Output.MessageOutput
	if mv.IsStreaming && mv.MessageStream != nil {
		return m.onStreamEvent(mv)
	}
	return m.onMessageEvent(mv)
}

func (m *eventMapper) onStreamEvent(mv *adk.TypedMessageVariant[*schema.Message]) ([]domain.RunEvent, error) {
	var out []domain.RunEvent
	var content strings.Builder
	for {
		chunk, err := mv.MessageStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			var ce *adk.CancelError
			if errors.As(err, &ce) {
				return out, errRunCancelled
			}
			return out, fmt.Errorf("model stream recv: %w", err)
		}
		if chunk == nil {
			continue
		}
		content.WriteString(chunk.Content)
		if mv.Role == schema.Tool {
			continue // assembled below as a tool result
		}
		out = append(out, m.deltaEvent(chunk.Content))
		m.pendingText.WriteString(chunk.Content)
		m.hasPending = true
	}
	if mv.Role == schema.Tool {
		out = append(out, m.toolResultEvents(mv.ToolName, "", content.String(), "")...)
	}
	return out, nil
}

func (m *eventMapper) onMessageEvent(mv *adk.TypedMessageVariant[*schema.Message]) ([]domain.RunEvent, error) {
	msg := mv.Message
	if msg == nil {
		return nil, nil
	}
	switch {
	case len(msg.ToolCalls) > 0:
		return m.toolCallEvents(msg), nil
	case msg.Role == schema.Tool:
		return m.toolResultEvents(mv.ToolName, msg.ToolCallID, msg.Content, ""), nil
	default:
		// Final assistant message of the model turn.
		content := msg.Content
		if m.hasPending {
			content = m.pendingText.String()
		}
		m.resetPending()
		return []domain.RunEvent{m.build(domain.EventModelCompleted, payloadModelCompleted{Content: content})}, nil
	}
}

// onTurnEnd flushes a pending model.completed when the engine closed the
// turn without a final whole-message event.
func (m *eventMapper) onTurnEnd() []domain.RunEvent {
	if !m.hasPending {
		return nil
	}
	content := m.pendingText.String()
	m.resetPending()
	return []domain.RunEvent{m.build(domain.EventModelCompleted, payloadModelCompleted{Content: content})}
}

func (m *eventMapper) resetPending() {
	m.pendingText.Reset()
	m.hasPending = false
}

func (m *eventMapper) toolCallEvents(msg *schema.Message) []domain.RunEvent {
	var out []domain.RunEvent
	for _, tc := range msg.ToolCalls {
		args := map[string]any{}
		if tc.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				slog.Warn("tool call arguments are not a JSON object", "tool", tc.Function.Name, "err", err)
			}
		}
		out = append(out, m.build(domain.EventToolRequested, payloadToolRequested{
			ToolCallID: tc.ID,
			ToolName:   tc.Function.Name,
			Args:       args,
		}))
		m.openCalls = append(m.openCalls, openToolCall{id: tc.ID, name: tc.Function.Name})
	}
	return out
}

// toolResultEvents emits tool.started immediately followed by
// tool.finished: the engine delivers tool results as a single event, so
// the start boundary is reconstructed at result time.
func (m *eventMapper) toolResultEvents(toolName, callID, result, errMsg string) []domain.RunEvent {
	if callID == "" {
		callID, toolName = m.popOpenCall(toolName)
	}
	return []domain.RunEvent{
		m.build(domain.EventToolStarted, payloadToolStarted{ToolCallID: callID, ToolName: toolName}),
		m.build(domain.EventToolFinished, payloadToolFinished{ToolCallID: callID, ToolName: toolName, Result: result, Error: errMsg}),
	}
}

func (m *eventMapper) popOpenCall(toolName string) (string, string) {
	for i, oc := range m.openCalls {
		if toolName == "" || oc.name == toolName {
			m.openCalls = append(m.openCalls[:i], m.openCalls[i+1:]...)
			return oc.id, oc.name
		}
	}
	return "", toolName
}

// deltaEvent builds a model.delta event, clamping the delta so the
// marshaled payload stays within maxPayload (NFR: bounded).
func (m *eventMapper) deltaEvent(delta string) domain.RunEvent {
	if m.maxPayload > 0 {
		delta = clampText(delta, m.maxPayload)
	}
	return m.build(domain.EventModelDelta, payloadModelDelta{Delta: delta})
}

// clampText shrinks s (worst-case JSON escaping assumed) until
// {"delta":...}-style marshaling fits budget bytes; it logs when it
// truncates.
func clampText(s string, budget int) string {
	// Envelope overhead plus worst-case \uXXXX escaping (6 bytes per rune).
	const envelope = 16
	allowed := (budget - envelope) / 6
	if allowed < 0 {
		allowed = 0
	}
	runes := []rune(s)
	if len(runes) <= allowed {
		return s
	}
	slog.Warn("event payload clamped to max_event_payload_bytes", "original_runes", len(runes), "kept_runes", allowed)
	return string(runes[:allowed])
}

func (m *eventMapper) build(t domain.EventType, payload any) domain.RunEvent {
	b, err := json.Marshal(payload)
	if err != nil {
		// Payload structs are plain data; marshaling cannot fail in
		// practice. Fall back to an empty object rather than dropping
		// the event.
		slog.Error("event payload marshal failed", "type", string(t), "err", err)
		b = []byte("{}")
	}
	return domain.RunEvent{
		RunID:          m.runID,
		Type:           t,
		CreatedAt:      time.Now().UnixMilli(),
		PayloadVersion: 1,
		Payload:        b,
	}
}
