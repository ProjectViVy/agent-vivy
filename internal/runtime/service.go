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

// ServiceDeps groups the storage and fan-out dependencies of Service.
type ServiceDeps struct {
	Journal  storage.Journal
	Runs     storage.RunStore
	Messages storage.MessageStore
	Sink     EventSink
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
func (s *Service) Cancel(runID domain.RunID) bool {
	s.mu.Lock()
	cancel, ok := s.active[runID]
	s.mu.Unlock()
	if !ok {
		return false
	}
	cancel() // the drive closes the run via the run.cancelled path
	return true
}

// CancelAll cancels every in-flight run. Shutdown calls it before closing
// storage so terminal events can still be persisted (E4 hardens the
// ordering guarantees).
func (s *Service) CancelAll() {
	s.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.active))
	for _, c := range s.active {
		cancels = append(cancels, c)
	}
	s.mu.Unlock()
	for _, c := range cancels {
		c()
	}
}

func (s *Service) drive(ctx context.Context, m *eventMapper, sessionID domain.SessionID, userText string) {
	// The checkpoint id is derived from the run id so Query and Resume
	// always agree without a second assignment (spike §2.1: without
	// WithCheckPointID an interrupt persists no checkpoint).
	iter := s.engine.Query(ctx, userText, adk.WithCheckPointID(checkpointIDFor(m.runID)))
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		events, err := m.onEvent(ev)
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
		slog.Error("journal append of terminal event failed", "run", string(terminal.RunID), "type", string(terminal.Type), "err", err)
	} else {
		terminal.Seq = seq
	}

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
