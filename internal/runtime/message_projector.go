package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"agent-vivy/internal/domain"
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
	retained := false
	if sources, ok := s.deps.Sessions.(storage.SubjectSourceStore); ok {
		_, err := sources.GetRunSubjectSource(ctx, runID)
		if err == nil {
			retained = true
		} else if !errors.Is(err, storage.ErrNotFound) {
			return nil, err
		}
	}
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		return nil, fmt.Errorf("replay Journal for message projection: %w", err)
	}
	// Release Replay's backend connection before resolving operation metadata.
	// SQLite's single-connection Journal cannot service a nested lookup.
	var events []domain.RunEvent
	for it.Next() {
		events = append(events, it.Value().Event)
	}
	replayErr := it.Err()
	it.Close()
	if replayErr != nil {
		return nil, replayErr
	}
	var out []domain.Message
	receipts := map[string]domain.RunEvent{}
	var terminal bool
	var pending strings.Builder
	var provenance *storage.InputProvenance
	textMessage := func(e domain.RunEvent, suffix, content string) domain.Message {
		m := projectedTextMessage(sessionID, runID, e, suffix, content)
		if provenance != nil {
			m.ContentOrigin = provenance.ContentOrigin
			m.ExcludeAutomaticIngest = provenance.ExcludeAutomaticIngest
		}
		return m
	}
	for _, re := range events {
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
		case domain.EventModelRequest:
			var request payloadModelRequestV3
			if json.Unmarshal(re.Payload, &request) == nil && request.Source == modelCallSourceMaintenance {
				continue
			}
			if retained && pending.Len() > 0 {
				m := textMessage(re, "0", pending.String())
				m.Source = "journal_partial"
				out = append(out, m)
			}
			pending.Reset()
			if re.PayloadVersion >= 4 {
				p, err := storage.DecodeModelInputProvenance(re)
				if err != nil {
					return nil, err
				}
				provenance = &p
			} else {
				provenance = nil
			}
		case domain.EventModelDelta:
			var p payloadModelDelta
			if err := json.Unmarshal(re.Payload, &p); err != nil {
				return nil, fmt.Errorf("decode model.delta seq %d: %w", re.Seq, err)
			}
			pending.WriteString(p.Delta)
		case domain.EventToolRequested:
			if pending.Len() > 0 {
				out = append(out, textMessage(re, "0", pending.String()))
				pending.Reset()
			}
			var p payloadToolRequested
			if err := json.Unmarshal(re.Payload, &p); err != nil {
				return nil, fmt.Errorf("decode tool.requested seq %d: %w", re.Seq, err)
			}
			if retained && (re.PayloadVersion >= 2 || p.OriginalArgumentsInvalidUTF8 || len(p.OriginalArgumentsBytes) > 0) {
				return nil, storage.ErrSubjectEvidenceUnresolvable
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
			if provenance != nil {
				out[len(out)-1].ContentOrigin = provenance.ContentOrigin
				out[len(out)-1].ExcludeAutomaticIngest = provenance.ExcludeAutomaticIngest
			}
		case domain.EventToolOperation:
			var p struct {
				OperationID string                    `json:"operation_id"`
				State       domain.ToolOperationState `json:"state"`
			}
			if json.Unmarshal(re.Payload, &p) == nil && p.State == domain.ToolOperationCompleted {
				receipts[p.OperationID] = re
			}
		case domain.EventToolFinished:
			var p payloadToolFinished
			if err := json.Unmarshal(re.Payload, &p); err != nil {
				return nil, fmt.Errorf("decode tool.finished seq %d: %w", re.Seq, err)
			}
			delete(receipts, p.ToolCallID)
			content := p.Result
			if p.Error != "" {
				content = p.Error
			}
			msg := domain.Message{
				ID: projectedMessageID(runID, re.Seq, "0"), SessionID: sessionID, RunID: runID,
				Role: domain.RoleTool, CreatedAt: re.CreatedAt, Content: content,
				ToolCallID: p.ToolCallID, ToolName: p.ToolName,
			}
			if s.deps.ToolOperations != nil {
				op, err := s.deps.ToolOperations.GetToolOperation(ctx, runID, p.ToolCallID)
				if err != nil && !errors.Is(err, storage.ErrNotFound) {
					return nil, err
				}
				if err == nil {
					msg.ContentOrigin = op.ContentOrigin
					msg.ExcludeAutomaticIngest = op.ExcludeAutomaticIngest
				}
			} else if isNotebookTool(p.ToolName) {
				msg.ContentOrigin = domain.ContentOriginNotebook
				msg.ExcludeAutomaticIngest = true
			}
			out = append(out, msg)
		case domain.EventRunCompleted:
			terminal = true
		case domain.EventRunFailed, domain.EventRunCancelled:
			terminal = true
			if retained && pending.Len() > 0 {
				m := textMessage(re, "0", pending.String())
				m.Source = "journal_partial"
				out = append(out, m)
				pending.Reset()
			}
		case domain.EventModelCompleted:
			content, err := completedProjectionContent(re, pending.String())
			if err != nil {
				return nil, err
			}
			out = append(out, textMessage(re, "0", content))
			pending.Reset()
		}
	}
	if terminal && len(receipts) > 0 {
		if sources, ok := s.deps.Sessions.(storage.SubjectSourceStore); ok {
			if _, err := sources.GetRunSubjectSource(ctx, runID); err == nil {
				events := make([]domain.RunEvent, 0, len(receipts))
				for _, e := range receipts {
					events = append(events, e)
				}
				sort.Slice(events, func(i, j int) bool { return events[i].Seq < events[j].Seq })
				for _, e := range events {
					var p struct {
						OperationID string `json:"operation_id"`
					}
					if err := json.Unmarshal(e.Payload, &p); err != nil {
						return nil, err
					}
					op, err := s.deps.ToolOperations.GetToolOperation(ctx, runID, p.OperationID)
					if err != nil {
						return nil, err
					}
					ref, err := storage.DecodeToolEvidenceReference(op)
					if err != nil {
						return nil, err
					}
					raw, found, err := s.toolEvidenceBlobs().Get(ctx, ref.ID)
					if err != nil {
						return nil, err
					}
					if !found {
						return nil, storage.ErrSubjectEvidenceIncomplete
					}
					if _, err := storage.VerifyToolEvidence(ref, raw); err != nil {
						return nil, err
					}
					content := ref.Projection
					if op.Failure != "" {
						content = op.Failure
					}
					out = append(out, domain.Message{ID: projectedMessageID(runID, e.Seq, "0"), SessionID: sessionID, RunID: runID, Role: domain.RoleTool, CreatedAt: e.CreatedAt, Content: content, ToolCallID: op.OperationID, ToolName: op.ToolName, Source: "journal_partial", ContentOrigin: op.ContentOrigin, ExcludeAutomaticIngest: op.ExcludeAutomaticIngest})
				}
			} else if !errors.Is(err, storage.ErrNotFound) {
				return nil, err
			}
		}
	}
	return out, nil
}

// errUnsupportedCompletedVersion marks model.completed events that predate
// the v2 hash-commit shape. Reconcile must not hard-fail a whole session on
// them: the legacy run is skipped and whatever projection was persisted by
// the old writer stays readable. Fresh runs are v2-only, so the run
// completion path still fails closed on this error.
var errUnsupportedCompletedVersion = errors.New("unsupported model.completed payload version")

func completedProjectionContent(re domain.RunEvent, deltas string) (string, error) {
	if re.PayloadVersion != 2 {
		return "", fmt.Errorf("%w %d at seq %d", errUnsupportedCompletedVersion, re.PayloadVersion, re.Seq)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(re.Payload, &fields); err != nil {
		return "", fmt.Errorf("decode model.completed seq %d: %w", re.Seq, err)
	}
	if _, hasContent := fields["content"]; hasContent {
		return "", fmt.Errorf("model.completed seq %d v2 must not contain content", re.Seq)
	}
	if len(fields) != 2 {
		return "", fmt.Errorf("model.completed seq %d v2 has unknown or missing fields", re.Seq)
	}
	var p payloadModelCompletedV2
	if err := json.Unmarshal(re.Payload, &p); err != nil {
		return "", fmt.Errorf("decode model.completed metadata seq %d: %w", re.Seq, err)
	}
	if p.ByteLen != len([]byte(deltas)) {
		return "", fmt.Errorf("model.completed seq %d byte length mismatch", re.Seq)
	}
	sum := sha256.Sum256([]byte(deltas))
	if len(p.ContentSHA256) != sha256.Size*2 || strings.ToLower(p.ContentSHA256) != p.ContentSHA256 {
		return "", fmt.Errorf("model.completed seq %d invalid content digest", re.Seq)
	}
	if _, err := hex.DecodeString(p.ContentSHA256); err != nil || p.ContentSHA256 != hex.EncodeToString(sum[:]) {
		return "", fmt.Errorf("model.completed seq %d content digest mismatch", re.Seq)
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
		a.ToolCallID == b.ToolCallID && a.ToolName == b.ToolName && bytes.Equal(a.ToolArgs, b.ToolArgs) &&
		a.ContentOrigin == b.ContentOrigin && a.ExcludeAutomaticIngest == b.ExcludeAutomaticIngest
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
