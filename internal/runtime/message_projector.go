package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/journalview"
	"agent-vivy/internal/storage"
)

// projectRunMessages rebuilds the model-visible assistant/tool transcript
// from the Journal. Deterministic ids plus AppendMessageIfAbsent make retries
// harmless; semantic matching adopts legacy random-id projections without
// duplicating them.
func (s *Service) projectRunMessages(ctx context.Context, sessionID domain.SessionID, runID domain.RunID) error {
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	return s.projectRunMessagesLocked(ctx, sessionID, runID)
}

func (s *Service) projectRunMessagesLocked(ctx context.Context, sessionID domain.SessionID, runID domain.RunID) error {
	if s.sessionDeleted(sessionID) {
		return nil
	}
	desired, err := s.projectedMessages(ctx, sessionID, runID)
	if err != nil {
		return err
	}
	if len(desired) == 0 {
		return nil
	}
	existing, err := s.deps.Messages.ListMessages(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("list existing message projection: %w", err)
	}
	_, err = s.appendProjectedMessages(ctx, runID, desired, existing)
	return err
}

func (s *Service) appendProjectedMessages(ctx context.Context, runID domain.RunID, desired, existing []domain.Message) ([]domain.Message, error) {
	claimed := make([]bool, len(existing))
	for _, want := range desired {
		matched := false
		for i, got := range existing {
			if claimed[i] || got.RunID != runID {
				continue
			}
			if got.ID == want.ID {
				if !storage.SameProjectedMessage(got, want) {
					return existing, fmt.Errorf("projected message %s: %w", want.ID, storage.ErrProjectionConflict)
				}
				claimed[i] = true
				matched = true
				break
			}
			if sameProjectedMessage(got, want) {
				claimed[i] = true
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		inserted, err := s.deps.Messages.AppendMessageIfAbsent(ctx, want)
		if err != nil {
			return existing, fmt.Errorf("append Journal message projection: %w", err)
		}
		if inserted {
			existing = append(existing, want)
			claimed = append(claimed, true)
		}
	}
	return existing, nil
}

func (s *Service) projectedMessages(ctx context.Context, sessionID domain.SessionID, runID domain.RunID) ([]domain.Message, error) {
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		return nil, fmt.Errorf("replay Journal for message projection: %w", err)
	}
	defer it.Close()
	var out []domain.Message
	// Assistant text reduction is the shared journalview reducer — the
	// same semantics the channel task projection uses (design §7.1).
	reducer := journalview.NewTextReducer(runID)
	for it.Next() {
		re := it.Value().Event
		switch re.Type {
		case domain.EventRunStarted:
			// `!!` no-context shell runs journal for the transcript only;
			// their tool rows must never enter the model feed.
			var p payloadRunStarted
			if err := json.Unmarshal(re.Payload, &p); err != nil {
				return nil, fmt.Errorf("decode run.started seq %d: %w", re.Seq, err)
			}
			if p.NoContext {
				return nil, nil
			}
		case domain.EventToolRequested:
			segments, err := reducer.Apply(re)
			if err != nil {
				return nil, err
			}
			for _, segment := range segments {
				out = append(out, domain.Message{
					ID: segment.ID, SessionID: sessionID, RunID: runID,
					Role: domain.RoleAssistant, CreatedAt: re.CreatedAt, Content: segment.Text,
				})
			}
			var p payloadToolRequested
			if err := json.Unmarshal(re.Payload, &p); err != nil {
				return nil, fmt.Errorf("decode tool.requested seq %d: %w", re.Seq, err)
			}
			args, err := json.Marshal(p.Args)
			if err != nil {
				return nil, fmt.Errorf("encode tool.requested seq %d: %w", re.Seq, err)
			}
			out = append(out, domain.Message{
				ID: projectedMessageID(runID, re.Seq, "1"), SessionID: sessionID, RunID: runID,
				Role: domain.RoleAssistant, CreatedAt: re.CreatedAt,
				ToolCallID: p.ToolCallID, ToolName: p.ToolName, ToolArgs: args,
			})
		case domain.EventToolFinished:
			var p payloadToolFinished
			if err := json.Unmarshal(re.Payload, &p); err != nil {
				return nil, fmt.Errorf("decode tool.finished seq %d: %w", re.Seq, err)
			}
			content := p.Result
			if p.Error != "" {
				content = p.Error
			}
			out = append(out, domain.Message{
				ID: projectedMessageID(runID, re.Seq, "0"), SessionID: sessionID, RunID: runID,
				Role: domain.RoleTool, CreatedAt: re.CreatedAt, Content: content,
				ToolCallID: p.ToolCallID, ToolName: p.ToolName,
			})
		default:
			segments, err := reducer.Apply(re)
			if err != nil {
				return nil, err
			}
			for _, segment := range segments {
				out = append(out, domain.Message{
					ID: segment.ID, SessionID: sessionID, RunID: runID,
					Role: domain.RoleAssistant, CreatedAt: re.CreatedAt, Content: segment.Text,
				})
			}
		}
	}
	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("iterate Journal message projection: %w", err)
	}
	return out, nil
}

// errUnsupportedCompletedVersion marks model.completed events that predate
// the v2 hash-commit shape. Reconcile must not hard-fail a whole session on
// them: the legacy run is skipped and whatever projection was persisted by
// the old writer stays readable. Fresh runs are v2-only, so the run
// completion path still fails closed on this error. The rule itself lives
// in internal/journalview so both projections validate identically.
var errUnsupportedCompletedVersion = journalview.ErrUnsupportedCompletedVersion

func completedProjectionContent(re domain.RunEvent, deltas string) (string, error) {
	return journalview.CompletedProjectionContent(re, deltas)
}

func projectedTextMessage(sessionID domain.SessionID, runID domain.RunID, re domain.RunEvent, slot, content string) domain.Message {
	return domain.Message{
		ID: projectedMessageID(runID, re.Seq, slot), SessionID: sessionID, RunID: runID,
		Role: domain.RoleAssistant, CreatedAt: re.CreatedAt, Content: content,
	}
}

func projectedMessageID(runID domain.RunID, seq domain.EventSeq, slot string) string {
	return journalview.TextMessageID(runID, seq, slot)
}

func sameProjectedMessage(a, b domain.Message) bool {
	return a.RunID == b.RunID && a.Role == b.Role && a.Content == b.Content &&
		a.ToolCallID == b.ToolCallID && a.ToolName == b.ToolName && bytes.Equal(a.ToolArgs, b.ToolArgs)
}

func (s *Service) reconcileSessionMessageProjection(ctx context.Context, sessionID domain.SessionID) error {
	s.projectionMu.Lock()
	defer s.projectionMu.Unlock()
	if s.sessionDeleted(sessionID) {
		return storage.ErrNotFound
	}
	runs, err := s.deps.Runs.ListRunsBySession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("list runs for message projection: %w", err)
	}
	existing, err := s.deps.Messages.ListMessages(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("list existing message projection: %w", err)
	}
	for _, run := range runs {
		desired, err := s.projectedMessages(ctx, sessionID, run.ID)
		if errors.Is(err, errUnsupportedCompletedVersion) {
			continue
		}
		if err != nil {
			return err
		}
		existing, err = s.appendProjectedMessages(ctx, run.ID, desired, existing)
		if err != nil {
			return err
		}
	}
	return nil
}

// ReconcileSessionMessages repairs the derived MessageStore view from the
// durable Journal. Read surfaces call it before returning history so a crash
// between Journal commit and projection cannot leave a completed turn hidden.
func (s *Service) ReconcileSessionMessages(ctx context.Context, sessionID domain.SessionID) error {
	return s.reconcileSessionMessageProjection(ctx, sessionID)
}
