package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// terminalPersistTimeout caps the detached persistence window for the
// terminal event: closing a run durably must survive the user cancelling
// their own context, but must also stay bounded (NFR: bounded).
const terminalPersistTimeout = 5 * time.Second

// EventSink receives persisted run events for live fan-out. events.Bus
// implements it; the interface lives here so the runtime never imports
// the events package (app wires the two together).
type EventSink interface {
	Publish(domain.RunEvent)
}

// DecideApproval error sentinels; the API layer maps them to HTTP
// semantics (404 / 409; D-009 stays server-enforced).
var (
	ErrApprovalNotFound        = errors.New("runtime: approval not found")
	ErrApprovalInvalidDecision = errors.New("runtime: approval decision must be approved or denied")
	ErrApprovalAlreadyDecided  = errors.New("runtime: approval already decided")
	ErrApprovalExpired         = errors.New("runtime: approval expired")
)

// ServiceDeps groups the storage and fan-out dependencies of Service.
type ServiceDeps struct {
	Journal  storage.Journal
	Runs     storage.RunStore
	Messages storage.MessageStore
	// Approvals persists the approval rows behind the effectful tool
	// gate (C6); nil leaves interrupts unable to suspend.
	Approvals storage.ApprovalStore
	// ApprovalExpiration bounds how long a pending approval stays valid
	// (D-009).
	ApprovalExpiration time.Duration
	Sink               EventSink
}

// Service orchestrates runs: it persists the user message and the
// accepted run row, journals run.started before driving the engine, and
// persists each mapped event BEFORE publishing it (durability precedes
// visibility). The run's context is detached from the request: an SSE
// disconnect or page refresh must never kill the run (AS-7); only
// Cancel / CancelAll do.
type Service struct {
	engine   *Engine
	deps     ServiceDeps
	provider string
	modelID  string

	mu     sync.Mutex
	active map[domain.RunID]context.CancelFunc
	// pending tracks runs suspended on an approval: no engine work is in
	// flight, the checkpoint is durable, and the run row stays active
	// until a decision resumes it or Cancel closes it (C6).
	pending map[domain.RunID]pendingRun
}

type pendingRun struct {
	sessionID domain.SessionID
	mapper    *eventMapper
}

// NewService wires the run service over an engine and its dependencies.
// provider and modelID label the run.started payload.
func NewService(eng *Engine, provider, modelID string, deps ServiceDeps) *Service {
	return &Service{
		engine:   eng,
		deps:     deps,
		provider: provider,
		modelID:  modelID,
		active:   make(map[domain.RunID]context.CancelFunc),
		pending:  make(map[domain.RunID]pendingRun),
	}
}

// Run starts one run for the session and returns its id after the write
// sequence is durable: user message -> run row (accepted) -> run.started
// in the journal -> run row (active). The engine drive then proceeds
// asynchronously on a detached context.
func (s *Service) Run(ctx context.Context, sessionID domain.SessionID, userText string) (domain.RunID, error) {
	if s.engine == nil || s.deps.Journal == nil || s.deps.Runs == nil || s.deps.Messages == nil || s.deps.Sink == nil {
		return "", errors.New("runtime: service not wired")
	}
	runID := newRunID()
	now := time.Now().UnixMilli()

	if err := s.deps.Messages.AppendMessage(ctx, domain.Message{
		ID:        newMessageID(),
		SessionID: sessionID,
		Role:      domain.RoleUser,
		CreatedAt: now,
		Content:   userText,
	}); err != nil {
		return "", fmt.Errorf("runtime: append user message: %w", err)
	}
	if err := s.deps.Runs.CreateRun(ctx, domain.Run{
		ID: runID, SessionID: sessionID, Status: domain.RunAccepted, CreatedAt: now,
	}); err != nil {
		return "", fmt.Errorf("runtime: create run: %w", err)
	}

	m := newEventMapper(runID, s.engine.cfg.MaxEventPayloadBytes)
	started := m.build(domain.EventRunStarted, payloadRunStarted{Provider: s.provider, Model: s.modelID})
	seq, err := s.deps.Journal.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{started}})
	if err != nil {
		return "", fmt.Errorf("runtime: persist run.started: %w", err)
	}
	started.Seq = seq
	s.deps.Sink.Publish(started)

	if err := s.deps.Runs.SetRunStatus(ctx, runID, domain.RunActive); err != nil {
		return "", fmt.Errorf("runtime: activate run: %w", err)
	}

	// Detach the run from the request lifecycle: SSE disconnects and page
	// refreshes must not cancel the work (AS-7). Cancel/CancelAll hold the
	// only handles that end it early.
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s.mu.Lock()
	s.active[runID] = cancel
	s.mu.Unlock()

	go s.drive(runCtx, m, sessionID, userText)
	return runID, nil
}

// Cancel ends an in-flight run of this process. It reports false when the
// run is not active here (unknown, already terminal, or another process).
// Runs suspended on an approval close directly: no engine work is in
// flight for them (minimal E1 coverage).
func (s *Service) Cancel(runID domain.RunID) bool {
	s.mu.Lock()
	cancel, active := s.active[runID]
	p, isPending := s.pending[runID]
	if isPending {
		delete(s.pending, runID)
	}
	s.mu.Unlock()

	if isPending {
		s.emitTerminal(context.Background(), p.mapper,
			p.mapper.build(domain.EventRunCancelled, payloadRunCancelled{Reason: reasonUserRequested}))
		return true
	}
	if !active {
		return false
	}
	cancel() // the drive closes the run via the run.cancelled path
	return true
}

// CancelAll cancels every in-flight run. Shutdown calls it before closing
// storage so terminal events can still be persisted (E4 hardens the
// ordering guarantees). Runs pending on approvals close the same way.
func (s *Service) CancelAll() {
	s.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.active))
	for _, c := range s.active {
		cancels = append(cancels, c)
	}
	pendingIDs := make([]domain.RunID, 0, len(s.pending))
	for id := range s.pending {
		pendingIDs = append(pendingIDs, id)
	}
	s.mu.Unlock()
	for _, id := range pendingIDs {
		s.Cancel(id)
	}
	for _, c := range cancels {
		c()
	}
}

func (s *Service) drive(ctx context.Context, m *eventMapper, sessionID domain.SessionID, userText string) {
	// The checkpoint id is derived from the run id so Query and Resume
	// always agree without a second assignment (spike §2.1: without
	// WithCheckPointID an interrupt persists no checkpoint).
	iter := s.engine.Query(ctx, userText, adk.WithCheckPointID(checkpointIDFor(m.runID)))
	s.consume(ctx, m, sessionID, iter)
}

// consume maps engine events into the journal until the iterator closes,
// then emits run.completed. Interrupts suspend the run without a terminal
// event; any other error closes it via the matching terminal. Both the
// first drive and approval resumes go through here, so every run closes
// exactly once (D-008).
func (s *Service) consume(ctx context.Context, m *eventMapper, sessionID domain.SessionID, iter *adk.AsyncIterator[*adk.AgentEvent]) {
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		events, err := m.onEvent(ev)
		if errors.Is(err, errRunInterrupted) {
			s.handleInterrupt(ctx, m, sessionID)
			return
		}
		if err != nil {
			s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, err))
			return
		}
		for _, re := range events {
			if !s.persistAndPublish(ctx, sessionID, re) {
				return
			}
		}
	}

	for _, re := range m.onTurnEnd() {
		if !s.persistAndPublish(ctx, sessionID, re) {
			return
		}
	}
	s.emitTerminal(ctx, m, m.build(domain.EventRunCompleted, payloadRunCompleted{}))
}

// handleInterrupt suspends the run on a server-side approval (D-029 write
// order): verify the checkpoint is readable, persist the pending approval
// row, commit the single tool.approval_required event, publish it, and
// register the run as pending. The run row stays active and no terminal
// event is emitted; DecideApproval (or Cancel) closes it later.
func (s *Service) handleInterrupt(ctx context.Context, m *eventMapper, sessionID domain.SessionID) {
	runID := m.runID
	fail := func(err error) {
		slog.Warn("interrupt handling failed", "run", string(runID), "err", err)
		s.emitTerminal(ctx, m, m.build(domain.EventRunFailed, payloadRunFailed{
			CauseCategory: causeInternalError,
			Message:       "The run could not be paused for approval. Please try again.",
		}))
	}

	details := m.interrupt
	switch {
	case details == nil || details.ResumeTarget == "":
		fail(errors.New("interrupt without a root-cause resume target"))
		return
	case s.engine.cfg.Checkpoints == nil:
		fail(errors.New("checkpoint bridge not wired"))
		return
	case s.deps.Approvals == nil:
		fail(errors.New("approval store not wired"))
		return
	}
	// The suspend commit is detached from the run context: a concurrent
	// Cancel must not strand the run half-suspended; the ctx check after
	// the commit routes it to run.cancelled instead.
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistTimeout)
	defer cancel()
	if _, ok, err := s.engine.cfg.Checkpoints.Get(persistCtx, checkpointIDFor(runID)); err != nil || !ok {
		fail(fmt.Errorf("checkpoint not readable: ok=%v: %w", ok, err))
		return
	}

	expiresAt := time.Now().Add(s.deps.ApprovalExpiration).UnixMilli()
	approval := domain.Approval{
		ID:           newPrefixedID("apr_"),
		RunID:        runID,
		ToolCallID:   details.ToolCallID,
		Decision:     domain.ApprovalPending,
		ExpiresAt:    expiresAt,
		ResumeTarget: details.ResumeTarget,
	}
	if err := s.deps.Approvals.CreateApproval(persistCtx, approval); err != nil {
		fail(err)
		return
	}

	ev := m.build(domain.EventToolApprovalRequired, payloadToolApprovalRequired{
		ApprovalID: approval.ID,
		ToolCallID: details.ToolCallID,
		ToolName:   details.ToolName,
		Args:       details.Args,
		ExpiresAt:  expiresAt,
	})
	seq, err := s.deps.Journal.Append(persistCtx, storage.Commit{RunID: runID, Events: []domain.RunEvent{ev}})
	if err != nil {
		fail(err)
		return
	}
	ev.Seq = seq
	s.deps.Sink.Publish(ev)

	if ctx.Err() != nil {
		// Cancelled while suspending: close as cancelled. The approval
		// row stays pending in the store; a later decision finds no
		// pending run and stands as a no-op resume.
		s.emitTerminal(ctx, m, m.build(domain.EventRunCancelled, payloadRunCancelled{Reason: reasonUserRequested}))
		return
	}

	s.mu.Lock()
	s.pending[runID] = pendingRun{sessionID: sessionID, mapper: m}
	s.mu.Unlock()
}

// DecideApproval settles a pending approval and resumes the suspended run
// with the decision (FR-6, FR-7). First-writer-wins: a concurrent second
// decision loses with ErrApprovalAlreadyDecided. The resume runs in the
// background; the caller only learns whether the decision was accepted.
func (s *Service) DecideApproval(ctx context.Context, approvalID, decision string) error {
	if decision != domain.ApprovalApproved && decision != domain.ApprovalDenied {
		return ErrApprovalInvalidDecision
	}
	if s.deps.Approvals == nil {
		return errors.New("runtime: approval store not wired")
	}
	approval, err := s.deps.Approvals.GetApproval(ctx, approvalID)
	if errors.Is(err, storage.ErrNotFound) {
		return ErrApprovalNotFound
	}
	if err != nil {
		return fmt.Errorf("runtime: get approval: %w", err)
	}
	if approval.Decision != domain.ApprovalPending {
		return ErrApprovalAlreadyDecided
	}
	if time.Now().UnixMilli() >= approval.ExpiresAt {
		return ErrApprovalExpired
	}
	decided, err := s.deps.Approvals.DecideApproval(ctx, approvalID, decision)
	if err != nil {
		return fmt.Errorf("runtime: decide approval: %w", err)
	}
	if !decided {
		// Lost the first-writer-wins race to a concurrent decision.
		return ErrApprovalAlreadyDecided
	}

	s.mu.Lock()
	p, ok := s.pending[approval.RunID]
	delete(s.pending, approval.RunID)
	s.mu.Unlock()
	if !ok {
		// The approval outlived its run in this process (cancelled or
		// restarted): the decision stands, nothing resumes (E2 recovers).
		slog.Warn("approval decided without a pending run", "approval", approvalID, "run", string(approval.RunID))
		return nil
	}

	// Recover the tool name from the suspended mapper so the resumed
	// tool result events keep their call identity.
	toolName := ""
	for _, oc := range p.mapper.openCalls {
		if oc.id == approval.ToolCallID {
			toolName = oc.name
			break
		}
	}

	go s.resumeRun(p.sessionID, toolName, approval, decision)
	return nil
}

// resumeRun feeds the decision back into the engine and maps the resumed
// events into the same journal (the journal continues the seq).
func (s *Service) resumeRun(sessionID domain.SessionID, toolName string, approval domain.Approval, decision string) {
	runID := approval.RunID
	m := newEventMapper(runID, s.engine.cfg.MaxEventPayloadBytes)
	if approval.ToolCallID != "" {
		// The resume replays the decided tool result first; seed the open
		// call so reconstructed tool.started/finished keep the call id.
		m.openCalls = append(m.openCalls, openToolCall{id: approval.ToolCallID, name: toolName})
	}
	ctx := context.Background()
	iter, err := s.engine.Resume(ctx, checkpointIDFor(runID), &adk.ResumeParams{
		Targets: map[string]any{approval.ResumeTarget: decision},
	})
	if err != nil {
		slog.Warn("resume failed", "run", string(runID), "err", err)
		s.emitTerminal(ctx, m, m.build(domain.EventRunFailed, payloadRunFailed{
			CauseCategory: causeInternalError,
			Message:       "The run could not be resumed. Please try again.",
		}))
		return
	}
	s.consume(ctx, m, sessionID, iter)
}

// terminalEvent classifies the failure path: context cancellation and
// engine CancelErrors close the run as cancelled; everything else fails
// it with a structured, user-visible message (FR-11).
func (s *Service) terminalEvent(ctx context.Context, m *eventMapper, cause error) domain.RunEvent {
	var ce *adk.CancelError
	if errors.Is(cause, errRunCancelled) || ctx.Err() != nil || errors.As(cause, &ce) {
		return m.build(domain.EventRunCancelled, payloadRunCancelled{Reason: reasonUserRequested})
	}
	slog.Warn("run failed", "err", cause)
	return m.build(domain.EventRunFailed, payloadRunFailed{
		CauseCategory: causeCategoryOf(cause),
		Message:       "The model run could not be completed. Please try again.",
	})
}

func causeCategoryOf(err error) string {
	// V0 classifies coarsely; provider/tool distinction joins with the
	// typed error wrappers of C2/C6.
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return causeCancelled
	default:
		return causeInternalError
	}
}

// persistAndPublish appends the single event as its own commit and, only
// on success, writes back the assigned seq and hands the event to the
// sink. A journal failure converts to a run.failed terminal so the run
// still closes exactly once.
func (s *Service) persistAndPublish(ctx context.Context, sessionID domain.SessionID, re domain.RunEvent) bool {
	seq, err := s.deps.Journal.Append(ctx, storage.Commit{RunID: re.RunID, Events: []domain.RunEvent{re}})
	if err != nil {
		slog.Error("journal append failed", "run", string(re.RunID), "type", string(re.Type), "err", err)
		m := newEventMapper(re.RunID, 0)
		s.emitTerminal(ctx, m, m.build(domain.EventRunFailed, payloadRunFailed{
			CauseCategory: causeInternalError,
			Message:       "The run could not be recorded. Please try again.",
		}))
		return false
	}
	re.Seq = seq
	s.deps.Sink.Publish(re)

	if re.Type == domain.EventModelCompleted {
		s.appendAssistantMessage(ctx, sessionID, re)
	}
	return true
}

// appendAssistantMessage mirrors the assistant turn into the message log
// once its model.completed event is durable. A failure only logs: the
// journal already holds the truth and the run must not fail for a
// bookkeeping write.
func (s *Service) appendAssistantMessage(ctx context.Context, sessionID domain.SessionID, re domain.RunEvent) {
	var p payloadModelCompleted
	if err := json.Unmarshal(re.Payload, &p); err != nil {
		slog.Error("decode model.completed payload", "run", string(re.RunID), "err", err)
		return
	}
	msg := domain.Message{
		ID:        newMessageID(),
		SessionID: sessionID,
		RunID:     re.RunID,
		Role:      domain.RoleAssistant,
		CreatedAt: time.Now().UnixMilli(),
		Content:   p.Content,
	}
	if err := s.deps.Messages.AppendMessage(ctx, msg); err != nil {
		slog.Error("append assistant message", "run", string(re.RunID), "err", err)
	}
}

// emitTerminal persists the terminal event best-effort, flips the run row
// to the matching status and unregisters the cancel handle. Persistence is
// detached from the run context: a cancellation must not strand the run
// without its terminal record (AS-5, FR-8). The terminal event is never
// published to the sink: the bus closes its subscribers on terminals, and
// SSE delivers the event from the journal replay.
func (s *Service) emitTerminal(ctx context.Context, m *eventMapper, terminal domain.RunEvent) {
	terminal.RunID = m.runID
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistTimeout)
	defer cancel()
	seq, err := s.deps.Journal.Append(persistCtx, storage.Commit{RunID: terminal.RunID, Events: []domain.RunEvent{terminal}})
	if err != nil {
		// The run already holds a terminal (concurrent close) or the
		// backend failed; either way the stored truth wins and the row
		// must not be flipped here.
		slog.Error("journal append of terminal event failed", "run", string(terminal.RunID), "type", string(terminal.Type), "err", err)
		s.mu.Lock()
		if c, ok := s.active[terminal.RunID]; ok {
			delete(s.active, terminal.RunID)
			c()
		}
		s.mu.Unlock()
		return
	}
	terminal.Seq = seq

	status := domain.RunCompleted
	switch terminal.Type {
	case domain.EventRunFailed:
		status = domain.RunFailed
	case domain.EventRunCancelled:
		status = domain.RunCancelled
	}
	if err := s.deps.Runs.SetRunStatus(persistCtx, terminal.RunID, status); err != nil {
		slog.Error("set terminal run status", "run", string(terminal.RunID), "status", string(status), "err", err)
	}

	s.mu.Lock()
	if c, ok := s.active[terminal.RunID]; ok {
		delete(s.active, terminal.RunID)
		c() // idempotent: releases the detached run context
	}
	s.mu.Unlock()
}

func newRunID() domain.RunID {
	return domain.RunID(newPrefixedID("run_"))
}

// checkpointIDFor derives the run's checkpoint id; it is stable for the
// run's lifetime so a later ResumeWithParams finds the exact checkpoint
// the interrupt wrote (docs/eino-capability-verify.md §2.2).
func checkpointIDFor(runID domain.RunID) string {
	return "ckpt-" + string(runID)
}

func newMessageID() string {
	return newPrefixedID("msg_")
}

func newPrefixedID(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing means the platform is broken; there is no
		// safe fallback for identity-grade randomness.
		panic(fmt.Sprintf("runtime: crypto/rand unavailable: %v", err))
	}
	return prefix + hex.EncodeToString(b)
}
