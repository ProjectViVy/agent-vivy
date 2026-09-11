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
	"agent-vivy/internal/provider"
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

// WorkspaceAllocator is the filesystem isolation seam for background runs.
// The runtime never executes filesystem commands through this interface.
type WorkspaceAllocator interface {
	Ensure(context.Context, domain.RunID) (Workspace, error)
}

// ChildApprovalRouter receives a durable decision for an approval owned by a
// supervised child. The router is app-owned because it holds live worker
// waiters; runtime still owns validation and first-writer-wins persistence.
type ChildApprovalRouter interface {
	ResolveChildApproval(context.Context, domain.Approval, string) error
}

// ChildRunCanceller stops app-owned supervised worker processes. The runtime
// calls it while deleting a session so sealing Journal writes also stops any
// in-flight model/tool side effects.
type ChildRunCanceller interface {
	CancelChildRun(domain.RunID) bool
	CancelSessionChildren(domain.SessionID)
}

// ChannelDeliverer delivers outbound content to an external channel (CH-0).
// Defined as an interface here so runtime stays free of internal/channelhost imports.
type ChannelDeliverer interface {
	Deliver(ctx context.Context, channel string, to string, content string) error
}

// DecideApproval error sentinels; the API layer maps them to HTTP
// semantics (404 / 409; D-009 stays server-enforced).
var (
	ErrApprovalNotFound        = errors.New("runtime: approval not found")
	ErrApprovalInvalidDecision = errors.New("runtime: approval decision must be approved or denied")
	ErrApprovalInvalidReason   = errors.New("runtime: approval reason is too long")
	ErrApprovalAlreadyDecided  = errors.New("runtime: approval already decided")
	ErrApprovalExpired         = errors.New("runtime: approval expired")
	ErrQuestionNotFound        = errors.New("runtime: question not found")
	ErrQuestionInvalidAnswer   = errors.New("runtime: question answer must not be empty")
	ErrQuestionAlreadyAnswered = errors.New("runtime: question already answered")
	ErrQuestionExpired         = errors.New("runtime: question expired")
	ErrRecoveryBusy            = errors.New("runtime: background runs are active")
	// ErrShellUnavailable is returned when the configured active tool surface
	// does not contain the governed bash tool. The RPC layer maps this to a
	// fail-closed method-not-found response; callers must never fall back to a
	// local process invocation.
	ErrShellUnavailable = errors.New("runtime: governed shell is unavailable")
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
	// Budget bounds the complete run tree, including resumed work. A zero
	// value uses DefaultBudgetPolicy so services stay fail-safe by default.
	Budget               BudgetPolicy
	Workspaces           WorkspaceAllocator
	Sessions             storage.SessionStore
	PolicyDefaultProfile domain.PolicyProfile
	Hooks                []RunHook
	Sink                 EventSink
	ChildApprovals       ChildApprovalRouter
	ChildRuns            ChildRunCanceller
	// Compactions persists session-level durable compaction summaries
	// (context/compact + feed folding). Nil keeps automatic in-run
	// compression working; only durable summary folding is disabled.
	Compactions storage.CompactionStore
	// ShellState is an opaque protected blob store for the exact input of a
	// pending direct shell run. Raw scripts never enter Journal or Approval
	// metadata; when this store is wired, restart recovery can reload the
	// pending input without replaying an already-started process.
	ShellState storage.BlobStore
	// Truncations persists the session rewind/edit/fork cutoff markers
	// (JOURNAL-REWIND-AND-FORK). Nil keeps sessions un-truncatable: the
	// full history stays in every view and session/rewind is refused.
	Truncations storage.TruncationStore
	// Crons persists the control plane's scheduled jobs. Nil keeps the
	// whole cron family (scheduler + cron/* RPCs) disabled.
	Crons storage.CronStore
	// Titles generates session auto-titles after the first completed
	// exchange (VC-2 small→large chain). Nil keeps sessions untitled until
	// a user renames them.
	Titles TitleGenerator
	// Channels delivers outbound results to external channels (e.g. cron
	// payload delivery). Nil leaves external channel delivery disabled.
	Channels ChannelDeliverer
	// RebuildEngine rebuilds the Engine with a new config. App wires it to
	// the composition root so settings saves can hot-swap compaction
	// middleware; nil disables ScheduleEngineReload.
	RebuildEngine func(ctx context.Context, cfg EngineConfig) (*Engine, error)
}

// Service orchestrates runs: it persists the user message and the
// accepted run row, journals run.started before driving the engine, and
// persists each mapped event BEFORE publishing it (durability precedes
// visibility). The run's context is detached from the request: an RPC
// disconnect or page refresh must never kill the run (AS-7); only
// Cancel / CancelAll do.
type Service struct {
	engine         *Engine
	deps           ServiceDeps
	provider       string
	modelID        string
	catalog        *provider.Catalog // optional; enables model metadata queries
	defaultProfile domain.PolicyProfile

	mu     sync.Mutex
	active map[domain.RunID]context.CancelFunc
	// runSessions keeps the session identity for live/suspended runs so a
	// concurrent session deletion can seal every producer before removing the
	// durable rows. deletedSessions is a process-local tombstone: once delete
	// starts, no later event or projection may resurrect that session.
	runSessions     map[domain.RunID]domain.SessionID
	deletedSessions map[domain.SessionID]struct{}
	// pending tracks runs suspended on an approval: no engine work is in
	// flight, the checkpoint is durable, and the run row stays active
	// until a decision resumes it or Cancel closes it (C6).
	pending map[domain.RunID]pendingRun
	// shellPending tracks direct governed-shell runs suspended on a durable
	// approval. It is intentionally separate from Eino pending state: shell
	// approval recovery never relies on a model checkpoint or Resume().
	shellPending map[domain.RunID]shellPendingRun
	// shellStates retains exact shell arguments behind opaque state keys while
	// a process is alive. The optional ShellState blob store makes the same
	// state recoverable after restart.
	shellStates map[string]shellState
	// ledgers survive approval/question suspension and are shared by every
	// resume of the same run.
	ledgers map[domain.RunID]*BudgetLedger
	// snapshots pin the policy authority for active child workers. This is
	// process state only; durable run.started remains the restart truth.
	snapshots map[domain.RunID]domain.PolicySnapshot
	// runTools pins the compiler/runtime-selected ToolHost surface for each
	// live run. Action bridges must use this exact set rather than resolving
	// the process-wide registry again.
	runTools map[domain.RunID]map[string]struct{}
	// pendingEngine holds a settings-save engine rebuild that was deferred
	// because runs were in flight; it is applied at the next idle run
	// start (ScheduleEngineReload).
	pendingEngine *EngineConfig
	// lastCompaction is process-local observability for the UI (the
	// durable record lives in the journal as context.compacted events).
	lastCompaction map[domain.SessionID]*LastCompaction
	// wg tracks every drive/resume goroutine so shutdown can drain the
	// service before closing storage (E4): terminal events must persist
	// while the journal is still open.
	wg           sync.WaitGroup
	recoveryMu   sync.Mutex
	sweepMu      sync.Mutex
	projectionMu sync.Mutex
	sweepCancel  context.CancelFunc
	sweepWG      sync.WaitGroup

	// cron holds the CRON scheduler state; nil until first use and only
	// meaningful when deps.Crons is wired (lazy init guarded by cronInit).
	cron     *cronState
	cronInit sync.Mutex
}

// ErrModelChangeBusy means a model selection cannot be changed while any
// main, suspended, shell, or child run may still issue another model call.
var ErrModelChangeBusy = errors.New("runtime: model cannot change while runs are active or suspended")

type pendingRun struct {
	sessionID     domain.SessionID
	mapper        *eventMapper
	selectedTools []string
	// mounted is the run's skill-mount registry captured at suspend time.
	// Restart recovery rebuilds it from the journal's tool.mounted events
	// (recoveredMounts); nil means the run mounted nothing.
	mounted        *tools.MountedTools
	mode           domain.RunMode
	profile        domain.PolicyProfile
	snapshot       domain.PolicySnapshot
	sandboxMode    domain.SandboxMode
	approvalPolicy domain.ApprovalPolicy
	face           domain.Face
	questionID     string
	ledger         *BudgetLedger
}

// RunOptions controls the physical policy applied to one run.
type RunOptions struct {
	Mode    domain.RunMode
	Profile domain.PolicyProfile
	// Face attributes the run to its serving assembly (domain.Face).
	// Empty keeps the web face.
	Face domain.Face
	// Provenance marks the user turn's world entry (domain.Provenance).
	// nil keeps the built-in UI provenance ("ui"); a non-nil value must
	// carry a non-empty Source and is stamped onto the user message row.
	// Provenance never enters the run.started payload (contract §12).
	Provenance *domain.Provenance
	// Attachments are image files carried on the user message (VC-1g-2).
	// The RPC boundary validates mime whitelist and size caps; the service
	// persists them with the user row and the context build turns them
	// into multimodal input parts.
	Attachments []domain.Attachment
	// Thinking is the per-run extended-thinking preference. Empty means
	// auto: normalizeThinkingMode maps it before the run starts.
	Thinking domain.ThinkingMode
	// FileContexts are server-resolved, bounded text snapshots attached to the
	// current user turn. The RPC layer resolves project paths before calling
	// RunWithOptions; the runtime validates and persists the snapshot without
	// reading the host filesystem.
	FileContexts []domain.FileContext
}

// NewService wires the run service over an engine and its dependencies.
// provider and modelID label the run.started payload.
func NewService(eng *Engine, provider, modelID string, deps ServiceDeps) *Service {
	if deps.Budget == (BudgetPolicy{}) {
		deps.Budget = DefaultBudgetPolicy()
	}
	if !deps.PolicyDefaultProfile.Valid() {
		deps.PolicyDefaultProfile = domain.PolicyProfileDefault
	}
	return &Service{
		engine:          eng,
		deps:            deps,
		provider:        provider,
		modelID:         modelID,
		defaultProfile:  deps.PolicyDefaultProfile,
		active:          make(map[domain.RunID]context.CancelFunc),
		runSessions:     make(map[domain.RunID]domain.SessionID),
		deletedSessions: make(map[domain.SessionID]struct{}),
		pending:         make(map[domain.RunID]pendingRun),
		shellPending:    make(map[domain.RunID]shellPendingRun),
		shellStates:     make(map[string]shellState),
		ledgers:         make(map[domain.RunID]*BudgetLedger),
		snapshots:       make(map[domain.RunID]domain.PolicySnapshot),
		runTools:        make(map[domain.RunID]map[string]struct{}),
		lastCompaction:  make(map[domain.SessionID]*LastCompaction),
	}
}

// SetChildApprovalRouter wires the app-owned live worker registry after the
// runtime service has been constructed. This avoids a composition cycle:
// the manager depends on Service, while Service only calls the small router
// seam when a child approval is decided.
func (s *Service) SetChildApprovalRouter(router ChildApprovalRouter) {
	s.mu.Lock()
	s.deps.ChildApprovals = router
	s.mu.Unlock()
}

// SetChildRunCanceller wires the app-owned live worker registry.
func (s *Service) SetChildRunCanceller(canceller ChildRunCanceller) {
	s.mu.Lock()
	s.deps.ChildRuns = canceller
	s.mu.Unlock()
}

// SetCatalog wires the provider catalog so the service can query model
// capacity metadata. This is optional; without it, GetModelInfo returns
// conservative defaults.
func (s *Service) SetCatalog(catalog *provider.Catalog) {
	s.catalog = catalog
}

// SetModel updates the provider/model labels used on run.started. A
// settings save calls this so the next turn is labeled without a restart.
func (s *Service) SetModel(providerName, modelID string) {
	s.mu.Lock()
	s.provider = providerName
	s.modelID = modelID
	s.mu.Unlock()
}

// ChangeModelWhenIdle serializes a persisted model selection with every run
// and child-process startup. persist runs while the startup fence is held;
// only after it succeeds are the run.started labels changed. This prevents a
// settings write from switching the provider between model steps of one run.
func (s *Service) ChangeModelWhenIdle(providerName, modelID string, persist func() error) error {
	if s == nil || persist == nil {
		return errors.New("runtime: model change is not wired")
	}
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	s.mu.Lock()
	busy := len(s.active) != 0 || len(s.pending) != 0 || len(s.shellPending) != 0 || len(s.snapshots) != 0
	s.mu.Unlock()
	if busy {
		return ErrModelChangeBusy
	}
	if err := persist(); err != nil {
		return err
	}
	s.mu.Lock()
	s.provider = providerName
	s.modelID = modelID
	s.mu.Unlock()
	return nil
}

// CurrentModel returns the effective provider/model labels used for the next
// run. It is intentionally a small read seam for control-plane projections;
// callers that need pricing or capabilities should use GetModelInfo too.
func (s *Service) CurrentModel() (providerName, modelID string) {
	if s == nil {
		return "", ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.provider, s.modelID
}

// MaxEventPayloadBytes returns the encoded Journal payload ceiling shared by
// native runs and supervised external child producers.
func (s *Service) MaxEventPayloadBytes() int {
	return s.engine.cfg.MaxEventPayloadBytes
}

// DeleteSession first seals the session against new producers, then serializes
// deletion with run startup and Journal-to-message projection. The tombstone
// prevents a cancelled drive from appending a late event after storage delete.
func (s *Service) DeleteSession(ctx context.Context, id domain.SessionID) error {
	s.mu.Lock()
	s.deletedSessions[id] = struct{}{}
	liveRunIDs := make([]domain.RunID, 0)
	for runID, sessionID := range s.runSessions {
		if sessionID == id {
			liveRunIDs = append(liveRunIDs, runID)
		}
	}
	childRuns := s.deps.ChildRuns
	s.mu.Unlock()
	if childRuns != nil {
		childRuns.CancelSessionChildren(id)
	}
	for _, runID := range liveRunIDs {
		s.Cancel(runID)
	}

	s.projectionMu.Lock()
	runs, err := s.deps.Runs.ListRunsBySession(ctx, id)
	s.projectionMu.Unlock()
	if err != nil {
		return err
	}
	for _, run := range runs {
		if run.Kind == domain.RunKindChild && childRuns != nil {
			childRuns.CancelChildRun(run.ID)
		}
		s.Cancel(run.ID)
	}
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	if err := s.deps.Sessions.DeleteSession(ctx, id); err != nil {
		// Runs were already cancelled under the tombstone. Keep the session
		// sealed so a partial backend failure cannot revive producers after
		// their in-memory authority has been removed; deletion may be retried.
		return err
	}
	return nil
}

func (s *Service) sessionDeleted(id domain.SessionID) bool {
	s.mu.Lock()
	_, deleted := s.deletedSessions[id]
	s.mu.Unlock()
	return deleted
}

func (s *Service) runSessionDeleted(runID domain.RunID) bool {
	s.mu.Lock()
	sessionID, ok := s.runSessions[runID]
	_, deleted := s.deletedSessions[sessionID]
	s.mu.Unlock()
	return ok && deleted
}

// GetModelInfo returns capacity metadata for the currently configured
// provider and model. If no catalog is wired or the lookup fails, it
// returns a ModelInfo with zero ContextWindow; callers should use
// conservative defaults in that case.
func (s *Service) GetModelInfo(ctx context.Context) domain.ModelInfo {
	providerName, modelID := s.CurrentModel()
	s.mu.Lock()
	catalog := s.catalog
	s.mu.Unlock()
	if catalog == nil || providerName == "" || modelID == "" {
		return domain.ModelInfo{
			ID:            modelID,
			Provider:      providerName,
			ContextWindow: 0, // unknown; caller uses default
		}
	}
	info, err := catalog.ResolveModelInfo(ctx, providerName, modelID)
	if err != nil {
		// Log but don't fail; fall back to zero window
		return domain.ModelInfo{
			ID:            modelID,
			Provider:      providerName,
			ContextWindow: 0,
		}
	}
	return info
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
	return s.runWithOptions(ctx, sessionID, userText, options, nil)
}

type runPersistence func(domain.Message, domain.Run, domain.RunEvent) (domain.RunEvent, error)

func (s *Service) runWithOptions(ctx context.Context, sessionID domain.SessionID, userText string, options RunOptions, persist runPersistence) (domain.RunID, error) {
	if s.engine == nil || s.deps.Journal == nil || s.deps.Runs == nil || s.deps.Messages == nil || s.deps.Sink == nil {
		return "", errors.New("runtime: service not wired")
	}
	// Session deletion shares this lock with startup. If deletion marks the
	// tombstone while an earlier startup owns the lock, it will subsequently
	// remove that run; if deletion wins, startup fails without writing.
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	if s.sessionDeleted(sessionID) {
		return "", storage.ErrNotFound
	}
	// A deferred settings-save engine rebuild applies here, while no run
	// is registered.
	if err := s.applyPendingEngineReload(ctx, nil); err != nil {
		return "", err
	}
	if options.Profile == "" {
		options.Profile = s.defaultProfile
	}
	mode, profile, err := normalizeRunPolicy(options.Mode, options.Profile)
	if err != nil {
		return "", err
	}
	thinking, err := normalizeThinkingMode(options.Thinking)
	if err != nil {
		return "", err
	}
	face, err := normalizeFace(options.Face)
	if err != nil {
		return "", err
	}
	fileContexts, err := normalizeFileContexts(options.FileContexts)
	if err != nil {
		return "", err
	}
	// Provenance is validated before anything is persisted so an invalid
	// world entry cannot leave a half-labeled user message behind.
	provenance := domain.Provenance{Source: "ui"}
	if options.Provenance != nil {
		if strings.TrimSpace(options.Provenance.Source) == "" {
			return "", errors.New("runtime: run provenance requires a non-empty source")
		}
		provenance = *options.Provenance
	}
	snapshot, err := s.engine.cfg.Policy.Snapshot(profile)
	if err != nil {
		return "", err
	}
	sandboxMode, approvalPolicy := s.sessionSandbox(ctx, sessionID)
	ledger, err := NewBudgetLedger(s.deps.Budget)
	if err != nil {
		return "", err
	}
	if err := ledger.ReserveEvent(); err != nil {
		return "", err
	}
	runID := newRunID()
	if s.deps.Workspaces != nil {
		if _, err := s.deps.Workspaces.Ensure(ctx, runID); err != nil {
			return "", fmt.Errorf("runtime: allocate isolated workspace: %w", err)
		}
	}
	now := time.Now().UnixMilli()

	message := domain.Message{
		ID:               newMessageID(),
		SessionID:        sessionID,
		RunID:            runID,
		Role:             domain.RoleUser,
		CreatedAt:        now,
		Content:          userText,
		Attachments:      options.Attachments,
		FileContexts:     fileContexts,
		Source:           provenance.Source,
		Channel:          provenance.Channel,
		ChatID:           provenance.ChatID,
		ChannelMessageID: provenance.ChannelMessageID,
	}
	run := domain.Run{
		ID: runID, SessionID: sessionID, Status: domain.RunAccepted, CreatedAt: now,
	}

	m := newEventMapper(runID, s.engine.cfg.MaxEventPayloadBytes)
	runProvider, runModel := s.CurrentModel()
	m.setUsageRoutes(runProvider, runModel, s.engine.cfg.SummaryModelID)
	started := m.build(domain.EventRunStarted, payloadRunStarted{
		Provider: runProvider, Model: runModel, Mode: string(mode), Face: string(face),
		PolicyProfile: string(profile), PolicyHash: snapshot.Hash,
		SandboxMode: string(sandboxMode), ApprovalPolicy: string(approvalPolicy),
	})
	if persist != nil {
		started, err = persist(message, run, started)
		if err != nil {
			return "", err
		}
	} else {
		if err := s.deps.Messages.AppendMessage(ctx, message); err != nil {
			return "", fmt.Errorf("runtime: append user message: %w", err)
		}
		if err := s.deps.Runs.CreateRun(ctx, run); err != nil {
			return "", fmt.Errorf("runtime: create run: %w", err)
		}
		seq, appendErr := s.deps.Journal.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{started}})
		if appendErr != nil {
			return "", fmt.Errorf("runtime: persist run.started: %w", appendErr)
		}
		started.Seq = seq
		if err := s.deps.Runs.SetRunStatus(ctx, runID, domain.RunActive); err != nil {
			return "", fmt.Errorf("runtime: activate run: %w", err)
		}
	}
	s.publish(ctx, started)

	// Detach the run from the request lifecycle: RPC disconnects and page
	// refreshes must not cancel the work (AS-7). Cancel/CancelAll hold the
	// only handles that end it early.
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	runCtx = domain.WithThinkingMode(runCtx, thinking)
	selectedToolSet := make(map[string]struct{})
	for _, name := range s.engine.SelectTools().Names() {
		selectedToolSet[name] = struct{}{}
	}
	s.mu.Lock()
	s.active[runID] = cancel
	s.runSessions[runID] = sessionID
	s.ledgers[runID] = ledger
	s.snapshots[runID] = snapshot
	s.runTools[runID] = selectedToolSet
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.drive(runCtx, m, sessionID, userText, mode, profile, snapshot, sandboxMode, approvalPolicy, face)
	}()
	return runID, nil
}

// Cancel ends an in-flight run of this process. It reports false when the
// run is not active here (unknown, already terminal, or another process).
// Runs suspended on an approval close directly: no engine work is in
// flight for them. Direct shell approvals use their own pending map and
// never enter the Eino checkpoint resume path.
func (s *Service) Cancel(runID domain.RunID) bool {
	s.mu.Lock()
	cancel, active := s.active[runID]
	p, isPending := s.pending[runID]
	sp, isShellPending := s.shellPending[runID]
	s.mu.Unlock()

	if isShellPending {
		if err := s.waitShellApprovalReady(context.Background(), runID); err != nil {
			slog.Warn("cancel shell approval readiness failed", "run", string(runID), "err", err)
			return true
		}
		settled := true
		if s.deps.Approvals != nil {
			if approval, err := s.approvalForRun(context.Background(), runID); err == nil {
				if err := s.cancelApproval(context.Background(), approval, "run cancelled"); err != nil {
					slog.Warn("cancel shell approval failed", "approval", approval.ID, "err", err)
					settled = false
				}
			} else if errors.Is(err, storage.ErrNotFound) {
				// A concurrent durable decision won. Cancel only the active run
				// context; its resume path owns the terminal event.
				if cancel != nil {
					cancel()
				}
				return true
			} else {
				slog.Warn("load shell approval for cancel failed", "run", string(runID), "err", err)
				settled = false
			}
		}
		if !settled {
			return true
		}
		s.removeShellPending(runID, sp.mapper)
		if cancel != nil {
			cancel()
		}
		s.deleteShellState(sp.stateRef)
		s.emitTerminal(context.Background(), sp.mapper, sp.mapper.build(domain.EventRunCancelled, payloadRunCancelled{Reason: reasonUserRequested}))
		return true
	}

	if isPending {
		settled := true
		if p.questionID != "" && s.deps.Questions != nil {
			if err := s.cancelQuestion(context.Background(), p.questionID, "run cancelled"); err != nil {
				slog.Warn("cancel question failed", "question", p.questionID, "err", err)
				settled = false
			}
		} else if s.deps.Approvals != nil {
			if approval, err := s.approvalForRun(context.Background(), runID); err == nil {
				if err := s.cancelApproval(context.Background(), approval, "run cancelled"); err != nil {
					slog.Warn("cancel approval failed", "approval", approval.ID, "err", err)
					settled = false
				}
			}
		}
		if !settled {
			// A concurrent answer/decision won the durable conditional
			// transition. Leave the in-memory suspension for that response.
			return true
		}
		s.mu.Lock()
		if current, ok := s.pending[runID]; ok && current.mapper == p.mapper {
			delete(s.pending, runID)
		}
		s.mu.Unlock()
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
	pendingIDs := make([]domain.RunID, 0, len(s.pending)+len(s.shellPending))
	for id := range s.pending {
		pendingIDs = append(pendingIDs, id)
	}
	for id := range s.shellPending {
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

// StartInteractionSweeper runs the server-owned timeout transition. It is
// separate from run workers because a suspended run has no model goroutine.
func (s *Service) StartInteractionSweeper(parent context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	s.sweepMu.Lock()
	if s.sweepCancel != nil {
		s.sweepMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.sweepCancel = cancel
	s.sweepWG.Add(1)
	s.sweepMu.Unlock()
	go func() {
		defer s.sweepWG.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.SweepExpired(ctx); err != nil {
					slog.Warn("interaction expiry sweep failed", "err", err)
				}
			}
		}
	}()
}

// StopInteractionSweeper drains the timeout worker before storage closes.
func (s *Service) StopInteractionSweeper() {
	s.sweepMu.Lock()
	cancel := s.sweepCancel
	s.sweepCancel = nil
	s.sweepMu.Unlock()
	if cancel != nil {
		cancel()
		s.sweepWG.Wait()
	}
}

// SweepExpired settles all expired pending interactions. Conditional storage
// transitions preserve first-writer-wins against a simultaneous response.
func (s *Service) SweepExpired(ctx context.Context) error {
	if s.deps.Approvals != nil {
		approvals, err := s.deps.Approvals.ListPendingApprovals(ctx)
		if err != nil {
			return fmt.Errorf("runtime: list approvals for expiry: %w", err)
		}
		for _, approval := range approvals {
			if approval.ExpiresAt > 0 && approval.ExpiresAt <= time.Now().UnixMilli() {
				if err := s.expireApproval(ctx, approval, "human review timed out"); err != nil {
					return err
				}
			}
		}
	}
	if s.deps.Questions != nil {
		questions, err := s.deps.Questions.ListPendingQuestions(ctx)
		if err != nil {
			return fmt.Errorf("runtime: list questions for expiry: %w", err)
		}
		for _, question := range questions {
			if question.ExpiresAt > 0 && question.ExpiresAt <= time.Now().UnixMilli() {
				if err := s.expireQuestion(ctx, question, "user response timed out"); err != nil {
					return err
				}
			}
		}
	}
	return nil
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
	s.recoveryMu.Lock()
	defer s.recoveryMu.Unlock()
	return s.recover(ctx)
}

// RecoverBackground is the operator-facing recovery entry point. It refuses
// to reinterpret work owned by this process; startup recovery remains the
// safe path when no live state exists after a restart.
func (s *Service) RecoverBackground(ctx context.Context) error {
	s.recoveryMu.Lock()
	defer s.recoveryMu.Unlock()
	s.mu.Lock()
	busy := len(s.active) > 0 || len(s.pending) > 0
	s.mu.Unlock()
	if busy {
		return ErrRecoveryBusy
	}
	return s.recover(ctx)
}

func (s *Service) recover(ctx context.Context) error {
	if s.engine == nil || s.deps.Journal == nil || s.deps.Runs == nil || s.deps.Sink == nil {
		return errors.New("runtime: service not wired")
	}
	runs, err := s.deps.Runs.ListActiveRuns(ctx)
	if err != nil {
		return fmt.Errorf("runtime: list active runs: %w", err)
	}
	s.cleanupOrphanedShellState(ctx)
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
		// Child processes are intentionally not re-executed after restart:
		// replaying a side-effecting child could duplicate an external action.
		// Close it before considering approvals or checkpoints.
		if run.Kind == domain.RunKindChild {
			s.failUnrecoverable(ctx, run.ID, "worker_lost_after_restart")
			continue
		}
		if s.deps.Workspaces != nil {
			if _, err := s.deps.Workspaces.Ensure(ctx, run.ID); err != nil {
				s.failUnrecoverable(ctx, run.ID, "workspace isolation unavailable")
				continue
			}
		}
		approval, hasApproval := pendingByRun[run.ID]
		question, hasQuestion := pendingQuestionsByRun[run.ID]
		switch {
		case hasApproval && hasQuestion:
			s.failUnrecoverable(ctx, run.ID, "multiple pending interaction types")
		case hasApproval && approval.ExpiresAt <= now:
			if isShellApproval(approval) {
				s.deleteShellState(shellStateRef(approval.ProposalData))
			}
			if err := s.expireApproval(ctx, approval, "human review timed out during restart"); err != nil {
				slog.Warn("restart recovery: expire approval failed", "approval", approval.ID, "err", err)
				s.failUnrecoverable(ctx, run.ID, "approval expiry could not be persisted")
			}
		case hasApproval && isShellApproval(approval):
			if err := s.rebuildShellPending(ctx, run, approval); err != nil {
				slog.Warn("restart recovery: shell approval could not be rebuilt", "run", string(run.ID), "err", shellPublicError(err))
				s.deleteShellState(shellStateRef(approval.ProposalData))
				_ = s.cancelApproval(ctx, approval, "protected shell state could not be recovered")
				s.failUnrecoverable(ctx, run.ID, "protected shell state could not be recovered")
			}
		case hasApproval && !s.checkpointReadable(ctx, run.ID):
			s.failUnrecoverable(ctx, run.ID, "checkpoint not readable")
		case hasApproval:
			s.rebuildPending(ctx, run, approval)
		case hasQuestion && question.ExpiresAt <= now:
			if err := s.expireQuestion(ctx, question, "user response timed out during restart"); err != nil {
				slog.Warn("restart recovery: expire question failed", "question", question.ID, "err", err)
				s.failUnrecoverable(ctx, run.ID, "question expiry could not be persisted")
			}
		case hasQuestion && !s.checkpointReadable(ctx, run.ID):
			s.failUnrecoverable(ctx, run.ID, "checkpoint not readable")
		case hasQuestion:
			s.rebuildPendingQuestion(ctx, run, question)
		default:
			s.deleteShellState(shellStateRefForRun(run.ID))
			s.failUnrecoverable(ctx, run.ID, "no pending approval")
		}
	}
	return nil
}

// Workspace returns the deterministic sandbox for a durable run. Attach and
// recovery callers use the same boundary, so a restart cannot silently fall
// back to the host working directory.
func (s *Service) Workspace(ctx context.Context, runID domain.RunID) (Workspace, error) {
	if s.deps.Workspaces == nil {
		return Workspace{}, errors.New("runtime: workspace isolation not wired")
	}
	if _, err := s.deps.Runs.GetRun(ctx, runID); err != nil {
		return Workspace{}, err
	}
	return s.deps.Workspaces.Ensure(ctx, runID)
}

// WorkerParentAuthority returns the immutable authority a child worker may
// inherit from an active parent. It intentionally exposes a workspace ID,
// never the host path, and returns the parent's shared budget ledger so a
// child cannot reset the run-tree circuit breaker.
func (s *Service) WorkerParentAuthority(ctx context.Context, runID domain.RunID) (domain.PolicySnapshot, *BudgetLedger, string, error) {
	if s.deps.Runs == nil {
		return domain.PolicySnapshot{}, nil, "", errors.New("runtime: run store not wired")
	}
	run, err := s.deps.Runs.GetRun(ctx, runID)
	if err != nil {
		return domain.PolicySnapshot{}, nil, "", err
	}
	if run.Status != domain.RunActive && run.Status != domain.RunAccepted {
		return domain.PolicySnapshot{}, nil, "", errors.New("runtime: worker parent is not active")
	}
	s.mu.Lock()
	snapshot := s.snapshots[runID]
	ledger := s.ledgers[runID]
	if pending, ok := s.pending[runID]; ok {
		snapshot = pending.snapshot
		ledger = pending.ledger
	}
	s.mu.Unlock()
	if snapshot.Profile == "" || snapshot.Hash == "" || ledger == nil {
		return domain.PolicySnapshot{}, nil, "", errors.New("runtime: worker parent authority is unavailable")
	}
	if s.deps.Workspaces == nil {
		return domain.PolicySnapshot{}, nil, "", errors.New("runtime: worker workspace authority is unavailable")
	}
	workspace, err := s.deps.Workspaces.Ensure(ctx, runID)
	if err != nil {
		return domain.PolicySnapshot{}, nil, "", err
	}
	return snapshot, ledger, workspace.ID, nil
}

// WorkerChildAuthority derives a child authority from its durable parent.
// Children receive a nested ledger and a private workspace; they never get
// the parent's host path or an independent budget circuit breaker.
func (s *Service) WorkerChildAuthority(ctx context.Context, parentID, childID domain.RunID) (domain.PolicySnapshot, *BudgetLedger, string, error) {
	if s.deps.Runs == nil {
		return domain.PolicySnapshot{}, nil, "", errors.New("runtime: run store not wired")
	}
	parent, err := s.deps.Runs.GetRun(ctx, parentID)
	if err != nil {
		return domain.PolicySnapshot{}, nil, "", err
	}
	child, err := s.deps.Runs.GetRun(ctx, childID)
	if err != nil {
		return domain.PolicySnapshot{}, nil, "", err
	}
	if child.Kind != domain.RunKindChild || child.ParentID != parentID || child.Depth != parent.Depth+1 {
		return domain.PolicySnapshot{}, nil, "", errors.New("runtime: invalid worker child relationship")
	}
	snapshot, parentLedger, _, err := s.WorkerParentAuthority(ctx, parentID)
	if err != nil {
		return domain.PolicySnapshot{}, nil, "", err
	}
	childLedger, err := parentLedger.Child(s.deps.Budget)
	if err != nil {
		return domain.PolicySnapshot{}, nil, "", err
	}
	if s.deps.Workspaces == nil {
		return domain.PolicySnapshot{}, nil, "", errors.New("runtime: worker workspace authority is unavailable")
	}
	workspace, err := s.deps.Workspaces.Ensure(ctx, childID)
	if err != nil {
		return domain.PolicySnapshot{}, nil, "", err
	}
	return snapshot, childLedger, workspace.ID, nil
}

// RegisterWorkerAuthority installs the in-process authority for a child so
// that a grandchild can inherit from it without widening the parent tree.
func (s *Service) RegisterWorkerAuthority(runID domain.RunID, snapshot domain.PolicySnapshot, ledger *BudgetLedger) error {
	if runID == "" || snapshot.Profile == "" || snapshot.Hash == "" || ledger == nil {
		return errors.New("runtime: invalid worker authority")
	}
	s.mu.Lock()
	s.snapshots[runID] = snapshot
	s.ledgers[runID] = ledger
	s.mu.Unlock()
	return nil
}

// CreateWorkerRun serializes child creation with session deletion. Worker
// managers must use this seam instead of writing the RunStore directly so a
// child cannot appear after DeleteSession has enumerated and removed the
// session's run tree.
func (s *Service) CreateWorkerRun(ctx context.Context, run domain.Run) error {
	if s.deps.Runs == nil {
		return errors.New("runtime: run store not wired")
	}
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	if s.sessionDeleted(run.SessionID) {
		return storage.ErrNotFound
	}
	return s.deps.Runs.CreateRun(ctx, run)
}

// RegisterWorkerProcess atomically installs an app-owned live child handle
// under the session deletion fence. If deletion wins, registration is
// rejected; if registration wins, DeleteSession cannot enumerate the run
// and invoke ChildRunCanceller until the callback has completed.
func (s *Service) RegisterWorkerProcess(ctx context.Context, runID domain.RunID, register func()) error {
	if register == nil || s.deps.Runs == nil {
		return errors.New("runtime: worker process registration is not wired")
	}
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	run, err := s.deps.Runs.GetRun(ctx, runID)
	if err != nil {
		return err
	}
	if s.sessionDeleted(run.SessionID) {
		return storage.ErrNotFound
	}
	register()
	return nil
}

// UnregisterWorkerAuthority removes process-only child authority after its
// terminal event is durable. The durable run and journal remain queryable.
func (s *Service) UnregisterWorkerAuthority(runID domain.RunID) {
	s.mu.Lock()
	delete(s.snapshots, runID)
	delete(s.ledgers, runID)
	delete(s.runTools, runID)
	s.mu.Unlock()
}

// RecordExternalRunEvent persists an event produced by a supervised worker.
// It is intentionally generic: the parent manager chooses the child event
// payload, while Journal remains the single durability and terminal guard.
func (s *Service) RecordExternalRunEvent(ctx context.Context, runID domain.RunID, typ domain.EventType, payload any) (domain.RunEvent, error) {
	return s.RecordExternalRunEventVersion(ctx, runID, typ, 1, payload)
}

// RecordExternalRunEventVersion persists a versioned supervised-worker event
// and enforces the same encoded payload ceiling as native mapper events.
func (s *Service) RecordExternalRunEventVersion(ctx context.Context, runID domain.RunID, typ domain.EventType, payloadVersion int, payload any) (domain.RunEvent, error) {
	if s.deps.Journal == nil || s.deps.Runs == nil || s.deps.Sink == nil {
		return domain.RunEvent{}, errors.New("runtime: service is not wired")
	}
	if !typ.Valid() {
		return domain.RunEvent{}, fmt.Errorf("runtime: invalid external event type %q", typ)
	}
	if payloadVersion < 1 {
		return domain.RunEvent{}, fmt.Errorf("runtime: invalid external payload version %d", payloadVersion)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return domain.RunEvent{}, fmt.Errorf("runtime: marshal external event: %w", err)
	}
	if limit := s.engine.cfg.MaxEventPayloadBytes; limit > 0 && len(data) > limit {
		return domain.RunEvent{}, fmt.Errorf("runtime: external event payload is %d bytes; limit is %d", len(data), limit)
	}
	event := domain.RunEvent{RunID: runID, Type: typ, CreatedAt: time.Now().UnixMilli(), PayloadVersion: payloadVersion, Payload: data}
	return s.appendRunEvent(ctx, event, true)
}

// appendRunEvent is the common deletion-safe path for runtime-owned events
// that are already encoded. Review, question, and worker writers all share
// the same session tombstone used by native model events.
func (s *Service) appendRunEvent(ctx context.Context, event domain.RunEvent, updateTerminalStatus bool) (domain.RunEvent, error) {
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	run, err := s.deps.Runs.GetRun(ctx, event.RunID)
	if err != nil {
		return domain.RunEvent{}, err
	}
	if s.sessionDeleted(run.SessionID) {
		return domain.RunEvent{}, storage.ErrNotFound
	}
	seq, err := s.deps.Journal.Append(ctx, storage.Commit{RunID: event.RunID, Events: []domain.RunEvent{event}})
	if err != nil {
		return domain.RunEvent{}, err
	}
	event.Seq = seq
	if status, ok := event.Type.RunStatus(); updateTerminalStatus && ok {
		if err := s.deps.Runs.SetRunStatus(ctx, event.RunID, status); err != nil {
			return domain.RunEvent{}, err
		}
	}
	s.publish(ctx, event)
	return event, nil
}

// recordSyntheticSessionEvent journals a runtime-owned synthetic run (for
// example manual compaction) that intentionally has no RunStore row, while
// still participating in the session deletion fence.
func (s *Service) recordSyntheticSessionEvent(ctx context.Context, sessionID domain.SessionID, runID domain.RunID, typ domain.EventType, payload any) (domain.RunEvent, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return domain.RunEvent{}, fmt.Errorf("runtime: marshal synthetic event: %w", err)
	}
	if limit := s.engine.cfg.MaxEventPayloadBytes; limit > 0 && len(data) > limit {
		return domain.RunEvent{}, fmt.Errorf("runtime: synthetic event payload is %d bytes; limit is %d", len(data), limit)
	}
	event := domain.RunEvent{RunID: runID, Type: typ, CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1, Payload: data}
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	if s.sessionDeleted(sessionID) {
		return domain.RunEvent{}, storage.ErrNotFound
	}
	seq, err := s.deps.Journal.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{event}})
	if err != nil {
		return domain.RunEvent{}, err
	}
	event.Seq = seq
	s.publish(ctx, event)
	return event, nil
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
	ledger := s.recoverBudgetLedger(ctx, run.ID)
	if ledger == nil {
		return
	}
	m := newEventMapper(run.ID, s.engine.cfg.MaxEventPayloadBytes)
	providerName, modelID := s.usageRoutesForRun(ctx, run.ID)
	m.setUsageRoutes(providerName, modelID, s.engine.cfg.SummaryModelID)
	toolName, selectedTools, mode, face, profile, snapshot, sandboxMode, approvalPolicy := s.approvalDetails(ctx, run.ID)
	if len(selectedTools) == 0 && toolName != "" {
		// Events written before request-scoped selection existed remain
		// recoverable, but only the interrupted tool is allowed on resume.
		selectedTools = []string{toolName}
	}
	m.registerOpenCall(openToolCall{
		id:   approval.ToolCallID,
		name: toolName,
	})
	s.mu.Lock()
	s.pending[run.ID] = pendingRun{sessionID: run.SessionID, mapper: m, selectedTools: selectedTools, mode: mode, profile: profile, snapshot: snapshot, sandboxMode: sandboxMode, approvalPolicy: approvalPolicy, face: face, mounted: s.recoveredMounts(ctx, run.ID), ledger: ledger}
	s.runSessions[run.ID] = run.SessionID
	s.ledgers[run.ID] = ledger
	s.snapshots[run.ID] = snapshot
	s.mu.Unlock()
	slog.Info("restart recovery: run waits on its approval decision",
		"run", string(run.ID), "approval", approval.ID)
}

// rebuildPendingQuestion restores a durable ask_user suspension after a
// restart. The question remains distinct from approval and resumes with the
// answer supplied to AnswerQuestion.
func (s *Service) rebuildPendingQuestion(ctx context.Context, run domain.Run, question domain.Question) {
	ledger := s.recoverBudgetLedger(ctx, run.ID)
	if ledger == nil {
		return
	}
	toolName, selectedTools, mode, face, profile, snapshot, sandboxMode, approvalPolicy, resumeTarget := s.questionDetails(ctx, run.ID)
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
	providerName, modelID := s.usageRoutesForRun(ctx, run.ID)
	m.setUsageRoutes(providerName, modelID, s.engine.cfg.SummaryModelID)
	m.registerOpenCall(openToolCall{id: question.ToolCallID, name: toolName})
	s.mu.Lock()
	s.pending[run.ID] = pendingRun{
		sessionID: run.SessionID, mapper: m, selectedTools: selectedTools,
		mode: mode, profile: profile, snapshot: snapshot, sandboxMode: sandboxMode, approvalPolicy: approvalPolicy, face: face, questionID: question.ID, mounted: s.recoveredMounts(ctx, run.ID), ledger: ledger,
	}
	s.runSessions[run.ID] = run.SessionID
	s.ledgers[run.ID] = ledger
	s.snapshots[run.ID] = snapshot
	s.mu.Unlock()
	slog.Info("restart recovery: run waits on user question",
		"run", string(run.ID), "question", question.ID, "resume_target", resumeTarget)
}

func (s *Service) usageRoutesForRun(ctx context.Context, runID domain.RunID) (string, string) {
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		return "", ""
	}
	defer func() { _ = it.Close() }()
	for it.Next() {
		event := it.Value().Event
		if event.Type != domain.EventRunStarted {
			continue
		}
		var started payloadRunStarted
		if json.Unmarshal(event.Payload, &started) == nil {
			return started.Provider, started.Model
		}
		return "", ""
	}
	return "", ""
}

// newResumeEventMapper rebuilds the usage routes that belong to the durable
// run. Resume paths intentionally create a fresh mapper (stream/tool state
// must not leak across an interrupt), so the attribution carried by the old
// mapper has to be restored from run.started as well.
func (s *Service) newResumeEventMapper(ctx context.Context, runID domain.RunID) *eventMapper {
	m := newEventMapper(runID, s.engine.cfg.MaxEventPayloadBytes)
	providerName, modelID := s.usageRoutesForRun(ctx, runID)
	m.setUsageRoutes(providerName, modelID, s.engine.cfg.SummaryModelID)
	return m
}

// recoverBudgetLedger rebuilds the shared run-tree accounting from durable
// events before a suspended run becomes resumable. If the current policy is
// already exceeded, recovery fails closed instead of resetting the budget.
func (s *Service) recoverBudgetLedger(ctx context.Context, runID domain.RunID) *BudgetLedger {
	ledger, err := NewBudgetLedger(s.deps.Budget)
	if err != nil {
		s.failUnrecoverable(ctx, runID, "invalid budget policy")
		return nil
	}
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		s.failUnrecoverable(ctx, runID, "budget replay failed")
		return nil
	}
	defer func() { _ = it.Close() }()
	for it.Next() {
		if err := ledger.ReplayEvent(it.Value().Event); err != nil {
			s.failUnrecoverable(ctx, runID, "budget exceeded before restart")
			return nil
		}
	}
	if err := it.Err(); err != nil {
		s.failUnrecoverable(ctx, runID, "budget replay failed")
		return nil
	}
	return ledger
}

// recoveredMounts rebuilds the run's skill-mount registry from the
// journal's tool.mounted events (TT-3). The live registry is memory-only,
// so restart recovery would otherwise drop every mount made before the
// restart and the resumed run would lose access to tools it had mounted.
// Mounts are add-only within a run, so replaying the events in order
// reproduces the suspend-time set exactly. nil when the run mounted
// nothing — the same shape rebuildPending produced before mount recovery
// existed.
func (s *Service) recoveredMounts(ctx context.Context, runID domain.RunID) *tools.MountedTools {
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		slog.Warn("restart recovery: mount replay failed", "run", string(runID), "err", err)
		return nil
	}
	defer func() { _ = it.Close() }()
	var mounts *tools.MountedTools
	for it.Next() {
		event := it.Value().Event
		if event.Type != domain.EventToolMounted || len(event.Payload) == 0 {
			continue
		}
		var p payloadToolMounted
		if err := json.Unmarshal(event.Payload, &p); err != nil || len(p.Tools) == 0 {
			slog.Warn("restart recovery: unreadable tool.mounted payload", "run", string(runID))
			continue
		}
		if mounts == nil {
			mounts = tools.NewMountedTools()
		}
		mounts.Mount(p.Tools...)
	}
	if err := it.Err(); err != nil {
		slog.Warn("restart recovery: mount replay failed", "run", string(runID), "err", err)
		return nil
	}
	return mounts
}

// sessionMounts seeds the run's skill-mount registry from every prior run
// of the same session (TT-1 session pin). The live registry is memory-only
// and per-run, so without this seed a tool mounted by an earlier run is
// invisible to the next one and the model must view the skill again. Like
// recoveredMounts the replay is additive and creation-ordered, so the seed
// reproduces the session's accumulated mounts; the current run is skipped
// (it mounts nothing yet). Best-effort: listing or replay failures degrade
// to fewer mounts with a warning and never fail the run.
func (s *Service) sessionMounts(ctx context.Context, sessionID domain.SessionID, runID domain.RunID) *tools.MountedTools {
	prior, err := s.deps.Runs.ListRunsBySession(ctx, sessionID)
	if err != nil {
		slog.Warn("session pin: run listing failed", "session", string(sessionID), "err", err)
		return nil
	}
	var mounts *tools.MountedTools
	for _, run := range prior {
		if run.ID == runID {
			continue
		}
		it, err := s.deps.Journal.Replay(ctx, run.ID, 0)
		if err != nil {
			slog.Warn("session pin: mount replay failed", "run", string(run.ID), "err", err)
			continue
		}
		for it.Next() {
			event := it.Value().Event
			if event.Type != domain.EventToolMounted || len(event.Payload) == 0 {
				continue
			}
			var p payloadToolMounted
			if err := json.Unmarshal(event.Payload, &p); err != nil || len(p.Tools) == 0 {
				slog.Warn("session pin: unreadable tool.mounted payload", "run", string(run.ID))
				continue
			}
			if mounts == nil {
				mounts = tools.NewMountedTools()
			}
			mounts.Mount(p.Tools...)
		}
		if err := it.Err(); err != nil {
			slog.Warn("session pin: mount replay failed", "run", string(run.ID), "err", err)
		}
		_ = it.Close()
	}
	return mounts
}

// approvalDetails recovers the interrupted tool and its request-scoped
// manifest from the durable approval event. Missing selection data is
// handled by rebuildPending for compatibility with pre-H2 events.
func (s *Service) approvalDetails(ctx context.Context, runID domain.RunID) (string, []string, domain.RunMode, domain.Face, domain.PolicyProfile, domain.PolicySnapshot, domain.SandboxMode, domain.ApprovalPolicy) {
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		slog.Warn("restart recovery: journal replay failed", "run", string(runID), "err", err)
		return "", nil, domain.RunModeNormal, domain.FaceWeb, domain.PolicyProfileDefault, domain.PolicySnapshot{Profile: domain.PolicyProfileDefault}, domain.SandboxModeWorkspaceWrite, domain.ApprovalPolicyAsk
	}
	defer func() { _ = it.Close() }()
	name := ""
	var selected []string
	mode := domain.RunModeNormal
	face := domain.FaceWeb
	profile := domain.PolicyProfileDefault
	snapshot := domain.PolicySnapshot{Profile: profile}
	sandboxMode := domain.SandboxModeWorkspaceWrite
	approvalPolicy := domain.ApprovalPolicyAsk
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
			if domain.Face(p.Face).Valid() {
				face = domain.Face(p.Face)
			}
			profile = recoveredProfile(mode, p.PolicyProfile)
			snapshot = domain.PolicySnapshot{Profile: profile, Hash: p.PolicyHash}
			if domain.SandboxMode(p.SandboxMode).Valid() {
				sandboxMode = domain.SandboxMode(p.SandboxMode)
			}
			if domain.ApprovalPolicy(p.ApprovalPolicy).Valid() {
				approvalPolicy = domain.ApprovalPolicy(p.ApprovalPolicy)
			}
		}
	}
	return name, selected, mode, face, profile, snapshot, sandboxMode, approvalPolicy
}

// questionDetails recovers the request-scoped selection and run mode from
// the durable user.question_required event.
func (s *Service) questionDetails(ctx context.Context, runID domain.RunID) (string, []string, domain.RunMode, domain.Face, domain.PolicyProfile, domain.PolicySnapshot, domain.SandboxMode, domain.ApprovalPolicy, string) {
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		slog.Warn("restart recovery: question replay failed", "run", string(runID), "err", err)
		return "", nil, domain.RunModeNormal, domain.FaceWeb, domain.PolicyProfileDefault, domain.PolicySnapshot{Profile: domain.PolicyProfileDefault}, domain.SandboxModeWorkspaceWrite, domain.ApprovalPolicyAsk, ""
	}
	defer func() { _ = it.Close() }()
	name := ""
	var selected []string
	mode := domain.RunModeNormal
	face := domain.FaceWeb
	profile := domain.PolicyProfileDefault
	snapshot := domain.PolicySnapshot{Profile: profile}
	sandboxMode := domain.SandboxModeWorkspaceWrite
	approvalPolicy := domain.ApprovalPolicyAsk
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
			if domain.Face(p.Face).Valid() {
				face = domain.Face(p.Face)
			}
			profile = recoveredProfile(mode, p.PolicyProfile)
			snapshot = domain.PolicySnapshot{Profile: profile, Hash: p.PolicyHash}
			if domain.SandboxMode(p.SandboxMode).Valid() {
				sandboxMode = domain.SandboxMode(p.SandboxMode)
			}
			if domain.ApprovalPolicy(p.ApprovalPolicy).Valid() {
				approvalPolicy = domain.ApprovalPolicy(p.ApprovalPolicy)
			}
			resumeTarget = p.ResumeTarget
		}
	}
	return name, selected, mode, face, profile, snapshot, sandboxMode, approvalPolicy, resumeTarget
}

// failUnrecoverable closes one restart-orphaned run with a definitive
// run.failed so no non-terminal row outlives the process (FR-8).
func (s *Service) failUnrecoverable(ctx context.Context, runID domain.RunID, reason string) {
	m := newEventMapper(runID, 0)
	if run, err := s.deps.Runs.GetRun(ctx, runID); err == nil && run.Kind == domain.RunKindChild {
		if s.deps.Approvals != nil {
			if approvals, listErr := s.deps.Approvals.ListPendingApprovals(ctx); listErr == nil {
				for _, approval := range approvals {
					if approval.RunID == runID {
						_, _ = s.deps.Approvals.DecideApproval(ctx, approval.ID, domain.ApprovalDenied)
					}
				}
			}
		}
		s.emitTerminal(ctx, m, m.build(domain.EventChildFailed, payloadChildFailed{
			CauseCategory: "worker_lost_after_restart",
			Message:       "The child worker was lost during server restart. Retry explicitly to avoid duplicate side effects.",
		}))
		slog.Info("restart recovery: child worker failed closed", "run", string(runID), "reason", reason)
		return
	}
	s.emitTerminal(ctx, m, m.build(domain.EventRunFailed, payloadRunFailed{
		CauseCategory: causeInternalError,
		Message:       "The run was interrupted by a server restart and could not be recovered. Please try again.",
	}))
	slog.Info("restart recovery: run failed definitively", "run", string(runID), "reason", reason)
}

func (s *Service) drive(ctx context.Context, m *eventMapper, sessionID domain.SessionID, userText string, mode domain.RunMode, profile domain.PolicyProfile, snapshot domain.PolicySnapshot, sandboxMode domain.SandboxMode, approvalPolicy domain.ApprovalPolicy, face domain.Face) {
	// The checkpoint id is derived from the run id so Run and Resume
	// always agree without a second assignment (spike §2.1: without
	// WithCheckPointID an interrupt persists no checkpoint).
	// Capture the engine once: a settings-save engine rebuild only happens
	// while no run is registered, so this reference is stable for the run.
	eng := s.engine
	msgs, selection, _, err := s.runMessages(ctx, sessionID, userText, eng, face)
	if err != nil {
		s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, err))
		return
	}
	if !s.persistAndPublish(ctx, sessionID, m.build(domain.EventModelRequest, digestModelRequest(msgs, selection.Names()))) {
		return
	}
	ledger := s.ledgerForRun(m.runID)
	runCtx := withSessionID(withRunID(withPolicySnapshot(withPolicyProfile(withRunMode(withFace(withSelectedTools(ctx, selection.Names()), face), mode), profile), snapshot), m.runID), sessionID)
	// Per-run mount registry: skill_view records declared tools here so the
	// mount projection can advertise them and the adapter can admit them
	// for the remainder of this run. TT-1 session pin: the fresh registry is
	// seeded with the mounts prior runs of this session accumulated, so a
	// skill mounted once stays callable without re-viewing.
	mounts := s.sessionMounts(ctx, sessionID, m.runID)
	if mounts == nil {
		mounts = tools.NewMountedTools()
	}
	runCtx = tools.WithMountedTools(runCtx, mounts)
	runCtx = withSessionSandbox(runCtx, sandboxMode, approvalPolicy)
	runCtx = tools.WithSessionID(runCtx, sessionID)
	runCtx = withGovernanceEventSink(runCtx, s.governanceSink(m, sessionID, ledger))
	runCtx = s.withLiveModelStreamObserver(runCtx, m, sessionID, ledger)
	iter := eng.RunHistory(runCtx, msgs, adk.WithCheckPointID(checkpointIDFor(m.runID)))
	s.consume(runCtx, m, sessionID, selection.Names(), mode, ledger, iter)
}

// withLiveModelStreamObserver installs the producer-path stream seam used by
// both initial runs and ResumeWithParams. Eino callbacks are a sibling Copy
// after ChatModel.Stream returns and cannot put persist, bounded backpressure,
// fail-closed errors, or the tool-settled barrier on the Recv→Send path.
// Keeping the construction in one place prevents approval/question resumes
// from reverting to EOF-batched output.
func (s *Service) withLiveModelStreamObserver(ctx context.Context, m *eventMapper, sessionID domain.SessionID, ledger *BudgetLedger) context.Context {
	return withModelStreamObserver(ctx, modelStreamObserver{
		Begin: m.beginObservedStream,
		Chunk: func(chunk *schema.Message) error {
			if err := m.waitForToolsSettled(ctx); err != nil {
				return err
			}
			events := m.observeStreamChunk(chunk)
			if err := reserveMappedBudget(ledger, events); err != nil {
				return err
			}
			for _, event := range events {
				if !s.persistAndPublish(ctx, sessionID, event) {
					return context.Canceled
				}
			}
			return nil
		},
	})
}

// runMessages rebuilds the session transcript for the engine (ADR-010):
// user, assistant, and paired tool turns in store order, with the current
// turn's user message last (Run persists it before driving, so the store
// already contains it). A listing failure degrades to the single new
// message with a warning — the run proceeds rather than failing on a
// bookkeeping read.
func (s *Service) runMessages(ctx context.Context, sessionID domain.SessionID, userText string, eng *Engine, face domain.Face) ([]*schema.Message, tools.Selection, ContextStats, error) {
	selection := eng.SelectTools()
	// The per-run preamble leads the feed (MA-2): it carries the facts the
	// static Instruction cannot (date, whether active tools exist, and the
	// bounded notebook digest of MA-3). Tool discovery is owned by Eino's
	// official middleware.
	preamble := composeRunPreamble(time.Now(), s.notesDigest(ctx), len(selection.Specs) > 0, face)
	if err := s.reconcileSessionMessageProjection(ctx, sessionID); err != nil {
		return nil, selection, ContextStats{}, fmt.Errorf("runtime: reconcile durable session history: %w", err)
	}
	stored, err := s.deps.Messages.ListMessages(ctx, sessionID)
	if err != nil {
		slog.Warn("history rebuild failed; running without session context", "session", string(sessionID), "err", err)
		stored = nil
	}
	// Rewind cutoff first (JOURNAL-REWIND-AND-FORK): the truncation winnows
	// the raw rows, then compaction folds what remains.
	stored, err = s.effectiveSessionMessages(ctx, sessionID, stored)
	if err != nil {
		return nil, selection, ContextStats{}, err
	}
	folded, _ := s.foldSessionHistory(ctx, sessionID, stored)
	msgs, stats, err := buildRunContext(ContextPolicy{
		MaxBytes:           eng.cfg.MaxContextBytes,
		MaxHistoryMessages: eng.cfg.MaxHistoryMessages,
	}, preamble, folded, userText)
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

// foldSessionHistory replaces the stored rows covered by the latest durable
// session compaction with the summary text, keeping the tail verbatim.
// ok=false means no compaction record applies (or the store is unavailable).
// The summary is a user-role message prefixed so the model can tell it
// apart from a real user turn.
func (s *Service) foldSessionHistory(ctx context.Context, sessionID domain.SessionID, stored []domain.Message) ([]domain.Message, bool) {
	if s.deps.Compactions == nil {
		return stored, false
	}
	latest, ok, err := s.deps.Compactions.LatestSessionCompaction(ctx, sessionID)
	if err != nil || !ok || latest.TailFrom <= 0 {
		return stored, false
	}
	idx := 0
	for idx < len(stored) && stored[idx].CreatedAt <= latest.TailFrom {
		idx++
	}
	if idx >= len(stored) {
		// Everything is folded; the feed becomes summary + nothing else.
		idx = len(stored)
	}
	if idx == 0 {
		return stored, false
	}
	kept := stored[idx:]
	summary := domain.Message{
		ID:        newMessageID(),
		SessionID: latest.SessionID,
		Role:      domain.RoleUser,
		CreatedAt: latest.TailFrom,
		Content:   compactionSummaryPrefix + latest.Summary,
	}
	out := make([]domain.Message, 0, len(kept)+1)
	out = append(out, summary)
	out = append(out, kept...)
	return out, true
}

// compactionSummaryPrefix marks a durable session summary inside the feed.
const compactionSummaryPrefix = "【会话压缩摘要，以下为较早对话与工具调用的浓缩：】\n"

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
func (s *Service) consume(ctx context.Context, m *eventMapper, sessionID domain.SessionID, selectedTools []string, mode domain.RunMode, ledger *BudgetLedger, iter *adk.AsyncIterator[*adk.AgentEvent]) {
	if ledger == nil {
		var err error
		ledger, err = NewBudgetLedger(DefaultBudgetPolicy())
		if err != nil {
			s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, err))
			return
		}
	}
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		persistStopped := false
		err := m.onEventEach(ev, func(events []domain.RunEvent) error {
			if err := reserveMappedBudget(ledger, events); err != nil {
				return err
			}
			for _, re := range events {
				if !s.persistAndPublish(ctx, sessionID, re) {
					persistStopped = true
					return context.Canceled
				}
			}
			return nil
		})
		if persistStopped {
			return
		}
		if errors.Is(err, errRunInterrupted) {
			s.handleInterrupt(ctx, m, sessionID, selectedTools, mode)
			return
		}
		if err != nil {
			s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, err))
			// A failed first exchange still leaves a user message worth a
			// title; the generator's truncation fallback names it when no
			// model is reachable.
			s.maybeAutoTitle(ctx, sessionID)
			return
		}
	}

	turnEnd := m.onTurnEnd()
	if err := reserveMappedBudget(ledger, turnEnd); err != nil {
		s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, err))
		return
	}
	for _, re := range turnEnd {
		if !s.persistAndPublish(ctx, sessionID, re) {
			return
		}
	}
	s.emitTerminal(ctx, m, m.build(domain.EventRunCompleted, payloadRunCompleted{}))
	s.maybeAutoTitle(ctx, sessionID)
}

// reserveMappedBudget charges durable non-terminal events and the logical
// model/tool work represented by one mapper batch. A tool-call response may
// contain several calls but is one model generation; the request event count
// remains the exact tool-call count.
//
// Streaming chunk events (model.delta / model.reasoning_delta) are excluded
// from the events budget: one mapped event per streamed chunk makes any
// substantive reply exceed MaxEvents on its own. Runaway-generation safety
// stays with the model-call budget (one charge per generation) and the
// tool-call budget; deltas themselves are also payload-clamped by the mapper.
func reserveMappedBudget(ledger *BudgetLedger, events []domain.RunEvent) error {
	modelCall := false
	for _, re := range events {
		switch re.Type {
		case domain.EventModelDelta, domain.EventModelReasoningDelta:
			continue
		case domain.EventProviderRetry:
			if err := ledger.ReserveRetry(); err != nil {
				return err
			}
		case domain.EventToolRequested:
			modelCall = true
			if err := ledger.ReserveToolCall(); err != nil {
				return err
			}
		case domain.EventModelCompleted:
			modelCall = true
		}
		if err := ledger.ReserveEvent(); err != nil {
			return err
		}
	}
	if modelCall {
		return ledger.ReserveModelCall()
	}
	return nil
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
	proposalData, proposalErr := json.Marshal(details.Args)
	if proposalErr != nil {
		fail(proposalErr)
		return
	}
	proposal, proposalErr := s.engine.PrepareProposal(ctx, details.ToolName, proposalData)
	if proposalErr != nil {
		fail(proposalErr)
		return
	}
	proposal = boundToolProposalReview(proposal, m.maxPayload)
	approval := domain.Approval{
		ID:               newPrefixedID("apr_"),
		RunID:            runID,
		ToolCallID:       details.ToolCallID,
		ToolName:         details.ToolName,
		Decision:         domain.ApprovalPending,
		ExpiresAt:        expiresAt,
		CreatedAt:        time.Now().UnixMilli(),
		ResumeTarget:     details.ResumeTarget,
		Action:           proposal.Action,
		Target:           proposal.Target,
		PreconditionHash: proposal.PreconditionHash,
		Preview:          proposal.Preview,
		RiskFindings:     append([]string(nil), proposal.RiskFindings...),
		ProposalData:     append([]byte(nil), proposal.Data...),
		SandboxMode:      string(sandboxMode(ctx)),
		ApprovalPolicy:   string(approvalPolicy(ctx)),
	}
	if err := s.deps.Approvals.CreateApproval(persistCtx, approval); err != nil {
		fail(err)
		return
	}

	ev := m.build(domain.EventToolApprovalRequired, payloadToolApprovalRequired{
		ApprovalID:       approval.ID,
		ToolCallID:       details.ToolCallID,
		ToolName:         details.ToolName,
		Args:             details.Args,
		ExpiresAt:        expiresAt,
		SelectedTools:    append([]string(nil), selectedTools...),
		Mode:             string(mode),
		Face:             string(runFace(ctx)),
		PolicyProfile:    string(policyProfile(ctx)),
		PolicyHash:       policySnapshot(ctx).Hash,
		SandboxMode:      string(sandboxMode(ctx)),
		ApprovalPolicy:   string(approvalPolicy(ctx)),
		Action:           approval.Action,
		Target:           approval.Target,
		PreconditionHash: approval.PreconditionHash,
		Preview:          approval.Preview,
		RiskFindings:     append([]string(nil), approval.RiskFindings...),
	})
	_, err := s.appendRunEvent(persistCtx, ev, false)
	if err != nil {
		fail(err)
		return
	}

	if ctx.Err() != nil {
		// Cancelled while suspending: close as cancelled. The approval
		// row stays pending in the store; a later decision finds no
		// pending run and stands as a no-op resume.
		s.emitTerminal(ctx, m, m.build(domain.EventRunCancelled, payloadRunCancelled{Reason: reasonUserRequested}))
		return
	}

	ledger := s.ledgerForRun(runID)
	s.mu.Lock()
	s.pending[runID] = pendingRun{
		sessionID: sessionID, mapper: m, selectedTools: append([]string(nil), selectedTools...),
		mounted: tools.MountedToolsFromContext(ctx),
		mode:    mode, profile: policyProfile(ctx), snapshot: policySnapshot(ctx),
		sandboxMode: sandboxMode(ctx), approvalPolicy: approvalPolicy(ctx), face: runFace(ctx), ledger: ledger,
	}
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
		CreatedAt:    time.Now().UnixMilli(),
		ResumeTarget: details.ResumeTarget,
	}
	if err := s.deps.Questions.CreateQuestion(persistCtx, question); err != nil {
		fail(err)
		return
	}
	ev := m.build(domain.EventUserQuestionRequired, payloadUserQuestionRequired{
		QuestionID:     question.ID,
		ToolCallID:     question.ToolCallID,
		Prompt:         prompt,
		ExpiresAt:      question.ExpiresAt,
		ResumeTarget:   question.ResumeTarget,
		SelectedTools:  append([]string(nil), selectedTools...),
		Mode:           string(mode),
		Face:           string(runFace(ctx)),
		PolicyProfile:  string(policyProfile(ctx)),
		PolicyHash:     policySnapshot(ctx).Hash,
		SandboxMode:    string(sandboxMode(ctx)),
		ApprovalPolicy: string(approvalPolicy(ctx)),
	})
	_, err := s.appendRunEvent(persistCtx, ev, false)
	if err != nil {
		fail(err)
		return
	}
	if ctx.Err() != nil {
		_ = s.cancelQuestion(context.Background(), question.ID, "run cancelled while suspending")
		s.emitTerminal(ctx, m, m.build(domain.EventRunCancelled, payloadRunCancelled{Reason: reasonUserRequested}))
		return
	}
	ledger := s.ledgerForRun(runID)
	s.mu.Lock()
	s.pending[runID] = pendingRun{
		sessionID: sessionID, mapper: m,
		selectedTools: append([]string(nil), selectedTools...),
		mounted:       tools.MountedToolsFromContext(ctx),
		mode:          mode, profile: policyProfile(ctx), snapshot: policySnapshot(ctx),
		sandboxMode: sandboxMode(ctx), approvalPolicy: approvalPolicy(ctx),
		face: runFace(ctx), questionID: question.ID, ledger: ledger,
	}
	s.mu.Unlock()
}

// DecideApproval settles a pending approval and resumes the suspended run
// with the decision (FR-6, FR-7). First-writer-wins: a concurrent second
// decision loses with ErrApprovalAlreadyDecided. The resume runs in the
// background; the caller only learns whether the decision was accepted.
func (s *Service) DecideApproval(ctx context.Context, approvalID, decision string) error {
	return s.DecideApprovalWithReason(ctx, approvalID, decision, "")
}

// DecideApprovalWithReason records an optional bounded human rationale and
// emits a durable decision event before any resumed model work is visible.
func (s *Service) DecideApprovalWithReason(ctx context.Context, approvalID, decision, reason string) error {
	if decision != domain.ApprovalApproved && decision != domain.ApprovalDenied {
		return ErrApprovalInvalidDecision
	}
	reason = strings.TrimSpace(reason)
	if len(reason) > 2000 {
		return ErrApprovalInvalidReason
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
	if isShellApproval(approval) {
		if err := s.waitShellApprovalReady(ctx, approval.RunID); err != nil {
			return err
		}
		if time.Now().UnixMilli() >= approval.ExpiresAt {
			return ErrApprovalExpired
		}
	} else if !s.ApprovalRequiredDurable(ctx, approval.RunID, approval.ID) {
		return errors.New("runtime: approval is not durable yet")
	}
	decided, err := s.decideApproval(ctx, approvalID, decision, reason)
	if err != nil {
		return fmt.Errorf("runtime: decide approval: %w", err)
	}
	if !decided {
		// Lost the first-writer-wins race to a concurrent decision.
		return ErrApprovalAlreadyDecided
	}
	decisionPersisted := s.journalReviewEvent(ctx, approval.RunID, domain.EventToolApprovalDecided, payloadApprovalDecided{
		ApprovalID: approval.ID, Decision: decision, Actor: "local_user", Reason: reason, DecidedAt: time.Now().UnixMilli(),
	})
	if approval.Kind == domain.ApprovalKindChild {
		if s.deps.ChildApprovals == nil {
			return errors.New("runtime: child approval router is not wired")
		}
		if err := s.deps.ChildApprovals.ResolveChildApproval(ctx, approval, decision); err != nil {
			return fmt.Errorf("runtime: resolve child approval: %w", err)
		}
		return nil
	}

	s.mu.Lock()
	p, ok := s.pending[approval.RunID]
	if ok {
		delete(s.pending, approval.RunID)
	}
	sp, shellOK := s.shellPending[approval.RunID]
	if shellOK {
		delete(s.shellPending, approval.RunID)
	}
	s.mu.Unlock()
	if shellOK {
		if !decisionPersisted {
			s.deleteShellState(sp.stateRef)
			s.emitTerminal(context.WithoutCancel(ctx), sp.mapper, sp.mapper.build(domain.EventRunFailed, payloadRunFailed{
				CauseCategory: causeInternalError, Message: "The shell approval decision could not be recorded; the command did not run.",
			}))
			return errors.New("runtime: persist shell approval decision failed")
		}
		// The durable decision is the no-replay boundary. Exact args remain in
		// this pending value for the imminent resume, so delete persisted state
		// before a process can start.
		s.deleteShellState(sp.stateRef)
		sp.stateRef = ""
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.resumeShell(sp, approval, decision)
		}()
		return nil
	}
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
		s.resumeRun(p.sessionID, toolName, p.selectedTools, p.mounted, p.mode, p.profile, p.snapshot, p.sandboxMode, p.approvalPolicy, p.face, p.ledger,
			approval.RunID, approval.ToolCallID, approval.ResumeTarget, decision, approval.ProposalData, approval.PreconditionHash, approval.ID)
	}()
	return nil
}

func (s *Service) decideApproval(ctx context.Context, id, decision, reason string) (bool, error) {
	if lifecycle, ok := s.deps.Approvals.(storage.ApprovalLifecycleStore); ok {
		return lifecycle.DecideApprovalWithMetadata(ctx, id, decision, "local_user", reason)
	}
	return s.deps.Approvals.DecideApproval(ctx, id, decision)
}

func (s *Service) cancelApproval(ctx context.Context, approval domain.Approval, reason string) error {
	lifecycle, ok := s.deps.Approvals.(storage.ApprovalLifecycleStore)
	if !ok {
		return nil
	}
	settled, err := lifecycle.CancelApproval(ctx, approval.ID, "local_user", reason)
	if err != nil || !settled {
		if err != nil {
			return err
		}
		return ErrApprovalAlreadyDecided
	}
	s.journalReviewEvent(ctx, approval.RunID, domain.EventToolApprovalCancelled, payloadApprovalCancelled{
		ApprovalID: approval.ID, Actor: "local_user", Reason: reason,
	})
	return nil
}

func (s *Service) cancelQuestion(ctx context.Context, questionID, reason string) error {
	if lifecycle, ok := s.deps.Questions.(storage.QuestionLifecycleStore); ok {
		question, err := s.deps.Questions.GetQuestion(ctx, questionID)
		if err != nil {
			return err
		}
		settled, err := lifecycle.CancelQuestionWithMetadata(ctx, questionID, "local_user", reason)
		if err != nil {
			return err
		}
		if !settled {
			return ErrQuestionAlreadyAnswered
		}
		s.journalReviewEvent(ctx, question.RunID, domain.EventUserQuestionCancelled, payloadQuestionCancelled{
			QuestionID: questionID, Actor: "local_user", Reason: reason,
		})
		return nil
	}
	return s.deps.Questions.CancelQuestion(ctx, questionID)
}

func (s *Service) approvalForRun(ctx context.Context, runID domain.RunID) (domain.Approval, error) {
	approvals, err := s.deps.Approvals.ListPendingApprovals(ctx)
	if err != nil {
		return domain.Approval{}, err
	}
	for _, approval := range approvals {
		if approval.RunID == runID {
			return approval, nil
		}
	}
	return domain.Approval{}, storage.ErrNotFound
}

func (s *Service) journalReviewEvent(ctx context.Context, runID domain.RunID, eventType domain.EventType, payload any) bool {
	m := newEventMapper(runID, s.engine.cfg.MaxEventPayloadBytes)
	ev := m.build(eventType, payload)
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistTimeout)
	defer cancel()
	_, err := s.appendRunEvent(persistCtx, ev, false)
	if err != nil {
		slog.Warn("review event persistence failed", "run", string(runID), "type", string(eventType), "err", err)
		return false
	}
	return true
}

func (s *Service) expireApproval(ctx context.Context, approval domain.Approval, reason string) error {
	if isShellApproval(approval) {
		s.mu.Lock()
		_, inProcess := s.shellPending[approval.RunID]
		s.mu.Unlock()
		if inProcess {
			if err := s.waitShellApprovalReady(ctx, approval.RunID); err != nil {
				return err
			}
		}
	}
	lifecycle, ok := s.deps.Approvals.(storage.ApprovalLifecycleStore)
	if !ok {
		return nil
	}
	settled, err := lifecycle.ExpireApproval(ctx, approval.ID, reason)
	if err != nil || !settled {
		return err
	}
	s.journalReviewEvent(ctx, approval.RunID, domain.EventToolApprovalExpired, payloadInteractionExpired{
		ReviewID: approval.ID, Kind: string(domain.ReviewKindApproval), ExpiresAt: approval.ExpiresAt, Reason: reason,
	})
	s.mu.Lock()
	p, pending := s.pending[approval.RunID]
	if pending {
		delete(s.pending, approval.RunID)
	}
	sp, shellPending := s.shellPending[approval.RunID]
	if shellPending {
		delete(s.shellPending, approval.RunID)
	}
	s.mu.Unlock()
	if pending {
		s.emitTerminal(ctx, p.mapper, p.mapper.build(domain.EventRunFailed, payloadRunFailed{
			CauseCategory: causeHumanTimeout,
			Message:       "The run stopped because human review timed out.",
		}))
	} else if shellPending {
		s.deleteShellState(sp.stateRef)
		s.emitTerminal(ctx, sp.mapper, sp.mapper.build(domain.EventRunFailed, payloadRunFailed{
			CauseCategory: causeHumanTimeout,
			Message:       "The run stopped because human review timed out.",
		}))
	} else {
		s.failHumanTimeout(ctx, approval.RunID, reason)
	}
	return nil
}

func (s *Service) expireQuestion(ctx context.Context, question domain.Question, reason string) error {
	lifecycle, ok := s.deps.Questions.(storage.QuestionLifecycleStore)
	if !ok {
		return nil
	}
	settled, err := lifecycle.ExpireQuestion(ctx, question.ID, reason)
	if err != nil || !settled {
		return err
	}
	s.journalReviewEvent(ctx, question.RunID, domain.EventUserQuestionExpired, payloadInteractionExpired{
		ReviewID: question.ID, Kind: string(domain.ReviewKindQuestion), ExpiresAt: question.ExpiresAt, Reason: reason,
	})
	s.mu.Lock()
	p, pending := s.pending[question.RunID]
	if pending {
		delete(s.pending, question.RunID)
	}
	s.mu.Unlock()
	if pending {
		s.emitTerminal(ctx, p.mapper, p.mapper.build(domain.EventRunFailed, payloadRunFailed{
			CauseCategory: causeHumanTimeout,
			Message:       "The run stopped because a user response timed out.",
		}))
	} else {
		s.failHumanTimeout(ctx, question.RunID, reason)
	}
	return nil
}

func (s *Service) failHumanTimeout(ctx context.Context, runID domain.RunID, reason string) {
	m := newEventMapper(runID, s.engine.cfg.MaxEventPayloadBytes)
	s.emitTerminal(ctx, m, m.build(domain.EventRunFailed, payloadRunFailed{
		CauseCategory: causeHumanTimeout,
		Message:       "The run stopped because human review timed out.",
	}))
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
	answered, err := s.answerQuestion(ctx, questionID, answer)
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
	ev, err = s.appendRunEvent(persistCtx, ev, false)
	if err != nil {
		return fmt.Errorf("runtime: persist question answer: %w", err)
	}
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
		s.resumeRun(p.sessionID, toolName, p.selectedTools, p.mounted, p.mode, p.profile, p.snapshot, p.sandboxMode, p.approvalPolicy, p.face, p.ledger,
			question.RunID, question.ToolCallID, question.ResumeTarget, answer, nil, "", "")
	}()
	return nil
}

// CancelQuestion explicitly closes a pending ask_user interaction and the
// owning run. It is the Review Center equivalent of cancelling the run from
// the conversation view.
func (s *Service) CancelQuestion(ctx context.Context, questionID, reason string) error {
	if s.deps.Questions == nil {
		return errors.New("runtime: question store not wired")
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
	reason = strings.TrimSpace(reason)
	if len(reason) > 2000 {
		return ErrApprovalInvalidReason
	}
	if err := s.cancelQuestion(ctx, questionID, reason); err != nil {
		return fmt.Errorf("runtime: cancel question: %w", err)
	}
	s.mu.Lock()
	p, ok := s.pending[question.RunID]
	if ok {
		delete(s.pending, question.RunID)
	}
	s.mu.Unlock()
	if ok {
		s.emitTerminal(ctx, p.mapper, p.mapper.build(domain.EventRunCancelled, payloadRunCancelled{Reason: reasonUserRequested}))
	}
	return nil
}

func (s *Service) answerQuestion(ctx context.Context, questionID, answer string) (bool, error) {
	if lifecycle, ok := s.deps.Questions.(storage.QuestionLifecycleStore); ok {
		return lifecycle.AnswerQuestionWithMetadata(ctx, questionID, answer, "local_user", "")
	}
	return s.deps.Questions.AnswerQuestion(ctx, questionID, answer)
}

func pendingToolName(p pendingRun, callID string) string {
	for _, oc := range p.mapper.openCalls {
		if oc.id == callID {
			return oc.name
		}
	}
	return ""
}

func (s *Service) ledgerForRun(runID domain.RunID) *BudgetLedger {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ledgers[runID]
}

// resumeRun feeds the decision back into the engine and maps the resumed
// events into the same journal (the journal continues the seq).
func (s *Service) resumeRun(sessionID domain.SessionID, toolName string, selectedTools []string, mounted *tools.MountedTools, mode domain.RunMode, profile domain.PolicyProfile, snapshot domain.PolicySnapshot, sandboxMode domain.SandboxMode, approvalPolicy domain.ApprovalPolicy, face domain.Face, ledger *BudgetLedger, runID domain.RunID, toolCallID, resumeTarget, resumeValue string, proposalData []byte, preconditionHash, approvalID string) {
	s.mu.Lock()
	if s.runTools[runID] == nil {
		selected := make(map[string]struct{}, len(selectedTools))
		for _, name := range selectedTools {
			selected[name] = struct{}{}
		}
		s.runTools[runID] = selected
	}
	s.mu.Unlock()
	// A deferred settings-save engine rebuild applies here too, while the
	// resumed run is not yet registered.
	if err := s.applyPendingEngineReload(context.Background(), nil); err != nil {
		slog.Warn("pending engine reload failed before resume", "run", string(runID), "err", err)
	}
	m := s.newResumeEventMapper(context.Background(), runID)
	if toolCallID != "" {
		// The resume replays the decided tool result first; seed the open
		// call so reconstructed tool.started/finished keep the call id.
		m.registerOpenCall(openToolCall{id: toolCallID, name: toolName})
	}
	ctx := withSessionID(withRunID(withPolicySnapshot(withPolicyProfile(withRunMode(withFace(withSelectedTools(context.Background(), selectedTools), face), mode), profile), snapshot), runID), sessionID)
	ctx = withSessionSandbox(ctx, sandboxMode, approvalPolicy)
	ctx = tools.WithSessionID(ctx, sessionID)
	// Restore the skill mounts captured at suspend time so tools mounted
	// before the interrupt stay callable after resume (TT-2). A nil
	// registry (restart recovery) falls back to a fresh one so a
	// skill_view in the resumed segment can still mount new tools.
	mounts := mounted
	if mounts == nil {
		mounts = tools.NewMountedTools()
	}
	ctx = tools.WithMountedTools(ctx, mounts)
	ctx = tools.WithProposalData(ctx, proposalData)
	ctx = tools.WithProposalPrecondition(ctx, preconditionHash)
	if approvalID != "" {
		ctx = tools.WithProposalStaleReporter(ctx, func(reason string) {
			if lifecycle, ok := s.deps.Approvals.(storage.ApprovalLifecycleStore); ok {
				if _, err := lifecycle.MarkApprovalStale(context.Background(), approvalID, reason); err != nil {
					slog.Warn("mark stale approval failed", "approval", approvalID, "err", err)
				}
			}
			s.journalReviewEvent(context.Background(), runID, domain.EventToolProposalStale, payloadProposalStale{
				ApprovalID: approvalID, Reason: reason,
			})
		})
	}
	ctx = withGovernanceEventSink(ctx, s.governanceSink(m, sessionID, ledger))
	ctx = s.withLiveModelStreamObserver(ctx, m, sessionID, ledger)
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
	s.consume(ctx, m, sessionID, selectedTools, mode, ledger, iter)
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
	if errors.Is(cause, errLoopDetected) {
		// Tool-loop guardrail (VC-2, Crush-aligned StopWhen): the same
		// call+result signature repeated past the window limit. The
		// message stays bounded — no signatures or internals leak (FR-11).
		slog.Warn("run failed: tool loop detected", "run", string(m.runID))
		return m.build(domain.EventRunFailed, payloadRunFailed{
			CauseCategory: causeLoopDetected,
			Message:       "The run was stopped because the same tool call kept repeating without making progress. Please rephrase the request or adjust the task.",
		})
	}
	if errors.Is(cause, ErrContextBudgetExceeded) {
		slog.Warn("run failed: context budget exceeded", "run", string(m.runID), "err", cause)
		return m.build(domain.EventRunFailed, payloadRunFailed{
			CauseCategory: causeInternalError,
			Message:       "The run context exceeds the configured limit. Please start a shorter request or raise the context budget.",
		})
	}
	if errors.Is(cause, ErrBudgetExceeded) {
		var exceeded *BudgetExceededError
		if errors.As(cause, &exceeded) {
			slog.Warn("run budget circuit breaker opened", "run", string(m.runID), "kind", exceeded.Kind, "limit", exceeded.Limit)
		}
		return m.build(domain.EventRunFailed, payloadRunFailed{
			CauseCategory: causeInternalError,
			Message:       "The run was stopped because it reached a safety budget. Please try again with a smaller request.",
		})
	}
	slog.Warn("run failed", "run", string(m.runID), "err", cause)
	category := causeCategoryOf(cause)
	message := "The model run could not be completed. Please try again."
	if category == causeProviderError {
		message = providerUnavailableMessage
	}
	return m.build(domain.EventRunFailed, payloadRunFailed{
		CauseCategory: category,
		Message:       message,
	})
}

func causeCategoryOf(err error) string {
	// Provider failures are classified so the UI can distinguish "backend
	// unreachable" from "model/key problem" instead of a generic retry
	// message (FR-11: the message stays structured and never leaks values).
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return causeCancelled
	case isProviderFailure(err):
		return causeProviderError
	default:
		return causeInternalError
	}
}

func isProviderFailure(err error) bool {
	if errors.Is(err, provider.ErrModelNotConfigured) {
		return true
	}
	if _, ok := keyMissingMessage(err); ok {
		return true
	}
	return isProviderTransportError(err)
}

// keyMissingMessage returns a user-actionable hint when the failure is a
// missing provider key. The provider's KeyMissingError already carries the
// stable, non-leaky wording; when the chain is wrapped by the engine, fall
// back to the message marker (D-010: never a key value).
func keyMissingMessage(err error) (string, bool) {
	var keyMissing *provider.KeyMissingError
	if errors.As(err, &keyMissing) {
		return keyMissing.Error(), true
	}
	if strings.Contains(err.Error(), "API key missing") {
		return "API key missing: configure a model API key in the welcome wizard or Settings → Model.", true
	}
	return "", false
}

// providerTransportMarkers recognize transport-level provider failures
// (connection, DNS, TLS, HTTP status) so the UI can tell "model service
// unreachable" apart from an internal engine error.
var providerTransportMarkers = []string{
	"connection refused", "connect:", "no such host", "timeout", "timed out",
	"unexpected eof", "tls", "status code", "http: server gave", "http request failed",
	"network unreachable", "connection reset", "could not be reached",
}

func isProviderTransportError(err error) bool {
	msg := strings.ToLower(err.Error())
	for _, marker := range providerTransportMarkers {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

// persistAndPublish appends the single event as its own commit and, only
// on success, writes back the assigned seq and hands the event to the
// sink. A journal failure closes the run via the classified terminal: a
// cancellation racing the append must land as run.cancelled, not
// run.failed (AS-5); any other failure is run.failed so the run still
// closes exactly once.
func (s *Service) persistAndPublish(ctx context.Context, sessionID domain.SessionID, re domain.RunEvent) bool {
	s.projectionMu.Lock()
	if s.sessionDeleted(sessionID) {
		s.cleanupRunState(re.RunID)
		s.projectionMu.Unlock()
		return false
	}
	seq, err := s.deps.Journal.Append(ctx, storage.Commit{RunID: re.RunID, Events: []domain.RunEvent{re}})
	if err != nil {
		s.projectionMu.Unlock()
		slog.Error("journal append failed", "run", string(re.RunID), "type", string(re.Type), "err", err)
		m := newEventMapper(re.RunID, 0)
		s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, err))
		return false
	}
	re.Seq = seq
	if re.Type == domain.EventModelCompleted || re.Type == domain.EventToolRequested || re.Type == domain.EventToolFinished {
		projectCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistTimeout)
		err := s.projectRunMessagesLocked(projectCtx, sessionID, re.RunID)
		cancel()
		if err != nil {
			slog.Error("project Journal messages", "run", string(re.RunID), "type", string(re.Type), "err", err)
		}
	}
	s.publish(ctx, re)
	s.projectionMu.Unlock()
	return true
}

// emitTerminal persists the terminal event best-effort, flips the run row
// to the matching status, publishes the event, and unregisters the cancel
// handle. Persistence is detached from the run context: a cancellation
// must not strand the run without its terminal record (AS-5, FR-8). The
// publish carries no frame itself — the bus closes its live subscribers
// on a terminal publish, which sends RPC subscribers back to the journal
// replay where the event is delivered exactly once (AS-7).
func (s *Service) emitTerminal(ctx context.Context, m *eventMapper, terminal domain.RunEvent) {
	terminal.RunID = m.runID
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	if s.runSessionDeleted(terminal.RunID) {
		s.cleanupRunState(terminal.RunID)
		return
	}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistTimeout)
	defer cancel()
	seq, err := s.deps.Journal.Append(persistCtx, storage.Commit{RunID: terminal.RunID, Events: []domain.RunEvent{terminal}})
	if err != nil {
		// The run already holds a terminal (concurrent close) or the
		// backend failed; either way the stored truth wins and the row
		// must not be flipped here.
		slog.Error("journal append of terminal event failed", "run", string(terminal.RunID), "type", string(terminal.Type), "err", err)
		s.cleanupRunState(terminal.RunID)
		return
	}
	terminal.Seq = seq

	status := domain.RunCompleted
	if mapped, ok := terminal.Type.RunStatus(); ok {
		status = mapped
	}
	if err := s.deps.Runs.SetRunStatus(persistCtx, terminal.RunID, status); err != nil {
		slog.Error("set terminal run status", "run", string(terminal.RunID), "status", string(status), "err", err)
	}
	s.publish(persistCtx, terminal)

	s.mu.Lock()
	var shellStateRefToDelete string
	if c, ok := s.active[terminal.RunID]; ok {
		delete(s.active, terminal.RunID)
		c() // idempotent: releases the detached run context
	}
	if pending, ok := s.shellPending[terminal.RunID]; ok {
		delete(s.shellPending, terminal.RunID)
		shellStateRefToDelete = pending.stateRef
	}
	delete(s.ledgers, terminal.RunID)
	delete(s.snapshots, terminal.RunID)
	delete(s.runTools, terminal.RunID)
	delete(s.runSessions, terminal.RunID)
	s.mu.Unlock()
	s.deleteShellState(shellStateRefToDelete)
}

func (s *Service) cleanupRunState(runID domain.RunID) {
	var shellStateRefToDelete string
	s.mu.Lock()
	if c, ok := s.active[runID]; ok {
		delete(s.active, runID)
		c()
	}
	delete(s.pending, runID)
	if pending, ok := s.shellPending[runID]; ok {
		delete(s.shellPending, runID)
		shellStateRefToDelete = pending.stateRef
	}
	delete(s.ledgers, runID)
	delete(s.snapshots, runID)
	delete(s.runTools, runID)
	delete(s.runSessions, runID)
	s.mu.Unlock()
	s.deleteShellState(shellStateRefToDelete)
}

func (s *Service) publish(ctx context.Context, ev domain.RunEvent) {
	s.deps.Sink.Publish(ev)
	for _, hook := range s.deps.Hooks {
		hook.OnRunEvent(ctx, ev)
	}
}

func (s *Service) governanceSink(m *eventMapper, sessionID domain.SessionID, ledger *BudgetLedger) GovernanceEventSink {
	return func(ctx context.Context, event GovernanceEvent) error {
		event.Reason = tools.RedactSensitive(event.Reason)
		if isDirectShell(ctx) && event.Reason != "" {
			// Hook programs are untrusted and may echo their input in a reason.
			// Direct-shell audit data records the decision, never hook text that
			// could contain the raw script.
			event.Reason = "governed shell " + string(event.Type)
		}
		if len(event.Reason) > 512 {
			event.Reason = event.Reason[:512] + "..."
		}
		if event.Profile == "" {
			event.Profile = policyProfile(ctx)
		}
		if event.PolicyHash == "" {
			event.PolicyHash = policySnapshot(ctx).Hash
		}
		var payload any
		switch event.Type {
		case domain.EventPolicyEvaluated:
			payload = payloadPolicyEvaluated{
				ToolName: event.ToolName, Decision: event.Decision,
				Profile: string(event.Profile), Hash: event.PolicyHash, Reason: event.Reason,
			}
		case domain.EventHookStarted, domain.EventHookCompleted, domain.EventHookBlocked:
			payload = payloadHookLifecycle{
				ToolName: event.ToolName, HookName: event.HookName, Phase: event.Phase,
				Decision: event.Decision, Profile: string(event.Profile), Reason: event.Reason,
				DurationMs: event.DurationMs,
			}
		case domain.EventToolMounted:
			payload = payloadToolMounted{
				ToolName: event.ToolName, Tools: append([]string(nil), event.MountedTools...),
			}
		case domain.EventContextCompacted:
			payload = payloadContextCompacted{
				Mode: event.Mode, BeforeTokens: event.BeforeTokens, AfterTokens: event.AfterTokens,
				DroppedMessages: event.DroppedMessages, RetentionSuffix: event.RetentionSuffix,
			}
		default:
			return fmt.Errorf("runtime: unsupported governance event %q", event.Type)
		}
		if ledger != nil {
			if err := ledger.ReserveEvent(); err != nil {
				return err
			}
			// A summarization compaction ran one (or more) hidden model
			// generation; charge it against MaxModelCalls so the summary
			// call cannot bypass the run-tree budget (research P3 bridge ii).
			if event.Type == domain.EventContextCompacted && event.Mode == "summarization" {
				if err := ledger.ReserveModelCall(); err != nil {
					return err
				}
			}
		}
		re := m.build(event.Type, payload)
		if !s.persistAndPublish(ctx, sessionID, re) {
			return errors.New("runtime: persist governance event")
		}
		if event.Type == domain.EventContextCompacted {
			s.recordLastCompaction(sessionID, &LastCompaction{
				Mode: event.Mode, BeforeTokens: event.BeforeTokens, AfterTokens: event.AfterTokens, At: time.Now().UnixMilli(),
			})
		}
		return nil
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

func (s *Service) sessionSandbox(ctx context.Context, sessionID domain.SessionID) (domain.SandboxMode, domain.ApprovalPolicy) {
	if s.deps.Sessions == nil {
		return domain.SandboxModeWorkspaceWrite, domain.ApprovalPolicyAsk
	}
	session, err := s.deps.Sessions.GetSession(ctx, sessionID)
	if err != nil {
		return domain.SandboxModeWorkspaceWrite, domain.ApprovalPolicyAsk
	}
	return session.EffectiveSandbox()
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
