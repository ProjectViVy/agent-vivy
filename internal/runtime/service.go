package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
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

// RunHook observes durable lifecycle events after they are handed to the
// live sink. Hooks are advisory and cannot change run state.
type RunHook interface {
	OnRunEvent(context.Context, domain.RunEvent)
}

// DecideApproval error sentinels; the API layer maps them to HTTP
// semantics (404 / 409; D-009 stays server-enforced).
var (
	ErrApprovalNotFound        = errors.New("runtime: approval not found")
	ErrApprovalInvalidDecision = errors.New("runtime: approval decision must be approved or denied")
	ErrApprovalAlreadyDecided  = errors.New("runtime: approval already decided")
	ErrApprovalExpired         = errors.New("runtime: approval expired")
	ErrQuestionNotFound        = errors.New("runtime: question not found")
	ErrQuestionInvalidAnswer   = errors.New("runtime: question answer must not be empty")
	ErrQuestionAlreadyAnswered = errors.New("runtime: question already answered")
	ErrQuestionExpired         = errors.New("runtime: question expired")
)

// ServiceDeps groups the storage and fan-out dependencies of Service.
type ServiceDeps struct {
	Journal  storage.Journal
	Runs     storage.RunStore
	Messages storage.MessageStore
	// Notes feeds the preamble's notebook digest (MA-3); nil leaves the
	// digest out.
	Notes storage.NoteStore
	// Approvals persists the approval rows behind the effectful tool
	// gate (C6); nil leaves interrupts unable to suspend.
	Approvals storage.ApprovalStore
	// ApprovalExpiration bounds how long a pending approval stays valid
	// (D-009).
	ApprovalExpiration time.Duration
	// Questions persists ask_user interactions separately from approvals.
	Questions storage.QuestionStore
	Hooks     []RunHook
	Sink      EventSink
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
	// wg tracks every drive/resume goroutine so shutdown can drain the
	// service before closing storage (E4): terminal events must persist
	// while the journal is still open.
	wg sync.WaitGroup
}

type pendingRun struct {
	sessionID     domain.SessionID
	mapper        *eventMapper
	selectedTools []string
	mode          domain.RunMode
	questionID    string
}

// RunOptions controls the physical policy applied to one run.
type RunOptions struct {
	Mode domain.RunMode
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
	return s.RunWithOptions(ctx, sessionID, userText, RunOptions{})
}

// RunWithOptions starts one run with an explicit harness policy. The mode is
// validated before any user message, run row, or event is persisted.
func (s *Service) RunWithOptions(ctx context.Context, sessionID domain.SessionID, userText string, options RunOptions) (domain.RunID, error) {
	if s.engine == nil || s.deps.Journal == nil || s.deps.Runs == nil || s.deps.Messages == nil || s.deps.Sink == nil {
		return "", errors.New("runtime: service not wired")
	}
	mode, err := normalizeRunMode(options.Mode)
	if err != nil {
		return "", err
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
	started := m.build(domain.EventRunStarted, payloadRunStarted{Provider: s.provider, Model: s.modelID, Mode: string(mode)})
	seq, err := s.deps.Journal.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{started}})
	if err != nil {
		return "", fmt.Errorf("runtime: persist run.started: %w", err)
	}
	started.Seq = seq
	s.publish(ctx, started)

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

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.drive(runCtx, m, sessionID, userText, mode)
	}()
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
		if p.questionID != "" && s.deps.Questions != nil {
			if err := s.deps.Questions.CancelQuestion(context.Background(), p.questionID); err != nil {
				slog.Warn("cancel question failed", "question", p.questionID, "err", err)
			}
		}
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
// storage, then drains via WaitIdle so every run.cancelled terminal is
// persisted while the journal is still open (E4). Runs pending on
// approvals close the same way.
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

// WaitIdle blocks until every drive/resume goroutine has exited or ctx
// expires, reporting which happened. Shutdown uses it after CancelAll so
// storage is never closed underneath a run still persisting its terminal
// event (E4).
func (s *Service) WaitIdle(ctx context.Context) bool {
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}

// Recover settles every non-terminal run left behind by a process
// restart (FR-8, AS-6): a run suspended on a still-valid approval with a
// readable checkpoint is rebuilt as pending so a later decision resumes
// it through the ordinary DecideApproval path; every other run closes
// with a definitive run.failed. No run is left dangling, and the
// journal's exactly-one-terminal guard keeps the close idempotent
// (D-008). Called once at startup before the server listens; a listing
// failure aborts startup, per-run failures only log.
func (s *Service) Recover(ctx context.Context) error {
	if s.engine == nil || s.deps.Journal == nil || s.deps.Runs == nil || s.deps.Sink == nil {
		return errors.New("runtime: service not wired")
	}
	runs, err := s.deps.Runs.ListActiveRuns(ctx)
	if err != nil {
		return fmt.Errorf("runtime: list active runs: %w", err)
	}
	if len(runs) == 0 {
		return nil
	}

	// Index the pending approvals by run; the listing is ordered by
	// expiry descending, so the first row seen per run is the freshest.
	pendingByRun := make(map[domain.RunID]domain.Approval)
	if s.deps.Approvals != nil {
		approvals, err := s.deps.Approvals.ListPendingApprovals(ctx)
		if err != nil {
			return fmt.Errorf("runtime: list pending approvals: %w", err)
		}
		for _, a := range approvals {
			if _, exists := pendingByRun[a.RunID]; !exists {
				pendingByRun[a.RunID] = a
			}
		}
	}
	pendingQuestionsByRun := make(map[domain.RunID]domain.Question)
	if s.deps.Questions != nil {
		questions, err := s.deps.Questions.ListPendingQuestions(ctx)
		if err != nil {
			return fmt.Errorf("runtime: list pending questions: %w", err)
		}
		for _, q := range questions {
			if _, exists := pendingQuestionsByRun[q.RunID]; !exists {
				pendingQuestionsByRun[q.RunID] = q
			}
		}
	}

	now := time.Now().UnixMilli()
	for _, run := range runs {
		approval, hasApproval := pendingByRun[run.ID]
		question, hasQuestion := pendingQuestionsByRun[run.ID]
		switch {
		case hasApproval && hasQuestion:
			s.failUnrecoverable(ctx, run.ID, "multiple pending interaction types")
		case hasApproval && approval.ExpiresAt <= now:
			s.failUnrecoverable(ctx, run.ID, "approval expired")
		case hasApproval && !s.checkpointReadable(ctx, run.ID):
			s.failUnrecoverable(ctx, run.ID, "checkpoint not readable")
		case hasApproval:
			s.rebuildPending(ctx, run, approval)
		case hasQuestion && question.ExpiresAt <= now:
			s.failUnrecoverable(ctx, run.ID, "question expired")
		case hasQuestion && !s.checkpointReadable(ctx, run.ID):
			s.failUnrecoverable(ctx, run.ID, "checkpoint not readable")
		case hasQuestion:
			s.rebuildPendingQuestion(ctx, run, question)
		default:
			s.failUnrecoverable(ctx, run.ID, "no pending approval")
		}
	}
	return nil
}

// checkpointReadable mirrors the interrupt-time check: only a checkpoint
// the versioned bridge can actually serve back may anchor a resume.
func (s *Service) checkpointReadable(ctx context.Context, runID domain.RunID) bool {
	if s.engine.cfg.Checkpoints == nil {
		return false
	}
	_, ok, err := s.engine.cfg.Checkpoints.Get(ctx, checkpointIDFor(runID))
	return err == nil && ok
}

// rebuildPending restores the in-memory suspend state of one run so the
// next decision resumes it: a fresh mapper seeded with the interrupted
// tool call (its name recovered from the journal's approval event) and a
// pending registration. The run row stays active; no event is emitted.
func (s *Service) rebuildPending(ctx context.Context, run domain.Run, approval domain.Approval) {
	m := newEventMapper(run.ID, s.engine.cfg.MaxEventPayloadBytes)
	toolName, selectedTools, mode := s.approvalDetails(ctx, run.ID)
	if len(selectedTools) == 0 && toolName != "" {
		// Events written before request-scoped selection existed remain
		// recoverable, but only the interrupted tool is allowed on resume.
		selectedTools = []string{toolName}
	}
	m.openCalls = append(m.openCalls, openToolCall{
		id:   approval.ToolCallID,
		name: toolName,
	})
	s.mu.Lock()
	s.pending[run.ID] = pendingRun{sessionID: run.SessionID, mapper: m, selectedTools: selectedTools, mode: mode}
	s.mu.Unlock()
	slog.Info("restart recovery: run waits on its approval decision",
		"run", string(run.ID), "approval", approval.ID)
}

// rebuildPendingQuestion restores a durable ask_user suspension after a
// restart. The question remains distinct from approval and resumes with the
// answer supplied to AnswerQuestion.
func (s *Service) rebuildPendingQuestion(ctx context.Context, run domain.Run, question domain.Question) {
	toolName, selectedTools, mode, resumeTarget := s.questionDetails(ctx, run.ID)
	if toolName == "" {
		toolName = tools.AskUserName
	}
	if len(selectedTools) == 0 {
		selectedTools = []string{toolName}
	}
	if resumeTarget == "" {
		resumeTarget = question.ResumeTarget
	}
	m := newEventMapper(run.ID, s.engine.cfg.MaxEventPayloadBytes)
	m.openCalls = append(m.openCalls, openToolCall{id: question.ToolCallID, name: toolName})
	s.mu.Lock()
	s.pending[run.ID] = pendingRun{
		sessionID: run.SessionID, mapper: m, selectedTools: selectedTools,
		mode: mode, questionID: question.ID,
	}
	s.mu.Unlock()
	slog.Info("restart recovery: run waits on user question",
		"run", string(run.ID), "question", question.ID, "resume_target", resumeTarget)
}

// approvalDetails recovers the interrupted tool and its request-scoped
// manifest from the durable approval event. Missing selection data is
// handled by rebuildPending for compatibility with pre-H2 events.
func (s *Service) approvalDetails(ctx context.Context, runID domain.RunID) (string, []string, domain.RunMode) {
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		slog.Warn("restart recovery: journal replay failed", "run", string(runID), "err", err)
		return "", nil, domain.RunModeNormal
	}
	defer func() { _ = it.Close() }()
	name := ""
	var selected []string
	mode := domain.RunModeNormal
	for it.Next() {
		ev := it.Value().Event
		if ev.Type != domain.EventToolApprovalRequired {
			continue
		}
		var p payloadToolApprovalRequired
		if json.Unmarshal(ev.Payload, &p) == nil {
			name = p.ToolName
			selected = append([]string(nil), p.SelectedTools...)
			if p.Mode != "" {
				mode = domain.RunMode(p.Mode)
			}
		}
	}
	return name, selected, mode
}

// questionDetails recovers the request-scoped selection and run mode from
// the durable user.question_required event.
func (s *Service) questionDetails(ctx context.Context, runID domain.RunID) (string, []string, domain.RunMode, string) {
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		slog.Warn("restart recovery: question replay failed", "run", string(runID), "err", err)
		return "", nil, domain.RunModeNormal, ""
	}
	defer func() { _ = it.Close() }()
	name := ""
	var selected []string
	mode := domain.RunModeNormal
	resumeTarget := ""
	for it.Next() {
		ev := it.Value().Event
		if ev.Type != domain.EventUserQuestionRequired {
			continue
		}
		var p payloadUserQuestionRequired
		if json.Unmarshal(ev.Payload, &p) == nil {
			name = tools.AskUserName
			selected = append([]string(nil), p.SelectedTools...)
			mode = domain.RunMode(p.Mode)
			if mode == "" {
				mode = domain.RunModeNormal
			}
			resumeTarget = p.ResumeTarget
		}
	}
	return name, selected, mode, resumeTarget
}

// failUnrecoverable closes one restart-orphaned run with a definitive
// run.failed so no non-terminal row outlives the process (FR-8).
func (s *Service) failUnrecoverable(ctx context.Context, runID domain.RunID, reason string) {
	m := newEventMapper(runID, 0)
	s.emitTerminal(ctx, m, m.build(domain.EventRunFailed, payloadRunFailed{
		CauseCategory: causeInternalError,
		Message:       "The run was interrupted by a server restart and could not be recovered. Please try again.",
	}))
	slog.Info("restart recovery: run failed definitively", "run", string(runID), "reason", reason)
}

func (s *Service) drive(ctx context.Context, m *eventMapper, sessionID domain.SessionID, userText string, mode domain.RunMode) {
	// The checkpoint id is derived from the run id so Run and Resume
	// always agree without a second assignment (spike §2.1: without
	// WithCheckPointID an interrupt persists no checkpoint).
	msgs, selection, _, err := s.runMessages(ctx, sessionID, userText)
	if err != nil {
		s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, err))
		return
	}
	runCtx := withRunMode(withSelectedTools(ctx, selection.Names()), mode)
	iter := s.engine.RunHistory(runCtx, msgs, adk.WithCheckPointID(checkpointIDFor(m.runID)))
	s.consume(runCtx, m, sessionID, selection.Names(), mode, iter)
}

// runMessages rebuilds the session transcript for the engine (MA-1,
// ADR-009): user/assistant text pairs in store order, with the current
// turn's user message last (Run persists it before driving, so the store
// already contains it). A listing failure degrades to the single new
// message with a warning — the run proceeds exactly as before the feed
// existed rather than failing on a bookkeeping read. Tool-role rows never
// enter the feed: cross-turn context carries text pairs only.
func (s *Service) runMessages(ctx context.Context, sessionID domain.SessionID, userText string) ([]*schema.Message, tools.Selection, ContextStats, error) {
	selection := s.engine.SelectTools(userText)
	// The per-run preamble leads the feed (MA-2): it carries the facts the
	// static Instruction cannot (date, selected tool set, and the bounded notebook
	// digest of MA-3).
	preamble := composeRunPreamble(time.Now(), s.notesDigest(ctx), selection.Specs)
	stored, err := s.deps.Messages.ListMessages(ctx, sessionID)
	if err != nil {
		slog.Warn("history rebuild failed; running without session context", "session", string(sessionID), "err", err)
		stored = nil
	}
	msgs, stats, err := buildRunContext(ContextPolicy{
		MaxBytes:           s.engine.cfg.MaxContextBytes,
		MaxHistoryMessages: s.engine.cfg.MaxHistoryMessages,
	}, preamble, stored, userText)
	if err != nil {
		return nil, selection, stats, err
	}
	if stats.DroppedHistoryMessages > 0 {
		slog.Warn("run context history bounded",
			"session", string(sessionID),
			"included_history_messages", stats.IncludedHistoryMessages,
			"dropped_history_messages", stats.DroppedHistoryMessages,
			"context_bytes", stats.Bytes,
		)
	}
	return msgs, selection, stats, nil
}

// notesDigest builds the preamble's notebook section (MA-3). Any listing
// failure degrades to no digest with a warning: the preamble stays
// useful even when the notebook read fails.
func (s *Service) notesDigest(ctx context.Context) string {
	if s.deps.Notes == nil {
		return ""
	}
	notes, err := s.deps.Notes.ListNotes(ctx)
	if err != nil {
		slog.Warn("notes digest skipped; listing failed", "err", err)
		return ""
	}
	return formatNotesDigest(notes)
}

// consume maps engine events into the journal until the iterator closes,
// then emits run.completed. Interrupts suspend the run without a terminal
// event; any other error closes it via the matching terminal. Both the
// first drive and approval resumes go through here, so every run closes
// exactly once (D-008).
func (s *Service) consume(ctx context.Context, m *eventMapper, sessionID domain.SessionID, selectedTools []string, mode domain.RunMode, iter *adk.AsyncIterator[*adk.AgentEvent]) {
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		events, err := m.onEvent(ev)
		if errors.Is(err, errRunInterrupted) {
			s.handleInterrupt(ctx, m, sessionID, selectedTools, mode)
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
func (s *Service) handleInterrupt(ctx context.Context, m *eventMapper, sessionID domain.SessionID, selectedTools []string, mode domain.RunMode) {
	if m.interrupt != nil && m.interrupt.ToolName == tools.AskUserName {
		s.handleQuestionInterrupt(ctx, m, sessionID, selectedTools, mode)
		return
	}
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
		ApprovalID:    approval.ID,
		ToolCallID:    details.ToolCallID,
		ToolName:      details.ToolName,
		Args:          details.Args,
		ExpiresAt:     expiresAt,
		SelectedTools: append([]string(nil), selectedTools...),
		Mode:          string(mode),
	})
	seq, err := s.deps.Journal.Append(persistCtx, storage.Commit{RunID: runID, Events: []domain.RunEvent{ev}})
	if err != nil {
		fail(err)
		return
	}
	ev.Seq = seq
	s.publish(persistCtx, ev)

	if ctx.Err() != nil {
		// Cancelled while suspending: close as cancelled. The approval
		// row stays pending in the store; a later decision finds no
		// pending run and stands as a no-op resume.
		s.emitTerminal(ctx, m, m.build(domain.EventRunCancelled, payloadRunCancelled{Reason: reasonUserRequested}))
		return
	}

	s.mu.Lock()
	s.pending[runID] = pendingRun{sessionID: sessionID, mapper: m, selectedTools: append([]string(nil), selectedTools...), mode: mode}
	s.mu.Unlock()
}

// handleQuestionInterrupt persists the ask_user suspension and publishes a
// question lifecycle event. It deliberately does not touch ApprovalStore.
func (s *Service) handleQuestionInterrupt(ctx context.Context, m *eventMapper, sessionID domain.SessionID, selectedTools []string, mode domain.RunMode) {
	runID := m.runID
	fail := func(err error) {
		slog.Warn("question handling failed", "run", string(runID), "err", err)
		s.emitTerminal(ctx, m, m.build(domain.EventRunFailed, payloadRunFailed{
			CauseCategory: causeInternalError,
			Message:       "The run could not be paused for a user question. Please try again.",
		}))
	}
	details := m.interrupt
	if details == nil || details.ResumeTarget == "" {
		fail(errors.New("question interrupt without a resume target"))
		return
	}
	if s.engine.cfg.Checkpoints == nil {
		fail(errors.New("checkpoint bridge not wired"))
		return
	}
	if s.deps.Questions == nil {
		fail(errors.New("question store not wired"))
		return
	}
	prompt, _ := details.Args["question"].(string)
	prompt = strings.TrimSpace(prompt)
	if prompt == "" || len(prompt) > 4096 {
		fail(errors.New("question prompt is empty or exceeds 4096 bytes"))
		return
	}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistTimeout)
	defer cancel()
	if _, ok, err := s.engine.cfg.Checkpoints.Get(persistCtx, checkpointIDFor(runID)); err != nil || !ok {
		fail(fmt.Errorf("checkpoint not readable: ok=%v: %w", ok, err))
		return
	}
	expiresAt := time.Now().Add(s.deps.ApprovalExpiration).UnixMilli()
	question := domain.Question{
		ID:           newPrefixedID("que_"),
		RunID:        runID,
		ToolCallID:   details.ToolCallID,
		Prompt:       prompt,
		Status:       domain.QuestionPending,
		ExpiresAt:    expiresAt,
		ResumeTarget: details.ResumeTarget,
	}
	if err := s.deps.Questions.CreateQuestion(persistCtx, question); err != nil {
		fail(err)
		return
	}
	ev := m.build(domain.EventUserQuestionRequired, payloadUserQuestionRequired{
		QuestionID:    question.ID,
		ToolCallID:    question.ToolCallID,
		Prompt:        prompt,
		ExpiresAt:     question.ExpiresAt,
		ResumeTarget:  question.ResumeTarget,
		SelectedTools: append([]string(nil), selectedTools...),
		Mode:          string(mode),
	})
	seq, err := s.deps.Journal.Append(persistCtx, storage.Commit{RunID: runID, Events: []domain.RunEvent{ev}})
	if err != nil {
		fail(err)
		return
	}
	ev.Seq = seq
	s.publish(persistCtx, ev)
	if ctx.Err() != nil {
		_ = s.deps.Questions.CancelQuestion(context.Background(), question.ID)
		s.emitTerminal(ctx, m, m.build(domain.EventRunCancelled, payloadRunCancelled{Reason: reasonUserRequested}))
		return
	}
	s.mu.Lock()
	s.pending[runID] = pendingRun{
		sessionID: sessionID, mapper: m,
		selectedTools: append([]string(nil), selectedTools...),
		mode:          mode, questionID: question.ID,
	}
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
		// The approval outlived its run in this process (the run was
		// cancelled, or the decision raced the close): the decision
		// stands, nothing resumes. Restart-orphaned runs never land here:
		// startup recovery re-registers their pending state (E2).
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

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.resumeRun(p.sessionID, toolName, p.selectedTools, p.mode,
			approval.RunID, approval.ToolCallID, approval.ResumeTarget, decision)
	}()
	return nil
}

// AnswerQuestion settles a pending ask_user interaction and resumes the
// interrupted run with the answer. It never writes or changes ApprovalStore.
func (s *Service) AnswerQuestion(ctx context.Context, questionID, answer string) error {
	if s.deps.Questions == nil {
		return errors.New("runtime: question store not wired")
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return ErrQuestionInvalidAnswer
	}
	question, err := s.deps.Questions.GetQuestion(ctx, questionID)
	if errors.Is(err, storage.ErrNotFound) {
		return ErrQuestionNotFound
	}
	if err != nil {
		return fmt.Errorf("runtime: get question: %w", err)
	}
	if question.Status != domain.QuestionPending {
		return ErrQuestionAlreadyAnswered
	}
	if time.Now().UnixMilli() >= question.ExpiresAt {
		return ErrQuestionExpired
	}
	answered, err := s.deps.Questions.AnswerQuestion(ctx, questionID, answer)
	if err != nil {
		return fmt.Errorf("runtime: answer question: %w", err)
	}
	if !answered {
		return ErrQuestionAlreadyAnswered
	}

	// Journal the answer before the resumed model work becomes visible.
	m := newEventMapper(question.RunID, s.engine.cfg.MaxEventPayloadBytes)
	ev := m.build(domain.EventUserQuestionAnswered, payloadUserQuestionAnswered{
		QuestionID: question.ID,
		Answer:     answer,
	})
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistTimeout)
	defer cancel()
	seq, err := s.deps.Journal.Append(persistCtx, storage.Commit{RunID: question.RunID, Events: []domain.RunEvent{ev}})
	if err != nil {
		return fmt.Errorf("runtime: persist question answer: %w", err)
	}
	ev.Seq = seq
	s.publish(persistCtx, ev)
	s.mu.Lock()
	p, ok := s.pending[question.RunID]
	if ok {
		delete(s.pending, question.RunID)
	}
	s.mu.Unlock()
	if !ok {
		slog.Warn("question answered without a pending run", "question", questionID, "run", string(question.RunID))
		return nil
	}
	toolName := pendingToolName(p, question.ToolCallID)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.resumeRun(p.sessionID, toolName, p.selectedTools, p.mode,
			question.RunID, question.ToolCallID, question.ResumeTarget, answer)
	}()
	return nil
}

func pendingToolName(p pendingRun, callID string) string {
	for _, oc := range p.mapper.openCalls {
		if oc.id == callID {
			return oc.name
		}
	}
	return ""
}

// resumeRun feeds the decision back into the engine and maps the resumed
// events into the same journal (the journal continues the seq).
func (s *Service) resumeRun(sessionID domain.SessionID, toolName string, selectedTools []string, mode domain.RunMode, runID domain.RunID, toolCallID, resumeTarget, resumeValue string) {
	m := newEventMapper(runID, s.engine.cfg.MaxEventPayloadBytes)
	if toolCallID != "" {
		// The resume replays the decided tool result first; seed the open
		// call so reconstructed tool.started/finished keep the call id.
		m.openCalls = append(m.openCalls, openToolCall{id: toolCallID, name: toolName})
	}
	ctx := withRunMode(withSelectedTools(context.Background(), selectedTools), mode)
	iter, err := s.engine.Resume(ctx, checkpointIDFor(runID), &adk.ResumeParams{
		Targets: map[string]any{resumeTarget: resumeValue},
	})
	if err != nil {
		slog.Warn("resume failed", "run", string(runID), "err", err)
		s.emitTerminal(ctx, m, m.build(domain.EventRunFailed, payloadRunFailed{
			CauseCategory: causeInternalError,
			Message:       "The run could not be resumed. Please try again.",
		}))
		return
	}
	s.consume(ctx, m, sessionID, selectedTools, mode, iter)
}

// terminalEvent classifies the failure path: context cancellation and
// engine CancelErrors close the run as cancelled; everything else fails
// it with a structured, user-visible message (FR-11).
func (s *Service) terminalEvent(ctx context.Context, m *eventMapper, cause error) domain.RunEvent {
	var ce *adk.CancelError
	if errors.Is(cause, errRunCancelled) || ctx.Err() != nil || errors.As(cause, &ce) {
		return m.build(domain.EventRunCancelled, payloadRunCancelled{Reason: reasonUserRequested})
	}
	if errors.Is(cause, adk.ErrExceedMaxIterations) {
		// Loop guardrail (MA-4): the engine hit the tool-call turn cap.
		// The cause stays structured and bounded — no engine internals
		// leak into the user-visible message (FR-11).
		slog.Warn("run failed: tool-call turn limit exceeded", "run", string(m.runID))
		return m.build(domain.EventRunFailed, payloadRunFailed{
			CauseCategory: causeInternalError,
			Message:       "The run was stopped because it reached the limit of tool-call turns. Please try again with a simpler request.",
		})
	}
	if errors.Is(cause, ErrContextBudgetExceeded) {
		slog.Warn("run failed: context budget exceeded", "run", string(m.runID), "err", cause)
		return m.build(domain.EventRunFailed, payloadRunFailed{
			CauseCategory: causeInternalError,
			Message:       "The run context exceeds the configured limit. Please start a shorter request or raise the context budget.",
		})
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
// sink. A journal failure closes the run via the classified terminal: a
// cancellation racing the append must land as run.cancelled, not
// run.failed (AS-5); any other failure is run.failed so the run still
// closes exactly once.
func (s *Service) persistAndPublish(ctx context.Context, sessionID domain.SessionID, re domain.RunEvent) bool {
	seq, err := s.deps.Journal.Append(ctx, storage.Commit{RunID: re.RunID, Events: []domain.RunEvent{re}})
	if err != nil {
		slog.Error("journal append failed", "run", string(re.RunID), "type", string(re.Type), "err", err)
		m := newEventMapper(re.RunID, 0)
		s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, err))
		return false
	}
	re.Seq = seq
	s.publish(ctx, re)

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
// to the matching status, publishes the event, and unregisters the cancel
// handle. Persistence is detached from the run context: a cancellation
// must not strand the run without its terminal record (AS-5, FR-8). The
// publish carries no frame itself — the bus closes its live subscribers
// on a terminal publish, which sends the SSE streams back to the journal
// replay where the event is delivered exactly once (AS-7).
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
	s.publish(persistCtx, terminal)

	s.mu.Lock()
	if c, ok := s.active[terminal.RunID]; ok {
		delete(s.active, terminal.RunID)
		c() // idempotent: releases the detached run context
	}
	s.mu.Unlock()
}

func (s *Service) publish(ctx context.Context, ev domain.RunEvent) {
	s.deps.Sink.Publish(ev)
	for _, hook := range s.deps.Hooks {
		hook.OnRunEvent(ctx, ev)
	}
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
