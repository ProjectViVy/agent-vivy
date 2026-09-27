package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	history     *HistoryService
	sessions    storage.SessionStore
	runs        storage.RunStore
	journal     storage.Journal
	continuity  storage.ContinuityStore
	messages    storage.MessageStore
	truncations storage.TruncationStore
	limits      domain.ContinuityLimits
	now         func() time.Time
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

// SetViewStores wires the stores needed to report a snapshot's current feed
// status (whether its owning turn is still visible after rewinds). The
// stores are optional: without them reference/get reports feed_status
// "unknown" rather than guessing.
func (s *ReferenceService) SetViewStores(messages storage.MessageStore, truncations storage.TruncationStore) {
	s.messages = messages
	s.truncations = truncations
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
	for _, committed := range result.Events {
		publishCommittedRunEvent(ctx, committed)
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
	// Fork-copied snapshots live under the session's derived history run,
	// which intentionally has no run-store row.
	iter, err := s.journal.Replay(ctx, forkHistoryRunID(sessionID), 0)
	if err != nil {
		return domain.ContextReference{}, err
	}
	found, err := scanReferenceEvents(iter, referenceID)
	if err != nil {
		return domain.ContextReference{}, err
	}
	if found.ID == referenceID {
		return found, nil
	}
	return domain.ContextReference{}, ReferenceError{Status: string(domain.HistoryStatusNotFound), Reason: "reference not found"}
}

// Get reads the destination-owned copy plus live source and feed status,
// each computed independently of the stored snapshot content.
func (s *ReferenceService) Get(ctx context.Context, sessionID domain.SessionID, referenceID string) (domain.ReferenceView, error) {
	reference, err := s.Lookup(ctx, sessionID, referenceID)
	if err != nil {
		return domain.ReferenceView{}, err
	}
	view := domain.ReferenceView{Reference: reference, SourceStatus: "ok", FeedStatus: "unknown"}
	if s.sessions != nil {
		if _, err := s.sessions.GetSession(ctx, reference.SourceSessionID); err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				return domain.ReferenceView{}, err
			}
			view.SourceStatus = "source_unavailable"
		}
	}
	if s.messages != nil && s.truncations != nil {
		view.FeedStatus = s.referenceFeedStatus(ctx, sessionID, reference)
	}
	return view, nil
}

// referenceFeedStatus reports whether the snapshot's owning turn is still in
// the session's effective view: "included", "hidden" by a rewind/edit
// marker, or "detached" when no user turn carries its run.
func (s *ReferenceService) referenceFeedStatus(ctx context.Context, sessionID domain.SessionID, reference domain.ContextReference) string {
	stored, err := s.messages.ListMessages(ctx, sessionID)
	if err != nil {
		return "unknown"
	}
	owned := false
	for _, message := range stored {
		if message.RunID == reference.DestinationRunID && message.Role == domain.RoleUser {
			owned = true
			break
		}
	}
	markers, err := s.truncations.ListViewTruncations(ctx, sessionID)
	if err != nil {
		return "unknown"
	}
	for _, message := range storage.ApplySessionTruncations(stored, markers) {
		if message.RunID == reference.DestinationRunID && message.Role == domain.RoleUser {
			return "included"
		}
	}
	if owned {
		return "hidden"
	}
	return "detached"
}

// AttachedReferences collects the session's committed reference snapshots
// keyed by the run of the turn that owns them. Only startup-position events
// (admission) and fork copies participate: a mid-run model attach was
// already delivered to the model by its tool result and must not be
// re-injected as imported data.
func (s *ReferenceService) AttachedReferences(ctx context.Context, sessionID domain.SessionID) (map[domain.RunID][]domain.ContextReference, error) {
	out := make(map[domain.RunID][]domain.ContextReference)
	if s.runs == nil || s.journal == nil {
		return out, nil
	}
	runs, err := s.runs.ListRunsBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	for _, run := range runs {
		iter, err := s.journal.Replay(ctx, run.ID, 0)
		if err != nil {
			return nil, err
		}
		if err := collectStartupReferences(iter, sessionID, out); err != nil {
			return nil, err
		}
	}
	iter, err := s.journal.Replay(ctx, forkHistoryRunID(sessionID), 0)
	if err != nil {
		return nil, err
	}
	if err := collectAllRunReferences(iter, sessionID, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ForkReferenceEvents builds the child-owned copies of every startup
// snapshot attached to the given parent runs — the same startup set the feed
// would project, so a mid-run model attach never travels without its tool
// result. Copies keep the original provenance and the copied turns' run
// identity; they journal under the child's derived history run inside the
// same atomic fork commit.
func (s *ReferenceService) ForkReferenceEvents(ctx context.Context, dst, parent domain.SessionID, runs []domain.RunID, historyRun domain.RunID) ([]domain.RunEvent, error) {
	if s.journal == nil {
		return nil, nil
	}
	var events []domain.RunEvent
	seen := make(map[domain.RunID]bool, len(runs))
	for _, runID := range runs {
		if runID == "" || seen[runID] {
			continue
		}
		seen[runID] = true
		iter, err := s.journal.Replay(ctx, runID, 0)
		if err != nil {
			return nil, err
		}
		byRun := make(map[domain.RunID][]domain.ContextReference)
		if err := collectStartupReferences(iter, parent, byRun); err != nil {
			return nil, err
		}
		for _, copied := range byRun[runID] {
			copied.ID = newPrefixedID("ref_")
			copied.DestinationSessionID = dst
			// DestinationRunID stays the copied turn's provenance run: fork
			// message copies keep the parent run id for audit.
			forkEvent, err := historyEvent(historyRun, domain.EventContextReferenceAttached, payloadContextReferenceAttached{Reference: copied})
			if err != nil {
				return nil, err
			}
			events = append(events, forkEvent)
		}
	}
	return events, nil
}

// forkHistoryRunID derives the rowless journal run that owns a forked child
// session's copied reference events. One fork produces one child session, so
// the id is deterministic and never collides with a live run row.
func forkHistoryRunID(sessionID domain.SessionID) domain.RunID {
	return domain.RunID("frok_" + string(sessionID))
}

// collectStartupReferences walks only a run's committed startup set:
// run.started plus the contiguous reference events that follow it. A first
// non-reference event (model.request, deltas, a mid-run model attach) ends
// the scan.
func collectStartupReferences(iter storage.Iterator[storage.Entry], sessionID domain.SessionID, out map[domain.RunID][]domain.ContextReference) error {
	defer func() { _ = iter.Close() }()
	seenStarted := false
loop:
	for iter.Next() {
		event := iter.Value().Event
		switch event.Type {
		case domain.EventRunStarted:
			seenStarted = true
		case domain.EventContextReferenceAttached:
			if !seenStarted {
				break loop
			}
			appendDecodedReference(event, sessionID, out)
		default:
			break loop
		}
	}
	return iter.Err()
}

func collectAllRunReferences(iter storage.Iterator[storage.Entry], sessionID domain.SessionID, out map[domain.RunID][]domain.ContextReference) error {
	defer func() { _ = iter.Close() }()
	for iter.Next() {
		appendDecodedReference(iter.Value().Event, sessionID, out)
	}
	return iter.Err()
}

// appendDecodedReference records a well-formed snapshot under its owning
// run. Undecodable payloads are skipped here; Lookup surfaces them as an
// explicit error instead of an empty substituted reference.
func appendDecodedReference(event domain.RunEvent, sessionID domain.SessionID, out map[domain.RunID][]domain.ContextReference) {
	var payload payloadContextReferenceAttached
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return
	}
	reference := payload.Reference
	if reference.DestinationSessionID != sessionID || reference.DestinationRunID == "" || reference.ID == "" {
		return
	}
	for _, existing := range out[reference.DestinationRunID] {
		if existing.ID == reference.ID {
			return
		}
	}
	out[reference.DestinationRunID] = append(out[reference.DestinationRunID], reference)
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
