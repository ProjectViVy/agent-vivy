package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"agent-vivy/internal/domain"
)

// DeliverablePresentation is private state owned by the presentation event.
// Session/item identity comes from that event; the transaction supplies its run.
type DeliverablePresentation struct {
	SessionID domain.SessionID
	ItemID    string
	Content   []byte `json:"-"`
}

// ValidatePresentations validates only a new commit, after receipt replay.
// A retry never replaces immutable bytes with a later filesystem capture.
func (m ContinuityMutation) ValidatePresentations() error {
	if m.Event.Type != domain.EventDeliverablesPresented {
		if len(m.Presentations) != 0 {
			return fmt.Errorf("storage: presentation bytes require a presentation event")
		}
		return nil
	}
	var payload struct {
		Set domain.DeliverySet `json:"delivery_set"`
	}
	if err := json.Unmarshal(m.Event.Payload, &payload); err != nil {
		return fmt.Errorf("storage: decode presentation metadata: %w", err)
	}
	set := payload.Set
	limits := domain.DefaultContinuityLimits()
	if err := set.Validate(limits); err != nil {
		return fmt.Errorf("storage: invalid presentation metadata: %w", err)
	}
	if m.Receipt.Operation != "deliverable.present" || set.SessionID != m.SessionID || set.RunID != m.RunID || set.ToolCallID != m.Receipt.RequestID || len(set.Items) != len(m.Presentations) {
		return fmt.Errorf("storage: presentation ownership or item correspondence mismatch")
	}
	expected := make(map[string]int64, len(set.Items))
	for _, item := range set.Items {
		if item.SessionID != m.SessionID || item.RunID != m.RunID || item.OriginToolCallID != set.ToolCallID {
			return fmt.Errorf("storage: foreign presentation item owner")
		}
		if _, duplicate := expected[item.ID]; duplicate {
			return fmt.Errorf("storage: duplicate presentation item")
		}
		expected[item.ID] = item.Size
	}
	var total int64
	for _, presentation := range m.Presentations {
		size, exists := expected[presentation.ItemID]
		if !exists || presentation.SessionID != m.SessionID || int64(len(presentation.Content)) != size || int64(len(presentation.Content)) > limits.PresentFileBytes {
			return fmt.Errorf("storage: presentation bytes differ from declared items")
		}
		delete(expected, presentation.ItemID)
		total += int64(len(presentation.Content))
		if total > limits.PresentSetBytes {
			return fmt.Errorf("storage: presentation bytes exceed aggregate limit")
		}
	}
	return nil
}

type presentationTransaction interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// HiddenPresentationRuns uses the exact existing view fold over metadata in
// ListMessages order. Insertion positions would disagree for backdated turns.
func HiddenPresentationRuns(ctx context.Context, tx presentationTransaction, sessionID domain.SessionID, bind string) (map[domain.RunID]bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,run_id,role FROM messages WHERE session_id = `+bind+` ORDER BY created_at,id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("storage: read presentation owning turns: %w", err)
	}
	messages := []domain.Message{}
	for rows.Next() {
		var message domain.Message
		if err := rows.Scan(&message.ID, (*string)(&message.RunID), &message.Role); err != nil {
			_ = rows.Close()
			return nil, err
		}
		messages = append(messages, message)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT cutoff_message_id,tail_message_id,reason FROM session_truncations WHERE session_id = `+bind+` AND reason IN ('rewind','edit') ORDER BY id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("storage: read presentation visibility: %w", err)
	}
	markers := []SessionTruncation{}
	for rows.Next() {
		var marker SessionTruncation
		if err := rows.Scan(&marker.CutoffMessageID, &marker.TailMessageID, &marker.Reason); err != nil {
			_ = rows.Close()
			return nil, err
		}
		markers = append(markers, marker)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	hidden := map[domain.RunID]bool{}
	for _, message := range messages {
		if message.Role == domain.RoleUser {
			hidden[message.RunID] = true
		}
	}
	for _, message := range ApplySessionTruncations(messages, markers) {
		if message.Role == domain.RoleUser {
			delete(hidden, message.RunID)
		}
	}
	return hidden, nil
}

// PruneDeliverablePresentations runs inside the marker owner's transaction.
func PruneDeliverablePresentations(ctx context.Context, tx presentationTransaction, sessionID domain.SessionID, bind1, bind2 string) error {
	hidden, err := HiddenPresentationRuns(ctx, tx, sessionID, bind1)
	if err != nil {
		return err
	}
	for runID := range hidden {
		if _, err := tx.ExecContext(ctx, `DELETE FROM deliverable_presentations WHERE session_id = `+bind1+` AND run_id = `+bind2, sessionID, runID); err != nil {
			return fmt.Errorf("storage: prune hidden presentation: %w", err)
		}
	}
	return nil
}
