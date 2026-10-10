package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ProjectViVy/inofy"
	laputaevolution "github.com/ProjectViVy/laputa/laputa/evolution"
	laputadiva "github.com/ProjectViVy/laputa/laputa/evolution/diva"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/observerhost"
	"agent-vivy/internal/storage"
	"agent-vivy/sdk/port/observer"
)

const cognitiveStateKey = "cognitive/state"

// CognitiveSource reports the committed-activity watermark of the bound
// input source. The evolution window's Through is read here so a crash
// between capture and wake cannot fabricate or lose input.
type CognitiveSource = cognitivecontract.Source

// CognitiveMissionSource reports the current authority Mission revision.
// The bound Domain (or its persona adapter) implements it; 0 means
// unassigned.
type CognitiveMissionSource = cognitivecontract.MissionSource

// cognitiveState is the durable trigger record persisted under the
// host-owned snapshot store. Every admission decision reads it; every settle
// writes it before the next wake.
type cognitiveState struct {
	SupervisorSeq       int                           `json:"supervisor_seq"`
	SupervisorRunID     string                        `json:"supervisor_run_id,omitempty"`
	ActiveRunID         string                        `json:"active_run_id,omitempty"`
	PendingThrough      uint64                        `json:"pending_through"`
	Watermark           uint64                        `json:"watermark"`
	SourceHigh          uint64                        `json:"source_high"`
	Attempt             int                           `json:"attempt"`
	LastCompletedUnixMS int64                         `json:"last_completed_unix_ms"`
	Policy              laputaevolution.TriggerPolicy `json:"policy"`
	PolicyRevision      uint64                        `json:"policy_revision"`
	LastReason          string                        `json:"last_reason,omitempty"`
	// Blocked is the durable admission fence: a non-empty reason stops
	// automatic retries. Manual triggers may retry cancellation/exhaustion,
	// but cannot clear an unresolved effect. Disabling/enabling the loop
	// never clears the fence.
	Blocked string `json:"blocked,omitempty"`
}

// cognitiveSupervisorSessionID is the host-owned supervisor's session.
// Its primary runs are bookkeeping, never user evidence, so the capture
// provider and the foreground-busy check exclude them.
const cognitiveSupervisorSessionID = domain.SessionID("sess_cognitive_supervisor")

// cognitiveMaxAttempts bounds retries of one input window before durable
// block. Cancellation and unknown outcomes bypass it: they block at once.
const cognitiveMaxAttempts = 3

// Cognitive block reasons persisted in cognitiveState.Blocked.
const (
	cognitiveBlockCancelled   = "cancelled"
	cognitiveBlockUnknown     = "unknown_outcome"
	cognitiveBlockExhausted   = "attempts_exhausted"
	cognitiveBlockUnavailable = "unavailable"
)

// cognitiveRuntime is the automatic wake loop. The timer only delivers wake
// signals; all eligibility lives in the bound Evaluate gate.
type cognitiveRuntime struct {
	cancel context.CancelFunc
	wg     sync.WaitGroup
	wake   chan struct{}
	mu     sync.Mutex
	closed bool
}

// TriggerCognitive runs the manual entry path through the same admission as
// automatic wakeups. It is synchronous so callers see the gate outcome.
func (s *Service) TriggerCognitive(ctx context.Context) (laputaevolution.Eligibility, error) {
	return s.cognitiveAttempt(ctx, true)
}

// NotifyCognitiveInput records a durably accepted capture and wakes the
// trigger loop. Seq is the source ledger position; high-watermark sources
// are re-read per wake so this never manufactures input.
func (s *Service) NotifyCognitiveInput(ctx context.Context, seq uint64) error {
	b := s.deps.Cognitive
	if b == nil || b.Store == nil {
		return nil
	}
	if err := s.updateCognitiveState(ctx, func(st *cognitiveState) {
		if seq > st.SourceHigh {
			st.SourceHigh = seq
		}
	}); err != nil {
		return err
	}
	s.kickCognitive()
	return nil
}

// UpdateCognitivePolicy persists the enabled trigger policy; admission reads
// the durable copy on every wake. The durable policy revision increments on
// every accepted write so control-plane writers CAS against it.
func (s *Service) UpdateCognitivePolicy(ctx context.Context, policy laputaevolution.TriggerPolicy) error {
	if s.deps.Cognitive == nil || s.deps.Cognitive.Store == nil {
		return ErrCognitiveUnavailable
	}
	return s.updateCognitiveState(ctx, func(st *cognitiveState) {
		st.Policy = policy
		st.PolicyRevision++
	})
}

// CognitivePolicyState returns the durable trigger policy and its revision.
func (s *Service) CognitivePolicyState(ctx context.Context) (laputaevolution.TriggerPolicy, uint64, error) {
	if s.deps.Cognitive == nil || s.deps.Cognitive.Store == nil {
		return laputaevolution.TriggerPolicy{}, 0, ErrCognitiveUnavailable
	}
	st, _, err := s.loadCognitiveState(ctx)
	if err != nil {
		return laputaevolution.TriggerPolicy{}, 0, err
	}
	return st.Policy, st.PolicyRevision, nil
}

// UpdateCognitivePolicyCAS replaces the durable trigger policy only when the
// caller's base revision still matches; otherwise it fails conflicted and
// the write is rejected instead of silently dropped.
func (s *Service) UpdateCognitivePolicyCAS(ctx context.Context, policy laputaevolution.TriggerPolicy, baseRevision uint64) error {
	if s.deps.Cognitive == nil || s.deps.Cognitive.Store == nil {
		return ErrCognitiveUnavailable
	}
	s.cogStateMu.Lock()
	defer s.cogStateMu.Unlock()
	st, version, err := s.loadCognitiveState(ctx)
	if err != nil {
		return err
	}
	if st.PolicyRevision != baseRevision {
		return ErrPolicyConflict
	}
	st.Policy = policy
	st.PolicyRevision++
	return s.saveCognitiveState(ctx, st, version)
}

// ErrPolicyConflict rejects a policy write whose base revision is stale.
var ErrPolicyConflict = errors.New("runtime: cognitive policy revision conflict")

// CognitiveStatusView projects the single durable admission record, including
// its unresolved window. ActiveRunID also names terminal runs awaiting recovery.
type CognitiveStatusView struct {
	laputaevolution.TriggerState
	PendingThrough uint64
	Phase          string
	BlockReason    string
}

// CognitiveStatus exposes the durable trigger record for inspection.
func (s *Service) CognitiveStatus(ctx context.Context) (CognitiveStatusView, uint64, error) {
	if s.deps.Cognitive == nil || s.deps.Cognitive.Store == nil {
		return CognitiveStatusView{}, 0, ErrCognitiveUnavailable
	}
	st, _, err := s.loadCognitiveState(ctx)
	if err != nil {
		return CognitiveStatusView{}, 0, err
	}
	phase := "idle"
	switch {
	case st.Blocked != "":
		phase = "blocked"
	case st.ActiveRunID != "":
		phase = "running"
	case !st.Policy.Enabled:
		phase = "disabled"
	}
	return CognitiveStatusView{
		TriggerState: laputaevolution.TriggerState{
			LastCompletedUnixMS: st.LastCompletedUnixMS,
			ActiveRunID:         st.ActiveRunID,
		},
		PendingThrough: st.PendingThrough,
		Phase:          phase,
		BlockReason:    st.Blocked,
	}, st.Watermark, nil
}

// StartCognitiveLoop begins the automatic wake loop: one self-contained
// timer delivering wake signals on tick plus immediate wakes from accepted
// captures. It is idempotent and a no-op without a durable trigger store.
func (s *Service) StartCognitiveLoop(parent context.Context, interval time.Duration) {
	if s == nil {
		return
	}
	b := s.deps.Cognitive
	if b == nil || b.Store == nil {
		return
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	s.cogMu.Lock()
	defer s.cogMu.Unlock()
	if s.cognitive != nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	c := &cognitiveRuntime{cancel: cancel, wake: make(chan struct{}, 1)}
	s.cognitive = c
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.wake:
			case <-ticker.C:
			}
			if _, err := s.cognitiveAttempt(ctx, false); err != nil && ctx.Err() == nil {
				continue
			}
		}
	}()
}

// StopCognitiveLoop stops admission then waits for the loop to exit. In-
// flight workflow runs stay durable: recovery resumes them next session.
func (s *Service) StopCognitiveLoop() {
	s.cogMu.Lock()
	c := s.cognitive
	s.cognitive = nil
	s.cogMu.Unlock()
	if c == nil {
		return
	}
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	c.cancel()
	c.wg.Wait()
}

// CognitiveLoopActive reports whether the automatic wake loop is running;
// embedded lifecycle tests assert start-once semantics through it.
func (s *Service) CognitiveLoopActive() bool {
	if s == nil {
		return false
	}
	s.cogMu.Lock()
	c := s.cognitive
	s.cogMu.Unlock()
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.closed
}

// cognitiveRunBlocked classifies a terminal non-completed workflow run:
// unknown node outcomes, recovery-required engine state and missing
// bindings are durable fences, not transient failures.
func (s *Service) cognitiveRunBlocked(ctx context.Context, runID domain.RunID) bool {
	details, err := s.GetWorkflow(ctx, runID)
	if err != nil {
		return true
	}
	if details.EngineStatus == string(inofy.RunRecoveryRequired) {
		return true
	}
	for _, node := range details.Nodes {
		if node.ErrorCategory == string(inofy.ErrOutcomeUnknown) ||
			strings.Contains(node.Message, "outcome is unknown") {
			return true
		}
		// These stages can commit authority effects before their result is
		// persisted. A later engine failure must not mint new operation IDs.
		if (node.Key == "reconcile" || node.Key == "effects") &&
			(node.Status == "running" || node.Status == "completed" || node.Status == "failed" || node.Status == "cancelled") {
			return true
		}
	}
	return false
}

func (s *Service) cognitiveWindowResolved(ctx context.Context, st cognitiveState) bool {
	details, err := s.GetWorkflow(ctx, domain.RunID(st.ActiveRunID))
	if err != nil {
		return false
	}
	var outcome laputadiva.Outcome
	if err := laputaevolution.DecodeStrictJSON([]byte(details.Outputs["outcome"]), &outcome); err != nil {
		return false
	}
	want := laputaevolution.Window{SourceID: s.deps.Cognitive.SourceID, After: st.Watermark, Through: st.PendingThrough}
	if outcome.Window != want {
		return false
	}
	switch outcome.Status {
	case laputadiva.OutcomeApplied, laputadiva.OutcomeSubmitted, laputadiva.OutcomeNoChange:
	default:
		return false
	}
	for _, receipt := range outcome.Receipts {
		switch receipt.Status {
		case laputaevolution.StatusApplied, laputaevolution.StatusSubmitted, laputaevolution.StatusNoChange:
		default:
			return false
		}
	}
	return true
}

func (s *Service) kickCognitive() {
	s.cogMu.Lock()
	c := s.cognitive
	s.cogMu.Unlock()
	if c == nil {
		return
	}
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return
	}
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// cognitiveAttempt runs one admission decision for the bound strategy.
// Order per wake: settle the recorded active run, read the source high
// watermark, Evaluate, then start the workflow under the host-owned
// supervisor run. Repeated wakes while a run is active coalesce on the
// persisted ActiveRunID.
func (s *Service) cognitiveAttempt(ctx context.Context, manual bool) (laputaevolution.Eligibility, error) {
	b := s.deps.Cognitive
	if b == nil || b.Domain == nil {
		return laputaevolution.Eligibility{}, ErrCognitiveUnavailable
	}
	if b.Store == nil {
		return laputaevolution.Eligibility{}, ErrCognitiveUnavailable
	}
	s.cogStateMu.Lock()
	defer s.cogStateMu.Unlock()
	st, version, err := s.loadCognitiveState(ctx)
	if err != nil {
		return laputaevolution.Eligibility{}, err
	}
	if !manual {
		s.cogMu.Lock()
		c := s.cognitive
		closed := c == nil
		if c != nil {
			c.mu.Lock()
			closed = c.closed
			c.mu.Unlock()
		}
		s.cogMu.Unlock()
		if closed {
			return laputaevolution.Eligibility{Reason: laputaevolution.ReasonDisabled}, nil
		}
	}
	now := b.now()
	if st.ActiveRunID != "" {
		run, getErr := s.deps.Runs.GetRun(ctx, domain.RunID(st.ActiveRunID))
		switch {
		case getErr != nil && errors.Is(getErr, storage.ErrNotFound):
			st.ActiveRunID = ""
		case getErr != nil:
			return laputaevolution.Eligibility{}, getErr
		case run.Status.Terminal():
			switch {
			case run.Status == domain.RunCompleted:
				if s.cognitiveWindowResolved(ctx, st) {
					st.Watermark = st.PendingThrough
					st.LastCompletedUnixMS = now
					st.Attempt = 0
				} else {
					st.Blocked = cognitiveBlockUnknown
				}
			case run.Status == domain.RunCancelled:
				// Human cancellation pauses; automatic admission never
				// retries it.
				if st.Blocked == cognitiveBlockUnknown || s.cognitiveRunBlocked(ctx, run.ID) {
					st.Blocked = cognitiveBlockUnknown
				} else {
					st.Blocked = cognitiveBlockCancelled
				}
			case s.cognitiveRunBlocked(ctx, run.ID):
				// Unknown outcome, missing binding or authority change:
				// effects may already be applied, so retrying could
				// duplicate them.
				st.Blocked = cognitiveBlockUnknown
			default:
				// A transiently failed run never advances the watermark; the
				// next wake retries the window under a new attempt suffix,
				// bounded at cognitiveMaxAttempts.
				st.Attempt++
				if st.Attempt >= cognitiveMaxAttempts {
					st.Blocked = cognitiveBlockExhausted
				}
			}
			if st.Blocked != cognitiveBlockUnknown {
				st.ActiveRunID = ""
			}
		case run.Kind == domain.RunKindWorkflow:
			// A recovered unknown workflow intentionally has no native
			// terminal. Expose its committed engine fence instead of
			// describing the lost process as still actively running.
			engine, err := s.inofyEngine()
			if err != nil {
				return laputaevolution.Eligibility{}, err
			}
			state, err := engine.LoadWorkflowStep(ctx, run.ID)
			if err != nil {
				return laputaevolution.Eligibility{}, err
			}
			if state.Projection != nil && state.Projection.Status == storage.WorkflowStepRecoveryRequired {
				st.Blocked = cognitiveBlockUnknown
			}
		}
	}
	high, err := s.cognitiveHighWatermark(ctx, st)
	if err != nil {
		return laputaevolution.Eligibility{}, err
	}
	if st.Blocked != "" && (!manual || st.Blocked == cognitiveBlockUnknown) {
		// A trigger is not receipt recovery. Preserve unresolved windows
		// across both automatic and manual wakes until actually reconciled.
		st.LastReason = "blocked:" + st.Blocked
		return laputaevolution.Eligibility{Reason: laputaevolution.EligibilityReason("blocked:" + st.Blocked)}, s.saveCognitiveState(ctx, st, version)
	}
	elig := laputaevolution.Evaluate(laputaevolution.Wake{
		NowUnixMS:      now,
		Manual:         manual,
		NewActivity:    high > st.Watermark,
		ForegroundBusy: s.cognitiveForegroundBusy(ctx, st),
	}, st.Policy, laputaevolution.TriggerState{
		LastCompletedUnixMS: st.LastCompletedUnixMS,
		ActiveRunID:         st.ActiveRunID,
	})
	st.LastReason = string(elig.Reason)
	if !elig.Run {
		return elig, s.saveCognitiveState(ctx, st, version)
	}
	if manual {
		st.Blocked = ""
	}
	// Per-admission binding resolution: each run pins the current
	// authority revision (Mission/policy) in its workflow input. A
	// resolved scope that drifts from the bound composition is refused.
	binding := b.Binding
	if b.Resolve != nil {
		resolved, resolveErr := b.Resolve(ctx)
		if resolveErr != nil {
			return laputaevolution.Eligibility{}, resolveErr
		}
		if resolved.SubjectID != b.Binding.SubjectID || resolved.WorkspaceID != b.Binding.WorkspaceID ||
			resolved.DestinationID != b.Binding.DestinationID {
			return laputaevolution.Eligibility{}, errors.New("runtime: resolved binding drifted from the bound composition")
		}
		binding = resolved
	}
	if b.Mission != nil {
		current, err := b.Mission.MissionRevision(ctx)
		if err != nil {
			return laputaevolution.Eligibility{}, err
		}
		if err := binding.CheckMissionRevision(current); err != nil {
			return laputaevolution.Eligibility{}, err
		}
		binding.MissionRevision = current
	}
	parentID, err := s.ensureCognitiveSupervisor(ctx, &st)
	if err != nil {
		return laputaevolution.Eligibility{}, err
	}
	input, err := json.Marshal(laputaevolution.Input{
		Binding: binding,
		Window:  laputaevolution.Window{SourceID: b.SourceID, After: st.Watermark, Through: high},
	})
	if err != nil {
		return laputaevolution.Eligibility{}, err
	}
	// The operation key carries the window and attempt so a failed run does
	// not permanently block the same input (terminal dedupe returns
	// Created:false instead of retrying).
	opKey := fmt.Sprintf("cognitive:%s:%d-%d:a%d", b.SourceID, st.Watermark, high, st.Attempt)
	started, err := s.StartCognitiveWorkflow(ctx, parentID, opKey, TrustedStrategyDIVA, input)
	if err != nil {
		return laputaevolution.Eligibility{}, err
	}
	if started.Created {
		st.ActiveRunID = string(started.Run.ID)
		st.PendingThrough = high
	} else if !started.Run.Status.Terminal() {
		// Crash between workflow creation and state save: reconcile the
		// persisted operation identity by adopting the live run instead
		// of minting a new attempt.
		st.ActiveRunID = string(started.Run.ID)
		st.PendingThrough = high
	} else {
		st.Attempt++
		if st.Attempt >= cognitiveMaxAttempts {
			st.Blocked = cognitiveBlockExhausted
		}
	}
	if err := s.saveCognitiveState(ctx, st, version); err != nil {
		return laputaevolution.Eligibility{}, err
	}
	return elig, nil
}

func (s *Service) cognitiveHighWatermark(ctx context.Context, st cognitiveState) (uint64, error) {
	if source := s.deps.Cognitive.Source; source != nil {
		// Notifications indicate acceptance, not completed native projection.
		// The bound source alone decides which durable prefix is admissible.
		return source.HighWatermark(ctx)
	}
	return st.SourceHigh, nil
}

// cognitiveForegroundBusy reports any live user-facing run. The supervisor
// and workflow/child runs are host-owned and never count as foreground.
func (s *Service) cognitiveForegroundBusy(ctx context.Context, st cognitiveState) bool {
	if s.deps.Runs == nil {
		return false
	}
	runs, err := s.deps.Runs.ListActiveRuns(ctx)
	if err != nil {
		return false
	}
	for _, run := range runs {
		if run.Kind != domain.RunKindPrimary {
			continue
		}
		if string(run.ID) == st.SupervisorRunID {
			continue
		}
		return true
	}
	return false
}

// ensureCognitiveSupervisor returns the durable active parent run for
// trusted strategy workflows, creating or re-adopting it. Restart recovery
// terminates orphan primaries, so a terminal supervisor is replaced, never
// resumed.
func (s *Service) ensureCognitiveSupervisor(ctx context.Context, st *cognitiveState) (domain.RunID, error) {
	if st.SupervisorRunID != "" {
		run, err := s.deps.Runs.GetRun(ctx, domain.RunID(st.SupervisorRunID))
		if err == nil && run.Status == domain.RunActive {
			s.adoptCognitiveSupervisor(run)
			return run.ID, nil
		}
	}
	sessionID := cognitiveSupervisorSessionID
	if _, err := s.deps.Sessions.GetSession(ctx, sessionID); errors.Is(err, storage.ErrNotFound) {
		if err := s.deps.Sessions.CreateSession(ctx, domain.Session{
			ID: sessionID, Title: "cognitive supervisor", CreatedAt: s.deps.Cognitive.now(),
		}); err != nil {
			return "", err
		}
	}
	st.SupervisorSeq++
	runID := domain.RunID(fmt.Sprintf("run_cognitive_supervisor_%d", st.SupervisorSeq))
	run := domain.Run{
		ID: runID, SessionID: sessionID, Status: domain.RunActive,
		Kind: domain.RunKindPrimary, CreatedAt: s.deps.Cognitive.now(), RootID: runID,
	}
	if err := s.deps.Runs.CreateRun(ctx, run); err != nil {
		return "", err
	}
	s.adoptCognitiveSupervisor(run)
	st.SupervisorRunID = string(runID)
	return runID, nil
}

// adoptCognitiveSupervisor repopulates the in-memory authorizer maps the
// child admission path requires. Supervisor runs have no tools of their
// own; trusted workflow admission applies the strategy ceiling.
func (s *Service) adoptCognitiveSupervisor(run domain.Run) {
	s.mu.Lock()
	if _, ok := s.snapshots[run.ID]; ok {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	snapshot, err := s.engine.cfg.Policy.Snapshot(domain.PolicyProfileDefault)
	if err != nil {
		return
	}
	ledger, err := NewBudgetLedger(s.deps.Budget)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.snapshots[run.ID] = snapshot
	s.ledgers[run.ID] = ledger
	s.runTools[run.ID] = childToolSet(nil)
	s.runSessions[run.ID] = run.SessionID
	s.mu.Unlock()
}

func (s *Service) loadCognitiveState(ctx context.Context) (cognitiveState, int64, error) {
	b := s.deps.Cognitive
	st := cognitiveState{Policy: b.Policy}
	raw, version, err := b.Store.Get(ctx, cognitiveStateKey)
	if err != nil {
		return cognitiveState{}, 0, err
	}
	if len(raw) == 0 {
		return st, version, nil
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return cognitiveState{}, 0, fmt.Errorf("runtime: decode cognitive state: %w", err)
	}
	return st, version, nil
}

func (s *Service) saveCognitiveState(ctx context.Context, st cognitiveState, version int64) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return s.deps.Cognitive.Store.Put(ctx, cognitiveStateKey, raw, version)
}

func (s *Service) updateCognitiveState(ctx context.Context, mutate func(*cognitiveState)) error {
	s.cogStateMu.Lock()
	defer s.cogStateMu.Unlock()
	st, version, err := s.loadCognitiveState(ctx)
	if err != nil {
		return err
	}
	mutate(&st)
	return s.saveCognitiveState(ctx, st, version)
}

// now is the bindable clock; tests inject a fake.
func (b *CognitiveBinding) now() int64 {
	if b != nil && b.Now != nil {
		return b.Now()
	}
	return time.Now().UnixMilli()
}

// CognitiveCaptureProviderID is the durable observer identity behind the
// cognitive capture cursor. Renaming it orphans prior cursors.
const CognitiveCaptureProviderID = "vivy.cognitive-capture"

// CognitiveCaptureEventTypes are the terminal conversation events the
// capture receiver consumes. Workflow and inference runs emit the same
// types; the provider filters those by Run kind so strategy reports can
// never become new source evidence.
var CognitiveCaptureEventTypes = []string{
	string(domain.EventRunCompleted),
	string(domain.EventRunFailed),
	string(domain.EventRunCancelled),
}

// CognitiveCapturePayloadFields are the only payload fields projected to the
// capture receiver; the subscription names them so nothing else leaks.
var CognitiveCapturePayloadFields = []string{
	"outcome", "summary", "tenant_id", "workspace_id", "session_id",
}

// CognitiveCapture is the host-owned capture request for one terminal
// primary run. EventID is the stable redelivery key
// ("<run_id>:<journal_seq>"): the sink's dedupe boundary.
type CognitiveCapture = cognitivecontract.Capture

// CognitiveCaptureReceipt is the durable acceptance returned by the bound
// capture surface. Seq is the committed-activity ledger position the
// evolution watermark advances against.
type CognitiveCaptureReceipt = cognitivecontract.CaptureReceipt

// CognitiveCaptureSink is the ViVy-owned port to the bound capture surface.
// Implementations must return the original receipt on redelivery.
type CognitiveCaptureSink = cognitivecontract.CaptureSink

// CognitiveCaptureProvider delivers terminal primary-run events into the
// bound capture sink through the ObserverHost durable-cursor path. Cursor
// advancement happens only after the sink reports acceptance, so a crash or
// retry replays the same stable event ID instead of duplicating memory.
type CognitiveCaptureProvider struct {
	sink   CognitiveCaptureSink
	runs   storage.RunStore
	notify func(CognitiveCaptureReceipt)
}

// NewCognitiveCaptureProvider binds a capture sink to the run store used for
// kind checks. notify fires once per durable acceptance (including replays)
// and may be nil.
func NewCognitiveCaptureProvider(runs storage.RunStore, sink CognitiveCaptureSink,
	notify func(CognitiveCaptureReceipt)) *CognitiveCaptureProvider {
	return &CognitiveCaptureProvider{sink: sink, runs: runs, notify: notify}
}

func (p *CognitiveCaptureProvider) ID() string { return CognitiveCaptureProviderID }

// CognitiveCaptureSubscription builds the host-owned observer subscription
// for the bound capture sink: terminal run events, projected to the fields
// the provider reads, delivered through the durable-cursor path.
func CognitiveCaptureSubscription(runs storage.RunStore, sink CognitiveCaptureSink,
	notify func(CognitiveCaptureReceipt)) observerhost.RunSubscription {
	return observerhost.RunSubscription{
		Provider:             NewCognitiveCaptureProvider(runs, sink, notify),
		EventTypes:           append([]string(nil), CognitiveCaptureEventTypes...),
		AllowedPayloadFields: append([]string(nil), CognitiveCapturePayloadFields...),
	}
}

func (p *CognitiveCaptureProvider) ObserveRun(ctx context.Context, event observer.RunEvent) error {
	_, err := p.ObserveRunWithReceipt(ctx, event)
	return err
}

func (p *CognitiveCaptureProvider) ObserveRunWithReceipt(ctx context.Context, event observer.RunEvent) (observer.DeliveryReceipt, error) {
	ack := func(id string) observer.DeliveryReceipt {
		return observer.NewDeliveryReceipt(event.ID, id, observer.DeliveryAccepted)
	}
	run, err := p.runs.GetRun(ctx, domain.RunID(event.ID.RunID))
	if err != nil {
		return observer.DeliveryReceipt{}, err
	}
	if run.Kind != domain.RunKindPrimary {
		// Workflow/inference terminal events are strategy output, never new
		// source evidence. They still advance the cursor.
		return ack("skip:" + string(run.Kind)), nil
	}
	if run.SessionID == cognitiveSupervisorSessionID {
		// The supervisor's own primary runs are bookkeeping, not
		// conversation evidence.
		return ack("skip:supervisor"), nil
	}
	var payload struct {
		Outcome     string `json:"outcome"`
		Summary     string `json:"summary"`
		TenantID    string `json:"tenant_id"`
		WorkspaceID string `json:"workspace_id"`
		SessionID   string `json:"session_id"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return observer.DeliveryReceipt{}, err
	}
	phase, ok := cognitiveCapturePhase(event.Type)
	if !ok {
		return ack("skip:" + event.Type), nil
	}
	if payload.SessionID != "" && payload.SessionID != string(run.SessionID) {
		return observer.DeliveryReceipt{}, fmt.Errorf("cognitive capture: terminal session differs from admitted run")
	}
	cap := CognitiveCapture{
		SubjectID: payload.TenantID, WorkspaceID: payload.WorkspaceID,
		SessionID: string(run.SessionID), RunID: run.ID, EventID: event.ID.String(),
		Phase: phase, OccurredAt: event.CreatedAt,
	}
	// Recovery joins a committed acceptance before constructing a new payload.
	// The run/session checks above still apply, including supervisor exclusion.
	var receipt CognitiveCaptureReceipt
	var found bool
	if lookup, ok := p.sink.(interface {
		LookupCapture(context.Context, CognitiveCapture) (CognitiveCaptureReceipt, bool, error)
	}); ok {
		receipt, found, err = lookup.LookupCapture(ctx, cap)
		if err != nil {
			return observer.DeliveryReceipt{}, err
		}
	}
	if !found {
		messages, ok := p.runs.(captureMessageReader)
		if !ok {
			return observer.DeliveryReceipt{}, fmt.Errorf("cognitive capture: durable message reader unavailable")
		}
		cap.Content, cap.UserContent, err = cognitiveConversationCapture(ctx, messages, run, payload.Summary)
		if err != nil {
			return observer.DeliveryReceipt{}, err
		}
		receipt, err = p.sink.Capture(ctx, cap)
	}
	if err != nil {
		return observer.DeliveryReceipt{}, err
	}
	if p.notify != nil {
		p.notify(receipt)
	}
	id := receipt.IngestionID
	if id == "" {
		id = event.ID.String()
	}
	return ack(id), nil
}

func cognitiveCapturePhase(eventType string) (string, bool) {
	switch domain.EventType(eventType) {
	case domain.EventRunCompleted:
		return "completed", true
	case domain.EventRunFailed:
		return "failed", true
	case domain.EventRunCancelled:
		return "canceled", true
	}
	return "", false
}
