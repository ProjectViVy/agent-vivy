// Package facerun is the shared non-interactive run engine for faces. It
// resolves a session, starts one turn through the face Host, subscribes to
// run events, verifies the assistant stream, cancels loudly when the run
// blocks on a human, and reports a terminal status. Faces plug in a Sink:
// the text sink renders pi-style print output, the JSONL sink projects the
// same stream into versioned records. The engine is identical either way.
package facerun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	faceport "agent-vivy/sdk/port/face"
)

// Options is the headless launch surface handed by a face Runner.
type Options struct {
	Prompt         string
	ContinueNewest bool
	Resume         bool   // -r: interactive picker, not a headless surface
	SessionID      string // --session-id: resume an explicit session
	Session        string // --session: named session selector (needs C1)
	Fork           string // --fork: fork-at-point selector (needs C1)
	SessionDir     string // --session-dir: custom journal root (needs C1)
	NoSession      bool   // --no-session: VIVY runs are always journaled
	Export         string // --export: session export (needs C1)
	Face           string // face attribute recorded on the turn
	Out, Err       io.Writer
}

// unsupported checks every session/run selector that has no honest headless
// behavior yet. These fail loudly instead of silently running in a different
// session than the user asked for.
func (o Options) unsupported() error {
	switch {
	case o.Export != "":
		return fmt.Errorf("%s: --export is not available yet (lands with session export in C1)", o.Face)
	case o.NoSession:
		return fmt.Errorf("%s: --no-session is not supported: every VIVY run is journaled", o.Face)
	case o.Resume:
		return fmt.Errorf("%s: -r/--resume is an interactive picker; use -c/--continue or --session-id in headless modes", o.Face)
	case o.Session != "":
		return fmt.Errorf("%s: --session is not available yet (lands with the session tree in C1); use --session-id", o.Face)
	case o.Fork != "":
		return fmt.Errorf("%s: --fork is not available yet (lands with the session tree in C1)", o.Face)
	case o.SessionDir != "":
		return fmt.Errorf("%s: --session-dir is not available yet (lands with the session tree in C1)", o.Face)
	}
	return nil
}

// Sink receives the semantic run stream. Implementations must be safe for
// calls from the engine's single notification goroutine; ordering is the
// journal's seq order.
type Sink interface {
	// AssistantDelta carries assistant text (byte framing checked
	// at model.completed before Run returns).
	AssistantDelta(text string)
	// AssistantMessageEnd fires after each verified model.completed.
	AssistantMessageEnd()
	ToolStarted(name string)
	ToolFinished(name, errText string)
	// ApprovalBlocked/QuestionBlocked report a run blocked on a human. The
	// engine has already cancelled the run; the sink only renders.
	ApprovalBlocked(notice string)
	QuestionBlocked(notice string)
	RunFailed(category, message string)
	RunCancelled()
	ProtocolError(msg string)
}

// RawSink, when implemented by the Sink, additionally receives every journal
// event verbatim (type, seq, raw payload) after semantic dispatch. JSONL mode
// uses it to emit lossless records.
type RawSink interface {
	JournalEvent(typ string, seq int64, payload json.RawMessage)
}

// LifecycleSink, when implemented by the Sink, receives the run's resolved
// identifiers and its terminal status so records outside the journal stream
// (session, turn_start, turn_end, agent_settled) can be emitted.
type LifecycleSink interface {
	RunStarted(sessionID, runID string)
	RunSettled(status string)
}

// Run drives opts.Prompt to a terminal status and returns the face Result.
func Run(ctx context.Context, env faceport.Host, opts Options, sink Sink) (faceport.Result, error) {
	if err := opts.unsupported(); err != nil {
		return faceport.Result{Status: "failed"}, err
	}
	prompt := strings.TrimSpace(opts.Prompt)
	if prompt == "" {
		return faceport.Result{}, fmt.Errorf("%s: prompt is empty", opts.Face)
	}
	e := &engine{env: env, opts: opts, sink: sink}
	if _, err := env.Call(ctx, "initialize", nil); err != nil {
		return faceport.Result{}, fmt.Errorf("%s: initialize: %w", opts.Face, err)
	}
	sessionID, err := e.resolveSession(ctx)
	if err != nil {
		return faceport.Result{}, err
	}
	turn, err := env.Call(ctx, "turn/start", map[string]any{
		"session_id": sessionID,
		"text":       prompt,
		"face":       opts.Face,
	})
	if err != nil {
		return faceport.Result{}, fmt.Errorf("%s: turn/start: %w", opts.Face, err)
	}
	var start struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(turn, &start); err != nil || start.RunID == "" {
		return faceport.Result{}, fmt.Errorf("%s: turn/start returned %s", opts.Face, turn)
	}
	e.runID = start.RunID
	if lc, ok := sink.(LifecycleSink); ok {
		lc.RunStarted(sessionID, start.RunID)
	}
	terminal := make(chan string, 1)
	streamErrors := make(chan error, 1)
	var subscriptionMu sync.Mutex
	subscriptionID := ""
	pendingErrors := make(map[string]error)
	publishStreamError := func(err error) {
		select {
		case streamErrors <- err:
		default:
		}
	}
	env.OnEvent(func(method string, params json.RawMessage) {
		switch method {
		case "run/event":
			e.onEvent(params, terminal)
		case "run/stream_error":
			var failure struct {
				SubscriptionID string `json:"subscription_id"`
				Message        string `json:"message"`
			}
			if json.Unmarshal(params, &failure) != nil || failure.SubscriptionID == "" {
				return
			}
			err := fmt.Errorf("run stream: %s", failure.Message)
			subscriptionMu.Lock()
			if subscriptionID == "" {
				// A synchronous Host can deliver replay failure before Call returns.
				if len(pendingErrors) >= 4 {
					for id := range pendingErrors {
						delete(pendingErrors, id)
						break
					}
				}
				pendingErrors[failure.SubscriptionID] = err
			} else if failure.SubscriptionID == subscriptionID {
				publishStreamError(err)
			}
			subscriptionMu.Unlock()
		}
	})
	subscribed, err := env.Call(ctx, "run/subscribe", map[string]any{"run_id": start.RunID})
	if err != nil {
		return faceport.Result{}, fmt.Errorf("%s: run/subscribe: %w", opts.Face, err)
	}
	var subscription struct {
		ID string `json:"subscription_id"`
	}
	if err := json.Unmarshal(subscribed, &subscription); err != nil {
		return faceport.Result{}, fmt.Errorf("decode run/subscribe: %w", err)
	}
	if subscription.ID == "" {
		return faceport.Result{}, fmt.Errorf("run/subscribe: missing subscription_id")
	}
	subscriptionMu.Lock()
	subscriptionID = subscription.ID
	if err := pendingErrors[subscriptionID]; err != nil {
		publishStreamError(err)
	}
	pendingErrors = nil
	subscriptionMu.Unlock()
	failStream := func(err error) (faceport.Result, error) {
		sink.ProtocolError(err.Error())
		return faceport.Result{Status: "failed"}, fmt.Errorf("%s: %w", opts.Face, err)
	}
	// A synchronous replay may already have supplied a failure and a terminal.
	select {
	case err := <-streamErrors:
		return failStream(err)
	default:
	}

	select {
	case err := <-streamErrors:
		return failStream(err)
	case status := <-terminal:
		select {
		case err := <-streamErrors:
			return failStream(err)
		default:
		}
		if lc, ok := sink.(LifecycleSink); ok {
			lc.RunSettled(status)
		}
		return faceport.Result{Status: status}, nil
	case <-ctx.Done():
		return faceport.Result{}, fmt.Errorf("%s: %w", opts.Face, ctx.Err())
	}
}

type engine struct {
	env    faceport.Host
	opts   Options
	sink   Sink
	runID  string
	cancel sync.Once

	stateMu        sync.Mutex
	lastEventSeq   int64
	completionLen  int
	protocolErr    string
	streamed       atomic.Bool
	protocolFailed atomic.Bool
}

// resolveSession picks the session the prompt lands in: an explicit
// --session-id, the newest one with --continue, or a fresh session titled by
// the prompt.
func (e *engine) resolveSession(ctx context.Context) (string, error) {
	if id := strings.TrimSpace(e.opts.SessionID); id != "" {
		return id, nil
	}
	if e.opts.ContinueNewest {
		raw, err := e.env.Call(ctx, "session/list", nil)
		if err != nil {
			return "", fmt.Errorf("%s: session/list: %w", e.opts.Face, err)
		}
		var list struct {
			Sessions []struct {
				ID string `json:"id"`
			} `json:"sessions"`
		}
		if err := json.Unmarshal(raw, &list); err != nil {
			return "", fmt.Errorf("%s: decode session/list: %w", e.opts.Face, err)
		}
		if len(list.Sessions) == 0 {
			return "", fmt.Errorf("%s: no sessions to continue; run without --continue to start one", e.opts.Face)
		}
		return list.Sessions[0].ID, nil
	}
	title := strings.TrimSpace(e.opts.Prompt)
	if runes := []rune(title); len(runes) > 60 {
		title = string(runes[:60]) + "..."
	}
	raw, err := e.env.Call(ctx, "session/create", map[string]any{"title": title})
	if err != nil {
		return "", fmt.Errorf("%s: session/create: %w", e.opts.Face, err)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &created); err != nil || created.ID == "" {
		return "", fmt.Errorf("%s: session/create returned %s", e.opts.Face, raw)
	}
	return created.ID, nil
}

// wireEvent is one run/event notification: the server wraps the journal
// event in {"subscription_id": ..., "event": {...}}.
type wireEvent struct {
	Event struct {
		RunID          string          `json:"run_id"`
		Seq            int64           `json:"seq"`
		Type           string          `json:"type"`
		PayloadVersion int             `json:"payload_version"`
		Payload        json.RawMessage `json:"payload"`
	} `json:"event"`
}

func (e *engine) onEvent(params json.RawMessage, terminal chan<- string) {
	var wire wireEvent
	if err := json.Unmarshal(params, &wire); err != nil || wire.Event.RunID != e.runID {
		return
	}
	e.stateMu.Lock()
	if wire.Event.Seq <= e.lastEventSeq {
		e.stateMu.Unlock()
		return
	}
	e.lastEventSeq = wire.Event.Seq
	e.stateMu.Unlock()
	payload := wire.Event.Payload
	if raw, ok := e.sink.(RawSink); ok {
		raw.JournalEvent(wire.Event.Type, wire.Event.Seq, payload)
	}
	switch wire.Event.Type {
	case "model.request", "tool.requested":
		e.stateMu.Lock()
		if e.protocolErr == "" {
			e.resetCompletionLocked()
		}
		e.stateMu.Unlock()
	case "model.delta":
		e.stateMu.Lock()
		if e.protocolErr == "" {
			delta, err := parseDelta(payload)
			if err != nil {
				e.failProtocolLocked(err)
			} else if delta != "" {
				e.completionLen += len([]byte(delta))
				e.sink.AssistantDelta(delta)
				e.streamed.Store(true)
			}
		}
		e.stateMu.Unlock()
	case "model.completed":
		e.stateMu.Lock()
		streamed := e.streamed.Swap(false)
		if e.protocolErr == "" {
			if err := e.completeModelLocked(wire.Event.PayloadVersion, payload); err != nil {
				e.failProtocolLocked(err)
			} else if streamed {
				e.sink.AssistantMessageEnd()
			}
		}
		e.stateMu.Unlock()
	case "tool.started":
		var p struct {
			ToolName string `json:"tool_name"`
		}
		if json.Unmarshal(payload, &p) == nil {
			e.sink.ToolStarted(p.ToolName)
		}
	case "tool.finished":
		var p struct {
			ToolName string `json:"tool_name"`
			Error    string `json:"error"`
		}
		if json.Unmarshal(payload, &p) == nil {
			e.sink.ToolFinished(p.ToolName, p.Error)
		}
	case "tool.approval_required":
		e.cancelLoudly(terminal, func() {
			var p struct {
				ToolName string `json:"tool_name"`
				Action   string `json:"action"`
				Target   string `json:"target"`
			}
			_ = json.Unmarshal(payload, &p)
			e.sink.ApprovalBlocked(fmt.Sprintf("vivy: blocked: %s (%s %s) requires human approval and the headless face cannot ask for it. Approve in the web UI or adjust the permission preset; the run was cancelled.\n", p.ToolName, p.Action, p.Target))
		})
	case "user.question_required":
		e.cancelLoudly(terminal, func() {
			var p struct {
				Prompt string `json:"prompt"`
			}
			_ = json.Unmarshal(payload, &p)
			e.sink.QuestionBlocked(fmt.Sprintf("vivy: blocked: the model asked %q and the headless face cannot answer. Re-run in the web UI; the run was cancelled.\n", p.Prompt))
		})
	case "run.failed":
		var p struct {
			CauseCategory string `json:"cause_category"`
			Message       string `json:"message"`
		}
		_ = json.Unmarshal(payload, &p)
		e.sink.RunFailed(p.CauseCategory, p.Message)
		emitTerminal(terminal, "failed")
	case "run.cancelled":
		e.sink.RunCancelled()
		emitTerminal(terminal, "cancelled")
	case "run.completed":
		if e.protocolFailed.Load() {
			emitTerminal(terminal, "failed")
		} else {
			emitTerminal(terminal, "completed")
		}
	}
}

func (e *engine) resetCompletionLocked() {
	e.completionLen = 0
}

// Keep the non-interactive Face independent of the TUI renderer package.
func parseDelta(payload json.RawMessage) (string, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return "", fmt.Errorf("decode model.delta: %w", err)
	}
	raw, ok := fields["delta"]
	if len(fields) != 1 || !ok || string(raw) == "null" {
		return "", fmt.Errorf("model.delta: invalid payload")
	}
	var delta string
	if err := json.Unmarshal(raw, &delta); err != nil {
		return "", fmt.Errorf("decode model.delta text: %w", err)
	}
	if !utf8.ValidString(delta) {
		return "", fmt.Errorf("model.delta: invalid UTF-8")
	}
	return delta, nil
}

func (e *engine) completeModelLocked(version int, payload json.RawMessage) error {
	if version != 3 {
		return fmt.Errorf("unsupported model.completed payload version %d", version)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return fmt.Errorf("decode model.completed v3: %w", err)
	}
	raw, ok := fields["byte_len"]
	if len(fields) != 1 || !ok || string(raw) == "null" {
		return fmt.Errorf("model.completed v3: invalid byte_len fields")
	}
	var byteLen int
	if err := json.Unmarshal(raw, &byteLen); err != nil {
		return fmt.Errorf("decode model.completed byte_len: %w", err)
	}
	if byteLen < 0 || byteLen != e.completionLen {
		return fmt.Errorf("model.completed v3: byte_len mismatch")
	}
	e.resetCompletionLocked()
	return nil
}

func (e *engine) failProtocolLocked(err error) {
	if e.protocolErr != "" {
		return
	}
	message := "invalid stream protocol"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	e.protocolErr = message
	e.protocolFailed.Store(true)
	e.sink.ProtocolError(message)
}

// cancelLoudly reports the block through the sink and cancels the run once —
// the durable cancelled terminal then arrives through the subscription.
func (e *engine) cancelLoudly(terminal chan<- string, report func()) {
	e.cancel.Do(func() {
		report()
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, _ = e.env.Call(ctx, "run/cancel", map[string]any{"run_id": e.runID})
		}()
	})
}

func emitTerminal(terminal chan<- string, status string) {
	select {
	case terminal <- status:
	default:
	}
}
