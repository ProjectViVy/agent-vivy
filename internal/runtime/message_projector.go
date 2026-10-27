package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/events"
	"agent-vivy/internal/storage"
)

// runProjectionState is one run's Journal→Message cursor. pending carries
// model deltas that no projection-flushing event has consumed yet, so a fold
// can resume between two tool calls without replaying the run from seq 0.
type runProjectionState struct {
	sessionID domain.SessionID
	seq       domain.EventSeq
	pending   string
	noContext bool
}

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
	prev, tracked := s.projectionStateLocked(runID)
	if tracked && prev.sessionID != sessionID {
		prev, tracked = runProjectionState{}, false
	}
	if !tracked {
		prev = runProjectionState{sessionID: sessionID}
	}
	desired, next, err := s.foldRunProjection(ctx, sessionID, runID, prev)
	if err != nil {
		return err
	}
	if len(desired) > 0 {
		if tracked {
			// The cursor proves every earlier derived row was already handed
			// to the store, so only new messages need writing.
			for _, want := range desired {
				if _, err := s.deps.Messages.AppendMessageIfAbsent(ctx, want); err != nil {
					return fmt.Errorf("append Journal message projection: %w", err)
				}
			}
		} else {
			// First projection in this process: the store may already hold
			// rows (restart, or legacy random ids), so reconcile semantically
			// once.
			existing, err := s.deps.Messages.ListMessages(ctx, sessionID)
			if err != nil {
				return fmt.Errorf("list existing message projection: %w", err)
			}
			if _, err := s.appendProjectedMessages(ctx, runID, desired, existing); err != nil {
				return err
			}
		}
	}
	s.storeProjectionLocked(runID, next)
	return nil
}

func (s *Service) projectionStateLocked(runID domain.RunID) (runProjectionState, bool) {
	state, ok := s.projections[runID]
	return state, ok
}

func (s *Service) storeProjectionLocked(runID domain.RunID, state runProjectionState) {
	if s.projections == nil {
		s.projections = make(map[domain.RunID]runProjectionState)
	}
	s.projections[runID] = state
}

func (s *Service) dropRunProjectionLocked(runID domain.RunID) {
	delete(s.projections, runID)
}

func (s *Service) dropSessionProjectionsLocked(sessionID domain.SessionID) {
	for runID, state := range s.projections {
		if state.sessionID == sessionID {
			delete(s.projections, runID)
		}
	}
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

// foldRunProjection derives the messages carried by the run's events after
// prev.seq. On any error it returns prev unchanged: the caller retries from
// the same cursor, and deterministic ids make the re-derived rows harmless.
func (s *Service) foldRunProjection(ctx context.Context, sessionID domain.SessionID, runID domain.RunID, prev runProjectionState) ([]domain.Message, runProjectionState, error) {
	if prev.noContext {
		return nil, prev, nil
	}
	it, err := s.deps.Journal.Replay(ctx, runID, prev.seq)
	if err != nil {
		return nil, prev, fmt.Errorf("replay Journal for message projection: %w", err)
	}
	defer it.Close()
	var out []domain.Message
	next := prev
	// Builder writes are local; only the final String crosses into the state,
	// so a failed fold never mutates the committed prefix.
	var pending strings.Builder
	pending.WriteString(prev.pending)
	for it.Next() {
		re := it.Value().Event
		switch re.Type {
		case domain.EventRunStarted:
			// `!!` no-context shell runs journal for the transcript only;
			// their tool rows must never enter the model feed.
			var p payloadRunStarted
			if err := json.Unmarshal(re.Payload, &p); err != nil {
				return nil, prev, fmt.Errorf("decode run.started seq %d: %w", re.Seq, err)
			}
			if p.NoContext {
				next.noContext = true
				next.pending = ""
				next.seq = re.Seq
				return nil, next, nil
			}
		case domain.EventModelRequest:
			pending.Reset()
		case domain.EventModelDelta:
			var p payloadModelDelta
			if err := json.Unmarshal(re.Payload, &p); err != nil {
				return nil, prev, fmt.Errorf("decode model.delta seq %d: %w", re.Seq, err)
			}
			pending.WriteString(p.Delta)
		case domain.EventToolRequested:
			if pending.Len() > 0 {
				out = append(out, projectedTextMessage(sessionID, runID, re, "0", pending.String()))
				pending.Reset()
			}
			var p payloadToolRequested
			if err := json.Unmarshal(re.Payload, &p); err != nil {
				return nil, prev, fmt.Errorf("decode tool.requested seq %d: %w", re.Seq, err)
			}
			args, err := json.Marshal(p.Args)
			if err != nil {
				return nil, prev, fmt.Errorf("encode tool.requested seq %d: %w", re.Seq, err)
			}
			out = append(out, domain.Message{
				ID: projectedMessageID(runID, re.Seq, "1"), SessionID: sessionID, RunID: runID,
				Role: domain.RoleAssistant, CreatedAt: re.CreatedAt,
				ToolCallID: p.ToolCallID, ToolName: p.ToolName, ToolArgs: args,
			})
		case domain.EventToolFinished:
			var p payloadToolFinished
			if err := json.Unmarshal(re.Payload, &p); err != nil {
				return nil, prev, fmt.Errorf("decode tool.finished seq %d: %w", re.Seq, err)
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
		case domain.EventModelCompleted:
			content, err := completedProjectionContent(re, pending.String())
			if err != nil {
				return nil, prev, err
			}
			out = append(out, projectedTextMessage(sessionID, runID, re, "0", content))
			pending.Reset()
		}
		next.seq = re.Seq
	}
	if err := it.Err(); err != nil {
		return nil, prev, fmt.Errorf("iterate Journal message projection: %w", err)
	}
	next.pending = pending.String()
	return out, next, nil
}

// errUnsupportedCompletedVersion marks model.completed events that predate
// the supported metadata shapes. Reconcile must not hard-fail a whole session on
// them: the legacy run is skipped and whatever projection was persisted by
// the old writer stays readable. Fresh runs are v3-only, so the run
// completion path still fails closed on this error.
var errUnsupportedCompletedVersion = errors.New("unsupported model.completed payload version")

func completedProjectionContent(re domain.RunEvent, deltas string) (string, error) {
	if re.PayloadVersion != 2 && re.PayloadVersion != 3 {
		return "", fmt.Errorf("%w %d at seq %d", errUnsupportedCompletedVersion, re.PayloadVersion, re.Seq)
	}
	normalized, err := events.NormalizeModelCompletion(re)
	if err != nil {
		return "", fmt.Errorf("model.completed seq %d: %w", re.Seq, err)
	}
	var p payloadModelCompletedV3
	if err := json.Unmarshal(normalized.Payload, &p); err != nil {
		return "", fmt.Errorf("decode model.completed: %w", err)
	}
	if p.ByteLen != len(deltas) {
		return "", fmt.Errorf("model.completed seq %d byte length mismatch", re.Seq)
	}
	return deltas, nil
}

func projectedTextMessage(sessionID domain.SessionID, runID domain.RunID, re domain.RunEvent, slot, content string) domain.Message {
	return domain.Message{
		ID: projectedMessageID(runID, re.Seq, slot), SessionID: sessionID, RunID: runID,
		Role: domain.RoleAssistant, CreatedAt: re.CreatedAt, Content: content,
	}
}

func projectedMessageID(runID domain.RunID, seq domain.EventSeq, slot string) string {
	return fmt.Sprintf("msgp_%s_%020d_%s", runID, seq, slot)
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

		// One-shot children share their parent's session id but their
		// transcript is internal execution detail: the live path suppresses it
		// and the agent tool result carries the answer. Only continuable
		// children own an addressable session transcript.
		if run.Kind == domain.RunKindChild && run.EffectiveChildMode() == domain.ChildModeOneShot {
			continue
		}
		prev, tracked := s.projectionStateLocked(run.ID)
		if tracked && prev.sessionID != sessionID {
			prev, tracked = runProjectionState{}, false
		}
		if !tracked {
			prev = runProjectionState{sessionID: sessionID}
		}
		desired, next, err := s.foldRunProjection(ctx, sessionID, run.ID, prev)

		if errors.Is(err, errUnsupportedCompletedVersion) {
			s.dropRunProjectionLocked(run.ID)
			continue
		}
		if err != nil {
			return err
		}
		existing, err = s.appendProjectedMessages(ctx, run.ID, desired, existing)
		if err != nil {
			return err
		}
		// A read repair keeps a live run's cursor warm for the next tool
		// exchange; finished runs are evicted so the map tracks only work in
		// flight.
		if run.Status.Terminal() {
			s.dropRunProjectionLocked(run.ID)
			continue
		}
		s.storeProjectionLocked(run.ID, next)
	}
	return nil
}

// ReconcileSessionMessages repairs the derived MessageStore view from the
// durable Journal. Read surfaces call it before returning history so a crash
// between Journal commit and projection cannot leave a completed turn hidden.
func (s *Service) ReconcileSessionMessages(ctx context.Context, sessionID domain.SessionID) error {
	return s.reconcileSessionMessageProjection(ctx, sessionID)
}

// parentFeed lists a session's messages and drops child-run transcripts,
// exactly like the run feed.
func (s *Service) parentFeed(ctx context.Context, sessionID domain.SessionID) ([]domain.Message, error) {
	stored, err := s.deps.Messages.ListMessages(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("runtime: list session messages: %w", err)
	}
	return s.parentFeedMessages(ctx, sessionID, stored), nil
}

// SessionViewMessages lists a session's parent conversation for transcript
// surfaces. One-shot child transcripts reuse the parent session id but are
// not part of that conversation, so they are excluded here even when an older
// projection already wrote them.
func (s *Service) SessionViewMessages(ctx context.Context, sessionID domain.SessionID) ([]domain.Message, error) {
	stored, err := s.deps.Messages.ListMessages(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("runtime: list session messages: %w", err)
	}
	if len(stored) == 0 || s.deps.Runs == nil {
		return stored, nil
	}
	oneShot, err := s.childRunIDs(ctx, sessionID, true)
	if err != nil {
		slog.Warn("one-shot child transcript filter skipped; listing runs failed",
			"session", string(sessionID), "err", err)
		return stored, nil
	}
	return dropRunMessages(stored, oneShot), nil
}

func (s *Service) childRunIDs(ctx context.Context, sessionID domain.SessionID, oneShotOnly bool) (map[domain.RunID]struct{}, error) {
	runs, err := s.deps.Runs.ListRunsBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	child := make(map[domain.RunID]struct{})
	for _, run := range runs {
		if run.Kind != domain.RunKindChild {
			continue
		}
		if oneShotOnly && run.EffectiveChildMode() != domain.ChildModeOneShot {
			continue
		}
		child[run.ID] = struct{}{}
	}
	return child, nil
}

func dropRunMessages(stored []domain.Message, drop map[domain.RunID]struct{}) []domain.Message {
	if len(drop) == 0 {
		return stored
	}
	kept := make([]domain.Message, 0, len(stored))
	for _, msg := range stored {
		if _, found := drop[msg.RunID]; found {
			continue
		}
		kept = append(kept, msg)
	}
	return kept
}

// parentFeedMessages drops child-run transcripts from the history that becomes
// the parent's model input. Child rows stay in the append-only session store,
// but a child's answer already reaches the parent as the `agent` tool result.
// Feeding the child's whole turn as well doubles it: one sub-agent transcript
// can exceed the entire history byte budget and leave no room for the parent's
// own turns.
//
// A run row we cannot see is treated as non-child: legacy rows have no run id
// and must stay in the feed.
func (s *Service) parentFeedMessages(ctx context.Context, sessionID domain.SessionID, stored []domain.Message) []domain.Message {
	if len(stored) == 0 || s.deps.Runs == nil {
		return stored
	}
	child, err := s.childRunIDs(ctx, sessionID, false)
	if err != nil {
		slog.Warn("child transcript filter skipped; listing runs failed",
			"session", string(sessionID), "err", err)
		return stored
	}
	return dropRunMessages(stored, child)
}
