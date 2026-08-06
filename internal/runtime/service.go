package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/cloudwego/eino/adk"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// terminalPersistTimeout caps the detached persistence window for the
// terminal event: closing a run durably must survive the user cancelling
// their own context, but must also stay bounded (NFR: bounded).
const terminalPersistTimeout = 5 * time.Second

// Service orchestrates runs: it allocates the run id, persists
// run.started before anything else, drives the engine, and persists each
// mapped event to the journal BEFORE fanning it out to the caller
// (durability precedes visibility).
type Service struct {
	engine   *Engine
	journal  storage.Journal
	provider string
	modelID  string
}

// NewService wires the run service over an engine and a journal. provider
// and modelID label the run.started payload.
func NewService(eng *Engine, j storage.Journal, provider, modelID string) *Service {
	return &Service{engine: eng, journal: j, provider: provider, modelID: modelID}
}

// RunHandle is the caller's view of one run. Events arrive on the
// channel in journal order; the channel closes after the single terminal
// event (D-008).
type RunHandle struct {
	RunID  domain.RunID
	Events <-chan domain.RunEvent
}

// Run starts one run for the session. It returns after run.started is
// durable; the rest proceeds asynchronously. Session/message persistence
// joins with the HTTP layer; sessionID is kept for signature stability.
func (s *Service) Run(ctx context.Context, sessionID domain.SessionID, userText string) (*RunHandle, error) {
	if s.engine == nil || s.journal == nil {
		return nil, errors.New("runtime: service not wired")
	}
	runID := newRunID()
	m := newEventMapper(runID, s.engine.cfg.MaxEventPayloadBytes)

	started := m.build(domain.EventRunStarted, payloadRunStarted{Provider: s.provider, Model: s.modelID})
	firstSeq, err := s.journal.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{started}})
	if err != nil {
		return nil, fmt.Errorf("runtime: persist run.started: %w", err)
	}
	started.Seq = firstSeq

	out := make(chan domain.RunEvent, s.engine.cfg.StreamBuffer)
	select {
	case out <- started:
	case <-ctx.Done():
	}

	go s.drive(ctx, m, out, userText)
	return &RunHandle{RunID: runID, Events: out}, nil
}

func (s *Service) drive(ctx context.Context, m *eventMapper, out chan<- domain.RunEvent, userText string) {
	defer close(out)

	iter := s.engine.Query(ctx, userText)
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		events, err := m.onEvent(ev)
		if err != nil {
			s.emitTerminal(ctx, m, out, s.terminalEvent(ctx, m, err))
			return
		}
		for _, re := range events {
			if !s.persistAndFan(ctx, out, re) {
				return
			}
		}
	}

	for _, re := range m.onTurnEnd() {
		if !s.persistAndFan(ctx, out, re) {
			return
		}
	}
	s.emitTerminal(ctx, m, out, m.build(domain.EventRunCompleted, payloadRunCompleted{}))
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

// persistAndFan appends the single event as its own commit and, only on
// success, writes back the assigned seq and hands the event to the
// consumer. A journal failure converts to a run.failed terminal so the
// run still closes exactly once.
func (s *Service) persistAndFan(ctx context.Context, out chan<- domain.RunEvent, re domain.RunEvent) bool {
	seq, err := s.journal.Append(ctx, storage.Commit{RunID: re.RunID, Events: []domain.RunEvent{re}})
	if err != nil {
		slog.Error("journal append failed", "run", string(re.RunID), "type", string(re.Type), "err", err)
		m := newEventMapper(re.RunID, 0)
		s.emitTerminal(ctx, m, out, m.build(domain.EventRunFailed, payloadRunFailed{
			CauseCategory: causeInternalError,
			Message:       "The run could not be recorded. Please try again.",
		}))
		return false
	}
	re.Seq = seq
	select {
	case out <- re:
	case <-ctx.Done():
	}
	return true
}

// emitTerminal persists the terminal event best-effort and always fans it
// out so consumers observe the close even if the journal is unreachable.
// Persistence is detached from the caller's context: a user cancellation
// must not strand the run without its terminal record (AS-5, FR-8).
func (s *Service) emitTerminal(ctx context.Context, m *eventMapper, out chan<- domain.RunEvent, terminal domain.RunEvent) {
	terminal.RunID = m.runID
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistTimeout)
	defer cancel()
	seq, err := s.journal.Append(persistCtx, storage.Commit{RunID: terminal.RunID, Events: []domain.RunEvent{terminal}})
	if err != nil {
		slog.Error("journal append of terminal event failed", "run", string(terminal.RunID), "type", string(terminal.Type), "err", err)
	} else {
		terminal.Seq = seq
	}
	select {
	case out <- terminal:
	default:
		// Consumers that walked away must not wedge the driver; the
		// journal already holds the truth.
	}
}

func newRunID() domain.RunID {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing means the platform is broken; there is no
		// safe fallback for identity-grade randomness.
		panic(fmt.Sprintf("runtime: crypto/rand unavailable: %v", err))
	}
	return domain.RunID("run_" + hex.EncodeToString(b))
}
