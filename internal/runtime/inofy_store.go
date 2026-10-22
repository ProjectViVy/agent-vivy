package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	inofy "github.com/ProjectViVy/inofy"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// inofyRunStore adapts the Core Storage WorkflowStepStore to INOFY's
// host-controlled RunStore boundary. Every Commit is one SQL transaction;
// protected outputs, native checkpoints and exact private claims are stored
// with the receipt and projection inside that transaction.
type inofyRunStore struct {
	steps storage.WorkflowStepStore
	// publish fans each committed event out to live subscribers after the
	// journal accepts the step; the journal stays the authority and the
	// bus-only events are re-fetched from it after any drop.
	publish func(ctx context.Context, ev domain.RunEvent)
}

func newINOFYRunStore(engine storage.Engine) *inofyRunStore {
	return &inofyRunStore{steps: engine}
}

func (s *inofyRunStore) Commit(ctx context.Context, ref inofy.ExecutionRef, change inofy.RunCommit) (inofy.Receipt, error) {
	runID := domain.RunID(ref.RunID)

	results := make([]storage.WorkflowStepResult, 0, len(change.Results))
	for _, r := range change.Results {
		results = append(results, storage.WorkflowStepResult{Path: r.Path, Attempt: r.Attempt, Output: append([]byte(nil), r.Output...), Bytes: int64(len(r.Output))})
	}
	var checkpoint *storage.WorkflowStepCheckpoint
	if change.Checkpoint != nil {
		raw, err := json.Marshal(change.Checkpoint)
		if err != nil {
			return inofy.Receipt{}, inofyStoreError(err, "encode checkpoint")
		}
		checkpoint = &storage.WorkflowStepCheckpoint{Envelope: raw}
	}
	events := make([]storage.WorkflowStepEvent, 0, len(change.Events))
	now := time.Now().UnixMilli()
	for _, e := range change.Events {
		payload, err := inofyEventPayload(e)
		if e.Kind == inofy.EventRunStarted || e.Kind == inofy.EventRunAdmitted {
			var projection map[string]any
			if err == nil {
				err = json.Unmarshal(payload, &projection)
			}
			if err == nil {
				projection["program_id"] = ref.ProgramID
				payload, err = json.Marshal(projection)
			}
		}
		if err != nil {
			return inofy.Receipt{}, err
		}
		events = append(events, storage.WorkflowStepEvent{
			Type:           inofyEventType(e.Kind),
			CreatedAt:      now,
			PayloadVersion: 1,
			Payload:        payload,
		})
	}

	var resumeAnswers []byte
	if change.ResumeKey != "" || change.ResumeAnswers != nil {
		var err error
		resumeAnswers, err = json.Marshal(change.ResumeAnswers)
		if err != nil {
			return inofy.Receipt{}, inofyStoreError(err, "encode private resume answers")
		}
	}
	receipt, err := s.steps.CommitWorkflowStep(ctx, storage.WorkflowStepCommit{
		RunID:         runID,
		CommitID:      change.CommitID,
		AdmittedInput: append(json.RawMessage(nil), change.AdmittedInput...),
		ResumeKey:     change.ResumeKey, ResumeAnswersJSON: resumeAnswers,
		Epoch:         ref.Epoch,
		ProgramID:     ref.ProgramID,
		HostBindingID: ref.HostBindingID,
		Expected:      storage.WorkflowStepStatus(change.Transition.Expected),
		Target:        storage.WorkflowStepStatus(change.Transition.Target),
		Events:        events,
		Results:       results,
		Checkpoint:    checkpoint,
	})
	if err != nil {
		return inofy.Receipt{}, mapINOFYStoreError(err)
	}
	if s.publish != nil && !receipt.Replayed {
		seq := receipt.FirstSequence
		for _, e := range events {
			s.publish(ctx, domain.RunEvent{
				RunID:          runID,
				Seq:            seq,
				Type:           e.Type,
				CreatedAt:      e.CreatedAt,
				PayloadVersion: e.PayloadVersion,
				Payload:        e.Payload,
			})
			seq++
		}
	}
	return inofy.Receipt{
		FirstSequence:      uint64(receipt.FirstSequence),
		LastSequence:       uint64(receipt.LastSequence),
		ProjectionRevision: receipt.Revision,
	}, nil
}

func (s *inofyRunStore) Load(ctx context.Context, runID string) (inofy.RecoveryState, error) {
	// INOFY must read the admitted state before it can classify a cancelled
	// Run and commit that known outcome through its native settlement path.
	// Keep this read bounded; node execution still receives the cancelled ctx.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistTimeout)
	defer cancel()
	state, err := s.steps.LoadWorkflowStep(ctx, domain.RunID(runID))
	if err != nil {
		return inofy.RecoveryState{}, mapINOFYStoreError(err)
	}
	st := inofy.RecoveryState{
		Ref: inofy.ExecutionRef{
			RunID:         runID,
			ProgramID:     state.Revision.ProgramID,
			HostBindingID: state.Revision.HostBindingID,
		},
		Status:        inofy.RunAdmitted,
		AdmittedInput: append(json.RawMessage(nil), state.Revision.InputJSON...),
	}
	if len(state.Revision.EffectiveLimits) > 0 {
		if err := json.Unmarshal(state.Revision.EffectiveLimits, &st.Limits); err != nil {
			return inofy.RecoveryState{}, inofyStoreError(err, "decode limits")
		}
	}
	if state.Projection == nil {
		return st, nil
	}
	p := state.Projection
	st.Ref.Epoch = p.Epoch
	st.Status = inofy.RunStatus(p.Status)
	if len(p.UsageJSON) > 0 {
		if err := json.Unmarshal(p.UsageJSON, &st.Usage); err != nil {
			return inofy.RecoveryState{}, inofyStoreError(err, "decode usage")
		}
	}
	if len(p.CheckpointJSON) > 0 {
		var envelope inofy.CheckpointEnvelope
		if err := json.Unmarshal(p.CheckpointJSON, &envelope); err != nil {
			return inofy.RecoveryState{}, inofyStoreError(err, "decode checkpoint envelope")
		}
		st.LatestCheckpoint = &envelope
	}
	if len(p.WaitsJSON) > 0 {
		if err := json.Unmarshal(p.WaitsJSON, &st.Waits); err != nil {
			return inofy.RecoveryState{}, inofyStoreError(err, "decode waits")
		}
	}
	if len(p.InterruptsJSON) > 0 {
		if err := json.Unmarshal(p.InterruptsJSON, &st.Interrupts); err != nil {
			return inofy.RecoveryState{}, inofyStoreError(err, "decode interrupts")
		}
	}
	if len(p.GatesJSON) > 0 {
		if err := json.Unmarshal(p.GatesJSON, &st.Gates); err != nil {
			return inofy.RecoveryState{}, inofyStoreError(err, "decode gates")
		}
	}
	if len(p.UnresolvedJSON) > 0 {
		var ops []workflowStepOp
		if err := json.Unmarshal(p.UnresolvedJSON, &ops); err != nil {
			return inofy.RecoveryState{}, inofyStoreError(err, "decode unresolved")
		}
		for _, op := range ops {
			st.UnresolvedOperations = append(st.UnresolvedOperations, inofy.OperationRef{
				OperationKey: op.OperationKey,
				Path:         op.Path,
				Attempt:      op.Attempt,
			})
		}
	}
	st.ResumeKey = p.ResumeKey
	if len(p.ResumeAnswersJSON) > 0 {
		if err := json.Unmarshal(p.ResumeAnswersJSON, &st.ResumeAnswers); err != nil {
			return inofy.RecoveryState{}, inofyStoreError(err, "decode private resume answers")
		}
	}
	return st, nil
}

type workflowStepOp struct {
	OperationKey string `json:"operation_key"`
	Path         string `json:"path"`
	Attempt      int    `json:"attempt"`
}

func inofyEventType(kind inofy.EventKind) domain.EventType {
	switch kind {
	case inofy.EventRunAdmitted:
		return domain.EventWorkflowAdmitted
	case inofy.EventRunStarted:
		return domain.EventWorkflowStarted
	case inofy.EventRunWaiting:
		return domain.EventWorkflowWaiting
	case inofy.EventRunResumed:
		return domain.EventWorkflowResumed
	case inofy.EventRunRecoveryRequired:
		return domain.EventWorkflowRecoveryRequired
	case inofy.EventRunSucceeded:
		return domain.EventRunCompleted
	case inofy.EventRunFailed:
		return domain.EventRunFailed
	case inofy.EventRunCancelled:
		return domain.EventRunCancelled
	case inofy.EventNodeStarted:
		return domain.EventWorkflowNodeStarted
	case inofy.EventNodeAttempt:
		return domain.EventWorkflowNodeAttempt
	case inofy.EventNodeCompleted:
		return domain.EventWorkflowNodeCompleted
	case inofy.EventNodeFailed:
		return domain.EventWorkflowNodeFailed
	case inofy.EventNodeWait:
		return domain.EventWorkflowNodeWaiting
	case inofy.EventNodeDegraded:
		return domain.EventWorkflowNodeDegraded
	case inofy.EventSwitchDecision:
		return domain.EventWorkflowSwitchDecision
	case inofy.EventRepeatIteration:
		return domain.EventWorkflowRepeatIteration
	default:
		return domain.EventType("workflow." + string(kind))
	}
}

// inofyEventPayload builds the VIVY journal payload for one engine event,
// honoring schemas/events/payloads/*.json (node_key is the engine path).
func inofyEventPayload(e inofy.Event) ([]byte, error) {
	nodeFields := func(extra map[string]any) []byte {
		m := map[string]any{"node_key": e.Path}
		if e.Attempt > 0 {
			m["attempt"] = e.Attempt
		}
		for k, v := range extra {
			m[k] = v
		}
		raw, _ := json.Marshal(m)
		return raw
	}
	extra := map[string]any{}
	switch e.Kind {
	case inofy.EventNodeStarted, inofy.EventNodeAttempt, inofy.EventNodeDegraded:
		// Contract fields only.
	case inofy.EventNodeCompleted:

	case inofy.EventNodeWait:
		var wait struct {
			RequestID       string `json:"request_id"`
			Kind            string `json:"kind"`
			Prompt          string `json:"prompt"`
			ContinuationRef string `json:"continuation_ref"`
		}
		_ = json.Unmarshal(e.Data, &wait)
		extra["request_id"] = wait.RequestID
		if wait.Kind != "" {
			extra["kind"] = wait.Kind
		}
		if wait.Prompt != "" {
			extra["prompt"] = wait.Prompt
		}
		if wait.ContinuationRef != "" {
			extra["continuation_ref"] = wait.ContinuationRef
		}
	case inofy.EventNodeFailed:
		var failed struct {
			CauseCategory string `json:"cause_category"`
			Message       string `json:"message"`
		}
		_ = json.Unmarshal(e.Data, &failed)
		if failed.CauseCategory == "" {
			failed.CauseCategory = "engine"
		}
		if failed.Message == "" {
			failed.Message = "node failed"
		}
		extra["cause_category"] = failed.CauseCategory
		extra["message"] = failed.Message
	case inofy.EventSwitchDecision:
		if len(e.Data) > 0 {
			extra["decision"] = json.RawMessage(e.Data)
		}
	case inofy.EventRepeatIteration:
		if len(e.Data) > 0 {
			extra["record"] = json.RawMessage(e.Data)
		}
	default:
		if len(e.Data) > 0 {
			return e.Data, nil
		}
		return []byte(`{}`), nil
	}
	return nodeFields(extra), nil
}

func unresolvedKeyOf(path string, attempt int) string {
	return fmt.Sprintf("%s/%d", path, attempt)
}

func mapINOFYStoreError(err error) error {
	switch {
	case errors.Is(err, storage.ErrWorkflowStepIdempotency):
		return &inofy.Error{Code: inofy.ErrIdempotencyConflict, Message: err.Error(), Err: err}
	case errors.Is(err, storage.ErrStaleWriter):
		return &inofy.Error{Code: inofy.ErrStaleWriter, Message: err.Error(), Err: err}
	case errors.Is(err, storage.ErrWorkflowStepIdentity):
		return &inofy.Error{Code: inofy.ErrCheckpointIncompatible, Message: err.Error(), Err: err}
	case errors.Is(err, storage.ErrWorkflowStepState), errors.Is(err, storage.ErrWorkflowStepTerminal),
		errors.Is(err, storage.ErrNotFound):
		return &inofy.Error{Code: inofy.ErrRevisionConflict, Message: err.Error(), Err: err}
	default:
		return &inofy.Error{Code: inofy.ErrStorageFailed, Message: err.Error(), Err: err}
	}
}

func inofyStoreError(err error, op string) error {
	return &inofy.Error{Code: inofy.ErrStorageFailed, Message: fmt.Sprintf("%s: %v", op, err), Err: err}
}
