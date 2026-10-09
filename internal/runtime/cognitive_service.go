package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ProjectViVy/inofy"
	laputaevolution "github.com/ProjectViVy/laputa/laputa/evolution"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/observerhost"
	"agent-vivy/internal/storage"
	"agent-vivy/sdk/port/observer"
)

const cognitiveStateKey = "cognitive/state"
const cognitiveStateSchema = 2

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
	StateSchema         int                           `json:"state_schema,omitempty"`
	Phase               string                        `json:"phase,omitempty"`
	Intent              *cognitiveIntent              `json:"intent,omitempty"`
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
	// automatic retries; only known cancellation/exhaustion may be cleared
	// by the manual retry path.
	// Unknown outcomes, human cancellation and exhausted retries record
	// their cause here; disabling/enabling the loop never clears it.
	Blocked string `json:"blocked,omitempty"`
}

// cognitiveIntent is the immutable admission identity persisted before the
// workflow admission transaction. Its original parent and exact canonical
// input are the recovery lookup key after a process restart.
type cognitiveIntent struct {
	ParentRunID  domain.RunID    `json:"parent_run_id"`
	OperationKey string          `json:"operation_key"`
	StrategyID   string          `json:"strategy_id"`
	Input        json.RawMessage `json:"input"`
	Attempt      int             `json:"attempt"`
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
	if err := s.updateCognitiveState(ctx, func(st *cognitiveState) error {
		if seq > st.SourceHigh {
			st.SourceHigh = seq
		}
		return nil
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
	return s.updateCognitiveState(ctx, func(st *cognitiveState) error {
		st.Policy = policy
		st.PolicyRevision++
		return nil
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
	if err := s.saveCognitiveState(ctx, st, version); errors.Is(err, storage.ErrVersionConflict) {
		return ErrPolicyConflict
	} else {
		return err
	}
}

// ErrPolicyConflict rejects a policy write whose base revision is stale.
var ErrPolicyConflict = errors.New("runtime: cognitive policy revision conflict")

// CognitiveStatus exposes the durable trigger record for inspection.
func (s *Service) CognitiveStatus(ctx context.Context) (laputaevolution.TriggerState, uint64, error) {
	if s.deps.Cognitive == nil || s.deps.Cognitive.Store == nil {
		return laputaevolution.TriggerState{}, 0, ErrCognitiveUnavailable
	}
	st, _, err := s.loadCognitiveState(ctx)
	if err != nil {
		return laputaevolution.TriggerState{}, 0, err
	}
	return laputaevolution.TriggerState{
		LastCompletedUnixMS: st.LastCompletedUnixMS,
		ActiveRunID:         st.ActiveRunID,
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
	safe, err := s.cognitiveRunRetrySafe(ctx, runID)
	return err != nil || !safe
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

// cognitiveAttempt reconciles any persisted admission identity first, then
// reads the source watermark, evaluates eligibility and persists the next
// immutable intent before workflow admission. Repeated wakes coalesce on the
// exact ActiveRunID recovered from that intent.
func (s *Service) cognitiveAttempt(ctx context.Context, manual bool) (laputaevolution.Eligibility, error) {
	s.cogAttemptMu.Lock()
	defer s.cogAttemptMu.Unlock()

	b := s.deps.Cognitive
	if b == nil || b.Domain == nil || b.Store == nil {
		return laputaevolution.Eligibility{}, ErrCognitiveUnavailable
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
	st, version, err := s.loadCognitiveState(ctx)
	if err != nil {
		return laputaevolution.Eligibility{}, err
	}
	if st.StateSchema != cognitiveStateSchema {
		st, version, err = s.upgradeCognitiveState(ctx, st, version)
		if err != nil {
			return laputaevolution.Eligibility{}, err
		}
	}
	if st.Intent == nil && st.ActiveRunID != "" {
		base := st
		st.Blocked, st.Phase = cognitiveBlockUnknown, "blocked"
		st.LastReason = "blocked:" + cognitiveBlockUnknown
		if err := s.saveCognitiveAttemptState(ctx, base, st, version); err != nil {
			return laputaevolution.Eligibility{}, err
		}
		return laputaevolution.Eligibility{Reason: laputaevolution.EligibilityReason(st.LastReason)}, nil
	}
	if st.Intent != nil {
		st, version, err = s.reconcileCognitiveIntent(ctx, st, version)
		if err != nil {
			return laputaevolution.Eligibility{}, err
		}
	}
	baseState := st
	if st.ActiveRunID != "" {
		return laputaevolution.Eligibility{Reason: laputaevolution.ReasonActive}, nil
	}
	if st.Blocked == cognitiveBlockUnknown {
		st.LastReason = "blocked:" + st.Blocked
		return laputaevolution.Eligibility{Reason: laputaevolution.EligibilityReason(st.LastReason)}, nil
	}
	if st.Blocked != "" && !manual {
		st.LastReason = "blocked:" + st.Blocked
		return laputaevolution.Eligibility{Reason: laputaevolution.EligibilityReason(st.LastReason)}, nil
	}
	if st.Blocked != "" && manual {
		// A manual wake may clear a known cancellation or exhausted-attempt
		// fence. Unknown outcomes remain fenced because a new key could
		// duplicate an effect whose receipt is unavailable.
		if st.Blocked != cognitiveBlockCancelled && st.Blocked != cognitiveBlockExhausted {
			return laputaevolution.Eligibility{Reason: laputaevolution.EligibilityReason("blocked:" + st.Blocked)}, nil
		}
		st.Blocked = ""
		st.Intent = nil
		st.ActiveRunID = ""
		st.Attempt++
		st.Phase = "idle"
	}

	now := b.now()
	high, err := s.cognitiveHighWatermark(ctx, st)
	if err != nil {
		return laputaevolution.Eligibility{}, err
	}
	if st.Attempt > 0 && st.PendingThrough > st.Watermark {
		// A retry keeps the exact previously admitted input window, even if
		// new activity has arrived while that window was running.
		high = st.PendingThrough
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
		if err := s.saveCognitiveAttemptState(ctx, baseState, st, version); err != nil {
			return laputaevolution.Eligibility{}, err
		}
		return elig, nil
	}

	// Resolve the current binding once and freeze the canonical input in the
	// intent before the workflow admission side effect.
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
	if b.Binding.MissionAssigned() && b.Mission != nil {
		current, missionErr := b.Mission.MissionRevision(ctx)
		if missionErr != nil {
			return laputaevolution.Eligibility{}, missionErr
		}
		if err := b.Binding.CheckMissionRevision(current); err != nil {
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
	input, _, err = normalizeINOFYInput(input)
	if err != nil {
		return laputaevolution.Eligibility{}, err
	}
	opKey := fmt.Sprintf("cognitive:%s:%d-%d:a%d", b.SourceID, st.Watermark, high, st.Attempt)
	st.Intent = &cognitiveIntent{ParentRunID: parentID, OperationKey: opKey,
		StrategyID: TrustedStrategyDIVA, Input: append(json.RawMessage(nil), input...), Attempt: st.Attempt}
	st.PendingThrough = high
	st.ActiveRunID = ""
	st.StateSchema = cognitiveStateSchema
	st.Phase = "admitting"
	st.LastReason = string(elig.Reason)
	if err := s.saveCognitiveAttemptState(ctx, baseState, st, version); err != nil {
		return laputaevolution.Eligibility{}, err
	}
	st, version, err = s.loadCognitiveState(ctx)
	if err != nil {
		return laputaevolution.Eligibility{}, err
	}
	if st.Intent == nil {
		return laputaevolution.Eligibility{}, errors.New("runtime: cognitive admission intent disappeared before reconciliation")
	}
	if _, _, err := s.reconcileCognitiveIntent(ctx, st, version); err != nil {
		return laputaevolution.Eligibility{}, err
	}
	return elig, nil
}

func (s *Service) cognitiveHighWatermark(ctx context.Context, st cognitiveState) (uint64, error) {
	b := s.deps.Cognitive
	high := st.SourceHigh
	if b.Source != nil {
		sourceHigh, err := b.Source.HighWatermark(ctx)
		if err != nil {
			return 0, err
		}
		if sourceHigh > high {
			high = sourceHigh
		}
	}
	return high, nil
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
	raw, version, err := b.Store.Get(ctx, cognitiveStateKey)
	if err != nil {
		return cognitiveState{}, 0, err
	}
	if len(raw) == 0 {
		return cognitiveState{Policy: b.Policy, StateSchema: cognitiveStateSchema, Phase: "idle"}, version, nil
	}
	var st cognitiveState
	if err := json.Unmarshal(raw, &st); err != nil {
		return cognitiveState{}, 0, fmt.Errorf("runtime: decode cognitive state: %w", err)
	}
	if st.StateSchema != 0 && st.StateSchema != cognitiveStateSchema {
		return cognitiveState{}, 0, fmt.Errorf("runtime: unsupported cognitive state schema %d", st.StateSchema)
	}
	return st, version, nil
}

func (s *Service) saveCognitiveState(ctx context.Context, st cognitiveState, expectedVersion int64) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return s.deps.Cognitive.Store.Put(ctx, cognitiveStateKey, raw, expectedVersion)
}

func (s *Service) commitCognitiveAttempt(ctx context.Context, base, next cognitiveState, version int64) (cognitiveState, int64, error) {
	if err := s.saveCognitiveAttemptState(ctx, base, next, version); err != nil {
		return cognitiveState{}, 0, err
	}
	return s.loadCognitiveState(ctx)
}

func (s *Service) upgradeCognitiveState(ctx context.Context, st cognitiveState, version int64) (cognitiveState, int64, error) {
	if st.StateSchema == cognitiveStateSchema {
		return st, version, nil
	}
	base, next := st, st
	next.StateSchema = cognitiveStateSchema
	if next.ActiveRunID != "" {
		runID := domain.RunID(next.ActiveRunID)
		run, runErr := s.deps.Runs.GetRun(ctx, runID)
		revision, revisionErr := s.deps.WorkflowRevisions.GetWorkflowRevision(ctx, runID)
		intent, through, intentErr := cognitiveIntentFromRevision(revision)
		if intentErr == nil && !cognitiveRevisionMatchesCatalog(ctx, revision) {
			intentErr = errors.New("runtime: legacy cognitive revision differs from the trusted catalog")
		}
		switch {
		case runErr != nil, revisionErr != nil, intentErr != nil,
			run.ID != runID, run.Kind != domain.RunKindWorkflow, intent.ParentRunID != run.ParentID,
			(next.PendingThrough != 0 && next.PendingThrough != through):
			next.Blocked = cognitiveBlockUnknown
			next.Phase = "blocked"
			next.LastReason = "blocked:" + cognitiveBlockUnknown
		default:
			next.Intent = &intent
			next.PendingThrough = through
			if run.Status.Terminal() {
				next.Phase = "admitting"
			} else {
				next.Phase = "running"
			}
		}
	} else {
		high, highErr := s.cognitiveHighWatermark(ctx, next)
		if highErr != nil || next.PendingThrough > next.Watermark || next.SourceHigh > next.Watermark ||
			next.Attempt != 0 || high > next.Watermark {
			// The legacy shape cannot tell whether its old code crashed before
			// admission or after an unrecorded side effect. Never invent a key.
			next.Blocked = cognitiveBlockUnknown
			next.Phase = "blocked"
			next.LastReason = "blocked:" + cognitiveBlockUnknown
		} else if next.Blocked != "" {
			next.Phase = "blocked"
		} else {
			next.Phase = "idle"
		}
	}
	return s.commitCognitiveAttempt(ctx, base, next, version)
}

func cognitiveIntentFromRevision(revision domain.WorkflowRevision) (cognitiveIntent, uint64, error) {
	if revision.SchemaVersion != 2 || revision.ParentRunID == "" || revision.OperationKey == "" ||
		len(revision.InputJSON) == 0 {
		return cognitiveIntent{}, 0, errors.New("runtime: cognitive revision is incomplete")
	}
	authority, err := decodeWorkflowAuthority(revision)
	if err != nil || authority.TrustedStrategy != TrustedStrategyDIVA {
		return cognitiveIntent{}, 0, errors.New("runtime: cognitive revision strategy is unknown")
	}
	canonical, digest, err := normalizeINOFYInput(revision.InputJSON)
	if err != nil || !bytes.Equal(canonical, revision.InputJSON) || digest != revision.InputDigest {
		return cognitiveIntent{}, 0, errors.New("runtime: cognitive revision input is not canonical")
	}
	var input laputaevolution.Input
	if err := json.Unmarshal(revision.InputJSON, &input); err != nil || input.Window.SourceID == "" || input.Window.Through < input.Window.After {
		return cognitiveIntent{}, 0, errors.New("runtime: cognitive revision window is invalid")
	}
	return cognitiveIntent{
		ParentRunID: revision.ParentRunID, OperationKey: revision.OperationKey,
		StrategyID: authority.TrustedStrategy, Input: append(json.RawMessage(nil), revision.InputJSON...),
	}, input.Window.Through, nil
}

func cognitiveRevisionMatchesCatalog(ctx context.Context, revision domain.WorkflowRevision) bool {
	if revision.SchemaVersion != 2 {
		return false
	}
	admitted, err := trustedStrategyAdmission(ctx, TrustedStrategyDIVA)
	if err != nil {
		return false
	}
	return bytes.Equal(revision.DescriptorJSON, admitted.CanonicalJSON) &&
		revision.DescriptorDigest == sha256Hex(admitted.CanonicalJSON) &&
		revision.ProgramDigest == admitted.Meta.ProgramDigest &&
		revision.CatalogDigest == admitted.Meta.CatalogDigest &&
		revision.CompilerVersion == admitted.Meta.CompilerVersion &&
		revision.EinoBuild == admitted.Meta.EinoBuild
}

func (s *Service) reconcileCognitiveIntent(ctx context.Context, st cognitiveState, version int64) (cognitiveState, int64, error) {
	if st.Intent == nil {
		return st, version, nil
	}
	base := st
	intent := *st.Intent
	intent.Input = append(json.RawMessage(nil), st.Intent.Input...)
	if intent.ParentRunID == "" || intent.OperationKey == "" || intent.StrategyID != TrustedStrategyDIVA ||
		len(intent.Input) == 0 || intent.Attempt < 0 {
		return s.fenceCognitiveIntent(ctx, base, version)
	}
	canonical, _, err := normalizeINOFYInput(intent.Input)
	if err != nil || !bytes.Equal(canonical, intent.Input) {
		return s.fenceCognitiveIntent(ctx, base, version)
	}
	var requested laputaevolution.Input
	if err := json.Unmarshal(intent.Input, &requested); err != nil || requested.Window.SourceID == "" || requested.Window.Through < requested.Window.After {
		return s.fenceCognitiveIntent(ctx, base, version)
	}
	if st.PendingThrough != requested.Window.Through {
		return s.fenceCognitiveIntent(ctx, base, version)
	}

	revision, lookupErr := s.deps.WorkflowRevisions.GetWorkflowRevisionByOperation(ctx, intent.ParentRunID, intent.OperationKey)
	var run domain.Run
	if errors.Is(lookupErr, storage.ErrNotFound) {
		parent, parentErr := s.deps.Runs.GetRun(ctx, intent.ParentRunID)
		if parentErr != nil || parent.Status != domain.RunActive || parent.Kind != domain.RunKindPrimary || parent.SessionID != cognitiveSupervisorSessionID {
			return s.fenceCognitiveIntent(ctx, base, version)
		}
		s.adoptCognitiveSupervisor(parent)
		started, startErr := s.StartCognitiveWorkflow(ctx, intent.ParentRunID, intent.OperationKey, intent.StrategyID, intent.Input)
		if startErr != nil {
			// An admission error can race the transaction's acknowledgement;
			// query the exact durable identity again before deciding whether it
			// was committed. A proven absence leaves this intent retryable.
			revision, lookupErr = s.deps.WorkflowRevisions.GetWorkflowRevisionByOperation(ctx, intent.ParentRunID, intent.OperationKey)
			if errors.Is(lookupErr, storage.ErrNotFound) {
				return st, version, startErr
			}
			if lookupErr != nil {
				return s.fenceCognitiveIntent(ctx, base, version)
			}
		} else {
			revision, run = started.Revision, started.Run
			lookupErr = nil
		}
	} else if lookupErr != nil {
		return s.fenceCognitiveIntent(ctx, base, version)
	}
	if revision.RunID == "" || revision.ParentRunID != intent.ParentRunID ||
		revision.OperationKey != intent.OperationKey || !bytes.Equal(revision.InputJSON, intent.Input) {
		return s.fenceCognitiveIntent(ctx, base, version)
	}
	revisionIntent, through, err := cognitiveIntentFromRevision(revision)
	if err != nil || revisionIntent.StrategyID != intent.StrategyID || through != requested.Window.Through {
		return s.fenceCognitiveIntent(ctx, base, version)
	}
	if !cognitiveRevisionMatchesCatalog(ctx, revision) {
		return s.fenceCognitiveIntent(ctx, base, version)
	}
	if run.ID == "" {
		run, err = s.deps.Runs.GetRun(ctx, revision.RunID)
		if err != nil || run.Kind != domain.RunKindWorkflow || run.ParentID != intent.ParentRunID {
			return s.fenceCognitiveIntent(ctx, base, version)
		}
	}
	next := st
	next.ActiveRunID = string(run.ID)
	next.PendingThrough = through
	switch run.Status {
	case domain.RunCompleted:
		next.Watermark = through
		next.LastCompletedUnixMS = s.deps.Cognitive.now()
		next.Attempt = 0
		next.ActiveRunID = ""
		next.Intent = nil
		next.Blocked = ""
		next.Phase = "completed"
	case domain.RunCancelled:
		next.ActiveRunID = ""
		next.Blocked = cognitiveBlockCancelled
		next.Phase = "cancelled"
	case domain.RunFailed:
		safe, safeErr := s.cognitiveRunRetrySafe(ctx, run.ID)
		if safeErr != nil || !safe {
			next.ActiveRunID = ""
			next.Blocked = cognitiveBlockUnknown
			next.Phase = "blocked"
		} else {
			next.ActiveRunID = ""
			next.Intent = nil
			next.Attempt = intent.Attempt + 1
			next.Phase = "failed"
			if next.Attempt >= cognitiveMaxAttempts {
				next.Blocked = cognitiveBlockExhausted
				next.Phase = "blocked"
			}
		}
	case domain.RunAccepted, domain.RunQueued:
		parent, parentErr := s.deps.Runs.GetRun(ctx, intent.ParentRunID)
		if parentErr != nil || parent.Status != domain.RunActive || parent.Kind != domain.RunKindPrimary || parent.SessionID != cognitiveSupervisorSessionID {
			return s.fenceCognitiveIntent(ctx, base, version)
		}
		s.adoptCognitiveSupervisor(parent)
		started, startErr := s.StartCognitiveWorkflow(ctx, intent.ParentRunID, intent.OperationKey, intent.StrategyID, intent.Input)
		if startErr != nil {
			current, getErr := s.deps.Runs.GetRun(ctx, run.ID)
			if getErr != nil || current.Status != domain.RunActive {
				return s.fenceCognitiveIntent(ctx, base, version)
			}
		} else {
			run = started.Run
		}
		if run.Status.Terminal() {
			return s.reconcileCognitiveIntent(ctx, st, version)
		}
		next.ActiveRunID = string(run.ID)
		next.Phase = "running"
	case domain.RunActive:
		next.ActiveRunID = string(run.ID)
		next.Phase = "running"
	default:
		return s.fenceCognitiveIntent(ctx, base, version)
	}
	return s.commitCognitiveAttempt(ctx, base, next, version)
}

func (s *Service) fenceCognitiveIntent(ctx context.Context, base cognitiveState, version int64) (cognitiveState, int64, error) {
	next := base
	next.Blocked = cognitiveBlockUnknown
	next.Phase = "blocked"
	next.LastReason = "blocked:" + cognitiveBlockUnknown
	return s.commitCognitiveAttempt(ctx, base, next, version)
}

func (s *Service) cognitiveRunRetrySafe(ctx context.Context, runID domain.RunID) (bool, error) {
	details, err := s.GetWorkflow(ctx, runID)
	if err != nil {
		return false, err
	}
	if details.Run.Status != domain.RunFailed || details.EngineStatus != string(inofy.RunFailed) {
		return false, nil
	}
	engine, err := s.inofyEngine()
	if err != nil {
		return false, err
	}
	step, err := engine.LoadWorkflowStep(ctx, runID)
	if err != nil {
		return false, err
	}
	if step.Projection == nil || string(step.Projection.Status) != string(inofy.RunFailed) {
		// GetWorkflow falls back to the native Run status when the durable
		// workflow projection is absent; that alone is not settlement proof.
		return false, nil
	}
	revision, err := s.deps.WorkflowRevisions.GetWorkflowRevision(ctx, runID)
	if err != nil {
		return false, err
	}
	intent, _, err := cognitiveIntentFromRevision(revision)
	if err != nil || intent.StrategyID != TrustedStrategyDIVA {
		return false, nil
	}
	if !cognitiveRevisionMatchesCatalog(ctx, revision) {
		return false, nil
	}
	var definition inofy.Definition
	if err := json.Unmarshal(details.Definition, &definition); err != nil {
		return false, err
	}
	expected := map[string]bool{"collect": true, "prepare": true, "reconcile": true, "reflect": true, "effects": true, "finish": true}
	seen := make(map[string]WorkflowNodeProjection, len(details.Nodes))
	for _, node := range details.Nodes {
		if !expected[node.Key] || seen[node.Key].Key != "" {
			return false, nil
		}
		seen[node.Key] = node
	}
	if len(seen) != len(expected) || len(definition.Graph.Nodes) != len(expected) {
		return false, nil
	}
	for key := range expected {
		node, ok := seen[key]
		if !ok || node.ErrorCategory == string(inofy.ErrOutcomeUnknown) || strings.Contains(node.Message, "outcome is unknown") {
			return false, nil
		}
		switch node.Status {
		case "waiting", "blocked", "completed", "failed", "cancelled", "running":
		default:
			return false, nil
		}
		if node.Status == "running" || node.Status == "cancelled" {
			return false, nil
		}
		if (key == "reconcile" || key == "effects") && node.Status != "waiting" && node.Status != "blocked" {
			// Both stages may write through Domain.Apply; even a failed stage
			// has an ambiguous external effect unless it never started.
			return false, nil
		}
	}
	return s.cognitiveInferenceEvidenceSafe(ctx, runID)
}

func (s *Service) cognitiveInferenceEvidenceSafe(ctx context.Context, workflowID domain.RunID) (bool, error) {
	children, err := s.deps.Runs.ListChildRuns(ctx, workflowID)
	if err != nil {
		return false, err
	}
	for _, child := range children {
		if child.Kind != domain.RunKindChild || !child.Status.Terminal() || child.Status == domain.RunCancelled {
			return false, nil
		}
		it, err := s.deps.Journal.Replay(ctx, child.ID, 0)
		if err != nil {
			return false, err
		}
		requests := map[string]bool{}
		finished := map[string]bool{}
		started, terminal := false, domain.EventType("")
		for it.Next() {
			event := it.Value().Event
			switch event.Type {
			case domain.EventRunStarted:
				if started {
					_ = it.Close()
					return false, nil
				}
				started = true
			case domain.EventRunCompleted, domain.EventRunFailed:
				if terminal != "" {
					_ = it.Close()
					return false, nil
				}
				terminal = event.Type
			case domain.EventRunCancelled:
				_ = it.Close()
				return false, nil
			case domain.EventProviderRetry, domain.EventProviderStall,
				domain.EventModelDelta, domain.EventModelReasoningDelta, domain.EventModelCompleted:
			case domain.EventModelRequest:
				var request payloadModelRequestV3
				if err := json.Unmarshal(event.Payload, &request); err != nil || request.CallID == "" || requests[request.CallID] {
					_ = it.Close()
					return false, nil
				}
				requests[request.CallID] = true
			case domain.EventModelUsage:
				var usage payloadModelUsageV2
				if err := json.Unmarshal(event.Payload, &usage); err != nil || usage.CallID == "" ||
					!requests[usage.CallID] || finished[usage.CallID] {
					_ = it.Close()
					return false, nil
				}
			case domain.EventModelCallFinished:
				var finish payloadModelCallFinished
				if err := json.Unmarshal(event.Payload, &finish); err != nil || finish.CallID == "" || !requests[finish.CallID] || finished[finish.CallID] {
					_ = it.Close()
					return false, nil
				}
				switch finish.Status {
				case "completed":
					if !finish.ResponseComplete || finish.Error != nil {
						_ = it.Close()
						return false, nil
					}
				case "failed":
					if finish.ResponseComplete || finish.Error == nil || finish.Error.Name == "" || finish.Error.Message == "" {
						_ = it.Close()
						return false, nil
					}
				default:
					_ = it.Close()
					return false, nil
				}
				finished[finish.CallID] = true
			default:
				// These children are the strategy's inference-only lane. An
				// unknown event could represent an effect without a receipt.
				_ = it.Close()
				return false, nil
			}
		}
		iterErr := it.Err()
		closeErr := it.Close()
		if closeErr != nil {
			return false, closeErr
		}
		if iterErr != nil {
			return false, iterErr
		}
		if len(requests) != len(finished) {
			return false, nil
		}
		wantTerminal := domain.EventRunCompleted
		if child.Status == domain.RunFailed {
			wantTerminal = domain.EventRunFailed
		}
		if !started || terminal != wantTerminal {
			return false, nil
		}
	}
	return true, nil
}

// saveCognitiveAttemptState first commits against the snapshot the attempt
// read. If a pure state update won in the meantime, it rebases only fields
// this attempt changed onto the newest state. It never repeats the attempt's
// external reads or effects.
func (s *Service) saveCognitiveAttemptState(ctx context.Context, base, attempted cognitiveState, expectedVersion int64) error {
	s.cogStateMu.Lock()
	defer s.cogStateMu.Unlock()
	if err := s.saveCognitiveState(ctx, attempted, expectedVersion); err == nil || !errors.Is(err, storage.ErrVersionConflict) {
		return err
	}

	latest, latestVersion, err := s.loadCognitiveState(ctx)
	if err != nil {
		return err
	}
	latest = rebaseCognitiveAttemptState(latest, base, attempted)
	return s.saveCognitiveState(ctx, latest, latestVersion)
}

func rebaseCognitiveAttemptState(latest, base, attempted cognitiveState) cognitiveState {
	if attempted.StateSchema != base.StateSchema {
		latest.StateSchema = attempted.StateSchema
	}
	if attempted.Phase != base.Phase {
		latest.Phase = attempted.Phase
	}
	if attempted.Intent != base.Intent {
		if attempted.Intent == nil {
			latest.Intent = nil
		} else {
			intent := *attempted.Intent
			intent.Input = append(json.RawMessage(nil), attempted.Intent.Input...)
			latest.Intent = &intent
		}
	}
	if attempted.SupervisorSeq != base.SupervisorSeq {
		latest.SupervisorSeq = attempted.SupervisorSeq
	}
	if attempted.SupervisorRunID != base.SupervisorRunID {
		latest.SupervisorRunID = attempted.SupervisorRunID
	}
	if attempted.ActiveRunID != base.ActiveRunID {
		latest.ActiveRunID = attempted.ActiveRunID
	}
	if attempted.PendingThrough != base.PendingThrough {
		latest.PendingThrough = attempted.PendingThrough
	}
	if attempted.Watermark != base.Watermark {
		latest.Watermark = attempted.Watermark
	}
	if attempted.Attempt != base.Attempt {
		latest.Attempt = attempted.Attempt
	}
	if attempted.LastCompletedUnixMS != base.LastCompletedUnixMS {
		latest.LastCompletedUnixMS = attempted.LastCompletedUnixMS
	}
	if attempted.LastReason != base.LastReason {
		latest.LastReason = attempted.LastReason
	}
	if attempted.Blocked != base.Blocked {
		latest.Blocked = attempted.Blocked
	}
	return latest
}

func (s *Service) updateCognitiveState(ctx context.Context, mutate func(*cognitiveState) error) error {
	s.cogStateMu.Lock()
	defer s.cogStateMu.Unlock()
	st, version, err := s.loadCognitiveState(ctx)
	if err != nil {
		return err
	}
	if err := mutate(&st); err != nil {
		return err
	}
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
	if payload.SessionID == "" {
		payload.SessionID = string(run.SessionID)
	}
	receipt, err := p.sink.Capture(ctx, CognitiveCapture{
		SubjectID:   payload.TenantID,
		WorkspaceID: payload.WorkspaceID,
		SessionID:   payload.SessionID,
		RunID:       run.ID,
		EventID:     event.ID.String(),
		Phase:       phase,
		Content:     payload.Summary,
		OccurredAt:  event.CreatedAt,
	})
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
