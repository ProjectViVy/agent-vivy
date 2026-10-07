package acp

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	acp "github.com/eino-contrib/acp"
)

// Spec bounds (§10): four active prompts per connection (one per session —
// the session slot enforces that), a 5 s cancel latch TTL, 5 s missing
// sequence gap, and a 256-event ingress queue per run.
const (
	maxActivePrompts = 4
	eventIngressCap  = 256
)

// Timers are vars so tests can shrink the windows deterministically.
var (
	cancelLatchTTL    = 5 * time.Second
	missingSeqTimeout = 5 * time.Second
)

// promptScope correlates one owned prompt with its durable run (spec: the
// generation token so a late response cannot mutate a newer prompt).
type promptScope struct {
	SessionID  string
	RunID      string
	Generation uint64
}

// committedEvent is the decoded `run/event` envelope payload from Control.
type committedEvent struct {
	RunID          string          `json:"run_id"`
	Seq            int64           `json:"seq"`
	Type           string          `json:"type"`
	PayloadVersion int             `json:"payload_version"`
	Payload        json.RawMessage `json:"payload"`
}

// pendingEvent is a `run/event` that arrived before its subscription_id was
// bound to a prompt; it is keyed by run id until run/subscribe returns.
type pendingEvent struct {
	subID string
	ev    committedEvent
}

// promptState is the correlation state of one owned prompt — never a second
// run store (spec §5).
type promptState struct {
	scope promptScope

	mu                sync.Mutex
	clientCancel      bool // a client cancel was accepted for this prompt
	interactionCancel bool // an interaction event cancelled this run (ACP-04 handoff)
	subID             string
	decided           bool
	result            chan promptResult
	inbox             chan committedEvent
	overflow          chan struct{}
	once              sync.Once

	// Bounded ingress accounting (spec §10): unprocessed events in the
	// inbox and the reorder buffer count together.
	pendingEvents atomic.Int64
	pendingBytes  atomic.Int64

	// updates records projected session updates in emission order; tests
	// read it, and emitUpdates writes each entry to the wire when a
	// connection is bound.
	updates []acp.SessionUpdate
}

type promptResult struct {
	resp acp.PromptResponse
	err  *acp.RPCError
}

func (p *promptState) setRunID(id string) {
	p.mu.Lock()
	p.scope.RunID = id
	p.mu.Unlock()
}

func (p *promptState) setSubID(id string) {
	p.mu.Lock()
	p.subID = id
	p.mu.Unlock()
}

func (p *promptState) subscriptionID() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.subID
}

// deliver enqueues one inbound event; it never blocks the notification
// handler (spec §7). Events exceeding the per-run ingress/reorder budget or
// a full queue flag overflow — the reducer converts it into a
// stream-integrity failure.
func (p *promptState) deliver(ev committedEvent) {
	cost := int64(len(ev.Payload) + eventOverhead)
	if p.pendingEvents.Add(1) > maxBufferedEvents ||
		p.pendingBytes.Add(cost) > maxEventBytes {
		p.pendingEvents.Add(-1)
		p.pendingBytes.Add(-cost)
		p.flagOverflow()
		return
	}
	select {
	case p.inbox <- ev:
	default:
		p.pendingEvents.Add(-1)
		p.pendingBytes.Add(-cost)
		p.flagOverflow()
	}
}

func (p *promptState) flagOverflow() {
	p.once.Do(func() { close(p.overflow) })
}

// releaseEvent returns an ingress slot once the reducer has consumed it.
func (p *promptState) releaseEvent(ev committedEvent) {
	p.pendingEvents.Add(-1)
	p.pendingBytes.Add(-int64(len(ev.Payload) + eventOverhead))
}

// emitUpdates serializes projected updates: each is recorded on the prompt
// and written through the SDK writer in order (spec §7 step 6). A write
// failure kills the connection — an unwritable transport emits no
// fabricated response (spec §9).
func (a *agent) emitUpdates(ctx context.Context, p *promptState, updates []acp.SessionUpdate) bool {
	// Split oversized text chunks at rune boundaries so each encoded frame
	// stays under the 256 KiB bound (spec §10).
	var expanded []acp.SessionUpdate
	for _, u := range updates {
		if u.AgentMessageChunk != nil && u.AgentMessageChunk.Content.Text != nil {
			for _, piece := range splitChunk(p.scope.SessionID, u.AgentMessageChunk.Content.Text.Text) {
				expanded = append(expanded, textChunkUpdate(piece))
			}
			continue
		}
		expanded = append(expanded, u)
	}
	updates = expanded
	for _, u := range updates {
		p.mu.Lock()
		p.updates = append(p.updates, u)
		p.mu.Unlock()
		a.mu.Lock()
		conn := a.conn
		a.mu.Unlock()
		if conn == nil {
			continue // unit tests record updates without a transport
		}
		if err := conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionID: acp.SessionID(p.scope.SessionID),
			Update:    u,
		}); err != nil {
			a.drain()
			p.decide(promptResult{err: rpcError(-32603, "transport write failed", "INTERNAL_FAILURE")})
			return false
		}
	}
	return true
}

func (p *promptState) decide(r promptResult) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.decided {
		return
	}
	p.decided = true
	p.result <- r
}

// turnStartParams is the private Control DTO for turn/start — only the
// fields this adapter uses (source: internal/rpc/control.go turnParams).
type turnStartParams struct {
	SessionID    string   `json:"session_id"`
	Text         string   `json:"text"`
	Face         string   `json:"face,omitempty"`
	ContextPaths []string `json:"context_paths,omitempty"`
}

// Prompt owns one run from admission to finalization (spec §5-§9): validate
// fully, reserve capacity atomically, turn/start once, subscribe with a
// pre-registered event route, project committed events, then return exactly
// one final response.
func (a *agent) Prompt(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
	if !a.isInitialized() {
		return acp.PromptResponse{},
			rpcError(-32602, "connection is not initialized", "INVALID_INPUT")
	}
	s, err := a.lookupSession(string(req.SessionID))
	if err != nil {
		return acp.PromptResponse{}, err
	}
	text, contextPaths, err := normalizePrompt(s.root, req)
	if err != nil {
		return acp.PromptResponse{}, err
	}

	// Atomic admission: session slot + global cap under a.mu, consuming a
	// fresh cancel latch when present.
	a.mu.Lock()
	if a.draining {
		a.mu.Unlock()
		return acp.PromptResponse{}, rpcError(-32603, "connection is draining", "INTERNAL_FAILURE")
	}
	a.mu.Unlock()

	s.mu.Lock()
	if s.active != nil {
		s.mu.Unlock()
		return acp.PromptResponse{}, rpcError(-32001, "session has an active prompt", "SESSION_BUSY")
	}
	a.mu.Lock()
	if a.activePrompts >= maxActivePrompts {
		a.mu.Unlock()
		s.mu.Unlock()
		return acp.PromptResponse{}, rpcError(-32001, "prompt capacity reached", "CAPACITY_EXCEEDED")
	}
	a.activePrompts++
	a.mu.Unlock()

	gen := s.nextGeneration + 1
	s.nextGeneration = gen
	p := &promptState{
		scope:    promptScope{SessionID: string(req.SessionID), Generation: gen},
		result:   make(chan promptResult, 1),
		inbox:    make(chan committedEvent, eventIngressCap),
		overflow: make(chan struct{}),
	}
	if !s.lastCancelAt.IsZero() && time.Since(s.lastCancelAt) <= cancelLatchTTL {
		p.mu.Lock()
		p.clientCancel = true
		p.mu.Unlock()
	}
	s.lastCancelAt = time.Time{}
	s.active = p
	s.mu.Unlock()

	defer a.finalizePrompt(s, p)

	// turn/start: one durable run; ambiguous admission drains the
	// connection rather than resubmitting (spec §6).
	raw, err := a.call(ctx, "turn/start", turnStartParams{
		SessionID:    string(req.SessionID),
		Text:         text,
		Face:         "code",
		ContextPaths: contextPaths,
	})
	if err != nil {
		if _, coded := rpcCodeOf(err); !coded {
			a.drain()
		}
		return acp.PromptResponse{}, safeRPCError(err)
	}
	var started struct {
		RunID  string `json:"run_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &started); err != nil || started.RunID == "" {
		a.drain()
		return acp.PromptResponse{}, rpcError(-32603, "malformed turn result", "INTERNAL_FAILURE")
	}
	p.setRunID(started.RunID)

	// Register the run-id buffer route BEFORE run/subscribe so events
	// racing the response have somewhere to land (spec §7 step 1).
	a.mu.Lock()
	a.pending[started.RunID] = nil
	a.mu.Unlock()

	// A latched cancel applies as soon as the run ID is known.
	if p.isClientCancelled() {
		_, _ = a.call(ctx, "run/cancel", map[string]string{"run_id": started.RunID})
	}

	subRaw, err := a.call(ctx, "run/subscribe", map[string]any{"run_id": started.RunID, "after_seq": 0})
	if err != nil {
		// The run exists; fail it so nothing dangles.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), controlCallTimeout)
		_, _ = a.call(cleanupCtx, "run/cancel", map[string]string{"run_id": started.RunID})
		cancel()
		return acp.PromptResponse{}, safeRPCError(err)
	}
	var sub struct {
		SubscriptionID string `json:"subscription_id"`
		RunID          string `json:"run_id"`
	}
	if err := json.Unmarshal(subRaw, &sub); err != nil || sub.SubscriptionID == "" ||
		(sub.RunID != "" && sub.RunID != started.RunID) {
		a.drain()
		return acp.PromptResponse{}, rpcError(-32603, "malformed subscribe result", "INTERNAL_FAILURE")
	}
	p.setSubID(sub.SubscriptionID)
	a.bindRoute(sub.SubscriptionID, started.RunID, p)

	go a.reduce(p, ctx)

	select {
	case out := <-p.result:
		if out.err != nil {
			return acp.PromptResponse{}, out.err
		}
		return out.resp, nil
	case <-ctx.Done():
		// Unwritable transport: the wire answer cannot reach the client;
		// return the sanitized internal error rather than leaking ctx.Err().
		return acp.PromptResponse{}, rpcError(-32603, "prompt aborted: transport closed", "INTERNAL_FAILURE")
	}
}

func (p *promptState) isClientCancelled() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.clientCancel
}

func (p *promptState) isInteractionCancelled() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.interactionCancel
}

func endTurnResponse() acp.PromptResponse {
	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}
}

func cancelledResponse() acp.PromptResponse {
	return acp.PromptResponse{StopReason: acp.StopReasonCancelled}
}

func cancelledRunError() *acp.RPCError {
	return acp.ErrRequestCanceled("run cancelled")
}

// finalizePrompt releases exactly once: unsubscribe the owned subscription,
// free the session slot and the global capacity.
func (a *agent) finalizePrompt(s *sessionState, p *promptState) {
	subID := p.subscriptionID()
	if subID != "" {
		a.mu.Lock()
		delete(a.routes, subID)
		a.mu.Unlock()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), controlCallTimeout)
		_, _ = a.call(cleanupCtx, "run/unsubscribe", map[string]string{"subscription_id": subID})
		cancel()
	}
	s.mu.Lock()
	if s.active == p {
		s.active = nil
	}
	s.mu.Unlock()
	a.mu.Lock()
	a.activePrompts--
	delete(a.pending, p.scope.RunID)
	a.mu.Unlock()
}

// bindRoute attaches the subscription to the prompt and replays buffered
// events whose subscription_id matches; a mismatched buffered event is a
// foreign notification and is dropped (spec §7 step 2).
func (a *agent) bindRoute(subID, runID string, p *promptState) {
	a.mu.Lock()
	a.routes[subID] = p
	buffered := a.pending[runID]
	overflowed := a.pendingOverflow[runID]
	delete(a.pending, runID)
	delete(a.pendingOverflow, runID)
	a.mu.Unlock()
	for _, b := range buffered {
		if b.subID == subID {
			p.deliver(b.ev)
		}
	}
	if overflowed {
		p.flagOverflow()
	}
}

// onEvent routes Control notifications to owned prompts; only run/event is
// consumed. It never blocks on prompt state.
func (a *agent) onEvent(method string, params json.RawMessage) {
	if method == "run/stream_error" {
		var note struct {
			SubscriptionID string `json:"subscription_id"`
		}
		if err := json.Unmarshal(params, &note); err != nil || note.SubscriptionID == "" {
			return
		}
		a.mu.Lock()
		p := a.routes[note.SubscriptionID]
		a.mu.Unlock()
		if p != nil {
			a.failStream(p, "run event stream error")
		}
		return
	}
	if method != "run/event" {
		return
	}
	var note struct {
		SubscriptionID string         `json:"subscription_id"`
		Event          committedEvent `json:"event"`
	}
	if err := json.Unmarshal(params, &note); err != nil || note.SubscriptionID == "" {
		return
	}
	a.mu.Lock()
	p := a.routes[note.SubscriptionID]
	if p == nil && note.Event.RunID != "" {
		buf := a.pending[note.Event.RunID]
		if len(buf) >= maxBufferedEvents {
			a.pendingOverflow[note.Event.RunID] = true
		} else {
			a.pending[note.Event.RunID] = append(buf,
				pendingEvent{subID: note.SubscriptionID, ev: note.Event})
		}
	}
	a.mu.Unlock()
	if p != nil {
		p.deliver(note.Event)
	}
}

// SessionCancel stamps the unconditional latch on an owned session and, for
// an active prompt, marks the cancel and forwards run/cancel once the run
// ID exists. Unknown sessions are a no-op (spec §8).
func (a *agent) SessionCancel(_ context.Context, req acp.CancelNotification) error {
	a.mu.Lock()
	s := a.sessions[string(req.SessionID)]
	a.mu.Unlock()
	if s == nil {
		return nil
	}
	s.mu.Lock()
	p := s.active
	if p == nil {
		// No active prompt: latch the cancel so a prompt admitted within
		// the TTL window consumes it (the accepted ordering contract).
		// When a prompt is already active the cancel applies to it
		// directly — arming the latch on top would wrongly cancel the
		// NEXT prompt too.
		s.lastCancelAt = time.Now()
	}
	s.mu.Unlock()
	if p == nil {
		return nil
	}
	p.mu.Lock()
	p.clientCancel = true
	runID := p.scope.RunID
	p.mu.Unlock()
	if runID != "" {
		// No response rides on a notification; forward the cancel outside
		// any lock.
		ctx, cancel := context.WithTimeout(context.Background(), controlCallTimeout)
		_, _ = a.call(ctx, "run/cancel", map[string]string{"run_id": runID})
		cancel()
	}
	return nil
}
