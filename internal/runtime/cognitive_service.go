package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	laputaevolution "github.com/dashimaki/laputa/evolution"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/observerhost"
	"agent-vivy/internal/storage"
	"agent-vivy/sdk/port/observer"
)

const cognitiveStateKey = "cognitive/state"

// CognitiveSource reports the committed-activity watermark of the bound
// input source. The evolution window's Through is read here so a crash
// between capture and wake cannot fabricate or lose input.
type CognitiveSource interface {
	HighWatermark(ctx context.Context) (uint64, error)
}

// CognitiveMissionSource reports the current authority Mission revision.
// The bound Domain (or its persona adapter) implements it; 0 means
// unassigned.
type CognitiveMissionSource interface {
	MissionRevision(ctx context.Context) (uint64, error)
}

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
	LastReason          string                        `json:"last_reason,omitempty"`
}

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
// the durable copy on every wake.
func (s *Service) UpdateCognitivePolicy(ctx context.Context, policy laputaevolution.TriggerPolicy) error {
	if s.deps.Cognitive == nil || s.deps.Cognitive.Store == nil {
		return ErrCognitiveUnavailable
	}
	return s.updateCognitiveState(ctx, func(st *cognitiveState) { st.Policy = policy })
}

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
	st, _, err := s.loadCognitiveState(ctx)
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
			if run.Status == domain.RunCompleted {
				st.Watermark = st.PendingThrough
				st.LastCompletedUnixMS = now
				st.Attempt = 0
			} else {
				// A failed run never advances the watermark; the next wake
				// retries the same window under a new attempt suffix.
				st.Attempt++
			}
			st.ActiveRunID = ""
		}
	}
	high, err := s.cognitiveHighWatermark(ctx, st)
	if err != nil {
		return laputaevolution.Eligibility{}, err
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
		return elig, s.saveCognitiveState(ctx, st)
	}
	if b.Binding.MissionAssigned() && b.Mission != nil {
		current, err := b.Mission.MissionRevision(ctx)
		if err != nil {
			return laputaevolution.Eligibility{}, err
		}
		if err := b.Binding.CheckMissionRevision(current); err != nil {
			return laputaevolution.Eligibility{}, err
		}
	}
	parentID, err := s.ensureCognitiveSupervisor(ctx, &st)
	if err != nil {
		return laputaevolution.Eligibility{}, err
	}
	input, err := json.Marshal(laputaevolution.Input{
		Binding: b.Binding,
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
	} else {
		st.Attempt++
	}
	if err := s.saveCognitiveState(ctx, st); err != nil {
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
	const sessionID = domain.SessionID("sess_cognitive_supervisor")
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

func (s *Service) saveCognitiveState(ctx context.Context, st cognitiveState) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	_, version, err := s.deps.Cognitive.Store.Get(ctx, cognitiveStateKey)
	if err != nil {
		return err
	}
	return s.deps.Cognitive.Store.Put(ctx, cognitiveStateKey, raw, version)
}

func (s *Service) updateCognitiveState(ctx context.Context, mutate func(*cognitiveState)) error {
	st, _, err := s.loadCognitiveState(ctx)
	if err != nil {
		return err
	}
	mutate(&st)
	return s.saveCognitiveState(ctx, st)
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
type CognitiveCapture struct {
	SubjectID   string
	WorkspaceID string
	SessionID   string
	RunID       domain.RunID
	EventID     string
	Phase       string // completed | failed | canceled
	Content     string
	OccurredAt  int64 // unix ms
}

// CognitiveCaptureReceipt is the durable acceptance returned by the bound
// capture surface. Seq is the committed-activity ledger position the
// evolution watermark advances against.
type CognitiveCaptureReceipt struct {
	IngestionID string
	Seq         uint64
	Status      string
}

// CognitiveCaptureSink is the ViVy-owned port to the bound capture surface.
// Implementations must return the original receipt on redelivery.
type CognitiveCaptureSink interface {
	Capture(ctx context.Context, capture CognitiveCapture) (CognitiveCaptureReceipt, error)
}

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
