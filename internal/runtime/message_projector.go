package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		return nil, fmt.Errorf("replay Journal for message projection: %w", err)
	}
	defer it.Close()
	var out []domain.Message
	var pending strings.Builder
	for it.Next() {
		re := it.Value().Event
		switch re.Type {
		case domain.EventModelRequest:
			pending.Reset()
		case domain.EventModelDelta:
			var p payloadModelDelta
			if err := json.Unmarshal(re.Payload, &p); err != nil {
				return nil, fmt.Errorf("decode model.delta seq %d: %w", re.Seq, err)
			}
			pending.WriteString(p.Delta)
		case domain.EventToolRequested:
			if pending.Len() > 0 {
				out = append(out, projectedTextMessage(sessionID, runID, re, "0", pending.String()))
				pending.Reset()
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
		case domain.EventModelCompleted:
			content, err := completedProjectionContent(re, pending.String())
			if err != nil {
				return nil, err
			}
			out = append(out, projectedTextMessage(sessionID, runID, re, "0", content))
			pending.Reset()
		}
	}
	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("iterate Journal message projection: %w", err)
	}
	return out, nil
}

func completedProjectionContent(re domain.RunEvent, deltas string) (string, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(re.Payload, &fields); err != nil {
		return "", fmt.Errorf("decode model.completed seq %d: %w", re.Seq, err)
	}
	raw, hasContent := fields["content"]
	if re.PayloadVersion == 0 && hasContent {
		re.PayloadVersion = 1
	}
	if re.PayloadVersion == 1 {
		if !hasContent {
			return "", fmt.Errorf("model.completed seq %d missing v1 content", re.Seq)
		}
		var content string
		if err := json.Unmarshal(raw, &content); err != nil {
			return "", fmt.Errorf("decode model.completed content seq %d: %w", re.Seq, err)
		}
		return content, nil
	}
	if re.PayloadVersion != 2 {
		return "", fmt.Errorf("model.completed seq %d unsupported payload version %d", re.Seq, re.PayloadVersion)
	}
	if hasContent {
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
