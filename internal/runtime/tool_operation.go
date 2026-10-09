package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	Execute(context.Context, domain.ToolOperation, func(context.Context) (string, error)) (string, error)
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
// isNotebookTool marks the notebook-owned note tools whose results carry
// notebook provenance and the automatic-ingest exclusion.
func isNotebookTool(name string) bool {
	switch name {
	case tools.ListNotesName, tools.ReadNoteName, tools.WriteNoteName:
		return true
	}
	return false
}

func isModelWorkTool(name string) bool {
	switch name {
	case tools.EnterPlanModeName, tools.SubmitPlanName, tools.GetGoalName,
		tools.CreateGoalName, tools.ReportGoalName:
		return true
	}
	return false
}

func operationDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
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
	if op.ToolName != toolName || op.RequestDigest != operationDigest(request) {
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
		RequestDigest: operationDigest(request), MiddlewareInputArguments: append([]byte(nil), middlewareInput...),
		ArgumentsDigest: operationDigest(effective), EffectiveArguments: append([]byte(nil), effective...),
		CreatedAt: now, UpdatedAt: now,
	}
	if isNotebookTool(toolName) {
		// N2: notebook-owned tool results are excluded from automatic
		// BML/cognitive ingestion and compaction summaries.
		op.ContentOrigin = domain.ContentOriginNotebook
		op.ExcludeAutomaticIngest = true
	}
	if err := storage.ValidateToolOperationAdmission(domain.ToolOperation{
		RunID: op.RunID, OperationID: op.OperationID, ToolName: op.ToolName,
		RequestDigest: op.RequestDigest, MiddlewareInputArguments: op.MiddlewareInputArguments,
		ArgumentsDigest: op.ArgumentsDigest, EffectiveArguments: op.EffectiveArguments,
	}); err != nil {
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

func (c serviceToolOperationCoordinator) Execute(ctx context.Context, op domain.ToolOperation, invoke func(context.Context) (string, error)) (string, error) {
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
	result, invokeErr := invoke(ctx)
	var surfacedRefusal error
	if invokeErr != nil {
		// A tool interrupt (approval, question, plan review) suspends the run;
		// it is not an operation outcome and must keep its signal type so the
		// ToolsNode can surface it as an interrupt instead of a failure.
		if _, interrupted := compose.IsInterruptRerunError(invokeErr); interrupted {
			return finish("", invokeErr)
		}
		// A governance refusal is a model-visible outcome, not an operation
		// failure: record the refusal text as the result so the run continues
		// and a resumed run replays the same response.
		if refusal, isRefusal := asToolRefusal(invokeErr); isRefusal {
			result = refusalToolResult(op.ToolName, publicRefusalReason(refusal.cause))
			surfacedRefusal = invokeErr
			invokeErr = nil
		}
	}
	failure := ""
	if invokeErr != nil {
		failure = boundToolOperationFailure(invokeErr.Error())
		result = ""
	}
	result, failure, err = fitToolOperationResolution(op, result, failure, service.engine)
	if err != nil {
		return finish("", err)
	}
	completed, completeEvent, err := service.deps.ToolOperations.CompleteToolOperation(ctx, c.runID, op.OperationID, owner, result, failure)
	if err != nil {
		// Keep the durable claim unresolved. Returning an ordinary result here
		// would let Eino advance past an outcome the Journal cannot recover.
		return finish("", fmt.Errorf("runtime: persist tool operation completion: %w", err))
	}
	if completeEvent.Seq != 0 {
		c.publish(completeEvent)
	}
	if surfacedRefusal != nil && completed.State == domain.ToolOperationCompleted && completed.Failure == "" {
		// The refusal text is durable as the operation result; still surface
		// the typed refusal so the caller runs invocation-failure marking and
		// governance journaling for this attempt.
		return finish("", surfacedRefusal)
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
	return a.RunID == b.RunID && a.OperationID == b.OperationID && a.ToolName == b.ToolName &&
		a.RequestDigest == b.RequestDigest && a.ArgumentsDigest == b.ArgumentsDigest &&
		string(a.MiddlewareInputArguments) == string(b.MiddlewareInputArguments) &&
		string(a.EffectiveArguments) == string(b.EffectiveArguments)
}

func toolOperationResolution(op domain.ToolOperation) (string, error) {
	if op.State != domain.ToolOperationCompleted {
		return "", fmt.Errorf("runtime: tool operation %s is not complete", op.OperationID)
	}
	if op.Failure != "" {
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
	failure = strings.TrimSpace(failure)
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
