package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
	"github.com/cloudwego/eino/schema"
)

const maxChildActivationContextBytes = 128 << 10

type childActivationDrive struct {
	binding  domain.ChildSessionBinding
	task     string
	pending  []domain.ChildMailboxMessage
	included []domain.ChildMailboxMessage
	outputMu sync.Mutex
	output   strings.Builder
}

func (state *childActivationDrive) appendOutput(content string) {
	if state == nil {
		return
	}
	state.outputMu.Lock()
	state.output.WriteString(content)
	state.outputMu.Unlock()
}

func (state *childActivationDrive) resetOutput() {
	if state == nil {
		return
	}
	state.outputMu.Lock()
	state.output.Reset()
	state.outputMu.Unlock()
}

func (state *childActivationDrive) result() string {
	if state == nil {
		return ""
	}
	state.outputMu.Lock()
	defer state.outputMu.Unlock()
	return state.output.String()
}

// StartChildActivation starts an already-admitted continuable child Run using
// the normal Service event, policy, budget, checkpoint, and Eino runner path.
// Repeated calls while the Run is active are idempotent.
func (s *Service) StartChildActivation(ctx context.Context, childSessionID domain.SessionID, runID domain.RunID) error {
	if s == nil || s.engine == nil || s.deps.ChildSessions == nil || s.deps.ChildMailbox == nil ||
		s.deps.Runs == nil || s.deps.Messages == nil || s.deps.Sessions == nil || s.deps.Journal == nil ||
		s.deps.Sink == nil || s.deps.Workspaces == nil {
		return errors.New("runtime: native child activation is not wired")
	}
	if childSessionID == "" || runID == "" {
		return errors.New("runtime: child session and activation run are required")
	}
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	if s.sessionDeleted(childSessionID) {
		return storage.ErrNotFound
	}
	binding, err := s.deps.ChildSessions.GetChildSessionBinding(ctx, childSessionID)
	if err != nil {
		return err
	}
	if s.sessionDeleted(binding.OriginParentSessionID) {
		return storage.ErrNotFound
	}
	if binding.State != domain.ChildSessionOpen || binding.ActivationRunID != runID {
		return storage.ErrChildAdmissionConflict
	}
	run, err := s.deps.Runs.GetRun(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status != domain.RunActive || run.SessionID != childSessionID || run.Kind != domain.RunKindChild ||
		run.EffectiveChildMode() != domain.ChildModeContinuable || run.ParentID != binding.AuthorizerRunID {
		return storage.ErrChildAdmissionConflict
	}
	s.mu.Lock()
	if _, started := s.active[runID]; started {
		s.mu.Unlock()
		return nil
	}
	snapshot, snapshotOK := s.snapshots[runID]
	ledger := s.ledgers[runID]
	selected := s.runTools[runID]
	toolsOK := selected != nil
	toolNames := make([]string, 0, len(selected))
	for name := range selected {
		toolNames = append(toolNames, name)
	}
	s.mu.Unlock()
	if !snapshotOK || ledger == nil || !toolsOK {
		return s.failAdmittedChildActivation(ctx, run, errors.New("runtime: admitted child authority is unavailable for this activation"))
	}
	canonical, err := domain.CanonicalToolNames(toolNames)
	if err != nil || !sameStrings(canonical, binding.ActivationToolNames) {
		return s.failAdmittedChildActivation(ctx, run, errors.New("runtime: admitted child tool surface does not match its binding"))
	}
	childEngine, err := s.engine.ChildView(ctx, canonical)
	if err != nil {
		return s.failAdmittedChildActivation(ctx, run, err)
	}
	childSession, err := s.deps.Sessions.GetSession(ctx, childSessionID)
	if err != nil {
		return s.failAdmittedChildActivation(ctx, run, err)
	}
	sandboxMode, approvalPolicy := childSession.EffectiveSandbox()
	workspace, err := s.deps.Workspaces.Ensure(withSessionID(ctx, childSessionID), runID)
	if err != nil {
		return s.failAdmittedChildActivation(ctx, run, fmt.Errorf("runtime: resolve child workspace: %w", err))
	}
	pending, err := s.deps.ChildMailbox.ListPendingChildMessages(ctx, childSessionID, childSessionID, binding.ConsumedMessageSequence, maxChildMailboxBatch)
	if err != nil {
		return s.failAdmittedChildActivation(ctx, run, fmt.Errorf("runtime: list child inbox at activation safe point: %w", err))
	}
	for i := range pending {
		pending[i].Body = append([]byte(nil), pending[i].Body...)
	}
	task, err := activationTask(ctx, s.deps.Messages, childSessionID, runID)
	if err != nil {
		return s.failAdmittedChildActivation(ctx, run, err)
	}
	if err := s.ensureChildRequestedLocked(ctx, run, binding.AuthorizerRunID, task); err != nil {
		return err
	}
	_, childStarted, err := s.readChildLifecycle(ctx, runID)
	if err != nil {
		return err
	}
	if childStarted {
		// A previous process crossed the model boundary. Startup recovery owns
		// the narrow resumable-approval case; a normal start must not replay it.
		return ErrWorkflowNodeUnknownOutcome
	}
	var face domain.Face
	face, err = normalizeFace(domain.FaceWeb)
	if err != nil {
		return s.failAdmittedChildActivation(ctx, run, err)
	}
	mapper := newEventMapper(runID, childEngine.cfg.MaxEventPayloadBytes)
	mapper.setRunScope(s.deps.TenantID, workspace.ID, string(childSessionID))
	providerName, modelID := s.CurrentModel()
	mapper.setUsageRoutes(providerName, modelID, childEngine.cfg.SummaryModelID)
	startedPayload, err := json.Marshal(map[string]string{
		"parent_run_id": string(binding.AuthorizerRunID), "workspace_id": workspace.ID,
	})
	if err != nil {
		return err
	}
	if _, err := s.appendRunEventLocked(ctx, domain.RunEvent{
		RunID: runID, Type: domain.EventChildStarted, CreatedAt: time.Now().UnixMilli(),
		PayloadVersion: 1, Payload: startedPayload,
	}, false); err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s.mu.Lock()
	if _, started := s.active[runID]; started {
		s.mu.Unlock()
		cancel()
		return nil
	}
	s.active[runID] = cancel
	s.runSessions[runID] = childSessionID
	s.ledgers[runID] = ledger
	s.snapshots[runID] = snapshot
	s.runTools[runID] = childToolSet(canonical)
	s.mu.Unlock()
	state := &childActivationDrive{binding: binding, task: task, pending: pending}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.driveWithExecution(runCtx, mapper, childSessionID, state.task, domain.RunModeNormal,
			snapshot.Profile, "", snapshot, sandboxMode, approvalPolicy, face, workspace.ID, nil,
			runExecutionOptions{engine: childEngine, child: state})
		s.failUnsettledChildMail(state)
	}()
	return nil
}

func (s *Service) ensureChildRequestedLocked(ctx context.Context, run domain.Run, parentRunID domain.RunID, task string) error {
	requested, _, err := s.readChildLifecycle(ctx, run.ID)
	if err != nil {
		return err
	}
	if requested != nil {
		if requested.ParentRunID != string(parentRunID) || requested.Depth != run.Depth || requested.Text != task {
			return storage.ErrChildAdmissionConflict
		}
		return nil
	}
	payload, err := json.Marshal(map[string]any{
		"parent_run_id": string(parentRunID), "depth": run.Depth, "text": task,
	})
	if err != nil {
		return err
	}
	_, err = s.appendRunEventLocked(ctx, domain.RunEvent{
		RunID: run.ID, Type: domain.EventChildRequested, CreatedAt: time.Now().UnixMilli(),
		PayloadVersion: 1, Payload: payload,
	}, false)
	return err
}

type childRequestedPayload struct {
	ParentRunID string `json:"parent_run_id"`
	Depth       int    `json:"depth"`
	Text        string `json:"text"`
}

func (s *Service) readChildLifecycle(ctx context.Context, runID domain.RunID) (*childRequestedPayload, bool, error) {
	iterator, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = iterator.Close() }()
	var requested *childRequestedPayload
	var started bool
	for iterator.Next() {
		event := iterator.Value().Event
		switch event.Type {
		case domain.EventChildRequested:
			if requested != nil {
				return nil, false, storage.ErrChildAdmissionConflict
			}
			var payload childRequestedPayload
			if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.ParentRunID == "" || payload.Depth < 1 || strings.TrimSpace(payload.Text) == "" {
				return nil, false, storage.ErrChildAdmissionConflict
			}
			requested = &payload
		case domain.EventChildStarted:
			if started {
				return nil, false, storage.ErrChildAdmissionConflict
			}
			started = true
		}
	}
	if err := iterator.Err(); err != nil {
		return nil, false, err
	}
	return requested, started, nil
}

func activationTask(ctx context.Context, messages storage.MessageStore, sessionID domain.SessionID, runID domain.RunID) (string, error) {
	rows, err := messages.ListMessages(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("runtime: list child activation task: %w", err)
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].RunID == runID && rows[i].Role == domain.RoleUser {
			if strings.TrimSpace(rows[i].Content) == "" {
				return "", errors.New("runtime: child activation task message is empty")
			}
			return rows[i].Content, nil
		}
	}
	return "", errors.New("runtime: admitted child activation task message is missing")
}

func (s *Service) failAdmittedChildActivation(ctx context.Context, run domain.Run, cause error) error {
	mapper := newEventMapper(run.ID, s.engine.cfg.MaxEventPayloadBytes)
	mapper.setRunScope(s.deps.TenantID, "", string(run.SessionID))
	providerName, modelID := s.CurrentModel()
	mapper.setUsageRoutes(providerName, modelID, s.engine.cfg.SummaryModelID)
	s.emitTerminal(ctx, mapper, s.terminalEvent(ctx, mapper, cause))
	return cause
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (s *Service) runMessagesForChild(ctx context.Context, task string, pending []domain.ChildMailboxMessage, eng *Engine) ([]*schema.Message, tools.Selection, ContextStats, []domain.ChildMailboxMessage, error) {
	selection := eng.SelectTools()
	reservedPromptBytes, err := promptInstructionReservation(ctx)
	if err != nil {
		return nil, selection, ContextStats{}, nil, err
	}
	limit := maxChildActivationContextBytes
	if eng.cfg.MaxContextBytes > 0 && eng.cfg.MaxContextBytes < limit {
		limit = eng.cfg.MaxContextBytes
	}
	messages := []*schema.Message{schema.UserMessage(task)}
	if projectedContextBytes(messages)+reservedPromptBytes > limit {
		return nil, selection, ContextStats{}, nil, fmt.Errorf("%w: child task and instruction require more than %d bytes", ErrContextBudgetExceeded, limit)
	}
	included := make([]domain.ChildMailboxMessage, 0, len(pending))
	for _, item := range pending {
		content := fmt.Sprintf("Direct message from parent (%s):\n%s", item.ID, string(item.Body))
		candidate := append(append([]*schema.Message(nil), messages...), schema.UserMessage(content))
		if projectedContextBytes(candidate)+reservedPromptBytes > limit {
			break
		}
		messages = candidate
		included = append(included, item)
	}
	return messages, selection, ContextStats{}, included, nil
}

func (s *Service) beginChildMailboxSafePoint(ctx context.Context, state *childActivationDrive) error {
	for _, item := range state.included {
		if _, _, err := s.RecordChildMessageReceipt(ctx, state.binding.ChildSessionID, state.binding.ActivationRunID, item.ID, domain.ChildMessageReceiptInProgress); err != nil {
			return fmt.Errorf("record child message %s in-progress receipt: %w", item.ID, err)
		}
	}
	return nil
}

func (s *Service) consumeChildMailboxSafePoint(ctx context.Context, state *childActivationDrive) error {
	for _, item := range state.included {
		if _, _, err := s.RecordChildMessageReceipt(ctx, state.binding.ChildSessionID, state.binding.ActivationRunID, item.ID, domain.ChildMessageReceiptConsumed); err != nil {
			return fmt.Errorf("consume child message %s: %w", item.ID, err)
		}
	}
	return nil
}

func (s *Service) failUnsettledChildMail(state *childActivationDrive) {
	if state == nil || state.binding.ChildSessionID == "" || s.deps.ChildMailbox == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), terminalPersistTimeout)
	defer cancel()
	run, runErr := s.deps.Runs.GetRun(ctx, state.binding.ActivationRunID)
	if runErr == nil && run.Status == domain.RunActive {
		s.mu.Lock()
		_, pending := s.pending[state.binding.ActivationRunID]
		s.mu.Unlock()
		if pending {
			return
		}
	}
	for _, item := range state.included {
		receipt, err := s.deps.ChildMailbox.GetChildMessageReceipt(ctx, state.binding.ChildSessionID, item.ID, state.binding.ActivationRunID)
		if err != nil || receipt.State != domain.ChildMessageReceiptInProgress {
			continue
		}
		if _, _, err := s.deps.ChildMailbox.RecordChildMessageReceipt(ctx, domain.ChildMessageReceipt{
			ChildSessionID: state.binding.ChildSessionID, MessageID: item.ID,
			ConsumerRunID: state.binding.ActivationRunID, State: domain.ChildMessageReceiptFailed,
		}); err != nil {
			// The original inbox row and in-progress receipt remain durable for
			// recovery if this bounded settlement attempt cannot be committed.
			continue
		}
	}
}
