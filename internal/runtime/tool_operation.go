package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/compose"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

var (
	ErrToolOperationUnavailable = errors.New("runtime: durable tool operation storage is unavailable")
	ErrToolOperationUnknown     = errors.New("runtime: tool operation outcome is unknown; automatic replay is blocked")
)

type toolOperationCoordinator interface {
	Lookup(context.Context, string, string, []byte) (domain.ToolOperation, bool, error)
	Admit(context.Context, string, string, []byte, []byte, []byte) (domain.ToolOperation, error)
	Execute(context.Context, domain.ToolOperation, func(context.Context) (domain.ToolOperationOutcome, error)) (string, error)
}

type toolOperationCoordinatorKey struct{}

func withToolOperationCoordinator(ctx context.Context, coordinator toolOperationCoordinator) context.Context {
	return context.WithValue(ctx, toolOperationCoordinatorKey{}, coordinator)
}

func toolOperationCoordinatorFromContext(ctx context.Context) toolOperationCoordinator {
	coordinator, _ := ctx.Value(toolOperationCoordinatorKey{}).(toolOperationCoordinator)
	return coordinator
}

type serviceToolOperationCoordinator struct {
	service   *Service
	runID     domain.RunID
	sessionID domain.SessionID
}

type toolOperationFlight struct {
	done   chan struct{}
	result string
	err    error
}

func (s *Service) newToolOperationCoordinator(runID domain.RunID, sessionID domain.SessionID) toolOperationCoordinator {
	return serviceToolOperationCoordinator{service: s, runID: runID, sessionID: sessionID}
}

// isModelWorkTool reports whether a tool's effect is a journaled Work
// mutation. Those tools already deduplicate retried calls inside CommitWork
// by their caller-stable request identity, so a durable tool operation would
// only shadow the bookkeeping the work stream provides.
func isModelWorkTool(name string) bool {
	switch name {
	case tools.EnterPlanModeName, tools.SubmitPlanName, tools.GetGoalName,
		tools.CreateGoalName, tools.ReportGoalName:
		return true
	}
	return false
}

func (c serviceToolOperationCoordinator) Lookup(ctx context.Context, operationID, toolName string, request []byte) (domain.ToolOperation, bool, error) {
	if c.service == nil || c.service.deps.ToolOperations == nil {
		return domain.ToolOperation{}, false, ErrToolOperationUnavailable
	}
	if strings.TrimSpace(operationID) == "" || strings.TrimSpace(toolName) == "" {
		return domain.ToolOperation{}, false, errors.New("runtime: tool call is missing its stable operation identity")
	}
	op, err := c.service.deps.ToolOperations.GetToolOperation(ctx, c.runID, operationID)
	if errors.Is(err, storage.ErrNotFound) {
		return domain.ToolOperation{}, false, nil
	}
	if err != nil {
		return domain.ToolOperation{}, false, fmt.Errorf("runtime: read tool operation: %w", err)
	}
	if err := storage.ValidateToolOperationSource(op); err != nil {
		return domain.ToolOperation{}, false, err
	}
	if op.ToolName != toolName || !bytes.Equal(op.RequestArguments, request) {
		return domain.ToolOperation{}, false, storage.ErrToolOperationConflict
	}
	return op, true, nil
}

func (c serviceToolOperationCoordinator) Admit(ctx context.Context, operationID, toolName string, request, middlewareInput, effective []byte) (domain.ToolOperation, error) {
	if c.service == nil || c.service.deps.ToolOperations == nil {
		return domain.ToolOperation{}, ErrToolOperationUnavailable
	}
	if strings.TrimSpace(operationID) == "" || strings.TrimSpace(toolName) == "" {
		return domain.ToolOperation{}, errors.New("runtime: tool call is missing its stable operation identity")
	}
	if err := c.ensureRunCanExecute(ctx); err != nil {
		return domain.ToolOperation{}, err
	}
	now := time.Now().UnixMilli()
	op := domain.ToolOperation{
		RunID: c.runID, OperationID: operationID, ToolName: toolName,
		RequestArguments: append([]byte(nil), request...), MiddlewareInputArguments: append([]byte(nil), middlewareInput...),
		EffectiveArguments: append([]byte(nil), effective...),
		CreatedAt:          now, UpdatedAt: now,
	}
	if err := storage.ValidateToolOperationAdmission(op); err != nil {
		return domain.ToolOperation{}, err
	}

	eventOp := op
	eventOp.State = domain.ToolOperationAdmitted
	if err := c.checkOperationEventSize(eventOp); err != nil {
		return domain.ToolOperation{}, err
	}
	stored, created, event, err := c.service.deps.ToolOperations.AdmitToolOperation(ctx, op)
	if err != nil {
		return domain.ToolOperation{}, fmt.Errorf("runtime: admit tool operation: %w", err)
	}
	if created {
		c.publish(event)
	}
	return stored, nil
}

func (c serviceToolOperationCoordinator) Execute(ctx context.Context, op domain.ToolOperation, invoke func(context.Context) (domain.ToolOperationOutcome, error)) (string, error) {
	if c.service == nil || c.service.deps.ToolOperations == nil {
		return "", ErrToolOperationUnavailable
	}
	if err := c.ensureRunCanExecute(ctx); err != nil {
		return "", err
	}
	stored, err := c.service.deps.ToolOperations.GetToolOperation(ctx, c.runID, op.OperationID)
	if err != nil {
		return "", fmt.Errorf("runtime: read tool operation before claim: %w", err)
	}
	if err := storage.ValidateToolOperationSource(stored); err != nil {
		return "", err
	}
	if !sameRuntimeToolOperationBinding(stored, op) {
		return "", storage.ErrToolOperationConflict
	}
	if stored.State == domain.ToolOperationCompleted {
		return toolOperationResolution(stored)
	}
	key := string(c.runID) + "\x00" + op.OperationID
	service := c.service
	service.operationMu.Lock()
	if flight := service.operationFlights[key]; flight != nil {
		service.operationMu.Unlock()
		return waitToolOperationFlight(ctx, flight)
	}
	if stored.State == domain.ToolOperationClaimed {
		service.operationMu.Unlock()
		return "", fmt.Errorf("%w: %s", ErrToolOperationUnknown, op.OperationID)
	}
	if stored.State != domain.ToolOperationAdmitted {
		service.operationMu.Unlock()
		return "", fmt.Errorf("runtime: invalid tool operation state %q", stored.State)
	}
	flight := &toolOperationFlight{done: make(chan struct{})}
	service.operationFlights[key] = flight
	service.operationMu.Unlock()

	finish := func(result string, invokeErr error) (string, error) {
		service.operationMu.Lock()
		flight.result = result
		flight.err = invokeErr
		if service.operationFlights[key] == flight {
			delete(service.operationFlights, key)
		}
		close(flight.done)
		service.operationMu.Unlock()
		return result, invokeErr
	}
	owner := newPrefixedID("op_owner_")
	claimed, acquired, claimEvent, err := service.deps.ToolOperations.ClaimToolOperation(ctx, c.runID, op.OperationID, owner)
	if err != nil {
		return finish("", fmt.Errorf("runtime: claim tool operation: %w", err))
	}
	if !acquired {
		if claimed.State == domain.ToolOperationCompleted {
			result, resolveErr := toolOperationResolution(claimed)
			return finish(result, resolveErr)
		}
		return finish("", fmt.Errorf("%w: %s", ErrToolOperationUnknown, op.OperationID))
	}
	c.publish(claimEvent)
	if err := c.ensureRunCanExecute(ctx); err != nil {
		// The durable claim remains as a fence. A cancelled/deleted Run does
		// not prove that no external effect happened during an interruption.
		return finish("", err)
	}
	outcome, invokeErr := invoke(ctx)
	var surfacedRefusal error
	if invokeErr != nil {
		if _, interrupted := compose.IsInterruptRerunError(invokeErr); interrupted {
			return finish("", invokeErr)
		}
		if ctx.Err() != nil || errors.Is(invokeErr, context.Canceled) || errors.Is(invokeErr, context.DeadlineExceeded) {
			return finish("", redactToolError(invokeErr))
		}
		if refusal, ok := asToolRefusal(invokeErr); ok {
			reason := tools.RedactSensitive(refusal.reason)
			failure := refusalFailure(refusal.classification, reason)
			outcome = outcomeWithFailure(refusalToolResult(op.ToolName, reason), failure)
			surfacedRefusal = invokeErr
			invokeErr = nil
		} else {
			outcome.Status, outcome.Reason, outcome.Effects = domain.ToolOutcomeFatal, "", domain.ToolEffectsUnknown
			outcome.Result, outcome.Failure = "", boundToolOperationFailure(invokeErr.Error())
		}
	}
	outcome.Result = tools.RedactSensitive(outcome.Result)
	outcome.Failure = boundToolOperationFailure(outcome.Failure)
	outcome.Result, outcome.Failure, err = fitToolOperationResolution(op, outcome.Result, outcome.Failure, service.engine)
	if err != nil {
		return finish("", err)
	}
	completed, completeEvent, err := service.deps.ToolOperations.CompleteToolOperation(ctx, c.runID, op.OperationID, owner, outcome)
	if err != nil {
		return finish("", fmt.Errorf("runtime: persist tool operation completion: %w", err))
	}
	if completeEvent.Seq != 0 {
		c.publish(completeEvent)
	}
	if surfacedRefusal != nil && completed.State == domain.ToolOperationCompleted && completed.OutcomeStatus == domain.ToolOutcomeRefused {
		// The refusal text is durable as the operation result; still surface
		// the typed refusal so the caller runs invocation-failure marking and
		// governance journaling for this attempt.
		refusal, _ := asToolRefusal(surfacedRefusal)
		copy := *refusal
		result := completed.Result
		copy.durableResult = &result
		return finish("", &copy)
	}
	if invokeErr != nil {
		return finish("", redactToolError(invokeErr))
	}
	resolved, resolveErr := toolOperationResolution(completed)
	return finish(resolved, resolveErr)
}

func (c serviceToolOperationCoordinator) ensureRunCanExecute(ctx context.Context) error {
	if c.service == nil || c.service.deps.Runs == nil {
		return errors.New("runtime: tool operation run store is unavailable")
	}
	if c.service.sessionDeleted(c.sessionID) {
		return storage.ErrNotFound
	}
	run, err := c.service.deps.Runs.GetRun(ctx, c.runID)
	if err != nil {
		return err
	}
	if run.Status.Terminal() {
		return storage.ErrRunClosed
	}
	return nil
}

func (c serviceToolOperationCoordinator) checkOperationEventSize(op domain.ToolOperation) error {
	if c.service.engine == nil || c.service.engine.cfg.MaxEventPayloadBytes <= 0 {
		return nil
	}
	event, err := storage.NewToolOperationEvent(op)
	if err != nil {
		return err
	}
	if len(event.Payload) > c.service.engine.cfg.MaxEventPayloadBytes {
		return fmt.Errorf("runtime: tool operation event payload is %d bytes; limit is %d", len(event.Payload), c.service.engine.cfg.MaxEventPayloadBytes)
	}
	return nil
}

func (c serviceToolOperationCoordinator) publish(event domain.RunEvent) {
	if event.Seq == 0 || c.service == nil || c.service.deps.Sink == nil || c.service.sessionDeleted(c.sessionID) {
		return
	}
	c.service.publish(context.Background(), event)
}

func sameRuntimeToolOperationBinding(a, b domain.ToolOperation) bool {
	return storage.SameToolOperationBinding(a, b)
}

func toolOperationResolution(op domain.ToolOperation) (string, error) {
	if op.State != domain.ToolOperationCompleted {
		return "", fmt.Errorf("runtime: tool operation %s is not complete", op.OperationID)
	}
	if err := storage.ValidateToolOperationSource(op); err != nil {
		return "", err
	}
	if err := storage.ValidateToolOperationOutcome(op.Outcome()); err != nil {
		return "", err
	}
	if op.OutcomeStatus == domain.ToolOutcomeFatal {
		return "", errors.New(op.Failure)
	}
	return op.Result, nil
}

func waitToolOperationFlight(ctx context.Context, flight *toolOperationFlight) (string, error) {
	select {
	case <-flight.done:
		return flight.result, flight.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func boundToolOperationFailure(failure string) string {
	failure = tools.RedactSensitive(strings.TrimSpace(failure))
	if len(failure) > toolFailureDiagnosticMaxBytes {
		failure = truncateUTF8(failure, toolFailureDiagnosticMaxBytes)
	}
	return failure
}

func fitToolOperationResolution(op domain.ToolOperation, result, failure string, engine *Engine) (string, string, error) {
	if engine == nil || engine.cfg.MaxEventPayloadBytes <= 0 {
		return result, failure, nil
	}
	for {
		completed := op
		completed.State = domain.ToolOperationCompleted
		completed.Result = result
		completed.Failure = failure
		completed.UpdatedAt = time.Now().UnixMilli()
		event, err := storage.NewToolOperationEvent(completed)
		if err != nil {
			return "", "", err
		}
		if len(event.Payload) <= engine.cfg.MaxEventPayloadBytes {
			return result, failure, nil
		}
		if result != "" {
			result = truncateUTF8(result, len(result)/2)
			continue
		}
		if failure != "" {
			failure = truncateUTF8(failure, len(failure)/2)
			continue
		}
		return "", "", fmt.Errorf("runtime: tool operation completion event exceeds %d bytes", engine.cfg.MaxEventPayloadBytes)
	}
}
