Warning: truncated output (original token count: 53501)
Total output lines: 5540

package runtime

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/contexthost"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/maskcontract"
	"agent-vivy/internal/provider"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

// terminalPersistTimeout caps the detached persistence window for the
// terminal event: closing a run durably must survive the user cancelling
// their own context, but must also stay bounded (NFR: bounded).
const terminalPersistTimeout = 30 * time.Second

// EventSink receives persisted run events for live fan-out. events.Bus
// implements it; the interface lives here so the runtime never imports
// the events package (app wires the two together).
type EventSink interface {
	Publish(domain.RunEvent)
}

// WorkEventSink receives committed session work events for live fan-out.
// Durable work storage remains the source of truth; this is only a wake-up
// optimization for connected control-plane clients.
type WorkEventSink interface {
	Publish(domain.WorkEvent)
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

// WorkspaceReleaser is an optional rollback seam for allocators that create
// filesystem state during Ensure. Admission failures must not leave an
// unowned per-run directory behind; selected project and local workspaces may
// implement this as a no-op.
type WorkspaceReleaser interface {
	Release(context.Context, Workspace) error
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
	// ErrContinuityUnavailable rejects a continuity submission when the wired
	// backend does not implement the atomic ContinuityStore seam; submissions
	// must fail unavailable rather than silently degrade to a non-atomic write.
	ErrContinuityUnavailable = errors.New("runtime: continuity storage is unavailable")
)

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

// ServiceDeps groups the storage and fan-out dependencies of Service.
type ServiceDeps struct {
	Journal storage.Journal
	// Skills is the host-owned local Skill authority used by DIVA proposals and recovery.
	Skills tools.SkillOperations
	// ToolOperations is the Journal-transactional recovery index for logical
	// tool calls. NewService discovers it from Journal when the backend also
	// implements storage.ToolOperationStore.
	ToolOperations storage.ToolOperationStore
	Runs           storage.RunStore
	Messages       storage.MessageStore
	Work           storage.WorkStore
	GoalRuns       storage.GoalRunStore
	PrimaryRuns    storage.PrimaryRunStore
	// TenantID is the process-owned isolation identity forwarded to every
	// ContextHost request and terminal Observer projection. Empty means the
	// single-tenant local organism.
	TenantID string
	// Approvals persists the approval rows behind the effectful tool
	// gate (C6); nil leaves interrupts unable to suspend.
	Approvals storage.ApprovalStore
	// ApprovalExpiration bounds how long a pending approval stays valid
	// (D-009).
	ApprovalExpiration time.Duration
	// ApprovalSettleTimeout bounds human review under the smart preset
	// (runtime.sandbox.approval.timeout_seconds): at the deadline the runtime
	// approves the call on the user's behalf instead of letting it expire.
	// Zero disables timed auto-approval, so ApprovalExpiration governs alone.
	ApprovalSettleTimeout time.Duration
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
	WorkSink             WorkEventSink
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
	// ExportDir is the instance directory session/export writes standalone
	// HTML transcripts into (VCP C1). Empty leaves session/export
	// unavailable; the tree, clone, and import paths are unaffected.
	ExportDir string
	// Continuity commits task admissions and guarded operations atomically
	// (SC-D4 §7/§9). Nil keeps ordinary submission working; a continuity
	// submission then fails unavailable instead of silently degrading to a
	// non-atomic write.
	Continuity storage.ContinuityStore
	// References snapshots operator-selected context excerpts during
	// admission (SC-D4 §7). Nil fails a reference-bearing submission
	// unavailable; ordinary runs and model-side preview/attach are
	// unaffected by this dep.
	References *ReferenceService
	// Deliverables closes live verified-download transfers when a session is
	// deleted (SC-D4 §8/§9). Nil leaves delivery presentation unwired.
	Deliverables SessionTransferCloser
	// Admission is the optional atomic primary-run boundary. First-party App
	// wiring supplies it when prompt snapshots are enabled; legacy embedders
	// retain the existing sequential path when it is absent.
	Admission storage.RunAdmissionStore
	// Bundles is the shared catalog/evidence store used by the organism-wide
	// toolkit publication drain. It is optional for services that do not expose
	// toolkit publication; once wired, drain checks fail closed if storage is
	// unavailable.
	Bundles storage.BundleStore
	// Toolkits supplies immutable captured Views and live executable fences.
	// It is nil until the Toolkit coordinator is wired by the composition root.
	Toolkits ToolkitRuntime
	// Cognitive carries the bound authority ports for trusted strategy
	// workflows (StartCognitiveWorkflow). Nil keeps trusted strategy
	// admission unavailable; the authored workflow path is unaffected.
	Cognitive *CognitiveBinding
	// ChildSessions is the atomic addressable-child admission boundary. NewService
	// discovers it from Sessions when the backend supports continuable children.
	ChildSessions storage.ChildSessionStore
	// ChildMailbox persists direct parent-child messages. NewService discovers
	// it from Sessions when the backend supports continuable children.
	ChildMailbox storage.ChildMailboxStore
	// WorkflowRevisions stores immutable validated workflow descriptors. NewService
	// discovers it from Sessions when the backend supports workflow admission.
	WorkflowRevisions storage.WorkflowRevisionStore
	// WorkflowDefinitions stores reusable INOFY workflow definitions (draft CAS +
	// immutable published revisions, S11-F). NewService discovers it from
	// Sessions; when the backend does not implement it the whole workflow
	// product surface (drafts, publishing, product runs) stays unavailable
	// while the core task-graph path keeps working.
	WorkflowDefinitions storage.WorkflowDefinitionStore
	// Report is the bounded report authority (settings, generations, source
	// reads). NewService discovers it from Sessions; nil keeps the reports
	// capability closed — admission and effect dispatch fail unavailable.
	Report storage.ReportStore
	// MaskResolver is the selected generation's narrow runtime-facing mask
	// seam. Runtime never holds the provider's control-plane Manager.
	MaskResolver maskcontract.Resolver
	// MaskFrame and MaskFrameDigest are immutable provider assets copied by the
	// composition root. Keeping them separate prevents runtime from reaching
	// through the control-plane service for prompt data.
	MaskFrame       string
	MaskFrameDigest string
	GenerationID    string
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
	// CacheWarmingMode selects the prompt-cache warming trigger
	// (runtime.cache_warming): "off" or "streaming". Empty disables warming.
	CacheWarmingMode string
	// CacheWarmingMinSavingsUSD gates estimated gross savings on one
	// future reuse of the actual system/tools prefix, excluding warm cost.
	CacheWarmingMinSavingsUSD float64
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

	mu               sync.Mutex
	bundleGateMu     sync.Mutex
	bundleDrainID    string
	bundleAdmissions int
	active           map[domain.RunID]context.CancelFunc
	bundleViews      map[domain.RunID]ToolkitViewHandle
	bundleChecks     map[domain.RunID]bundlecontract.CaptureCheck
	workflowStartMu  sync.Mutex
	workflowNodeMu   sync.Mutex
	operationMu      sync.Mutex
	operationFlights map[string]*toolOperationFlight
	goalStarting     map[domain.SessionID]struct{}
	goalRuns         map[domain.SessionID]domain.RunID
	goalDisarmed     map[domain.SessionID]struct{}
	planTransitions  map[domain.SessionID]uint64
	planCancelling   map[domain.SessionID]uint64
	goalRunSessions  map[domain.RunID]domain.SessionID
	goalRunRefs      map[domain.RunID]domain.GoalRef
	admissionLocksMu sync.Mutex
	admissionLocks   map[domain.SessionID]*sync.Mutex
	humanIntentLocks map[domain.SessionID]*sync.Mutex
	goalWakeRunning  map[domain.SessionID]struct{}
	goalWakePending  map[domain.SessionID]struct{}
	goalWG           sync.WaitGroup
	stopping         bool
	humanPending     map[domain.SessionID]int
	// runSessions keeps the session identity for live/suspended runs so a
	// concurrent session deletion can seal every producer before removing the
	// durable rows. deletedSessions is a process-local tombstone: once delete
	// starts, no later event or projection may resurrect that session.
	runSessions     map[domain.RunID]domain.SessionID
	deletedSessions map[domain.SessionID]struct{}
	closingSessions map[domain.SessionID]struct{} // permits only terminal persistence before deletion
	// pending tracks runs suspended on a review or question: no engine work is
	// in flight, the checkpoint is durable, and the run row stays active until
	// a response resumes it or Cancel closes it.
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
	ledgers                map[domain.RunID]*BudgetLedger
	recoveredBudgetLedgers map[domain.RunID]*BudgetLedger
	// snapshots pin the policy authority for active child workers. This is
	// process state only; durable run.started remains the restart truth.
	snapshots map[domain.RunID]domain.PolicySnapshot
	// queues holds per-session dual-track turn queues (VCP-B1).
	queues map[domain.SessionID]*sessionQueue
	// toolActivations caches each session's deferred-tool activation set
	// (VCP-E2), folded once from the session's journals and updated live
	// by the journal pipeline and the tools/activate control path. Its own
	// mutex avoids ordering against s.mu/projectionMu.
	toolActivationMu sync.Mutex
	toolActivations  map[domain.SessionID]*tools.ToolActivation
	// steerCancels holds the adk boundary-cancel handle per live run; armed
	// marks a boundary cancel triggered by a steer (vs a user abort).
	steerCancels map[domain.RunID]adk.AgentCancelFunc
	steerArmed   map[domain.RunID]bool
	// runTools pins the compiler/runtime-selected ToolHost surface for each
	// live run. Action bridges must use this exact set rather than resolving
	// the process-wide registry again.
	runTools   map[domain.RunID]map[string]struct{}
	workFenced map[domain.RunID]struct{}
	workGates  map[domain.RunID]*sync.Mutex
	// cognitive is the automatic wake loop for trusted strategy admission;
	// nil until StartCognitiveLoop starts it.
	cogMu     sync.Mutex
	cognitive *cognitiveRuntime
	// cogStateMu serializes admission, policy CAS and accepted-source updates
	// on their shared durable record. Workflow execution remains asynchronous.
	cogStateMu sync.Mutex
	// workBlockedCalls records the sibling tool-call IDs from a model batch
	// interrupted by submit_plan. Fresh model calls remain eligible after the
	// human decision, while unreviewed siblings from the original batch stay
	// fenced for the rest of that run.
	workBlockedCalls map[domain.RunID]map[string]struct{}
	// contextViews memoizes each live run's committed Context View so resume
	// paths skip a full Journal replay. Entries die with the run in
	// cleanupRunState.
	contextViews map[domain.RunID]string
	// pendingEngine holds a settings-save engine rebuild that was deferred
	// because runs were in flight; it is applied at the next idle run
	// start (ScheduleEngineReload).
	pendingEngine *EngineConfig
	// lastCompaction is process-local observability for the UI (the
	// durable record lives in the journal as context.compacted events).
	lastCompaction map[domain.SessionID]*LastCompaction
	// overflowRecovered marks runs that already spent their single
	// compact-and-retry recovery (VCP-D2); entries die in cleanupRunState.
	overflowRecovered map[domain.RunID]bool
	// overflowAwaiting records a dispatched recovery attempt (run → attempt)
	// whose auto_retry.finished the consume loop still owes the journal:
	// cleared on the first event after the retried call resolves, or on the
	// run's terminal error.
	overflowAwaiting map[domain.RunID]int
	// wg tracks every drive/resume goroutine so shutdown can drain the
	// service before closing storage (E4): terminal events must persist
	// while the journal is still open.
	wg              sync.WaitGroup
	recoveryMu      sync.Mutex
	skillProposalMu sync.Mutex
	sweepMu         sync.Mutex
	projectionMu    sync.Mutex
	sweepCancel     context.CancelFunc
	sweepWG         sync.WaitGroup

	// cron holds the CRON scheduler state; nil until first use and only
	// meaningful when deps.Crons is wired (lazy init guarded by cronInit).
	cron     *cronState
	cronInit sync.Mutex

	// approvalSettle is the live smart-mode review window
	// (runtime.sandbox.approval.timeout_seconds). A settings save replaces it
	// without a restart; a value <= 0 disables timed auto-approval. Guarded
	// by mu.
	approvalSettle time.Duration
}

// ErrModelChangeBusy means a model selection cannot be changed while any
// main, suspended, shell, or child run may still issue another model call.
var ErrModelChangeBusy = errors.New("runtime: model cannot change while runs are active or suspended")

type pendingRun struct {
	// runCtx keeps the same cancellation lifetime across suspend/resume.
	// Recovered pending runs receive a fresh process-owned context.
	runCtx        context.Context
	sessionID     domain.SessionID
	workspaceID   string
	mapper        *eventMapper
	selectedTools []string
	// mounted is the run's skill-mount registry captured at suspend time.
	// Restart recovery rebuilds it from the journal's tool.mounted events
	// (recoveredMounts); nil means the run mounted nothing.
	mounted          *tools.MountedTools
	mode             domain.RunMode
	profile          domain.PolicyProfile
	snapshot         domain.PolicySnapshot
	sandboxMode      domain.SandboxMode
	approvalPolicy   domain.ApprovalPolicy
	face             domain.Face
	approvalID       string
	questionID       string
	ledger           *BudgetLedger
	engine           *Engine
	execution        runExecutionOptions
	planSubmissionID string
	planToolCallID   string
	planResumeTarget string
	planBlockedCalls []string
}

// RunOptions controls the physical policy applied to one run.
// GoalRoundAdmission carries the host-authenticated admission envelope for
// one automatic Goal round. RunID is allocated before the durable commit so
// retries can identify the same accepted run.
type GoalRoundAdmission struct {
	ExpectedVersion domain.WorkVersion
	RequestID       string
	RequestHash     string
	Goal            domain.GoalRef
	Round           int
	RunID           domain.RunID
}

// RunOptions controls the physical policy applied to one run.
type RunOptions struct {
	queueItems []domain.QueuedTurn
	queueAfter domain.RunID
	Mode       domain.RunMode
	Profile    domain.PolicyProfile
	// CollaborationMode is orthogonal soft guidance. It never changes
	// execution policy; legacy RunModePlan remains hard policy.
	CollaborationMode    domain.CollaborationMode
	CollaborationVersion int
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
	// Continuity carries an atomic task admission request (SC-D4 §7): a
	// caller-stable request_id deduplicates retries, HistoryScope resolves
	// into the accepted scope journaled on run.started, and reference
	// selectors pin source expectations the commit transaction rechecks.
	// Nil keeps the ordinary admission path.
	Continuity *domain.ContinuityInput
	// GoalRound requests the atomic Goal admission path. Ordinary callers
	// leave it nil and retain the legacy startup sequence.
	GoalRound *GoalRoundAdmission
	// HumanAdmission registers a process-local startup intent before it waits
	// on the session gate. It is set only by the authenticated control plane,
	// never by model/tool JSON; it is not a durable queued Run or ticket.
	HumanAdmission bool
	// BeforeStart runs synchronously after the run ID is minted and before
	// any run state is persisted or published. Channel ingress uses it to
	// durably arm the outbound delivery target before terminal events can race.
	BeforeStart func(domain.RunID) error
}

// NewService wires the run service over an engine and its dependencies.
// provider and modelID label the run.started payload.
func NewService(eng *Engine, provider, modelID string, deps ServiceDeps) *Service {
	if deps.ToolOperations == nil {
		deps.ToolOperations, _ = deps.Journal.(storage.ToolOperationStore)
	}
	if deps.ChildSessions == nil {
		deps.ChildSessions, _ = deps.Sessions.(storage.ChildSessionStore)
	}
	if deps.ChildMailbox == nil {
		deps.ChildMailbox, _ = deps.Sessions.(storage.ChildMailboxStore)
	}
	if deps.WorkflowRevisions == nil {
		deps.WorkflowRevisions, _ = deps.Sessions.(storage.WorkflowRevisionStore)
	}
	if deps.WorkflowDefinitions == nil {
		deps.WorkflowDefinitions, _ = deps.Sessions.(storage.WorkflowDefinitionStore)
	}
	if deps.Report == nil {
		deps.Report, _ = deps.Sessions.(storage.ReportStore)
	}
	if strings.TrimSpace(deps.TenantID) == "" {
		deps.TenantID = "local"
	} else {
		deps.TenantID = strings.TrimSpace(deps.TenantID)
	}
	if deps.Budget == (BudgetPolicy{}) {
		deps.Budget = DefaultBudgetPolicy()
	}
	if !deps.PolicyDefaultProfile.Valid() {
		deps.PolicyDefaultProfile = domain.PolicyProfileDefault
	}
	if deps.PrimaryRuns == nil {
		deps.PrimaryRuns, _ = deps.Runs.(storage.PrimaryRunStore)
	}
	svc := &Service{
		engine:            eng,
		deps:              deps,
		provider:          provider,
		modelID:           modelID,
		defaultProfile:    deps.PolicyDefaultProfile,
		approvalSettle:    deps.ApprovalSettleTimeout,
		active:            make(map[domain.RunID]context.CancelFunc),
		bundleViews:       make(map[domain.RunID]ToolkitViewHandle),
		bundleChecks:      make(map[domain.RunID]bundlecontract.CaptureCheck),
		operationFlights:  make(map[string]*toolOperationFlight),
		goalStarting:      make(map[domain.SessionID]struct{}),
		goalRuns:          make(map[domain.SessionID]domain.RunID),
		goalDisarmed:      make(map[domain.SessionID]struct{}),
		planTransitions:   make(map[domain.SessionID]uint64),
		planCancelling:    make(map[domain.SessionID]uint64),
		goalRunSessions:   make(map[domain.RunID]domain.SessionID),
		goalRunRefs:       make(map[domain.RunID]domain.GoalRef),
		admissionLocks:    make(map[domain.SessionID]*sync.Mutex),
		humanIntentLocks:  make(map[domain.SessionID]*sync.Mutex),
		goalWakeRunning:   make(map[domain.SessionID]struct{}),
		goalWakePending:   make(map[domain.SessionID]struct{}),
		runSessions:       make(map[domain.RunID]domain.SessionID),
		deletedSessions:   make(map[domain.SessionID]struct{}),
		closingSessions:   make(map[domain.SessionID]struct{}),
		pending:           make(map[domain.RunID]pendingRun),
		shellPending:      make(map[domain.RunID]shellPendingRun),
		shellStates:       make(map[string]shellState),
		ledgers:           make(map[domain.RunID]*BudgetLedger),
		snapshots:         make(map[domain.RunID]domain.PolicySnapshot),
		queues:            make(map[domain.SessionID]*sessionQueue),
		toolActivations:   make(map[domain.SessionID]*tools.ToolActivation),
		steerCancels:      make(map[domain.RunID]adk.AgentCancelFunc),
		steerArmed:        make(map[domain.RunID]bool),
		runTools:          make(map[domain.RunID]map[string]struct{}),
		contextViews:      make(map[domain.RunID]string),
		lastCompaction:    make(map[domain.SessionID]*LastCompaction),
		overflowRecovered: make(map[domain.RunID]bool),
		overflowAwaiting:  make(map[domain.RunID]int),
		humanPending:      make(map[domain.SessionID]int),
		workFenced:        make(map[domain.RunID]struct{}),
		workGates:         make(map[domain.RunID]*sync.Mutex),
		workBlockedCalls:  make(map[domain.RunID]map[string]struct{}),
	}
	if eng != nil {
		// VCP-D2: bind the compact-and-retry decider into the agent's
		// ModelRetryConfig slot; nil-safe, one recovery per run.
		eng.SetRetryDecider(svc.overflowRetryDecision)
	}
	return svc
}

// sessionAdmission returns the process-local startup gate for one session.
// It serializes human and automatic primary admissions without blocking
// unrelated sessions.
func (s *Service) sessionAdmission(sessionID domain.SessionID) *sync.Mutex {
	s.admissionLocksMu.Lock()
	defer s.admissionLocksMu.Unlock()
	if s.admissionLocks == nil {
		s.admissionLocks = make(map[domain.SessionID]*sync.Mutex)
	}
	gate := s.admissionLocks[sessionID]
	if gate == nil {
		gate = &sync.Mutex{}
		s.admissionLocks[sessionID] = gate
	}
	return gate
}

// humanIntentGate linearizes human intent registration against the final
// Goal-round commit. It is separate from the startup gate so a human request
// can register while an uncommitted Goal candidate owns session admission.
func (s *Service) humanIntentGate(sessionID domain.SessionID) *sync.Mutex {
	s.admissionLocksMu.Lock()
	defer s.admissionLocksMu.Unlock()
	gate := s.humanIntentLocks[sessionID]
	if gate == nil {
		gate = &sync.Mutex{}
		s.humanIntentLocks[sessionID] = gate
	}
	return gate
}

// withAdmissionIntent linearizes the durable startup commit against human
// intent registration. Human callers may commit despite their own registered
// intent; automatic callers yield without writing when a human is pending.
func (s *Service) withAdmissionIntent(sessionID domain.SessionID, human bool, commit func() error) error {
	gate := s.humanIntentGate(sessionID)
	gate.Lock()
	defer gate.Unlock()
	if !human {
		s.mu.Lock()
		pending := s.humanPending[sessionID] > 0
		s.mu.Unlock()
		if pending {
			return storage.ErrWorkRunConflict
		}
	}
	return commit()
}

// SetCatalog wires the provider catalog so the service can query model
// capacity metadata. This is optional; without it, GetModelInfo returns
// conservative defaults.
func (s *Service) SetCatalog(catalog *provider.Catalog) {
	s.catalog = catalog
}

// SetToolkitRuntime attaches the process-owned coordinator adapter after the
// Service exists, allowing its publication gate to point back to this Service.
// App calls this during startup before it exposes any admission surface.
func (s *Service) SetToolkitRuntime(toolkits ToolkitRuntime) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.deps.Toolkits = toolkits
	s.mu.Unlock()
}

// SetModel updates the provider/model labels used on run.started. A
// settings save calls this so the next turn is labeled without a restart.
func (s *Service) SetModel(providerName, modelID string) {
	s.mu.Lock()
	s.provider = providerName
	s.modelID = modelID
	s.mu.Unlock()
}

// SetApprovalSettleTimeout replaces the live smart-mode review window. A
// settings save calls this so the next approval honors the new deadline
// without a restart; a value <= 0 disables timed auto-approval.
func (s *Service) SetApprovalSettleTimeout(d time.Duration) {
	if s == nil {
		return
	}
	if d < 0 {
		d = 0
	}
	s.mu.Lock()
	s.approvalSettle = d
	s.mu.Unlock()
}

// ApprovalSettleWindow reports the live smart-mode review window.
func (s *Service) ApprovalSettleWindow() time.Duration {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.approvalSettle
}

// approvalDeadline is the single settlement deadline for a pending approval
// raised now: the configured smart-mode review window when it is enabled,
// clamped by the caller's hard bound. The interaction sweeper reads the same
// deadline, so human review, timed auto-approval, and expiry share one
// authority instead of racing.
func (s *Service) approvalDeadline(now time.Time, hard time.Duration) time.Time {
	window := s.ApprovalSettleWindow()
	switch {
	case hard <= 0:
		// An unset hard bound must not collapse the review window to zero,
		// which would expire the approval the moment it was raised: the
		// configured window then becomes the only bound.
		if window > 0 {
			return now.Add(window)
		}
		return now.Add(hard)
	case window > 0 && window < hard:
		return now.Add(window)
	default:
		return now.Add(hard)
	}
}

// autoApprovesOnTimeout reports whether a due approval may be approved on
// the user's behalf instead of expiring. It is deliberately the smart preset
// only (workspace-write sandbox + ask policy): cautious review stays strict,
// and the trusted preset already auto-approves its allowlist. The decision
// reads the row as recorded when the human was asked, never the current
// settings, so a preset change cannot retroactively authorize a call.
func autoApprovesOnTimeout(approval domain.Approval) bool {
	if approval.Kind == domain.ApprovalKindSkillRevision {
		return false
	}
	return domain.SandboxMode(approval.SandboxMode) == domain.SandboxModeWorkspaceWrite &&
		domain.ApprovalPolicy(approval.ApprovalPolicy) == domain.ApprovalPolicyAsk
}

// approvalSettleReason is the durable actor+reason pair for a timed
// auto-approval (D-010: numbers and preset names only, no user content).
func approvalSettleReason(window time.Duration) string {
	return fmt.Sprintf("auto-approved after %s with no response (smart mode)", window)
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
	// Serialize the tombstone with the final Plan entry check. A deletion
	// beginning during an unlocked Goal drain wins before Plan can commit.
	s.projectionMu.Lock()
	s.mu.Lock()
	s.deletedSessions[id] = struct{}{}
	s.closingSessions[id] = struct{}{}
	s.planTransitions[id]++
	liveRunIDs := make([]domain.RunID, 0)
	for runID, sessionID := range s.runSessions {
		if sessionID == id {
			liveRunIDs = append(liveRunIDs, runID)
		}
	}
	childRuns := s.deps.ChildRuns
	s.mu.Unlock()
	// Child admission and activation share projectionMu with this enumeration.
	// Once the root tombstone is installed, no new direct child can be admitted;
	// marking each discovered ChildSession before releasing the lock also fences
	// continuation started through a descendant Session.
	s.projectionMu.Unlock()
	if childRuns != nil {
		childRuns.CancelSessionChildren(id)
	}
	for _, runID := range liveRunIDs {
		s.Cancel(runID)
	}

	s.projectionMu.Lock()
	sessionIDs := []domain.SessionID{id}
	if childStore, ok := s.deps.Sessions.(storage.ChildSessionStore); ok {
		seen := map[domain.SessionID]struct{}{id: {}}
		for index := 0; index < len(sessionIDs); index++ {
			bindings, err := childStore.ListChildSessions(ctx, sessionIDs[index])
			if err != nil {
				s.projectionMu.Unlock()
				return err
			}
			for _, binding := range bindings {
				if _, found := seen[binding.ChildSessionID]; found {
					continue
				}
				seen[binding.ChildSessionID] = struct{}{}
				sessionIDs = append(sessionIDs, binding.ChildSessionID)
				s.mu.Lock()
				s.deletedSessions[binding.ChildSessionID] = struct{}{}
				s.closingSessions[binding.ChildSessionID] = struct{}{}
				s.mu.Unlock()
			}
		}
	}
	allRuns := make([]domain.Run, 0)
	for _, sessionID := range sessionIDs {
		runs, err := s.deps.Runs.ListRunsBySession(ctx, sessionID)
		if err != nil {
			s.projectionMu.Unlock()
			return err
		}
		allRuns = append(allRuns, runs...)
	}
	s.projectionMu.Unlock()

	seenRuns := make(map[domain.RunID]struct{}, len(allRuns))
	for _, run := range allRuns {
		if _, found := seenRuns[run.ID]; found {
			continue
		}
		seenRuns[run.ID] = struct{}{}
		s.Cancel(run.ID)
	}
	if err := s.finalizeCognitiveSessions(ctx, sessionIDs, allRuns); err != nil {
		return err
	}
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	if err := s.deps.Sessions.DeleteSession(ctx, id); err != nil {
		// Runs were already cancelled under the tombstone. Keep the session
		// sealed so a partial backend failure cannot revive producers after
		// their in-memory authority has been removed; deletion may be retried.
		return err
	}
	s.mu.Lock()
	for _, sessionID := range sessionIDs {
		delete(s.closingSessions, sessionID)
	}
	s.mu.Unlock()
	if s.deps.Deliverables != nil {
		s.deps.Deliverables.CloseSessionTransfers(id)
	}
	s.dropSessionToolActivation(id)
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

// Only the terminal event may finish while session producer admission is
// sealed. Ordinary projections remain blocked by deletedSessions.
func (s *Service) runSessionClosing(runID domain.RunID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, closing := s.closingSessions[s.runSessions[runID]]
	return closing
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
	return s.runWithAdmissionGate(ctx, sessionID, userText, options, nil, false)
}

type runPersistence func(storage.RunAdmission) (domain.RunEvent, error)

func (s *Service) runWithOptions(ctx context.Context, sessionID domain.SessionID, userText string, options RunOptions, persist runPersistence) (domain.RunID, error) {
	return s.runWithAdmissionGate(ctx, sessionID, userText, options, persist, false)
}

func (s *Service) runWithAdmissionGate(ctx context.Context, sessionID domain.SessionID, userText string, options RunOptions, persist runPersistence, startupGateHeld bool) (domain.RunID, error) {
	if s.engine == nil || s.deps.Journal == nil || s.deps.Runs == nil || s.deps.Messages == nil || s.deps.Sink == nil {
		return "", errors.New("runtime: service not wired")
	}
	if startupGateHeld && options.GoalRound == nil {
		return "", errors.New("runtime: only a Goal candidate may reuse a held startup gate")
	}
	releaseBundleAdmission, err := s.reserveBundleAdmission()
	if err != nil {
		return "", err
	}
	defer releaseBundleAdmission()
	if options.HumanAdmission && options.GoalRound == nil {
		intentGate := s.humanIntentGate(sessionID)
		intentGate.Lock()
		s.mu.Lock()
		s.humanPending[sessionID]++
		s.mu.Unlock()
		intentGate.Unlock()

		defer func() {
			intentGate.Lock()
			defer intentGate.Unlock()
			s.mu.Lock()
			if pending := s.humanPending[sessionID]; pending <= 1 {
				delete(s.humanPending, sessionID)
			} else {
				s.humanPending[sessionID] = pending - 1
			}
			s.mu.Unlock()
		}()
	}
	if !startupGateHeld {
		sessionAdmission := s.sessionAdmission(sessionID)
		sessionAdmission.Lock()
		defer sessionAdmission.Unlock()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	queue, queueErr := s.queueFor(ctx, sessionID)
	if queueErr != nil {
		return "", queueErr
	}
	queue.mu.Lock()
	if len(options.queueItems) > 0 && !queuePrefixMatches(append(append([]domain.QueuedTurn(nil), queue.steer...), queue.followUp...), options.queueItems) {
		queue.mu.Unlock()
		return "", errQueueChanged
	}
	queueLocked := true
	defer func() {
		if queueLocked {
			queue.mu.Unlock()
		}
	}()
	// Session deletion shares this lock with startup. If deletion marks the
	// tombstone while an earlier startup owns the lock, it will subsequently
	// remove that run; if deletion wins, startup fails without writing.
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	if s.sessionDeleted(sessionID) {
		return "", storage.ErrNotFound
	}
	if options.GoalRound == nil {
		runs, err := s.deps.Runs.ListRunsBySession(ctx, sessionID)
		if err != nil {
			return "", fmt.Errorf("runtime: inspect active session runs: %w", err)
		}
		for _, existing := range runs {
			if (existing.Kind == "" || existing.Kind == domain.RunKindPrimary) && !existing.Status.Terminal() {
				return "", storage.ErrWorkRunConflict
			}
		}
	}
	// A deferred settings-save engine rebuild applies here, while no run
	// is registered.
	if err := s.applyPendingEngineReload(ctx, nil); err != nil {
		return "", err
	}
	if options.Profile == "" {
		options.Profile = s.defaultProfile
	}
	mode, profile, collaborationMode, collaborationVersion, err := normalizeRunOptions(
		options.Mode, options.Profile, options.CollaborationMode, options.CollaborationVersion)
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
	fileContexts, err := normalizeFileContextsContext(ctx, options.FileContexts)
	if err != nil {
		return "", err
	}
	// Provenance is validated before anything is persisted so an invalid
	// world entry cannot leave a half-labeled user message behind. The
	// source must name a member of the closed ui|channel|headless
	// vocabulary (CH-C1-N4); the platform name travels in Channel.
	provenance := domain.Provenance{Source: domain.SourceUI}
	if options.Provenance != nil {
		if !domain.ValidMessageSource(options.Provenance.Source) {
			return "", fmt.Errorf("runtime: run provenance source %q is outside the ui|channel|headless vocabulary", options.Provenance.Source)
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

	if options.Continuity != nil && persist != nil {
		return "", errors.New("runtime: continuity admission does not compose with a persistence override")
	}
	var continuityScope *domain.AcceptedHistoryScope
	var continuityExpectations []storage.SourceExpectation
	var continuityHash string
	admissionAlloc, _ := s.deps.Workspaces.(AdmissionWorkspaceAllocator)
	if options.Continuity != nil {
		if s.deps.Continuity == nil {
			return "", ErrContinuityUnavailable
		}
		if s.deps.Workspaces != nil && admissionAlloc == nil {
			return "", ErrContinuityUnavailable
		}
		input := options.Continuity
		if err := input.Validate(domain.DefaultContinuityLimits()); err != nil {
			return "", err
		}
		resolved, err := ResolveHistoryScope(sessionID, input.HistoryScope.SessionIDs, admissionAllowedSessions(sessionID, input.HistoryScope.SessionIDs))
		if err != nil {
			return "", err
		}
		scope, err := domain.NewAcceptedHistoryScope(sessionID, resolved)
		if err != nil {
			return "", err
		}
		continuityScope = &scope
		if continuityExpectations, err = s.continuityExpectations(ctx, input); err != nil {
			return "", err
		}
		continuityHash = continuityInputHash(userText, input)
		// Cheap receipt pre-check: a lost-response retry returns the
		// original run without allocating anything. The in-transaction
		// recheck still guards the racing first commit.
		receipt, found, err := s.deps.Continuity.FindContinuityReceipt(ctx, sessionID, storage.ContinuityOperationAdmission, input.RequestID)
		if err != nil {
			return "", fmt.Errorf("runtime: read continuity receipt: %w", err)
		}
		if found {
			if receipt.InputHash != continuityHash {
				return "", storage.ErrConflict
			}
			return receipt.RunID, nil
		}
	}
	runID := newRunID()
	if options.GoalRound != nil && options.GoalRound.RunID != "" {
		runID = options.GoalRound.RunID
	}
	var bundleHandle ToolkitViewHandle
	var bundleSnapshot *storage.RunBundleSnapshot
	var expectedBundle *bundlecontract.CaptureCheck
	bundleHandleTransferred := false
	defer func() {
		if bundleHandle != nil && !bundleHandleTransferred {
			bundleHandle.Release()
		}
	}()
	if s.deps.Toolkits != nil {
		if s.deps.Bundles == nil {
			return "", ErrBundleDrainUnavailable
		}
		handle, captured, check, captureErr := s.deps.Toolkits.CaptureRun(ctx, runID)
		if captureErr != nil {
			return "", fmt.Errorf("runtime: capture toolkit catalog: %w", captureErr)
		}
		if handle == nil || captured.RunID != runID || handle.View().Identity() != check.Catalog {
			if handle != nil {
				handle.Release()
			}
			return "", errors.New("runtime: toolkit capture is inconsistent")
		}
		if _, captureErr := storage.ValidateRunBundleSnapshot(captured); captureErr != nil {
			handle.Release()
			return "", fmt.Errorf("runtime: toolkit snapshot is invalid: %w", captureErr)
		}
		bundleHandle = handle
		bundleSnapshot = &captured
		expectedBundle = &check
		if options.Continuity != nil && s.deps.Continuity == nil {
			return "", ErrContinuityUnavailable
		}
		if options.GoalRound != nil && s.deps.GoalRuns == nil {
			return "", errors.New("runtime: goal admission store is not wired")
		}
		if persist != nil && s.deps.Admission == nil {
			return "", ErrToolkitAdmissionUnavailable
		}
		if persist == nil && options.Continuity == nil && options.GoalRound == nil && s.deps.PrimaryRuns == nil && s.deps.Admission == nil {
			return "", ErrToolkitAdmissionUnavailable
		}
	}
	if options.BeforeStart != nil {
		if err := options.BeforeStart(runID); err != nil {
			return "", fmt.Errorf("runtime: prepare run %s: %w", runID, err)
		}
	}
	var capture maskcontract.Capture
	var prompt *storage.RunPromptSnapshot
	var expectedMask *storage.MaskCaptureCheck
	if s.deps.Admission != nil {
		capture = maskcontract.Capture{Selection: maskcontract.Selection{SessionID: sessionID}}
		if s.deps.MaskResolver != nil {
			if bundleHandle != nil {
				if resolver, ok := s.deps.MaskResolver.(maskcontract.ViewResolver); ok {
					capture, err = resolver.CaptureFromView(ctx, sessionID, bundleHandle.View())
				} else {
					capture, err = s.deps.MaskResolver.Capture(ctx, sessionID)
				}
			} else {
				capture, err = s.deps.MaskResolver.Capture(ctx, sessionID)
			}
			if err != nil {
				return "", fmt.Errorf("runtime: capture session mask: %w", err)
			}
			if capture.Selection.SessionID != sessionID {
				return "", maskcontract.NewError(maskcontract.CodeSnapshotCorrupt, errors.New("mask capture belongs to another session"))
			}
			expectedMask = &storage.MaskCaptureCheck{
				SessionID: sessionID, SelectionRevision: capture.Selection.Revision,
				MaskID: capture.Selection.MaskID,
			}
			if capture.Mask != nil && !maskcontract.IsBuiltinID(capture.Mask.ID) {
				expectedMask.DefinitionRevision = capture.Mask.DefinitionRevision
				expectedMask.DefinitionDigest = capture.Mask.Digest
				expectedMask.Origin = capture.Mask.Origin
				expectedMask.Bundle = cloneMaskBundleProvenance(capture.Mask.Bundle)
				expectedMask.RecommendedSkills = cloneMaskRecommendations(capture.Mask.RecommendedSkills)
			}
		}
		var frozenText, frozenDigest string
		var persona storage.PersonaSnapshot
		if b := s.deps.Cognitive; face != domain.FaceCode && b != nil && b.Primary != nil {
			// The coding face keeps its project instruction contract and
			// does not require companion-persona onboarding.
			// Session authority rides in the primary run's input before
			// optional evidence/history consumes budget. A missing or
			// corrupt FrozenCore gates inference explicitly.
			prepared, prepareErr := b.Primary.Prepare(ctx, cognitivecontract.PrimaryContextInput{
				SessionID:   sessionID,
				RunID:       runID,
				WorkspaceID: b.Binding.WorkspaceID,
				BudgetBytes: s.engine.cfg.MaxContextBytes,
			})
			if errors.Is(prepareErr, cognitivecontract.ErrPersonaUninitialized) {
				// First-arrival guidance is an explicit state, not a fallback
				// identity. Only initialized authority creates a FrozenCore.
				body := promptAsset("persona-uninitialized.md")
				if body == "" {
					return "", errors.New("runtime: uninitialized persona asset is empty")
				}
				persona = storage.PersonaSnapshot{
					Source: "runtime/persona-uninitialized", Revision: "1",
					Body: body,
				}
			} else if prepareErr != nil {
				return "", fmt.Errorf("runtime: prepare session authority: %w", prepareErr)
			} else {
				if strings.TrimSpace(prepared.Text) == "" {
					return "", errors.New("runtime: initialized session authority projection is empty")
				}
				frozenText, frozenDigest = prepared.Text, prepared.Digest
			}
		}
		var rules *storage.RunRulesSnapshot
		if bundleHandle != nil {
			rules, err = PrepareBundleRules(bundleHandle.View(), face)
			if err != nil {
				return "", fmt.Errorf("runtime: prepare Toolkit Rules: %w", err)
			}
		}
		built, buildErr := buildPromptSnapshot(PromptInput{
			RunID: runID, GenerationID: s.deps.GenerationID, Capture: capture,
			Persona: persona,
			Face:    face, Frame: s.deps.MaskFrame, FrameDigest: s.deps.MaskFrameDigest,
			FrozenText: frozenText, FrozenDigest: frozenDigest,
			Rules: rules, MaxContextBytes: s.engine.cfg.MaxContextBytes,
		})
		if buildErr != nil {
			return "", fmt.Errorf("runtime: build prompt snapshot: %w", buildErr)
		}
		prompt = &built
	}
	workspaceID := ""
	admissionNewPrivate := false
	var workspace Workspace
	workspaceReady := false
	releaseWorkspace := func() {
		if !workspaceReady || s.deps.Workspaces == nil {
			return
		}
		if releaser, ok := s.deps.Workspaces.(WorkspaceReleaser); ok {
			if err := releaser.Release(context.WithoutCancel(ctx), workspace); err != nil {
				slog.Warn("run admission rollback could not release workspace", "run", string(runID), "err", err)
			}
		}
		workspaceReady = false
	}
	if s.deps.Workspaces != nil {
		if options.Continuity != nil {
			workspace, newly, err := admissionAlloc.EnsureForAdmission(withSessionID(ctx, sessionID), runID)
			if err != nil {
				return "", fmt.Errorf("runtime: allocate isolated workspace: %w", err)
			}
			workspaceID = workspace.ID
			admissionNewPrivate = newly
		} else {
			var err error
			workspace, err = s.deps.Workspaces.Ensure(withSessionID(ctx, sessionID), runID)
			if err != nil {
				return "", fmt.Errorf("runtime: allocate isolated workspace: %w", err)
			}
			workspaceReady = true
			workspaceID = workspace.ID
		}
	}
	now := time.Now().UnixMilli()
	if now <= queue.latestCreatedAt {
		now = queue.latestCreatedAt + 1
	}

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
	m.setRunScope(s.deps.TenantID, workspaceID, string(sessionID))
	runProvider, runModel := s.CurrentModel()
	m.setUsageRoutes(runProvider, runModel, s.engine.cfg.SummaryModelID)
	var admittedScope *domain.AcceptedHistoryScope
	if continuityScope != nil {
		admittedScope = continuityScope
	}
	collaborationPayloadMode, collaborationPayloadVersion := collaborationPayload(collaborationMode, collaborationVersion)
	promptSchema, promptDigest := 0, ""
	if prompt != nil {
		promptSchema, promptDigest = prompt.SchemaVersion, prompt.PayloadSHA256
	}
	started := m.build(domain.EventRunStarted, payloadRunStarted{
		Provider: runProvider, Model: runModel, Mode: string(mode), Face: string(face),
		CollaborationMode: collaborationPayloadMode, CollaborationVersion: collaborationPayloadVersion,
		PolicyProfile: string(profile), PolicyHash: snapshot.Hash,
		SandboxMode: string(sandboxMode), ApprovalPolicy: string(approvalPolicy),
		HistoryScope: admittedScope,
		PromptSchema: promptSchema, PromptDigest: promptDigest,
		BundleSchema: func() int {
			if bundleSnapshot != nil {
				return bundleSnapshot.SchemaVersion
			}
			return 0
		}(),
		BundleDigest: func() string {
			if bundleSnapshot != nil {
				return bundleSnapshot.PayloadSHA256
			}
			return ""
		}(),
		BundleEpoch: func() int64 {
			if expectedBundle != nil {
				return expectedBundle.Catalog.Epoch
			}
			return 0
		}(),
		BundleCatalogDigest: func() string {
			if expectedBundle != nil {
				return expectedBundle.Catalog.EffectiveDigest
			}
			return ""
		}(),
	})
	admission := storage.RunAdmission{
		Message: message, Run: run, Started: started, Prompt: prompt, ExpectedMask: expectedMask,
		Bundle: bundleSnapshot, ExpectedBundle: expectedBundle,
	}
	queueEvents, queueErr := queueAdmissionEvents(m, queue, options.queueItems)
	if queueErr != nil {
		releaseWorkspace()
		return "", queueErr
	}
	admission.Events = queueEvents
	if len(queueEvents) > 0 && options.GoalRound != nil {
		releaseWorkspace()
		return "", errors.New("runtime: pending human queue must settle before Goal admission")
	}
	if len(queueEvents) > 0 && options.Continuity == nil && persist == nil && s.deps.PrimaryRuns == nil && s.deps.Admission == nil {
		releaseWorkspace()
		return "", errors.New("runtime: durable queue admission store is not wired")
	}
	var goalAdmissionEvent *domain.WorkEvent
	startupEvents := append([]domain.RunEvent{started}, queueEvents...)
	if options.Continuity != nil {
		// The atomic admission commits run.started alongside the run row, so
		// the committed row is already active rather than accepted.
		run.Status = domain.RunActive
		// Reference snapshots are built and bounded before commit and land
		// in the same startup set; no engine step ever touches their content.
		for _, selection := range options.Continuity.References {
			if s.deps.References == nil {
				if admissionNewPrivate {
					s.discardAdmissionWorkspace(sessionID, runID, admissionAlloc)
				}
				return "", ErrContinuityUnavailable
			}
			reference, refErr := s.deps.References.AttachForAdmission(ctx, selection, sessionID, runID)
			if refErr == nil {
				var event domain.RunEvent
				event, refErr = ReferenceEvent(reference, now)
				if refErr == nil {
					startupEvents = append(startupEvents, event)
					refErr = ledger.ReserveEvent()
				}
			}
			if refErr != nil {
				if admissionNewPrivate {
					s.discardAdmissionWorkspace(sessionID, runID, admissionAlloc)
				}
				return "", fmt.Errorf("runtime: snapshot admission reference: %w", refErr)
			}
		}
		result, commitErr := s.deps.Continuity.CommitContinuityRun(ctx, storage.ContinuityAdmission{
			SessionID:      sessionID,
			Message:        message,
			Run:            run,
			Events:         startupEvents,
			Scope:          *continuityScope,
			Expectations:   continuityExpectations,
			Prompt:         prompt,
			ExpectedMask:   expectedMask,
			Bundle:         bundleSnapshot,
			ExpectedBundle: expectedBundle,
			Receipt: domain.ContinuityReceipt{
				SessionID: sessionID,
				Operation: storage.ContinuityOperationAdmission,
				RequestID: options.Continuity.RequestID,
				InputHash: continuityHash,
			},
		})
		if commitErr != nil {
			// A definite pre-commit failure rolls the just-created empty
			// private directory back; an uncertain commit preserves it
			// because the rows may already be durable.
			if admissionNewPrivate && !errors.Is(commitErr, storage.ErrCommitUncertain) {
				s.discardAdmissionWorkspace(sessionID, runID, admissionAlloc)
			}
			return "", fmt.Errorf("runtime: commit continuity admission: %w", commitErr)
		}
		if !result.NewlyCommitted {
			// The receipt recheck inside the transaction raced a first
			// commit and lost: the original run stands; the never-committed
			// private directory is reaped before returning its identity.
			if admissionNewPrivate {
				s.discardAdmissionWorkspace(sessionID, runID, admissionAlloc)
			}
			return result.RunID, nil
		}
		startupEvents = result.Events
		started = result.Events[0]
	} else if persist != nil {
		if options.GoalRound != nil {
			return "", errors.New("runtime: goal admission cannot use custom persistence")
		}
		err = s.withAdmissionIntent(sessionID, options.HumanAdmission, func() error {
			started, err = persist(admission)
			return err
		})
		if err != nil {
			releaseWorkspace()
			return "", err
		}
	} else if options.GoalRound != nil {
		if s.deps.GoalRuns == nil {
			releaseWorkspace()
			return "", errors.New("runtime: goal admission store is not wired")
		}
		goal := options.GoalRound
		run.Status = domain.RunActive
		mutation := domain.WorkMutation{
			SessionID:       sessionID,
			ExpectedVersion: goal.ExpectedVersion,
			RequestID:       goal.RequestID,
			RequestHash:     goal.RequestHash,
			Kind:            domain.WorkEventGoalRoundAdmitted,
			Admission: domain.GoalRunAdmission{
				SessionID: sessionID,
				Goal:      goal.Goal,
				Round:     goal.Round,
				RunID:     runID,
			},
		}
		var admitted storage.GoalRunCommitResult
		err = s.withAdmissionIntent(sessionID, false, func() error {
			s.mu.Lock()
			defer s.mu.Unlock()
			_, disarmed := s.goalDisarmed[sessionID]
			if s.stopping || disarmed {
				return nil
			}
			var commitErr error
			admitted, commitErr = s.deps.GoalRuns.CommitGoalRun(ctx, storage.GoalRunCommit{
				Mutation:       mutation,
				Message:        message,
				Run:            run,
				Started:        started,
				Prompt:         prompt,
				ExpectedMask:   expectedMask,
				Bundle:         bundleSnapshot,
				ExpectedBundle: expectedBundle,
			})
			return commitErr
		})
		if err != nil {
			releaseWorkspace()
			if errors.Is(err, storage.ErrWorkRunConflict) {
				return "", err
			}
			return "", fmt.Errorf("runtime: admit goal round: %w", err)
		}
		if admitted.Run.ID == "" {
			releaseWorkspace()
			return "", nil
		}
		if admitted.Work.Replayed {
			return admitted.Run.ID, nil
		}
		run = admitted.Run
		runID = run.ID
		started = admitted.Started
		goalAdmissionEvent = &admitted.Work.Event
	} else if s.deps.PrimaryRuns != nil {
		run.Status = domain.RunActive
		run.Kind = domain.RunKindPrimary
		run.RootID = runID
		err = s.withAdmissionIntent(sessionID, options.HumanAdmission, func() error {
			var commitErr error
			started, commitErr = s.deps.PrimaryRuns.CommitPrimaryRun(ctx, storage.PrimaryRunCommit{
				Message:        message,
				Run:    …23501 tokens truncated…oRetryFinished, payloadAutoRetryFinished{Attempt: attempt, Success: success}))
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
func (m *eventMapper) reserveMappedBudget(ledger *BudgetLedger, events []domain.RunEvent) error {
	if ledger == nil {
		return nil
	}
	modelCall := false
	accountedCall := false
	for _, re := range events {
		switch re.Type {
		case domain.EventModelDelta, domain.EventModelReasoningDelta, domain.EventModelCallFinished:
			// Deltas are live-payload traffic and model.call.finished is
			// mandatory closure evidence; neither consumes MaxEvents.
			continue
		case domain.EventModelUsage:
			// The single settlement sample is exempt; ordinary
			// cumulative samples consume the quota.
			if eventHasSettlementFlag(re) {
				continue
			}
		case domain.EventProviderRetry:
			if err := ledger.ReserveRetry(); err != nil {
				return err
			}
		case domain.EventToolRequested:
			if !accountedCall {
				accountedCall = true
				if !m.consumeObservedCallCharge() {
					modelCall = true
				}
			}
			if err := ledger.ReserveToolCall(); err != nil {
				return err
			}
		case domain.EventModelCompleted:
			if !accountedCall {
				accountedCall = true
				if !m.consumeObservedCallCharge() {
					modelCall = true
				}
			}
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

// eventHasSettlementFlag probes a v2 usage payload for the settlement
// marker without decoding the whole document.
func eventHasSettlementFlag(re domain.RunEvent) bool {
	if re.PayloadVersion < 2 {
		return false
	}
	var probe struct {
		Settlement bool `json:"settlement"`
	}
	return json.Unmarshal(re.Payload, &probe) == nil && probe.Settlement
}

// handleInterrupt routes one exact Eino checkpoint target to its durable
// human-interaction lifecycle. Suspended runs remain active without a
// terminal event until a response resumes them or Cancel closes them.
func (s *Service) handleInterrupt(ctx context.Context, m *eventMapper, sessionID domain.SessionID, selectedTools []string, mode domain.RunMode) {
	if m.interrupt != nil && m.interrupt.ToolName == tools.AskUserName {
		s.handleQuestionInterrupt(ctx, m, sessionID, selectedTools, mode)
		return
	}
	if m.interrupt != nil && m.interrupt.PlanSubmissionID != "" {
		s.handlePlanReviewInterrupt(ctx, m, sessionID, selectedTools, mode)
		return
	}
	runID := m.runID
	fail := func(err error) {
		slog.Warn("interrupt handling failed", "run", string(runID), "err", err)
		if isChildTerminal(ctx) {
			s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, err))
			return
		}
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

	expiresAt := s.approvalDeadline(time.Now(), s.deps.ApprovalExpiration).UnixMilli()
	proposalData, proposalErr := json.Marshal(details.Args)
	if proposalErr != nil {
		fail(proposalErr)
		return
	}
	argumentsHash, hashErr := toolApprovalArgumentsHash(details.ToolName, proposalData)
	if hashErr != nil {
		fail(hashErr)
		return
	}
	if details.ArgumentsHash != "" && details.ArgumentsHash != argumentsHash {
		fail(errors.New("tool approval interrupt arguments hash mismatch"))
		return
	}
	proposal, proposalErr := s.engine.PrepareProposal(ctx, details.ToolName, proposalData)
	if proposalErr != nil {
		fail(proposalErr)
		return
	}
	proposal = boundToolProposalReview(proposal, m.maxPayload)
	boundProposalData, bindErr := bindToolApprovalProposal(proposal.Data, argumentsHash)
	if bindErr != nil {
		fail(bindErr)
		return
	}
	approvalKind := domain.ApprovalKindRun
	if s.deps.Runs != nil {
		run, runErr := s.deps.Runs.GetRun(persistCtx, runID)
		if runErr != nil {
			fail(fmt.Errorf("load run for approval ownership: %w", runErr))
			return
		}
		if run.Kind == domain.RunKindChild {
			approvalKind = domain.ApprovalKindChild
		}
	}
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
		ProposalData:     boundProposalData,
		Kind:             approvalKind,
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
	if isChildTerminal(ctx) {
		lifecycle := m.build(domain.EventChildSuspended, map[string]string{
			"approval_id": approval.ID, "tool_call_id": approval.ToolCallID,
		})
		if _, err := s.appendRunEvent(persistCtx, lifecycle, false); err != nil {
			fail(err)
			return
		}
	}

	if ctx.Err() != nil {
		// Cancelled while suspending: close the durable approval before the
		// terminal so a headless or concurrent cancel cannot leave a stale
		// review item after the run is already closed.
		if err := s.cancelApproval(persistCtx, approval, "run cancelled while suspending"); err != nil {
			slog.Warn("cancel approval while suspending failed", "approval", approval.ID, "err", err)
		}
		s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, context.Canceled))
		return
	}

	ledger := s.ledgerForRun(runID)
	s.mu.Lock()
	s.pending[runID] = pendingRun{
		runCtx: ctx, sessionID: sessionID, workspaceID: contextWorkspaceID(ctx), mapper: m, selectedTools: append([]string(nil), selectedTools...),
		mounted: tools.MountedToolsFromContext(ctx),
		mode:    mode, profile: policyProfile(ctx), snapshot: policySnapshot(ctx),
		sandboxMode: sandboxMode(ctx), approvalPolicy: approvalPolicy(ctx), face: runFace(ctx), ledger: ledger,
		approvalID: approval.ID,
		engine:     engineFromContext(ctx), execution: executionFromContext(ctx),
	}
	s.mu.Unlock()
}

// handleQuestionInterrupt persists the ask_user suspension and publishes a
// question lifecycle event. It deliberately does not touch ApprovalStore.
func (s *Service) handleQuestionInterrupt(ctx context.Context, m *eventMapper, sessionID domain.SessionID, selectedTools []string, mode domain.RunMode) {
	runID := m.runID
	fail := func(err error) {
		slog.Warn("question handling failed", "run", string(runID), "err", err)
		if isChildTerminal(ctx) {
			s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, err))
			return
		}
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
		runCtx: ctx, sessionID: sessionID, workspaceID: contextWorkspaceID(ctx), mapper: m,
		selectedTools: append([]string(nil), selectedTools...),
		mounted:       tools.MountedToolsFromContext(ctx),
		mode:          mode, profile: policyProfile(ctx), snapshot: policySnapshot(ctx),
		sandboxMode: sandboxMode(ctx), approvalPolicy: approvalPolicy(ctx),
		face: runFace(ctx), questionID: question.ID, ledger: ledger,
		engine: engineFromContext(ctx), execution: executionFromContext(ctx),
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
// The decision is attributed to the local user.
func (s *Service) DecideApprovalWithReason(ctx context.Context, approvalID, decision, reason string) error {
	return s.decideApprovalWithReason(ctx, approvalID, decision, reason, "local_user")
}

// DecideApprovalAsActor settles a pending approval attributed to the named
// actor (e.g. "channel:telegram:12345"). It runs the exact same validation,
// journaling, and resume machinery as the local path — the actor only
// changes the attribution recorded in the journal and the approval store,
// so a channel-side decision stays auditable without becoming a second
// decision path. The authorization boundary lives with the caller: the
// ChannelHost only forwards decisions from allow-listed senders scoped to
// the originating session (contract §12).
func (s *Service) DecideApprovalAsActor(ctx context.Context, approvalID, decision, reason, actor string) error {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return errors.New("runtime: approval actor is required")
	}
	if len(actor) > 200 {
		return errors.New("runtime: approval actor is too long")
	}
	return s.decideApprovalWithReason(ctx, approvalID, decision, reason, actor)
}

func (s *Service) decideApprovalWithReason(ctx context.Context, approvalID, decision, reason, actor string) error {
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
	} else if approval.Kind != domain.ApprovalKindSkillRevision && !s.ApprovalRequiredDurable(ctx, approval.RunID, approval.ID) {
		return errors.New("runtime: approval is not durable yet")
	}
	if approval.Kind == domain.ApprovalKindSkillRevision && decision == domain.ApprovalApproved {
		if err := s.checkCognitiveSkillApproval(ctx, approval); err != nil {
			return err
		}
	}
	return s.settleApproval(ctx, approval, decision, actor, reason)
}

// waitPendingRegistration polls for the in-memory suspension a durable
// approval/question row precedes. Bounded: a suspension that never lands (a
// run that died between the row write and the registration) still fails
// closed.
func (s *Service) waitPendingRegistration(runID domain.RunID) bool {
	deadline := time.Now().Add(2 * time.Second)
	for {
		s.mu.Lock()
		_, pending := s.pending[runID]
		s.mu.Unlock()
		if pending {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// settleApproval records a decision whose guards the caller already checked
// and then routes it. The durable row is the no-replay boundary, so the
// journal entry and the resume follow it; actor distinguishes a human
// decision (local_user) from a timed auto-approval (system).
func (s *Service) settleApproval(ctx context.Context, approval domain.Approval, decision, actor, reason string) error {
	if approval.Kind == domain.ApprovalKindSkillRevision {
		decided, err := s.decideApproval(ctx, approval.ID, decision, actor, reason)
		if err != nil {
			return fmt.Errorf("runtime: decide Skill revision approval: %w", err)
		}
		if !decided {
			return ErrApprovalAlreadyDecided
		}
		return s.settleCognitiveSkillDecision(ctx, approval, decision)
	}
	s.mu.Lock()
	_, nativeChildPending := s.pending[approval.RunID]
	s.mu.Unlock()
	if approval.Kind == domain.ApprovalKindChild && !nativeChildPending {
		// The durable approval row precedes in-memory suspension registration
		// on the suspend path, so a decider observing the row early must wait
		// for the suspension to land rather than fail the resume.
		nativeChildPending = s.waitPendingRegistration(approval.RunID)
	}
	if approval.Kind == domain.ApprovalKindChild && !nativeChildPending {
		return errors.New("runtime: child approval is not active for Service resume")
	}
	if (approval.Kind != domain.ApprovalKindChild || nativeChildPending) && !isShellApproval(approval) {
		if _, _, err := s.promptSnapshotContext(ctx, approval.RunID); err != nil {
			// Validate before first-writer settlement. A corrupt or incompatible
			// snapshot leaves the review pending, so recovery can surface the
			// same fail-closed state without losing the user's decision slot.
			return fmt.Errorf("runtime: approval resume rejected by prompt snapshot: %w", err)
		}
	}
	decided, err := s.decideApproval(ctx, approval.ID, decision, actor, reason)
	if err != nil {
		return fmt.Errorf("runtime: decide approval: %w", err)
	}
	if !decided {
		// Lost the first-writer-wins race to a concurrent decision.
		return ErrApprovalAlreadyDecided
	}
	decisionPersisted := s.journalReviewEvent(ctx, approval.RunID, domain.EventToolApprovalDecided, payloadApprovalDecided{
		ApprovalID: approval.ID, Decision: decision, Actor: actor, Reason: reason, DecidedAt: time.Now().UnixMilli(),
	})
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
		slog.Warn("approval decided without a pending run", "approval", approval.ID, "run", string(approval.RunID))
		return nil
	}
	if isNativeOrchestrationResumeTarget(approval.ResumeTarget) {
		if !decisionPersisted {
			s.emitTerminal(context.WithoutCancel(ctx), p.mapper, p.mapper.build(domain.EventRunFailed, payloadRunFailed{
				CauseCategory: causeInternalError,
				Message:       "The workflow approval decision could not be recorded; the node did not run.",
			}))
			return errors.New("runtime: persist workflow approval decision failed")
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.resumeNativeOrchestrationApproval(p, approval, decision)
		}()
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
		s.resumeRun(p.runCtx, p.sessionID, p.workspaceID, toolName, p.selectedTools, p.mounted, p.mode, p.profile, p.snapshot, p.sandboxMode, p.approvalPolicy, p.face, p.ledger, p.engine, p.execution,
			approval.RunID, approval.ToolCallID, approval.ResumeTarget, decision, approval.ProposalData, approval.PreconditionHash, approval.ID, nil)
	}()
	return nil
}

func (s *Service) decideApproval(ctx context.Context, id, decision, actor, reason string) (bool, error) {
	if lifecycle, ok := s.deps.Approvals.(storage.ApprovalLifecycleStore); ok {
		return lifecycle.DecideApprovalWithMetadata(ctx, id, decision, actor, reason)
	}
	return s.deps.Approvals.DecideApproval(ctx, id, decision)
}

// settleApprovalAsSystem approves a due approval on the user's behalf after
// the smart-mode review window elapsed. It is the timed counterpart of
// DecideApprovalWithReason: the row must still be pending and past its
// deadline, and first-writer-wins still arbitrates against a human decision
// that lands at the same moment.
func (s *Service) settleApprovalAsSystem(ctx context.Context, approval domain.Approval) error {
	current, err := s.deps.Approvals.GetApproval(ctx, approval.ID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("runtime: get approval for timed settlement: %w", err)
	}
	if current.Decision != domain.ApprovalPending || current.ExpiresAt > time.Now().UnixMilli() {
		return nil
	}
	if !autoApprovesOnTimeout(current) {
		return nil
	}
	window := s.ApprovalSettleWindow()
	if window <= 0 {
		return nil
	}
	if isShellApproval(current) {
		if err := s.waitShellApprovalReady(ctx, current.RunID); err != nil {
			return err
		}
	} else if !s.ApprovalRequiredDurable(ctx, current.RunID, current.ID) {
		// The approval is not durable yet: leave it for the next sweep.
		return nil
	}
	reason := approvalSettleReason(window)
	if err := s.settleApproval(ctx, current, domain.ApprovalApproved, "system", reason); err != nil {
		if errors.Is(err, ErrApprovalAlreadyDecided) {
			return nil
		}
		return err
	}
	slog.Info("approval auto-approved on timeout", "approval", current.ID, "run", string(current.RunID), "window", window.String())
	return nil
}

// A parked cancellation can retry its terminal write after its interaction
// already settled. A human answer or decision still owns its resume path.
func (s *Service) cancelPendingQuestion(ctx context.Context, runID domain.RunID, questionID string) error {
	question, err := s.deps.Questions.GetQuestion(ctx, questionID)
	if err != nil {
		return err
	}
	if question.RunID != runID {
		return ErrQuestionAlreadyAnswered
	}
	if question.Status == domain.QuestionCancelled {
		return nil
	}
	if question.Status != domain.QuestionPending {
		return ErrQuestionAlreadyAnswered
	}
	if err := s.cancelQuestion(ctx, questionID, "run cancelled"); err != nil {
		if errors.Is(err, ErrQuestionAlreadyAnswered) {
			current, readErr := s.deps.Questions.GetQuestion(ctx, questionID)
			if readErr == nil && current.RunID == runID && current.Status == domain.QuestionCancelled {
				return nil
			}
		}
		return err
	}
	return nil
}

func (s *Service) cancelPendingApproval(ctx context.Context, runID domain.RunID, approval domain.Approval) error {
	if approval.RunID != runID {
		return ErrApprovalAlreadyDecided
	}
	if approval.Decision == domain.ApprovalCancelled {
		return nil
	}
	if approval.Decision != domain.ApprovalPending {
		return ErrApprovalAlreadyDecided
	}
	if err := s.cancelApproval(ctx, approval, "run cancelled"); err != nil {
		if errors.Is(err, ErrApprovalAlreadyDecided) {
			current, readErr := s.deps.Approvals.GetApproval(ctx, approval.ID)
			if readErr == nil && current.RunID == runID && current.Decision == domain.ApprovalCancelled {
				return nil
			}
		}
		return err
	}
	return nil
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
	if approval.Kind == domain.ApprovalKindSkillRevision {
		return s.setCognitiveSkillApprovalStatus(ctx, approval, domain.SkillRevisionCancelled)
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
		if approval.RunID == runID && approval.Kind != domain.ApprovalKindSkillRevision {
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
	if approval.Kind == domain.ApprovalKindSkillRevision {
		return s.setCognitiveSkillApprovalStatus(ctx, approval, domain.SkillRevisionExpired)
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
		if p.execution.child != nil {
			terminalCtx := withRunExecution(ctx, p.engine, p.execution)
			terminalCtx = withChildTerminal(terminalCtx)
			s.emitTerminal(terminalCtx, p.mapper, s.terminalEvent(terminalCtx, p.mapper, context.DeadlineExceeded))
		} else {
			s.emitTerminal(ctx, p.mapper, p.mapper.build(domain.EventRunFailed, payloadRunFailed{
				CauseCategory: causeHumanTimeout,
				Message:       "The run stopped because human review timed out.",
			}))
		}
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
	if _, _, err := s.promptSnapshotContext(ctx, question.RunID); err != nil {
		// Keep the durable question pending when the immutable prompt cannot be
		// reconstructed. Answering it would otherwise consume the only resume
		// opportunity while no model/tool call is allowed to proceed.
		return fmt.Errorf("runtime: question resume rejected by prompt snapshot: %w", err)
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
		s.resumeRun(p.runCtx, p.sessionID, p.workspaceID, toolName, p.selectedTools, p.mounted, p.mode, p.profile, p.snapshot, p.sandboxMode, p.approvalPolicy, p.face, p.ledger, p.engine, p.execution,
			question.RunID, question.ToolCallID, question.ResumeTarget, answer, nil, "", "", nil)
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

// resumeRun feeds the response back into the engine and maps the resumed
// events into the same journal (the journal continues the seq).
func (s *Service) resumeRun(parent context.Context, sessionID domain.SessionID, workspaceID, toolName string, selectedTools []string, mounted *tools.MountedTools, mode domain.RunMode, profile domain.PolicyProfile, snapshot domain.PolicySnapshot, sandboxMode domain.SandboxMode, approvalPolicy domain.ApprovalPolicy, face domain.Face, ledger *BudgetLedger, eng *Engine, execution runExecutionOptions, runID domain.RunID, toolCallID, resumeTarget, resumeValue string, proposalData []byte, preconditionHash, approvalID string, resumeBatchIDs []string) {
	s.mu.Lock()
	if s.runTools[runID] == nil {
		selected := make(map[string]struct{}, len(selectedTools))
		for _, name := range selectedTools {
			selected[name] = struct{}{}
		}
		s.runTools[runID] = selected
	}
	s.mu.Unlock()
	if eng == nil {
		eng = s.engine
	}
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
	ctx := withWorkspaceID(withSessionID(withRunID(withPolicySnapshot(withPolicyProfile(withRunMode(withFace(withSelectedTools(parent, selectedTools), face), mode), profile), snapshot), runID), sessionID), workspaceID)
	ctx = withRunExecution(ctx, eng, execution)
	if execution.child != nil {
		ctx = withChildTerminal(ctx)
	}
	if execution.suppressSessionMessageProjection {
		ctx = withSessionMessageProjectionDisabled(ctx)
	}
	ctx = withToolOperationCoordinator(ctx, s.newToolOperationCoordinator(runID, sessionID))
	ctx = withSessionSandbox(ctx, sandboxMode, approvalPolicy)
	ctx = tools.WithSessionID(ctx, sessionID)
	if approvalID != "" && isChildTerminal(ctx) {
		lifecycle := m.build(domain.EventChildResumed, map[string]string{
			"approval_id": approvalID, "decision": resumeValue,
		})
		if _, err := s.appendRunEvent(context.Background(), lifecycle, false); err != nil {
			slog.Warn("persist child resume lifecycle failed", "run", string(runID), "approval", approvalID, "err", err)
			s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, err))
			return
		}
	}
	ctx = tools.WithWorkControl(ctx, s)
	// Restore the skill mounts captured at suspend time so tools mounted
	// before the interrupt stay callable after resume (TT-2). A nil
	// registry (restart recovery) falls back to a fresh one so a
	// skill_view in the resumed segment can still mount new tools.
	mounts := mounted
	if mounts == nil {
		mounts = tools.NewMountedTools()
	}
	ctx = tools.WithMountedTools(ctx, mounts)
	promptCtx, hasPrompt, promptErr := s.promptSnapshotContext(ctx, runID)
	if promptErr != nil {
		// Validate the immutable prompt before consuming proposal bytes or
		// handing a durable approval/question to Eino. The decision has already
		// been recorded by the caller, so close the run visibly if recovery is
		// no longer possible instead of leaving an active dangling run.
		slog.Warn("resume rejected prompt snapshot", "run", string(runID), "err", promptErr)
		s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, promptErr))
		return
	}
	if hasPrompt {
		ctx = promptCtx
	}
	approvedArgumentsHash := ""
	if approvalID != "" {
		var unbindErr error
		proposalData, approvedArgumentsHash, unbindErr = unbindToolApprovalProposal(proposalData)
		if unbindErr != nil {
			slog.Warn("resume rejected invalid tool approval binding", "run", string(runID), "approval", approvalID, "err", unbindErr)
			if isChildTerminal(ctx) {
				s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, unbindErr))
				return
			}
			s.emitTerminal(ctx, m, m.build(domain.EventRunFailed, payloadRunFailed{
				CauseCategory: causeInternalError,
				Message:       "The approved tool request could not be verified and did not run.",
			}))
			return
		}
	}
	ctx = tools.WithProposalData(ctx, proposalData)
	ctx = withApprovedToolArgumentsHash(ctx, approvedArgumentsHash)
	ctx = tools.WithWorkspaceID(ctx, workspaceID)
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
	ctx = withRunEventPublisher(ctx, s.deps.Sink)
	var onDelta func(string)
	if execution.child != nil {
		onDelta = execution.child.appendOutput
	}
	// The resume leg binds its own cache warmer (VCP F2): the prior leg's
	// scheduler died with its context.
	warmer := s.newRunCacheWarmer(ctx, m, sessionID, eng, ledger)
	ctx = s.withLiveModelStreamObserver(ctx, m, sessionID, ledger, onDelta, execution.child != nil, warmer)
	m.setRunScope(s.deps.TenantID, workspaceID, string(sessionID))
	// Resume legs get a fresh detector (ND-2, §6): no pending reminder or
	// window state carries over from the suspended leg.
	state := newNudgeState()
	if err := s.restoreResumeNudgeBatch(context.Background(), state, runID, toolCallID, resumeBatchIDs); err != nil {
		slog.Warn("resume could not restore tool result barrier", "run", string(runID), "err", err)
		s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, err))
		return
	}
	m.setNudgeState(state)
	ctx = withNudgeState(ctx, state)
	ctx = withNudgeEmitter(ctx, s.nudgeEmitter(m, sessionID))
	m.setRunScope(s.deps.TenantID, workspaceID, string(sessionID))
	cancelOpt, steerCancel := adk.WithCancel()
	if steerCancel != nil {
		s.mu.Lock()
		s.steerCancels[runID] = steerCancel
		s.mu.Unlock()
	}
	iter, err := eng.Resume(ctx, checkpointIDFor(runID), &adk.ResumeParams{
		Targets: map[string]any{resumeTarget: resumeValue},
	}, cancelOpt)
	if err != nil {
		slog.Warn("resume failed", "run", string(runID), "err", err)
		s.emitTerminal(ctx, m, s.terminalEvent(ctx, m, err))
		return
	}
	var beforeComplete func() error
	if execution.child != nil {
		beforeComplete = func() error { return s.consumeChildMailboxSafePoint(ctx, execution.child) }
	}
	s.consume(ctx, m, sessionID, selectedTools, mode, ledger, iter, state, beforeComplete, execution)
	if execution.child != nil {
		s.failUnsettledChildMail(execution.child)
	}
}

func (s *Service) restoreResumeNudgeBatch(ctx context.Context, state *nudgeState, runID domain.RunID, toolCallID string, batchIDs []string) error {
	if len(batchIDs) == 0 {
		return nil
	}
	containsTarget := false
	siblings := make(map[string]struct{}, len(batchIDs))
	for _, id := range batchIDs {
		if id == toolCallID {
			containsTarget = true
			continue
		}
		siblings[id] = struct{}{}
	}
	if !containsTarget {
		return errors.New("runtime: resume tool batch does not contain its interrupted call")
	}
	if err := state.Register(batchIDs); err != nil {
		return err
	}
	if len(siblings) == 0 {
		return nil
	}
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		return fmt.Errorf("runtime: replay resumed tool batch: %w", err)
	}
	defer func() { _ = it.Close() }()
	settled := make(map[string]struct{}, len(siblings))
	for it.Next() {
		event := it.Value().Event
		if event.Type != domain.EventToolFinished {
			continue
		}
		var finished payloadToolFinished
		if err := json.Unmarshal(event.Payload, &finished); err != nil {
			return fmt.Errorf("runtime: decode resumed tool result: %w", err)
		}
		if _, ok := siblings[finished.ToolCallID]; !ok {
			continue
		}
		if _, done := settled[finished.ToolCallID]; done {
			continue
		}
		if err := state.SatisfyDurable(finished.ToolCallID); err != nil {
			return err
		}
		settled[finished.ToolCallID] = struct{}{}
	}
	if err := it.Err(); err != nil {
		return fmt.Errorf("runtime: replay resumed tool batch: %w", err)
	}
	return nil
}

// terminalEvent classifies the failure path: context cancellation and
// engine CancelErrors close the run as cancelled; everything else fails
// it with a structured, user-visible message (FR-11).
func (s *Service) terminalEvent(ctx context.Context, m *eventMapper, cause error) domain.RunEvent {
	if isChildTerminal(ctx) {
		var cancelErr *adk.CancelError
		if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) ||
			errors.Is(cause, errRunCancelled) || errors.As(cause, &cancelErr) {
			return m.build(domain.EventChildCancelled, map[string]any{"reason": reasonUserRequested})
		}
		message := "The child task could not be completed. Please retry."
		if providerMessage, ok := keyMissingMessage(cause); ok {
			message = providerMessage
		} else if isProviderTransportError(cause) {
			message = providerUnavailableMessage
		} else if errors.Is(cause, adk.ErrExceedMaxIterations) {
			message = "The child task reached its tool-call limit."
		} else if errors.Is(cause, ErrContextBudgetExceeded) {
			message = "The child task context exceeds the configured limit."
		}
		return m.build(domain.EventChildFailed, map[string]any{
			"cause_category": causeCategoryOf(cause), "message": message,
		})
	}
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
	// tool_search completions widen the session's deferred-tool activation
	// synchronously — the next model leg may already call a matched tool.
	if re.Type == domain.EventToolFinished {
		s.noteToolSearchMatches(sessionID, re.Payload)
	}
	if !sessionMessageProjectionDisabled(ctx) && (re.Type == domain.EventModelCompleted || re.Type == domain.EventToolRequested || re.Type == domain.EventToolFinished) {
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
	var cancelledQueue *sessionQueue
	var cancelledMarkers []domain.RunEvent
	if terminal.Type == domain.EventRunCancelled {
		s.mu.Lock()
		sid := s.runSessions[m.runID]
		s.mu.Unlock()
		if sid != "" {
			var err error
			cancelledQueue, err = s.queueFor(context.WithoutCancel(ctx), sid)
			if err != nil {
				slog.Error("cancel queue replay failed", "err", err)
				return
			}
			cancelledQueue.mu.Lock()
			defer func() {
				if cancelledQueue != nil {
					cancelledQueue.mu.Unlock()
				}
			}()
			for _, item := range append(append([]domain.QueuedTurn(nil), cancelledQueue.steer...), cancelledQueue.followUp...) {
				cancelledMarkers = append(cancelledMarkers, m.build(domain.EventTurnDequeued, payloadTurnDequeued{QueueID: item.ID, Track: item.Track, Reason: "aborted", Text: item.Text, Turn: &item}))
			}
		}
	}
	s.projectionMu.Lock()
	if s.runSessionDeleted(terminal.RunID) && !s.runSessionClosing(terminal.RunID) {
		s.cleanupRunState(terminal.RunID)
		s.projectionMu.Unlock()
		return
	}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistTimeout)
	defer cancel()
	seq, err := s.deps.Journal.Append(persistCtx, storage.Commit{RunID: terminal.RunID, Events: append(cancelledMarkers, terminal)})
	if err != nil {
		// The run already holds a terminal (concurrent close) or the
		// backend failed; either way the stored truth wins and the row
		// must not be flipped here.
		slog.Error("journal append of terminal event failed", "run", string(terminal.RunID), "type", string(terminal.Type), "err", err)
		// A parked run has no driver to retry its terminal write. Keep its
		// cancellation registration until storage acknowledges the terminal.
		s.mu.Lock()
		_, retryable := s.pending[terminal.RunID]
		s.mu.Unlock()
		if !retryable {
			s.cleanupRunState(terminal.RunID)
		}
		s.projectionMu.Unlock()
		return
	}
	terminal.Seq = seq
	if cancelledQueue != nil {
		cancelledQueue.steer, cancelledQueue.followUp = nil, nil
		cancelledQueue.mu.Unlock()
		cancelledQueue = nil
	}
	for i, event := range cancelledMarkers {
		event.Seq = seq - domain.EventSeq(len(cancelledMarkers)-i)
		s.publish(persistCtx, event)
	}

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
	var bundleHandleToRelease ToolkitViewHandle
	var goalSession domain.SessionID
	var goalRef domain.GoalRef
	var runSession domain.SessionID
	goalSession = s.goalRunSessions[terminal.RunID]
	goalRef = s.goalRunRefs[terminal.RunID]
	runSession = s.runSessions[terminal.RunID]
	delete(s.goalRunSessions, terminal.RunID)
	delete(s.goalRunRefs, terminal.RunID)
	if goalSession != "" && s.goalRuns[goalSession] == terminal.RunID {
		delete(s.goalRuns, goalSession)
	}
	if c, ok := s.active[terminal.RunID]; ok {
		delete(s.active, terminal.RunID)
		c() // idempotent: releases the detached run context
	}
	delete(s.steerCancels, terminal.RunID)
	delete(s.steerArmed, terminal.RunID)
	if pending, ok := s.shellPending[terminal.RunID]; ok {
		delete(s.shellPending, terminal.RunID)
		shellStateRefToDelete = pending.stateRef
	}
	delete(s.ledgers, terminal.RunID)
	delete(s.snapshots, terminal.RunID)
	delete(s.runTools, terminal.RunID)
	delete(s.workFenced, terminal.RunID)
	delete(s.workBlockedCalls, terminal.RunID)
	delete(s.workGates, terminal.RunID)
	delete(s.runSessions, terminal.RunID)
	bundleHandleToRelease = s.bundleViews[terminal.RunID]
	delete(s.bundleViews, terminal.RunID)
	delete(s.bundleChecks, terminal.RunID)
	s.mu.Unlock()
	if bundleHandleToRelease != nil {
		bundleHandleToRelease.Release()
	}
	s.deleteShellState(shellStateRefToDelete)
	s.projectionMu.Unlock()
	if runSession != "" {
		s.cancelPlanReviewForRun(persistCtx, runSession, terminal.RunID)
	}
	if goalSession != "" {
		settleCtx, settleCancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistTimeout)
		s.settleGoalRound(settleCtx, goalSession, terminal.RunID, status, goalRef)
		settleCancel()
	} else if runSession != "" && (status == domain.RunCompleted || s.goalCreatedByRun(persistCtx, runSession, terminal.RunID)) {
		// A completed human turn, or a failed/cancelled turn that created
		// the current Goal, releases the session for the next candidate.
		s.wakeGoal(runSession, false)
	}
	// VCP-B1: a settling human turn admits the follow-up lane (steer items
	// left behind demote into it); a user abort flushes both lanes instead —
	// pi semantics: queued text returns to the caller via turn.dequeued, not
	// into a run the user just cancelled. Goal/cron runs are excluded.
	if runSession != "" && goalSession == "" {
		switch terminal.Type {
		case domain.EventRunCompleted, domain.EventRunFailed:
			s.drainFollowUps(runSession, terminal.RunID)
		}
	}
}

func (s *Service) cleanupRunState(runID domain.RunID) {
	var shellStateRefToDelete string
	var bundleHandleToRelease ToolkitViewHandle
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
	delete(s.workFenced, runID)
	delete(s.workBlockedCalls, runID)
	delete(s.workGates, runID)
	delete(s.runSessions, runID)
	if goalSession := s.goalRunSessions[runID]; goalSession != "" && s.goalRuns[goalSession] == runID {
		delete(s.goalRuns, goalSession)
	}
	delete(s.goalRunSessions, runID)
	delete(s.contextViews, runID)
	bundleHandleToRelease = s.bundleViews[runID]
	delete(s.bundleViews, runID)
	delete(s.bundleChecks, runID)
	delete(s.overflowRecovered, runID)
	delete(s.overflowAwaiting, runID)
	s.mu.Unlock()
	if bundleHandleToRelease != nil {
		bundleHandleToRelease.Release()
	}
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
		case domain.EventAutoRetryStarted:
			payload = payloadAutoRetryStarted{Attempt: event.Attempt, Reason: event.Reason}
		case domain.EventAutoRetryFinished:
			payload = payloadAutoRetryFinished{Attempt: event.Attempt, Success: event.Success}
		default:
			return fmt.Errorf("runtime: unsupported governance event %q", event.Type)
		}
		if ledger != nil {
			if err := ledger.ReserveEvent(); err != nil {
				return err
			}
			// The summarization model call's own charge lands at the
			// call-observer Begin (v3 model.request), not here — charging
			// again would double-count the same generation (OBS-02).
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
