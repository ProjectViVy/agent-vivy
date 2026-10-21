package runtime

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// replayContinuityOperation resolves an already committed destination-owned
// result before mutable source/workspace reads. It never admits a new mutation;
// the existing transaction still arbitrates two racing first commits.
func replayContinuityOperation(ctx context.Context, store storage.ContinuityStore, journal storage.Journal, intended domain.ContinuityReceipt, runID domain.RunID, eventType domain.EventType) ([]domain.RunEvent, bool, error) {
	receipt, found, err := store.FindContinuityReceipt(ctx, intended.SessionID, intended.Operation, intended.RequestID)
	if err != nil || !found {
		return nil, false, err
	}
	if receipt.RunID != runID || !receipt.MatchesInput(intended.Input) {
		return nil, false, storage.ErrConflict
	}
	if receipt.EventSeq <= 0 {
		return nil, false, fmt.Errorf("runtime: continuity receipt has invalid event position")
	}
	iter, err := journal.Replay(ctx, runID, receipt.EventSeq-1)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = iter.Close() }()
	if iter.Next() {
		event := iter.Value().Event
		if event.RunID == runID && event.Seq == receipt.EventSeq && event.Type == eventType {
			return []domain.RunEvent{event}, true, nil
		}
	}
	if err := iter.Err(); err != nil {
		return nil, false, err
	}
	return nil, false, fmt.Errorf("runtime: continuity receipt has no matching committed event")
}

// Only caller-selected admitted values participate; generated IDs, provider
// settings, callbacks and current budget state are not new request intent.
type continuityAdmissionIntent struct {
	Text                 string
	Continuity           domain.ContinuityInput
	Scope                []domain.SessionID `json:"-"`
	Mode                 domain.RunMode
	Profile              domain.PolicyProfile // Empty is the caller's server-default declaration.
	CollaborationMode    domain.CollaborationMode
	CollaborationVersion int
	Thinking             domain.ThinkingMode
	Face                 domain.Face
	Provenance           domain.Provenance
	Attachments          []domain.Attachment
	FileContexts         []domain.FileContext
}

func continuityIntendedInput(intent continuityAdmissionIntent) ([]byte, error) {
	intent.Continuity.HistoryScope.SessionIDs = append([]domain.SessionID(nil), intent.Scope...)
	intent.Continuity.References = append([]domain.ReferenceSelection(nil), intent.Continuity.References...)
	for i := range intent.Continuity.References {
		intent.Continuity.References[i].Selection = normalizedReferenceSelection(intent.Continuity.References[i].Selection)
	}
	intent.Attachments = append([]domain.Attachment(nil), intent.Attachments...)
	for i := range intent.Attachments {
		intent.Attachments[i].Data = append([]byte(nil), intent.Attachments[i].Data...)
	}
	data, err := json.Marshal(intent)
	if err != nil {
		return nil, fmt.Errorf("runtime: encode continuity intended input: %w", err)
	}
	return data, nil
}
