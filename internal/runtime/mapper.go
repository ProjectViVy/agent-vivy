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
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
)

// errRunCancelled is the mapper's sentinel for engine-level cancellation;
// the service turns it into a run.cancelled terminal event.
var errRunCancelled = errors.New("runtime: run cancelled")

// errRunInterrupted is the mapper's sentinel for an approval interrupt:
// the service suspends the run on an approval instead of closing it.
// The details live on the mapper (m.interrupt).
var errRunInterrupted = errors.New("runtime: run interrupted for approval")

const defaultProviderStallThreshold = 15 * time.Second

// interruptDetails carries what the service needs to surface and later
// resume an approval-gated tool call (C6).
type interruptDetails struct {
	// ResumeTarget is the root-cause interrupt id: the key ResumeWithParams
	// targets (docs/eino-capability-verify.md §2.4).
	ResumeTarget string
	ToolCallID   string
	ToolName     string
	Args         map[string]any
}

// eventMapper converts Eino AgentEvents into Vivy domain.RunEvents
// (IMPLEMENTATION-PLAN section 6 mapping table, A1 deviations included:
// run.started and tool.requested are synthesized by Vivy, not the engine).
// Seq is left zero: the journal assigns it on append.
type eventMapper struct {
	runID          domain.RunID
	maxPayload     int
	stallThreshold time.Duration

	// pendingText accumulates the in-flight assistant turn so that
	// model.completed can carry the reassembled text even when the engine
	// never emits a final whole-message event.
	pendingText strings.Builder
	hasPending  bool

	// openCalls tracks tool calls requested by the model whose results
	// have not arrived yet, in request order.
	openCalls []openToolCall

	// loop watches completed tool calls for repetition (VC-2 tool-loop
	// guardrail); state is per run and resets on approval resume.
	loop loopWindow

	// interrupt holds the details of the latest interrupt action; set
	// together with the errRunInterrupted sentinel.
	interrupt *interruptDetails
}

type openToolCall struct {
	id   string
	name string
	args map[string]any
	// argsJSON is the canonical (key-sorted) form of args, captured at
	// request time for the loop detector.
	argsJSON string
}

func newEventMapper(runID domain.RunID, maxPayload int) *eventMapper {
	return &eventMapper{runID: runID, maxPayload: maxPayload, stallThreshold: defaultProviderStallThreshold}
}

// onEvent maps one engine event. A non-nil error means the run cannot
// continue; the service emits the terminal event.
func (m *eventMapper) onEvent(ev *adk.AgentEvent) ([]domain.RunEvent, error) {
	if ev.Err != nil {
		var retry *adk.WillRetryError
		if errors.As(ev.Err, &retry) {
			return []domain.RunEvent{m.build(domain.EventProviderRetry, payloadProviderRetry{Attempt: retry.RetryAttempt})}, nil
		}
		var ce *adk.CancelError
		if errors.As(ev.Err, &ce) {
			return nil, errRunCancelled
		}
		return nil, fmt.Errorf("engine event error: %w", ev.Err)
	}
	// Interrupt actions suspend the run on an approval (C6); the details
	// travel on the mapper, the sentinel on the error return.
	if ev.Action != nil && ev.Action.Interrupted != nil {
		m.interrupt = m.extractInterrupt(ev.Action.Interrupted)
		return nil, errRunInterrupted
	}
	// Middleware-internal customized actions (e.g. the Eino summarization
	// middleware's generate_summary events) carry no assistant output; only
	// the ones carrying provider usage are mapped.
	if ev.Action != nil && ev.Action.CustomizedAction != nil {
		return m.onCustomizedAction(ev.Action.CustomizedAction)
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
	var callsMsg *schema.Message
	started := time.Now()
	var usage *schema.TokenUsage
	var toolParts []json.RawMessage
	for {
		chunk, err := mv.MessageStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			var retry *adk.WillRetryError
			if errors.As(err, &retry) {
				out = append(out, m.build(domain.EventProviderRetry, payloadProviderRetry{Attempt: retry.RetryAttempt}))
				return out, nil
			}
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
		for _, part := range chunk.UserInputMultiContent {
			if data, marshalErr := json.Marshal(part); marshalErr == nil {
				toolParts = append(toolParts, data)
			}
		}
		if len(chunk.ToolCalls) > 0 {
			callsMsg = chunk // tool calls ride the accumulated chunk
		}
		if chunk.ResponseMeta != nil && chunk.ResponseMeta.Usage != nil {
			usage = chunk.ResponseMeta.Usage
		}
		if mv.Role == schema.Tool {
			continue // assembled below as a tool result
		}
		if reasoning := reasoningText(chunk); reasoning != "" {
			out = append(out, m.reasoningEvent(reasoning))
		}
		out = append(out, m.deltaEvent(chunk.Content))
		m.pendingText.WriteString(chunk.Content)
		m.hasPending = true
	}
	if mv.Role == schema.Tool {
		events, err := m.toolResultEventsParts(mv.ToolName, "", content.String(), toolParts, "")
		if err != nil {
			return out, err
		}
		return append(out, events...), nil
	}
	if elapsed := time.Since(started); m.stallThreshold >= 0 && elapsed >= m.stallThreshold {
		out = append(out, m.build(domain.EventProviderStall, payloadProviderStall{ElapsedMs: elapsed.Milliseconds()}))
	}
	if usage != nil {
		out = append(out, m.usageEvent(usage))
	}
	if callsMsg != nil {
		// Streaming engines deliver tool calls as chunks; map them like a
		// whole-message tool call turn.
		out = append(out, m.toolCallEvents(callsMsg)...)
	}
	return out, nil
}

// onCustomizedAction maps middleware-internal customized actions. Only the
// Eino summarization middleware's generate_summary events matter: they carry
// the real token usage of the hidden summary provider call, so the run
// accounts it like any other model.usage. Everything else is silent.
func (m *eventMapper) onCustomizedAction(action any) ([]domain.RunEvent, error) {
	ca, ok := action.(*summarization.CustomizedAction)
	if !ok || ca.Type != summarization.ActionTypeGenerateSummary || ca.GenerateSummary == nil {
		return nil, nil
	}
	resp := ca.GenerateSummary.ModelResponse
	if resp == nil || resp.ResponseMeta == nil || resp.ResponseMeta.Usage == nil {
		return nil, nil
	}
	return []domain.RunEvent{m.usageEvent(resp.ResponseMeta.Usage)}, nil
}

func (m *eventMapper) onMessageEvent(mv *adk.TypedMessageVariant[*schema.Message]) ([]domain.RunEvent, error) {
	msg := mv.Message
	if msg == nil {
		return nil, nil
	}
	out := m.messageMetaEvents(msg)
	switch {
	case len(msg.ToolCalls) > 0:
		return append(out, m.toolCallEvents(msg)...), nil
	case msg.Role == schema.Tool:
		events, err := m.toolResultEventsParts(mv.ToolName, msg.ToolCallID, toolMessageText(msg), toolMessageParts(msg), "")
		if err != nil {
			return nil, err
		}
		return append(out, events...), nil
	default:
		// Final assistant message of the model turn.
		content := msg.Content
		if m.hasPending {
			content = m.pendingText.String()
		}
		m.resetPending()
		return append(out, m.build(domain.EventModelCompleted, payloadModelCompleted{Content: content})), nil
	}
}

func (m *eventMapper) messageMetaEvents(msg *schema.Message) []domain.RunEvent {
	var out []domain.RunEvent
	if reasoning := reasoningText(msg); reasoning != "" {
		out = append(out, m.reasoningEvent(reasoning))
	}
	if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
		out = append(out, m.usageEvent(msg.ResponseMeta.Usage))
	}
	return out
}

func reasoningText(msg *schema.Message) string {
	if msg == nil {
		return ""
	}
	var out strings.Builder
	out.WriteString(msg.ReasoningContent)
	for _, part := range msg.AssistantGenMultiContent {
		if part.Type == schema.ChatMessagePartTypeReasoning && part.Reasoning != nil {
			out.WriteString(part.Reasoning.Text)
		}
	}
	return out.String()
}

func (m *eventMapper) reasoningEvent(text string) domain.RunEvent {
	if m.maxPayload > 0 {
		text = clampText(text, m.maxPayload)
	}
	return m.build(domain.EventModelReasoningDelta, payloadModelReasoningDelta{Delta: text})
}

func (m *eventMapper) usageEvent(usage *schema.TokenUsage) domain.RunEvent {
	return m.build(domain.EventModelUsage, payloadModelUsage{
		PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens,
		TotalTokens: usage.TotalTokens, ReasoningTokens: usage.CompletionTokensDetails.ReasoningTokens,
	})
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
		// Canonical form: re-marshaling the decoded map sorts keys, so
		// the same call with reordered JSON keys still counts as a
		// repeat for the loop detector. Decoded JSON cannot fail here.
		argsJSON, _ := json.Marshal(args)
		out = append(out, m.build(domain.EventToolRequested, payloadToolRequested{
			ToolCallID: tc.ID,
			ToolName:   tc.Function.Name,
			Args:       args,
		}))
		m.openCalls = append(m.openCalls, openToolCall{id: tc.ID, name: tc.Function.Name, args: args, argsJSON: string(argsJSON)})
	}
	return out
}

// extractInterrupt pulls the resume target and the affected tool call out
// of the interrupt chain: the root-cause InterruptCtx carries the resume
// key, and its tool-call address segment carries the call id (SubID) and
// name. Args resolve against the tracked tool.requested records, falling
// back to the most recent open call when the address names no id.
func (m *eventMapper) extractInterrupt(info *adk.InterruptInfo) *interruptDetails {
	d := &interruptDetails{}
	for _, c := range info.InterruptContexts {
		if !c.IsRootCause {
			continue
		}
		d.ResumeTarget = c.ID
		for _, seg := range c.Address {
			if seg.SubID != "" {
				d.ToolCallID = seg.SubID
				d.ToolName = seg.ID
			}
		}
		break
	}
	var matched *openToolCall
	for i := len(m.openCalls) - 1; i >= 0; i-- {
		if d.ToolCallID == "" || m.openCalls[i].id == d.ToolCallID {
			matched = &m.openCalls[i]
			break
		}
	}
	if matched != nil {
		if d.ToolCallID == "" {
			d.ToolCallID = matched.id
		}
		if d.ToolName == "" {
			d.ToolName = matched.name
		}
		d.Args = matched.args
	}
	if d.Args == nil {
		d.Args = map[string]any{}
	}
	return d
}

// toolResultEvents emits tool.started immediately followed by
// tool.finished: the engine delivers tool results as a single event, so
// the start boundary is reconstructed at result time.
func (m *eventMapper) toolResultEventsParts(toolName, callID, result string, parts []json.RawMessage, errMsg string) ([]domain.RunEvent, error) {
	argsJSON := m.argsJSONFor(callID, toolName)
	if callID == "" {
		callID, toolName = m.popOpenCall(toolName)
	}
	// The loop detector sees every completed call; exceeding the repeat
	// limit fails the run from here (the service emits the terminal).
	if err := m.loop.record(toolName, argsJSON, result, errMsg); err != nil {
		return nil, err
	}
	return []domain.RunEvent{
		m.build(domain.EventToolStarted, payloadToolStarted{ToolCallID: callID, ToolName: toolName}),
		m.build(domain.EventToolFinished, payloadToolFinished{ToolCallID: callID, ToolName: toolName, Result: result, Parts: parts, Error: errMsg}),
	}, nil
}

// argsJSONFor resolves the canonical arguments of a tracked open call by
// id, or by tool name when the result carries no id; empty when untracked.
func (m *eventMapper) argsJSONFor(callID, toolName string) string {
	for _, oc := range m.openCalls {
		if (callID != "" && oc.id == callID) || (callID == "" && oc.name == toolName) {
			return oc.argsJSON
		}
	}
	return ""
}

func toolMessageText(msg *schema.Message) string {
	if msg == nil {
		return ""
	}
	if msg.Content != "" {
		return msg.Content
	}
	var out strings.Builder
	for _, part := range msg.UserInputMultiContent {
		if part.Type == schema.ChatMessagePartTypeText {
			out.WriteString(part.Text)
		}
	}
	return out.String()
}

func toolMessageParts(msg *schema.Message) []json.RawMessage {
	if msg == nil || len(msg.UserInputMultiContent) == 0 {
		return nil
	}
	var out []json.RawMessage
	for _, part := range msg.UserInputMultiContent {
		if part.Type == schema.ChatMessagePartTypeText {
			continue
		}
		if data, err := json.Marshal(part); err == nil {
			out = append(out, data)
		}
	}
	return out
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
