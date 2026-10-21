package runtime

import (
	"context"
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

// errRunInterrupted is the mapper's sentinel for a review or question
// interrupt: the service persists the matching host-owned interaction.
// The details live on the mapper (m.interrupt).
var errRunInterrupted = errors.New("runtime: run interrupted for approval")

const defaultProviderStallThreshold = 15 * time.Second

// interruptDetails carries what the service needs to surface and later
// resume an interrupted tool call (C6 / PG-0).
type interruptDetails struct {
	// ResumeTarget is the root-cause interrupt id: the key ResumeWithParams
	// targets (docs/eino-capability-verify.md §2.4).
	ResumeTarget string
	// Raw is the engine interrupt info; steer resume walks its context
	// chain for the agent-level resume target.
	Raw *adk.InterruptInfo

	ToolCallID        string
	ToolName          string
	Args              map[string]any
	IntendedArguments json.RawMessage
	Message           string
	PlanSubmissionID  string
}

// eventMapper converts Eino AgentEvents into Vivy domain.RunEvents
// (IMPLEMENTATION-PLAN section 6 mapping table, A1 deviations included:
// run.started and tool.requested are synthesized by Vivy, not the engine).
// Seq is left zero: the journal assigns it on append.
type eventMapper struct {
	runID          domain.RunID
	maxPayload     int
	stallThreshold time.Duration
	runProvider    string
	runModel       string
	summaryModel   string
	tenantID       string
	workspaceID    string
	sessionID      string
	contextViewID  string
	lastSummary    string

	// pendingText accumulates the in-flight assistant turn so v3
	// model.completed can commit the exact bounded delta sequence even when
	// the engine never emits a final whole-message event.
	pendingText strings.Builder
	hasPending  bool

	// openCalls tracks tool calls requested by the model whose results
	// have not arrived yet, in request order.
	openCalls []openToolCall
	// toolBatch retains the most recently requested sibling set even after
	// a fenced sibling returns its refusal before an interrupt is surfaced.
	toolBatch []openToolCall

	// nudge is the leg's run-local detector: it owns the loop window
	// (VC-2 tool-loop guardrail, ND-2) and the typed failure side
	// channel. Service attaches one fresh state per drive/resume leg;
	// nil outside those legs.
	nudge *nudgeState

	resolveToolOutcome func(string, string) (domain.ToolOperationOutcome, bool, error)

	// interrupt holds the details of the latest interrupt action; set
	// together with the errRunInterrupted sentinel.
	interrupt *interruptDetails

	observedMu      sync.Mutex
	observedStreams int
	// pendingObservedCharges credits mapped accounting for calls that
	// charged their model-call unit at v4 request Begin (OBS-02): the
	// model.completed/tool.requested batch of an observed call consumes
	// one credit instead of charging again.
	pendingObservedCharges int
	// observedSummaryCalls counts materialized summary-route calls; each
	// summarization CustomizedAction usage event consumes one so the
	// observed call is not double-reported.
	observedSummaryCalls int
	toolMu               sync.Mutex
	toolsSettled         chan struct{}
	toolsWaiting         bool
}

type openToolCall struct {
	id   string
	name string
	args map[string]any
}

func newEventMapper(runID domain.RunID, maxPayload int) *eventMapper {
	settled := make(chan struct{})
	close(settled)
	return &eventMapper{runID: runID, maxPayload: maxPayload, stallThreshold: defaultProviderStallThreshold, toolsSettled: settled}
}

func (m *eventMapper) setUsageRoutes(provider, model, summaryModel string) {
	m.runProvider = strings.TrimSpace(provider)
	m.runModel = strings.TrimSpace(model)
	m.summaryModel = strings.TrimSpace(summaryModel)
}

func (m *eventMapper) setRunScope(tenantID, workspaceID, sessionID string) {
	m.tenantID = clampEscapedText(strings.TrimSpace(tenantID), 256)
	m.workspaceID = clampEscapedText(strings.TrimSpace(workspaceID), 512)
	m.sessionID = clampEscapedText(strings.TrimSpace(sessionID), 512)
}

func (m *eventMapper) setContextViewID(viewID string) {
	m.contextViewID = clampEscapedText(strings.TrimSpace(viewID), 256)
}

// onEvent maps one engine event. A non-nil error means the run cannot
// continue; the service emits the terminal event.
func (m *eventMapper) onEvent(ev *adk.TypedAgentEvent[*schema.AgenticMessage]) ([]domain.RunEvent, error) {
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
func (m *eventMapper) onEventEach(ev *adk.TypedAgentEvent[*schema.AgenticMessage], emit func([]domain.RunEvent) error) error {
	if ev.Err != nil {
		m.takeObservedStream()
		var retry *adk.WillRetryError
		if errors.As(ev.Err, &retry) {
			reason, _ := retry.RejectReason().(string)
			return emit([]domain.RunEvent{m.build(domain.EventProviderRetry, payloadProviderRetry{Attempt: retry.RetryAttempt, Reason: reason})})
		}
		var ce *adk.CancelError
		if errors.As(ev.Err, &ce) {
			if len(ce.InterruptContexts) > 0 {
				// Boundary-cancel checkpoints carry their interrupt
				// contexts; the steer resume needs the real ctx ids.
				m.interrupt = &interruptDetails{Raw: &adk.InterruptInfo{InterruptContexts: ce.InterruptContexts}}
			}
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
	if mv.Message != nil && agenticIsToolResult(mv.Message) {
		m.signalToolsSettled()
	}
	return err
}

func (m *eventMapper) onStreamEvent(mv *adk.TypedMessageVariant[*schema.AgenticMessage]) ([]domain.RunEvent, error) {
	var out []domain.RunEvent
	err := m.onStreamEventEach(mv, func(events []domain.RunEvent) error {
		out = append(out, events...)
		return nil
	})
	return out, err
}

func (m *eventMapper) onStreamEventEach(mv *adk.TypedMessageVariant[*schema.AgenticMessage], emit func([]domain.RunEvent) error) error {
	observedLive := m.takeObservedStream()
	var content strings.Builder
	var callChunks []*schema.AgenticMessage
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
				reason, _ := retry.RejectReason().(string)
				return emit([]domain.RunEvent{m.build(domain.EventProviderRetry, payloadProviderRetry{Attempt: retry.RetryAttempt, Reason: reason})})
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
		content.WriteString(agenticAssistantText(chunk))
		for _, result := range agenticToolResults(chunk) {
			toolParts = append(toolParts, agenticToolResultParts(result)...)
		}
		if len(agenticToolCalls(chunk)) > 0 {
			callChunks = append(callChunks, chunk)
		}
		if u := agenticUsage(chunk); u != nil {
			usage = u
		}
		if m.streamIsToolResult(mv) {
			continue // assembled below as a tool result
		}
		if !observedLive {
			var chunkEvents []domain.RunEvent
			if reasoning := agenticReasoningText(chunk); reasoning != "" {
				chunkEvents = append(chunkEvents, m.reasoningEvents(reasoning)...)
			}
			// Reasoning-only provider chunks commonly carry no assistant text.
			// A real content delta is the boundary between reasoning and answer.
			if text := agenticAssistantText(chunk); text != "" {
				chunkEvents = append(chunkEvents, m.deltaEvents(text)...)
				m.pendingText.WriteString(text)
				m.hasPending = true
			}
			if len(chunkEvents) > 0 {
				if err := emit(chunkEvents); err != nil {
					return err
				}
			}
		}
	}
	if m.streamIsToolResult(mv) {
		events, err := m.toolResultEventsParts("", "", content.String(), toolParts, "")
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
	// Observed calls emit their own cumulative model.usage v2 samples
	// through the observer seam; the materialized tail must not
	// re-report them as unattributed v1 events.
	if usage != nil && !observedLive {
		tail = append(tail, m.usageEvent(usage))
	}
	if len(callChunks) > 0 {
		callsMsg, err := schema.ConcatAgenticMessages(callChunks)
		if err != nil {
			return fmt.Errorf("concat streamed tool calls: %w", err)
		}
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

// streamIsToolResult reports whether one streaming model event carries a tool
// result. Agentic tool results arrive as user-role events; in streaming mode
// the message has not materialized yet, so the event role is the only signal.
func (m *eventMapper) streamIsToolResult(mv *adk.TypedMessageVariant[*schema.AgenticMessage]) bool {
	if mv == nil {
		return false
	}
	return mv.AgenticRole == schema.AgenticRoleTypeUser
}

// noteObservedCallStart credits one observed call whose v3 request
// Begin already charged the model-call budget. Mapped
// model.completed/tool.requested accounting consumes the credit.
func (m *eventMapper) noteObservedCallStart() {
	m.observedMu.Lock()
	m.pendingObservedCharges++
	m.observedMu.Unlock()
}

func (m *eventMapper) consumeObservedCallCharge() bool {
	m.observedMu.Lock()
	defer m.observedMu.Unlock()
	if m.pendingObservedCharges == 0 {
		return false
	}
	m.pendingObservedCharges--
	return true
}

// noteObservedCallMaterialized marks one call whose invocation produced
// a materialized result (successful inner Stream setup or a returned
// Generate message), suppressing duplicate mapper-side delta/usage
// emission for it. Summary-route calls additionally credit the
// summarization CustomizedAction usage suppression.
func (m *eventMapper) noteObservedCallMaterialized(source string) {
	m.observedMu.Lock()
	m.observedStreams++
	if source == modelCallSourceSummary {
		m.observedSummaryCalls++
	}
	m.observedMu.Unlock()
}

func (m *eventMapper) takeObservedSummaryCall() bool {
	m.observedMu.Lock()
	defer m.observedMu.Unlock()
	if m.observedSummaryCalls == 0 {
		return false
	}
	m.observedSummaryCalls--
	return true
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

func (m *eventMapper) observeStreamChunk(chunk *schema.AgenticMessage) []domain.RunEvent {
	if chunk == nil {
		return nil
	}
	var events []domain.RunEvent
	if reasoning := agenticReasoningText(chunk); reasoning != "" {
		events = append(events, m.reasoningEvents(reasoning)...)
	}
	if text := agenticAssistantText(chunk); text != "" {
		events = append(events, m.deltaEvents(text)...)
		m.pendingText.WriteString(text)
		m.hasPending = true
	}
	return events
}

// onCustomizedAction maps middleware-internal customized actions. Only the
// Eino summarization middleware's generate_summary events matter: they carry
// the real token usage of the hidden summary provider call, so the run
// accounts it like any other model.usage. Everything else is silent.
func (m *eventMapper) onCustomizedAction(action any) ([]domain.RunEvent, error) {
	ca, ok := action.(*summarization.TypedCustomizedAction[*schema.AgenticMessage])
	if !ok || ca.Type != summarization.ActionTypeGenerateSummary || ca.GenerateSummary == nil {
		return nil, nil
	}
	resp := ca.GenerateSummary.ModelResponse
	if usage := agenticUsage(resp); usage == nil {
		return nil, nil
	} else if m.takeObservedSummaryCall() {
		// The wrapped summary model already journaled its v2 usage
		// samples through the observer seam; re-reporting the action's
		// usage would double the call.
		return nil, nil
	} else {
		provider, model := m.runProvider, m.runModel
		if ca.GenerateSummary.Phase == summarization.GenerateSummaryPhasePrimary && m.summaryModel != "" {
			model = m.summaryModel
		}
		return []domain.RunEvent{m.usageEventForRoute(usage, "summary", provider, model)}, nil
	}
}

func (m *eventMapper) onMessageEvent(mv *adk.TypedMessageVariant[*schema.AgenticMessage]) ([]domain.RunEvent, error) {
	msg := mv.Message
	if msg == nil {
		return nil, nil
	}
	out := m.messageMetaEvents(msg)
	// Tool-result events are not model streams. Consuming an observer marker
	// here can steal the marker from a concurrently starting post-tool model
	// stream (notably approval resume), causing its deltas to be emitted twice.
	if agenticIsToolResult(msg) {
		results := agenticToolResults(msg)
		result := results[0]
		events, err := m.toolResultEventsParts(result.Name, result.CallID, agenticToolResultText(result), agenticToolResultParts(result), "")
		if err != nil {
			return nil, err
		}
		return append(out, events...), nil
	}
	observed := m.takeObservedStream()
	if observed {
		// The call emitted its own attributed v2 usage samples; drop the
		// unattributed v1 sample messageMetaEvents already produced.
		out = removeUsageEvents(out)
	}
	calls := agenticToolCalls(msg)
	switch {
	case len(calls) > 0:
		// Some providers attach assistant preamble text to the same message as
		// tool calls. Close that model phase before emitting tool.requested and
		// clear the pending accumulator so the next model round cannot inherit it.
		content := agenticAssistantText(msg)
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
		content := agenticAssistantText(msg)
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

func removeUsageEvents(events []domain.RunEvent) []domain.RunEvent {
	out := events[:0]
	for _, re := range events {
		if re.Type != domain.EventModelUsage {
			out = append(out, re)
		}
	}
	return out
}

func (m *eventMapper) messageMetaEvents(msg *schema.AgenticMessage) []domain.RunEvent {
	var out []domain.RunEvent
	if reasoning := agenticReasoningText(msg); reasoning != "" {
		out = append(out, m.reasoningEvents(reasoning)...)
	}
	if usage := agenticUsage(msg); usage != nil {
		out = append(out, m.usageEvent(usage))
	}
	return out
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
	return m.usageEventForRoute(usage, "main", m.runProvider, m.runModel)
}

func (m *eventMapper) usageEventForRoute(usage *schema.TokenUsage, source, provider, model string) domain.RunEvent {
	return m.build(domain.EventModelUsage, payloadModelUsage{
		PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens,
		TotalTokens: usage.TotalTokens, ReasoningTokens: usage.CompletionTokensDetails.ReasoningTokens,
		CachedTokens: usage.PromptTokenDetails.CachedTokens,
		Provider:     provider, Model: model, Source: source,
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
	m.lastSummary = clampEscapedText(tools.RedactSensitive(content), 8<<10)
	re := m.build(domain.EventModelCompleted, payloadModelCompletedV3{
		ByteLen: len([]byte(content)),
	})
	re.PayloadVersion = 3
	return re
}

func (m *eventMapper) resetPending() {
	m.pendingText.Reset()
	m.hasPending = false
}

func (m *eventMapper) toolCallEvents(msg *schema.AgenticMessage) []domain.RunEvent {
	var out []domain.RunEvent
	m.toolBatch = m.toolBatch[:0]
	for _, tc := range agenticToolCalls(msg) {
		args := map[string]any{}
		if tc.Arguments != "" {
			if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
				slog.Warn("tool call arguments are not a JSON object", "tool", tc.Name, "err", err)
			}
		}
		out = append(out, m.build(domain.EventToolRequested, payloadToolRequested{
			ToolCallID: tc.CallID,
			ToolName:   tc.Name,
			Args:       redactedApprovalArguments(args),
		}))
		call := openToolCall{id: tc.CallID, name: tc.Name, args: args}
		m.toolBatch = append(m.toolBatch, call)
		m.registerOpenCall(call)
	}
	return out
}

// extractInterrupt pulls the resume target and the affected tool call out
// of the interrupt chain: the root-cause InterruptCtx carries the resume
// key, and its tool-call address segment carries the call id (SubID) and
// name. Args resolve against the tracked tool.requested records, falling
// back to the most recent open call when the address names no id.
func (m *eventMapper) extractInterrupt(info *adk.InterruptInfo) *interruptDetails {
	d := &interruptDetails{Raw: info}
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
		if toolName, args, intendedArguments, message, ok := decodeToolApprovalInterrupt(c.Info); ok && (d.ToolName == "" || d.ToolName == toolName) {
			if d.ToolName == "" {
				d.ToolName = toolName
			}
			d.Args = args
			d.IntendedArguments = append(json.RawMessage(nil), intendedArguments...)
			d.Message = message
		}
		if submissionID, ok := decodePlanReviewInterrupt(c.Info); ok {
			d.PlanSubmissionID = submissionID
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
		if d.Args == nil {
			d.Args = matched.args
		}
	}
	if d.Args == nil {
		d.Args = map[string]any{}
	}
	return d
}

// toolResultEvents emits tool.started immediately followed by
// tool.finished: the engine delivers tool results as a single event, so
// the start boundary is reconstructed at result time. The existing durable
// outcome owner supplies typed classification; a non-operation producer mark
// supplies the optional
// outcome metadata and keeps error non-empty even where Eino received
// a nil error.
func (m *eventMapper) toolResultEventsParts(toolName, callID, result string, parts []json.RawMessage, errMsg string) ([]domain.RunEvent, error) {
	resolvedID, resolvedName := m.popOpenCall(callID, toolName)
	if callID == "" {
		callID = resolvedID
	}
	if toolName == "" {
		toolName = resolvedName
	}
	finished := payloadToolFinished{ToolCallID: callID, ToolName: toolName, Result: result, Parts: parts, Error: errMsg}
	var durableFailure *toolFailure
	durableFound := false
	if m.resolveToolOutcome != nil {
		outcome, found, err := m.resolveToolOutcome(callID, toolName)
		if err != nil {
			return nil, err
		}
		if found {
			durableFound = true
			if f, ok := failureFromOutcome(outcome); ok {
				durableFailure = &f
			}
		}
	}
	if durableFailure != nil || (!durableFound && m.nudge != nil) {
		var failure toolFailure
		var ok bool
		if durableFailure != nil {
			failure, ok = *durableFailure, true
		} else {
			failure, ok = m.nudge.Failure(callID)
		}
		if ok {
			failure := failure
			finished.Outcome = failure.Status
			finished.Reason = failure.Reason
			finished.Effects = failure.Effects
			// The journal row of a failed invocation keeps a non-empty
			// error even where Eino received a nil error (ND-2): the
			// bounded diagnostic stands in, never raw output.
			if finished.Error == "" {
				finished.Error = failure.Diagnostic
			}
		}
	}
	return []domain.RunEvent{
		m.build(domain.EventToolStarted, payloadToolStarted{ToolCallID: callID, ToolName: toolName}),
		m.build(domain.EventToolFinished, finished),
	}, nil
}

// setNudgeState shares the leg's detector with the mapper so typed
// failure marks resolve into tool.finished metadata.
func (m *eventMapper) setNudgeState(state *nudgeState) {
	m.nudge = state
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
	payloadVersion := 1
	switch terminal := payload.(type) {
	case payloadRunCompleted:
		payloadVersion = 2
		terminal.Outcome = "completed"
		terminal.Summary = m.lastSummary
		terminal.TenantID, terminal.WorkspaceID, terminal.SessionID, terminal.View = m.tenantID, m.workspaceID, m.sessionID, m.contextViewID
		payload = terminal
	case payloadRunFailed:
		payloadVersion = 2
		terminal.Outcome = "failed"
		terminal.TenantID, terminal.WorkspaceID, terminal.SessionID, terminal.View = m.tenantID, m.workspaceID, m.sessionID, m.contextViewID
		payload = terminal
	case payloadRunCancelled:
		payloadVersion = 2
		terminal.Outcome = "cancelled"
		terminal.TenantID, terminal.WorkspaceID, terminal.SessionID, terminal.View = m.tenantID, m.workspaceID, m.sessionID, m.contextViewID
		payload = terminal
	case payloadModelRequestV4:
		payloadVersion = 4
	case payloadModelUsageV2:
		payloadVersion = 2
	case payloadModelRequest:
		if terminal.ContextView != "" {
			payloadVersion = 2
		}
	}
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
		PayloadVersion: payloadVersion,
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
	payload.Action, payload.Target, payload.Preview, payload.RiskFindings = boundApprovalReviewFields(
		payload.Action, payload.Target, payload.Preview, payload.RiskFindings, budget,
	)
	encoded, _ := json.Marshal(payload)
	if len(encoded) <= budget {
		return payload
	}
	// Exact arguments remain in protected checkpoint state; the durable event
	// needs only a safe review projection and identity for resumption.
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
	proposal.Action, proposal.Target, proposal.Preview, proposal.RiskFindings = boundApprovalReviewFields(
		proposal.Action, proposal.Target, proposal.Preview, proposal.RiskFindings, budget,
	)
	return proposal
}

func boundApprovalReviewFields(action, target, preview string, risks []string, budget int) (string, string, string, []string) {
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
		return action, tools.RedactSensitive(target), tools.RedactSensitive(preview), boundedRisks
	}
	target = tools.RedactSensitive(target)
	preview = tools.RedactSensitive(preview)
	risks = append([]string(nil), risks...)
	for i := range risks {
		risks[i] = tools.RedactSensitive(risks[i])
	}
	action = clampEscapedText(action, 1024)
	target = clampEscapedText(target, 4096)
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
	return action, target, preview, boundedRisks
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
