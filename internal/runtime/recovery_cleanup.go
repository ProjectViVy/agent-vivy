package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

const checkpointCleanupPrefix = "runtime:checkpoint-cleanup:v1:"

// This intent owns only unfinished cleanup, never Run or checkpoint truth.
// Keep it until all independent durable writes have succeeded. Recover is
// serialized and runs before admission, so replay can deduplicate events.
type checkpointCleanup struct {
	RunID         domain.RunID     `json:"run_id"`
	SessionID     domain.SessionID `json:"session_id"`
	ApprovalID    string           `json:"approval_id,omitempty"`
	QuestionID    string           `json:"question_id,omitempty"`
	EngineChanged bool             `json:"engine_changed"`
}

func (s *Service) failLostCheckpoint(ctx context.Context, run domain.Run, cpErr error, reason, approvalID, questionID string) error {
	if s.engine.cfg.Checkpoints == nil {
		return errors.New("runtime: cleanup requires durable blob storage")
	}
	blobs := s.engine.cfg.Checkpoints.blobs
	if _, ok := blobs.(storage.BlobPrefixLister); !ok {
		return errors.New("runtime: cleanup requires enumerable blob storage")
	}
	intent := checkpointCleanup{run.ID, run.SessionID, approvalID, questionID, errors.Is(cpErr, ErrCheckpointEngineVersionMismatch)}
	data, err := json.Marshal(intent)
	if err != nil {
		return err
	}
	if err = blobs.Put(ctx, checkpointCleanupPrefix+string(run.ID), data); err != nil {
		return fmt.Errorf("runtime: persist checkpoint cleanup: %w", err)
	}
	if err = s.finishCheckpointCleanup(ctx, intent); err != nil {
		return fmt.Errorf("runtime: checkpoint cleanup %s (%s): %w", run.ID, reason, err)
	}
	return nil
}

func (s *Service) recoverCheckpointCleanup(ctx context.Context) error {
	if s.engine.cfg.Checkpoints == nil {
		return nil
	}
	blobs := s.engine.cfg.Checkpoints.blobs
	lister, ok := blobs.(storage.BlobPrefixLister)
	if !ok {
		return nil
	} // Such embedders cannot create cleanup intents either.
	ids, err := lister.ListPrefix(ctx, checkpointCleanupPrefix)
	if err != nil {
		return fmt.Errorf("runtime: list checkpoint cleanup: %w", err)
	}
	for _, id := range ids {
		data, ok, err := blobs.Get(ctx, id)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		var intent checkpointCleanup
		if err = json.Unmarshal(data, &intent); err != nil {
			return fmt.Errorf("runtime: decode checkpoint cleanup: %w", err)
		}
		if intent.RunID == "" || intent.SessionID == "" || id != checkpointCleanupPrefix+string(intent.RunID) {
			return errors.New("runtime: invalid checkpoint cleanup identity")
		}
		if err = s.finishCheckpointCleanup(ctx, intent); err != nil {
			return fmt.Errorf("runtime: retry checkpoint cleanup %s: %w", intent.RunID, err)
		}
	}
	return nil
}

func (s *Service) finishCheckpointCleanup(ctx context.Context, intent checkpointCleanup) error {
	blobs := s.engine.cfg.Checkpoints.blobs
	run, err := s.deps.Runs.GetRun(ctx, intent.RunID)
	if errors.Is(err, storage.ErrNotFound) {
		return blobs.Delete(ctx, checkpointCleanupPrefix+string(intent.RunID))
	}
	if err != nil {
		return err
	}
	if run.SessionID != intent.SessionID {
		return errors.New("runtime: checkpoint cleanup session mismatch")
	}
	if err = s.settleCheckpointInteraction(ctx, intent); err != nil {
		return err
	}
	terminal, err := s.cleanupTerminal(ctx, intent.RunID)
	if err != nil {
		return err
	}
	if terminal == "" {
		message := "The run was interrupted by a server restart and could not be recovered. Please try again."
		if intent.EngineChanged {
			message = engineChangedMessage
		}
		m := newEventMapper(intent.RunID, 0)
		if err = s.emitTerminal(ctx, m, m.build(domain.EventRunFailed, payloadRunFailed{CauseCategory: causeInternalError, Message: message})); err != nil {
			return err
		}
	} else if status, ok := terminal.RunStatus(); ok {
		// A crash can follow the Journal commit but precede the status projection.
		if err = s.deps.Runs.SetRunStatus(ctx, intent.RunID, status); err != nil {
			return err
		}
	}
	if err = s.settlePlanReviewForRun(ctx, intent.SessionID, intent.RunID); err != nil {
		return err
	}
	return blobs.Delete(ctx, checkpointCleanupPrefix+string(intent.RunID))
}

func (s *Service) cleanupTerminal(ctx context.Context, runID domain.RunID) (domain.EventType, error) {
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		return "", err
	}
	defer it.Close()
	for it.Next() {
		typ := it.Value().Event.Type
		if _, ok := typ.RunStatus(); ok {
			return typ, nil
		}
	}
	return "", it.Err()
}

func (s *Service) settleCheckpointInteraction(ctx context.Context, intent checkpointCleanup) error {
	const actor = "local_user"
	if intent.ApprovalID != "" {
		a, err := s.deps.Approvals.GetApproval(ctx, intent.ApprovalID)
		if err != nil {
			return err
		}
		if a.RunID != intent.RunID {
			return errors.New("runtime: cleanup approval owner mismatch")
		}
		if a.Decision == domain.ApprovalPending {
			lifecycle, ok := s.deps.Approvals.(storage.ApprovalLifecycleStore)
			if !ok {
				return errors.New("runtime: approval cancellation unavailable")
			}
			if _, err = lifecycle.CancelApproval(ctx, a.ID, actor, lostCheckpointReason); err != nil {
				return err
			}
			a, err = s.deps.Approvals.GetApproval(ctx, a.ID)
			if err != nil {
				return err
			}
		}
		if a.Decision != domain.ApprovalCancelled {
			return ErrApprovalAlreadyDecided
		}
		if err := s.ensureCleanupEvent(ctx, intent.RunID, domain.EventToolApprovalCancelled, payloadApprovalCancelled{ApprovalID: a.ID, Actor: a.Actor, Reason: a.DecisionReason}); err != nil {
			return err
		}
	}
	if intent.QuestionID != "" {
		q, err := s.deps.Questions.GetQuestion(ctx, intent.QuestionID)
		if err != nil {
			return err
		}
		if q.RunID != intent.RunID {
			return errors.New("runtime: cleanup question owner mismatch")
		}
		if q.Status == domain.QuestionPending {
			lifecycle, ok := s.deps.Questions.(storage.QuestionLifecycleStore)
			if !ok {
				return errors.New("runtime: question cancellation unavailable")
			}
			if _, err = lifecycle.CancelQuestionWithMetadata(ctx, q.ID, actor, lostCheckpointReason); err != nil {
				return err
			}
			q, err = s.deps.Questions.GetQuestion(ctx, q.ID)
			if err != nil {
				return err
			}
		}
		if q.Status != domain.QuestionCancelled {
			return ErrQuestionAlreadyAnswered
		}
		return s.ensureCleanupEvent(ctx, intent.RunID, domain.EventUserQuestionCancelled, payloadQuestionCancelled{QuestionID: q.ID, Actor: q.Actor, Reason: q.DecisionReason})
	}
	return nil
}

func (s *Service) ensureCleanupEvent(ctx context.Context, runID domain.RunID, typ domain.EventType, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		return err
	}
	found := false
	for it.Next() {
		ev := it.Value().Event
		if ev.Type == typ && string(ev.Payload) == string(data) {
			found = true
		}
	}
	err = it.Err()
	closeErr := it.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if found {
		return nil
	}
	m := newEventMapper(runID, s.engine.cfg.MaxEventPayloadBytes)
	_, err = s.appendRunEvent(ctx, m.build(typ, payload), false)
	return err
}
