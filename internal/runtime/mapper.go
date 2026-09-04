package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
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

	// pendingText accumulates the in-flight assistant turn so v2
	// model.completed can commit the exact bounded delta sequence even when
	// the engine never emits a final whole-message event.
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

	observedMu      sync.Mutex
	observedStreams int
	toolMu          sync.Mutex
	toolsSettled    chan struct{}
	toolsWaiting    bool
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
	settled := make(chan struct{})
	close(settled)
	return &eventMapper{runID: runID, maxPayload: maxPayload, stallThreshold: defaultProviderStallThreshold, toolsSettled: settled}
}

// onEvent maps one engine event. A non-nil error means the run cannot
// continue; the service emits the terminal event.
func (m *eventMapper) onEvent(ev *adk.AgentEvent) ([]domain.RunEvent, error) {
	var out []domain.RunEvent
	err := m.onEventEach(ev, func(events []domain.RunEvent) error {
		out = append(out, events...)
		return nil
	})
	return out, err
}

// onEventEach emits mapped batches as soon as they are available. In
// particular, provider stream chunks must reach the Journal before the stream
// reaches EOF; collecting the whole stream here turns token streaming into a
// burst and can starve bounded transports.
func (m *eventMapper) onEventEach(ev *adk.AgentEvent, emit func([]domain.RunEvent) error) error {
	if ev.Err != nil {
		m.takeObservedStream()
		var retry *adk.WillRetryError
		if errors.As(ev.Err, &retry) {
			return emit([]domain.RunEvent{m.build(domain.EventProviderRetry, payloadProviderRetry{Attempt: retry.RetryAttempt})})
		}
		var ce *adk.CancelError
		if errors.As(ev.Err, &ce) {
			return errRunCancelled
		}
		return fmt.Errorf("engine event error: %w", ev.Err)
	}
	// Interrupt actions suspend the run on an approval (C6); the details
	// travel on the mapper, the sentinel on the error return.
	if ev.Action != nil && ev.Action.Interrupted != nil {
		m.interrupt = m.extractInterrupt(ev.Action.Interrupted)
		return errRunInterrupted
	}
	// Middleware-internal customized actions (e.g. the Eino summarization
	// middleware's generate_summary events) carry no assistant output; only
	// the ones carrying provider usage are mapped.
	if ev.Action != nil && ev.Action.CustomizedAction != nil {
		events, err := m.onCustomizedAction(ev.Action.CustomizedAction)
		if err != nil {
			return err
		}
		return emit(events)
	}
	if ev.Output == nil || ev.Output.MessageOutput == nil {
		return nil
	}
	mv := ev.Output.MessageOutput
	if mv.IsStreaming && mv.MessageStream != nil {
		return m.onStreamEventEach(mv, emit)
	}
	events, err := m.onMessageEvent(mv)
	if err != nil {
		return err
	}
	err = emit(events)
	if mv.Message != nil && mv.Message.Role == schema.Tool {
		m.signalToolsSettled()
	}
	return err
}

func (m *eventMapper) onStreamEvent(mv *adk.TypedMessageVariant[*schema.Message]) ([]domain.RunEvent, error) {
	var out []domain.RunEvent
	err := m.onStreamEventEach(mv, func(events []domain.RunEvent) error {
		out = append(out, events...)
		return nil
	})
	return out, err
}

func (m *eventMapper) onStreamEventEach(mv *adk.TypedMessageVariant[*schema.Message], emit func([]domain.RunEvent) error) error {
	observedLive := m.takeObservedStream()
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
				return emit([]domain.RunEvent{m.build(domain.EventProviderRetry, payloadProviderRetry{Attempt: retry.RetryAttempt})})
			}
			var ce *adk.CancelError
			if errors.As(err, &ce) {
				return errRunCancelled
			}
			return fmt.Errorf("model stream recv: %w", err)
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
		if !observedLive {
			var chunkEvents []domain.RunEvent
			if reasoning := reasoningText(chunk); reasoning != "" {
				chunkEvents = append(chunkEvents, m.reasoningEvents(reasoning)...)
			}
			// Reasoning-only provider chunks commonly carry an empty Content.
			// A real content delta is the boundary between reasoning and answer.
			if chunk.Content != "" {
				chunkEvents = append(chunkEvents, m.deltaEvents(chunk.Content)...)
				m.pendingText.WriteString(chunk.Content)
				m.hasPending = true
			}
			if len(chunkEvents) > 0 {
				if err := emit(chunkEvents); err != nil {
					return err
				}
			}
		}
	}
	if mv.Role == schema.Tool {
		events, err := m.toolResultEventsParts(mv.ToolName, "", content.String(), toolParts, "")
		if err != nil {
			return err
		}
		err = emit(events)
		m.signalToolsSettled()
		return err
	}
	var tail []domain.RunEvent
	if elapsed := time.Since(started); m.stallThreshold >= 0 && elapsed >= m.stallThreshold {
		tail = append(tail, m.build(domain.EventProviderStall, payloadProviderStall{ElapsedMs: elapsed.Milliseconds()}))
	}
	if usage != nil {
		tail = append(tail, m.usageEvent(usage))
	}
	if callsMsg != nil {
		// Streaming engines deliver tool calls as chunks; map them like a
		// whole-message tool call turn. The preamble was already emitted as
		// durable deltas; only clear its accumulator so the post-tool model
		// round cannot inherit it. A second model.completed here would falsely
		// close the run's single model.request trajectory step.
		m.resetPending()
		tail = append(tail, m.toolCallEvents(callsMsg)...)
	}
	return emit(tail)
}

func (m *eventMapper) beginObservedStream() {
	m.observedMu.Lock()
	m.observedStreams++
	m.observedMu.Unlock()
}

func (m *eventMapper) takeObservedStream() bool {
	m.observedMu.Lock()
	defer m.observedMu.Unlock()
	if m.observedStreams == 0 {
		return false
	}
	m.observedStreams--
	return true
}

func (m *eventMapper) observeStreamChunk(chunk *schema.Message) []domain.RunEvent {
	if chunk == nil {
		return nil
	}
	var events []domain.RunEvent
	if reasoning := reasoningText(chunk); reasoning != "" {
		events = append(events, m.reasoningEvents(reasoning)...)
	}
	if chunk.Content != "" {
		events = append(events, m.deltaEvents(chunk.Content)...)
		m.pendingText.WriteString(chunk.Content)
		m.hasPending = true
	}
	return events
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
	// Tool-result events are not model streams. Consuming an observer marker
	// here can steal the marker from a concurrently starting post-tool model
	// stream (notably approval resume), causing its deltas to be emitted twice.
	if msg.Role == schema.Tool {
		events, err := m.toolResultEventsParts(mv.ToolName, msg.ToolCallID, toolMessageText(msg), toolMessageParts(msg), "")
		if err != nil {
			return nil, err
		}
		return append(out, events...), nil
	}
	_ = m.takeObservedStream()
	switch {
	case len(msg.ToolCalls) > 0:
		// Some providers attach assistant preamble text to the same message as
		// tool calls. Close that model phase before emitting tool.requested and
		// clear the pending accumulator so the next model round cannot inherit it.
		content := msg.Content
		if m.hasPending {
			content = m.pendingText.String()
		}
		if content != "" && !m.hasPending {
			// Non-stream providers can attach text to a tool-call message. Emit
			// it through the same durable presentation channel as streamed
			// preamble text; tool.requested is the phase boundary. Do not create
			// an extra model.completed, which would close the only model.request
			// trajectory record before the final answer.
			out = append(out, m.deltaEvents(content)...)
		}
		m.resetPending()
		return append(out, m.toolCallEvents(msg)...), nil
	default:
		// Final assistant message of the model turn.
		content := msg.Content
		if m.hasPending {
			content = m.pendingText.String()
		}
		if content != "" && !m.hasPending {
			// Non-stream providers still use the same bounded durable body
			// channel as streaming providers. Completion is metadata-only.
			out = append(out, m.deltaEvents(content)...)
		}
		m.resetPending()
		return append(out, m.completedEvent(content)), nil
	}
}

func (m *eventMapper) messageMetaEvents(msg *schema.Message) []domain.RunEvent {
	var out []domain.RunEvent
	if reasoning := reasoningText(msg); reasoning != "" {
		out = append(out, m.reasoningEvents(reasoning)...)
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

func (m *eventMapper) reasoningEvents(text string) []domain.RunEvent {
	parts := splitTextForPayload(text, m.maxPayload)
	out := make([]domain.RunEvent, 0, len(parts))
	for _, part := range parts {
		out = append(out, m.build(domain.EventModelReasoningDelta, payloadModelReasoningDelta{Delta: part}))
	}
	return out
}

func (m *eventMapper) usageEvent(usage *schema.TokenUsage) domain.RunEvent {
	return m.build(domain.EventModelUsage, payloadModelUsage{
		PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens,
		TotalTokens: usage.TotalTokens, ReasoningTokens: usage.CompletionTokensDetails.ReasoningTokens,
		CachedTokens: usage.PromptTokenDetails.CachedTokens,
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
	return []domain.RunEvent{m.completedEvent(content)}
}

func (m *eventMapper) completedEvent(content string) domain.RunEvent {
	sum := sha256.Sum256([]byte(content))
	re := m.build(domain.EventModelCompleted, payloadModelCompletedV2{
		ContentSHA256: fmt.Sprintf("%x", sum[:]),
		ByteLen:       len([]byte(content)),
	})
	re.PayloadVersion = 2
	return re
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
		m.registerOpenCall(openToolCall{id: tc.ID, name: tc.Function.Name, args: args, argsJSON: string(argsJSON)})
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
	resolvedID, resolvedName := m.popOpenCall(callID, toolName)
	if callID == "" {
		callID = resolvedID
	}
	if toolName == "" {
		toolName = resolvedName
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

func (m *eventMapper) registerOpenCall(call openToolCall) {
	m.toolMu.Lock()
	defer m.toolMu.Unlock()
	if len(m.openCalls) == 0 {
		m.toolsSettled = make(chan struct{})
		m.toolsWaiting = true
	}
	m.openCalls = append(m.openCalls, call)
}

func (m *eventMapper) popOpenCall(callID, toolName string) (string, string) {
	m.toolMu.Lock()
	defer m.toolMu.Unlock()
	for i, oc := range m.openCalls {
		if (callID != "" && oc.id == callID) || (callID == "" && (toolName == "" || oc.name == toolName)) {
			m.openCalls = append(m.openCalls[:i], m.openCalls[i+1:]...)
			return oc.id, oc.name
		}
	}
	return "", toolName
}

func (m *eventMapper) signalToolsSettled() {
	m.toolMu.Lock()
	defer m.toolMu.Unlock()
	if len(m.openCalls) == 0 && m.toolsWaiting {
		close(m.toolsSettled)
		m.toolsWaiting = false
	}
}

func (m *eventMapper) waitForToolsSettled(ctx context.Context) error {
	m.toolMu.Lock()
	settled := m.toolsSettled
	m.toolMu.Unlock()
	select {
	case <-settled:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// deltaEvents builds one or more bounded model.delta events without dropping
// any part of the provider chunk.
func (m *eventMapper) deltaEvents(delta string) []domain.RunEvent {
	parts := splitTextForPayload(delta, m.maxPayload)
	out := make([]domain.RunEvent, 0, len(parts))
	for _, part := range parts {
		out = append(out, m.build(domain.EventModelDelta, payloadModelDelta{Delta: part}))
	}
	return out
}

// splitTextForPayload preserves every rune while keeping each delta under the
// configured event payload budget. Streaming text is never a safe place to
// truncate: model.completed and the live transcript must describe the same
// answer byte-for-byte.
func splitTextForPayload(text string, budget int) []string {
	if text == "" {
		return nil
	}
	if budget <= 0 {
		return []string{text}
	}
	const envelope = 16
	allowed := (budget - envelope) / 6
	if allowed < 1 {
		allowed = 1
	}
	runes := []rune(text)
	out := make([]string, 0, (len(runes)+allowed-1)/allowed)
	for len(runes) > 0 {
		n := min(allowed, len(runes))
		out = append(out, string(runes[:n]))
		runes = runes[n:]
	}
	return out
}

// SplitModelTextForPayload exposes the kernel's lossless event-body chunking
// to supervised child producers, which share the same Journal contract.
func SplitModelTextForPayload(text string, budget int) []string {
	return splitTextForPayload(text, budget)
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
	if finished, ok := payload.(payloadToolFinished); ok && m.maxPayload > 0 {
		payload = boundToolFinishedEventPayload(finished, m.maxPayload)
	}
	if approval, ok := payload.(payloadToolApprovalRequired); ok && m.maxPayload > 0 {
		payload = boundApprovalEventPayload(approval, m.maxPayload)
	}
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

func boundToolFinishedEventPayload(payload payloadToolFinished, budget int) payloadToolFinished {
	fieldBudget := max(0, budget-128)
	payload.ToolCallID = clampEscapedText(payload.ToolCallID, min(256, fieldBudget/8))
	payload.ToolName = clampEscapedText(payload.ToolName, min(1024, fieldBudget/8))
	payload.Error = clampEscapedText(payload.Error, min(1024, fieldBudget/8))
	resultBudget := fieldBudget * 5 / 8
	payload.Result = clampEscapedText(payload.Result, resultBudget)
	omitted := 0
	for len(payload.Parts) > 0 {
		encoded, _ := json.Marshal(payload)
		if len(encoded) <= budget {
			break
		}
		payload.Parts = payload.Parts[:len(payload.Parts)-1]
		omitted++
	}
	if omitted > 0 {
		marker := fmt.Sprintf("\n[tool result parts omitted: %d exceeded event size limit]", omitted)
		markerJSON, _ := json.Marshal(marker)
		contentBudget := max(2, resultBudget-max(0, len(markerJSON)-2))
		payload.Result = clampEscapedText(payload.Result, contentBudget) + marker
	}
	encoded, _ := json.Marshal(payload)
	if len(encoded) > budget {
		return payloadToolFinished{
			ToolCallID: clampEscapedText(payload.ToolCallID, max(2, budget/8)),
			ToolName:   clampEscapedText(payload.ToolName, max(2, budget/8)),
			Result:     "[tool result omitted: event size limit]",
		}
	}
	return payload
}

func boundApprovalEventPayload(payload payloadToolApprovalRequired, budget int) payloadToolApprovalRequired {
	payload.Action, payload.Target, payload.PreconditionHash, payload.Preview, payload.RiskFindings = boundApprovalReviewFields(
		payload.Action, payload.Target, payload.PreconditionHash, payload.Preview, payload.RiskFindings, budget,
	)
	encoded, _ := json.Marshal(payload)
	if len(encoded) <= budget {
		return payload
	}
	// Exact arguments remain in the suspended approval record; the durable
	// event needs only a safe review summary and identity for resumption.
	payload.Args = map[string]any{"summary": "[approval arguments omitted: event size limit]"}
	encoded, _ = json.Marshal(payload)
	if len(encoded) <= budget {
		return payload
	}
	payload.Preview = "[preview omitted: event size limit]"
	payload.RiskFindings = []string{"review metadata truncated to durable event size limit"}
	encoded, _ = json.Marshal(payload)
	if len(encoded) > budget {
		minimal := payloadToolApprovalRequired{
			ApprovalID: clampEscapedText(payload.ApprovalID, max(2, budget/8)),
			ToolCallID: clampEscapedText(payload.ToolCallID, max(2, budget/8)),
			ToolName:   clampEscapedText(payload.ToolName, max(2, budget/8)),
			Args:       map[string]any{}, ExpiresAt: payload.ExpiresAt,
			Face: clampEscapedText(payload.Face, max(2, budget/16)),
		}
		encoded, _ = json.Marshal(minimal)
		if len(encoded) > budget {
			minimal.ToolCallID = ""
			minimal.ToolName = ""
			minimal.Face = ""
			minimal.ExpiresAt = 0
		}
		return minimal
	}
	return payload
}

func boundToolProposalReview(proposal domain.ToolProposal, budget int) domain.ToolProposal {
	proposal.Action, proposal.Target, proposal.PreconditionHash, proposal.Preview, proposal.RiskFindings = boundApprovalReviewFields(
		proposal.Action, proposal.Target, proposal.PreconditionHash, proposal.Preview, proposal.RiskFindings, budget,
	)
	return proposal
}

func boundApprovalReviewFields(action, target, hash, preview string, risks []string, budget int) (string, string, string, string, []string) {
	const (
		maxPreview = 64 << 10
		maxRisks   = 16
		maxRisk    = 1024
	)
	if budget <= 0 {
		boundedRisks := append([]string(nil), risks...)
		for i := range boundedRisks {
			boundedRisks[i] = tools.RedactSensitive(boundedRisks[i])
		}
		return action, tools.RedactSensitive(target), hash, tools.RedactSensitive(preview), boundedRisks
	}
	target = tools.RedactSensitive(target)
	preview = tools.RedactSensitive(preview)
	risks = append([]string(nil), risks...)
	for i := range risks {
		risks[i] = tools.RedactSensitive(risks[i])
	}
	action = clampEscapedText(action, 1024)
	target = clampEscapedText(target, 4096)
	hash = clampEscapedText(hash, 256)
	reviewBudget := max(0, budget-8192)
	riskBudget := min(16<<10, reviewBudget/4)
	boundedRisks := make([]string, 0, min(len(risks), maxRisks)+1)
	used := 0
	for i, finding := range risks {
		if i >= maxRisks || used >= riskBudget {
			boundedRisks = append(boundedRisks, "additional risk findings truncated")
			break
		}
		findingBudget := min(maxRisk, riskBudget-used)
		bounded := clampEscapedTextWithMarker(finding, findingBudget, "… [truncated]")
		boundedRisks = append(boundedRisks, bounded)
		encoded, _ := json.Marshal(bounded)
		used += len(encoded)
	}
	previewBudget := min(maxPreview, max(0, reviewBudget-used))
	preview = clampEscapedTextWithMarker(preview, previewBudget, "\n[preview truncated]\n")
	return action, target, hash, preview, boundedRisks
}

func clampEscapedTextWithMarker(value string, budget int, marker string) string {
	encoded, _ := json.Marshal(value)
	if len(encoded) <= budget {
		return value
	}
	encodedMarker, _ := json.Marshal(marker)
	contentBudget := max(2, budget-max(0, len(encodedMarker)-2))
	return clampEscapedText(value, contentBudget) + marker
}

func clampEscapedText(value string, budget int) string {
	if budget <= 2 {
		return ""
	}
	encoded, _ := json.Marshal(value)
	if len(encoded) <= budget {
		return value
	}
	runes := []rune(value)
	low, high := 0, len(runes)
	for low < high {
		mid := low + (high-low+1)/2
		candidate, _ := json.Marshal(string(runes[:mid]))
		if len(candidate) <= budget {
			low = mid
		} else {
			high = mid - 1
		}
	}
	slog.Warn("tool result clamped to encoded event budget", "original_runes", len(runes), "kept_runes", low)
	return string(runes[:low])
}
