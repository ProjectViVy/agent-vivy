package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

// ReferenceOperationAttach names the durable operation committed by the
// model-facing attach path. Receipts key on it plus the stable tool_call id.
const ReferenceOperationAttach = "reference.attach"

// ReferenceError keeps a read-side wire status explicit: callers surface the
// status instead of receiving an empty successful result.
type ReferenceError struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func (e ReferenceError) Error() string {
	return fmt.Sprintf("reference: %s: %s", e.Status, e.Reason)
}

// ReferenceService owns preview, attachment, and destination lookup of
// explicit context references. It never bypasses HistoryService
// authorization: a snapshot is exactly what one authorized read returned.
type ReferenceService struct {
	history    *HistoryService
	sessions   storage.SessionStore
	runs       storage.RunStore
	journal    storage.Journal
	continuity storage.ContinuityStore
	limits     domain.ContinuityLimits
	now        func() time.Time
}

var _ tools.ReferenceOperations = (*ReferenceService)(nil)

func NewReferenceService(history *HistoryService, sessions storage.SessionStore, runs storage.RunStore, journal storage.Journal, continuity storage.ContinuityStore) *ReferenceService {
	return &ReferenceService{
		history:    history,
		sessions:   sessions,
		runs:       runs,
		journal:    journal,
		continuity: continuity,
		limits:     domain.DefaultContinuityLimits(),
		now:        time.Now,
	}
}

// SetLimits narrows the effective budget without weakening defaults.
func (s *ReferenceService) SetLimits(limits domain.ContinuityLimits) {
	if limits.ReferenceBytes <= 0 {
		return
	}
	s.limits = limits
}

// Preview reads the selection once under the caller's authority and returns
// exactly what Attach would snapshot. An over-budget preview still reports
// its honest byte_count and status so the caller can narrow instead of
// receiving a truncated stand-in.
func (s *ReferenceService) Preview(ctx context.Context, selection domain.HistorySelection) (domain.ReferencePreview, error) {
	if err := selection.Validate(s.limits); err != nil {
		return domain.ReferencePreview{}, err
	}
	page, err := s.history.Read(ctx, domain.HistoryReadRequest{Selection: &selection})
	if err != nil {
		return domain.ReferencePreview{}, err
	}
	preview := domain.ReferencePreview{
		Selection:    selection,
		Items:        page.Items,
		Digest:       page.SelectionDigest,
		CapturedAt:   s.now().UnixMilli(),
		SourceStatus: page.Status,
	}
	if data, err := json.Marshal(page.Items); err == nil {
		preview.ByteCount = len(data)
	}
	return preview, nil
}

// Attach commits one sanitized snapshot as a context.reference_attached
// event of the caller's run. The effect is receipt-keyed on the stable
// tool_call id, so a checkpoint resume that re-executes the same call
// replays the committed reference instead of writing a duplicate.
func (s *ReferenceService) Attach(ctx context.Context, selection domain.ReferenceSelection) (domain.ContextReference, error) {
	sessionID := tools.SessionIDFromContext(ctx)
	runID := tools.RunIDFromContext(ctx)
	callID := tools.ToolCallIDFromContext(ctx)
	if sessionID == "" || runID == "" || callID == "" {
		return domain.ContextReference{}, ReferenceError{Status: string(domain.HistoryStatusForbidden), Reason: "attach requires an active run tool invocation"}
	}
	if s.continuity == nil || s.journal == nil {
		return domain.ContextReference{}, ReferenceError{Status: string(domain.HistoryStatusUnavailable), Reason: "reference commits unavailable"}
	}
	reference, err := s.snapshot(ctx, selection, sessionID, runID, "model_tool")
	if err != nil {
		return domain.ContextReference{}, err
	}
	count, totalBytes, err := s.runReferenceUsage(ctx, runID)
	if err != nil {
		return domain.ContextReference{}, err
	}
	if count+1 > s.limits.ReferencesPerTask {
		return domain.ContextReference{}, ReferenceError{Status: string(domain.HistoryStatusInvalidArgument), Reason: fmt.Sprintf("reference count %d exceeds per task maximum %d", count+1, s.limits.ReferencesPerTask)}
	}
	if int64(totalBytes)+int64(referenceByteCount(reference)) > int64(s.limits.ReferencesTotalBytes) {
		return domain.ContextReference{}, ReferenceError{Status: string(domain.HistoryStatusInvalidArgument), Reason: fmt.Sprintf("reference exceeds total bytes budget %d", s.limits.ReferencesTotalBytes)}
	}
	event, err := referenceEvent(reference, s.now().UnixMilli())
	if err != nil {
		return domain.ContextReference{}, err
	}
	inputHash := referenceInputHash(selection)
	result, err := s.continuity.CommitContinuityOperation(ctx, storage.ContinuityMutation{
		SessionID: sessionID,
		RunID:     runID,
		Event:     event,
		InputHash: inputHash,
		Receipt: domain.ContinuityReceipt{
			SessionID: sessionID,
			Operation: ReferenceOperationAttach,
			RequestID: callID,
			InputHash: inputHash,
		},
	})
	if err != nil {
		return domain.ContextReference{}, err
	}
	if !result.NewlyCommitted {
		return decodeReferenceEvents(result.Events)
	}
	return reference, nil
}

// AttachForAdmission builds the snapshot for an operator-selected GUI
// reference. The read runs under operator authority over the destination
// session; the caller (admission) commits the event inside its atomic
// startup set, so no engine step touches the content before commit.
func (s *ReferenceService) AttachForAdmission(ctx context.Context, selection domain.ReferenceSelection, dest domain.SessionID, runID domain.RunID) (domain.ContextReference, error) {
	readCtx := WithHistoryOperator(tools.WithSessionID(ctx, dest))
	return s.snapshot(readCtx, selection, dest, runID, "user_selection")
}

// Lookup returns the destination-owned snapshot by reference id. It reads
// the stored event copy only; the source session is never consulted.
func (s *ReferenceService) Lookup(ctx context.Context, sessionID domain.SessionID, referenceID string) (domain.ContextReference, error) {
	if referenceID == "" || !utf8.ValidString(referenceID) {
		return domain.ContextReference{}, ReferenceError{Status: string(domain.HistoryStatusInvalidArgument), Reason: "reference id is invalid"}
	}
	if s.runs == nil || s.journal == nil {
		return domain.ContextReference{}, ReferenceError{Status: string(domain.HistoryStatusUnavailable), Reason: "reference lookup unavailable"}
	}
	runs, err := s.runs.ListRunsBySession(ctx, sessionID)
	if err != nil {
		return domain.ContextReference{}, err
	}
	for _, run := range runs {
		iter, err := s.journal.Replay(ctx, run.ID, 0)
		if err != nil {
			return domain.ContextReference{}, err
		}
		found, foundErr := scanReferenceEvents(iter, referenceID)
		if foundErr != nil {
			return domain.ContextReference{}, foundErr
		}
		if found.ID == referenceID {
			return found, nil
		}
	}
	return domain.ContextReference{}, ReferenceError{Status: string(domain.HistoryStatusNotFound), Reason: "reference not found"}
}

func (s *ReferenceService) snapshot(ctx context.Context, selection domain.ReferenceSelection, dest domain.SessionID, runID domain.RunID, origin string) (domain.ContextReference, error) {
	if err := selection.Selection.Validate(s.limits); err != nil {
		return domain.ContextReference{}, err
	}
	if selection.ExpectedDigest != "" && !utf8.ValidString(selection.ExpectedDigest) {
		return domain.ContextReference{}, ReferenceError{Status: string(domain.HistoryStatusInvalidArgument), Reason: "expected digest is invalid"}
	}
	page, err := s.history.Read(ctx, domain.HistoryReadRequest{Selection: &selection.Selection})
	if err != nil {
		return domain.ContextReference{}, err
	}
	if page.SelectionDigest == "" {
		status := page.Status
		if status == "" {
			status = string(domain.HistoryStatusUnavailable)
		}
		reason := "selection is not a complete attachable read; narrow it"
		if page.Reason != nil {
			reason = *page.Reason
		}
		return domain.ContextReference{}, ReferenceError{Status: status, Reason: reason}
	}
	if selection.ExpectedDigest != "" && selection.ExpectedDigest != page.SelectionDigest {
		return domain.ContextReference{}, fmt.Errorf("reference: selection content changed since preview: %w", storage.ErrSourceChanged)
	}
	reference := domain.ContextReference{
		ID:                   newPrefixedID("ref_"),
		DestinationSessionID: dest,
		DestinationRunID:     runID,
		SourceSessionID:      selection.Selection.SourceSessionID,
		SourceWorkspace:      s.sourceWorkspaceLabel(ctx, selection.Selection.SourceSessionID),
		CapturedAt:           s.now().UnixMilli(),
		Items:                page.Items,
		Digest:               page.SelectionDigest,
		Origin:               origin,
	}
	if err := reference.Validate(s.limits); err != nil {
		return domain.ContextReference{}, ReferenceError{Status: string(domain.HistoryStatusInvalidArgument), Reason: err.Error()}
	}
	return reference, nil
}

// sourceWorkspaceLabel is display metadata only; it never carries authority.
func (s *ReferenceService) sourceWorkspaceLabel(ctx context.Context, id domain.SessionID) string {
	if s.sessions == nil {
		return ""
	}
	session, err := s.sessions.GetSession(ctx, id)
	if err != nil {
		return ""
	}
	return session.WorkspacePath
}

// runReferenceUsage counts this run's committed reference events so the
// per task and aggregate byte budgets hold across multiple attaches.
func (s *ReferenceService) runReferenceUsage(ctx context.Context, runID domain.RunID) (int, int, error) {
	iter, err := s.journal.Replay(ctx, runID, 0)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = iter.Close() }()
	count, total := 0, 0
	for iter.Next() {
		entry := iter.Value()
		if entry.Event.Type != domain.EventContextReferenceAttached {
			continue
		}
		count++
		total += len(entry.Event.Payload)
	}
	if err := iter.Err(); err != nil {
		return 0, 0, err
	}
	return count, total, nil
}

func scanReferenceEvents(iter storage.Iterator[storage.Entry], referenceID string) (domain.ContextReference, error) {
	defer func() { _ = iter.Close() }()
	for iter.Next() {
		entry := iter.Value()
		if entry.Event.Type != domain.EventContextReferenceAttached {
			continue
		}
		var payload payloadContextReferenceAttached
		if err := json.Unmarshal(entry.Event.Payload, &payload); err != nil {
			continue
		}
		if payload.Reference.ID == referenceID {
			return payload.Reference, nil
		}
	}
	if err := iter.Err(); err != nil {
		return domain.ContextReference{}, err
	}
	return domain.ContextReference{}, nil
}

// decodeReferenceEvents returns the reference carried by a previously
// committed event set (receipt replay path).
func decodeReferenceEvents(events []domain.RunEvent) (domain.ContextReference, error) {
	for _, event := range events {
		if event.Type != domain.EventContextReferenceAttached {
			continue
		}
		var payload payloadContextReferenceAttached
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return domain.ContextReference{}, fmt.Errorf("reference: decode committed event: %w", err)
		}
		return payload.Reference, nil
	}
	return domain.ContextReference{}, fmt.Errorf("reference: committed events carry no reference")
}

// ReferenceEvent builds the run event carrying one snapshot. Admission
// appends it to the atomic startup set; the tool path commits it as an
// operation.
func ReferenceEvent(reference domain.ContextReference, at int64) (domain.RunEvent, error) {
	return referenceEvent(reference, at)
}

func referenceEvent(reference domain.ContextReference, at int64) (domain.RunEvent, error) {
	payload, err := json.Marshal(payloadContextReferenceAttached{Reference: reference})
	if err != nil {
		return domain.RunEvent{}, fmt.Errorf("reference: marshal event payload: %w", err)
	}
	return domain.RunEvent{
		RunID:          reference.DestinationRunID,
		Type:           domain.EventContextReferenceAttached,
		CreatedAt:      at,
		PayloadVersion: 1,
		Payload:        payload,
	}, nil
}

func referenceInputHash(selection domain.ReferenceSelection) string {
	body := map[string]any{
		"selection":       selection.Selection,
		"expected_digest": selection.ExpectedDigest,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(append([]byte("v1:"), data...))
	return hex.EncodeToString(sum[:])
}

func referenceByteCount(reference domain.ContextReference) int {
	data, err := json.Marshal(reference)
	if err != nil {
		return 0
	}
	return len(data)
}
