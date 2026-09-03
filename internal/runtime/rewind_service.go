package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// Rewind failures are user-facing; the control plane maps them to
// dedicated RPC error codes (Conflict / NotFound).
var (
	// ErrSessionBusy rejects a rewind while the session still has a
	// non-terminal run: cutting history under an in-flight run would race
	// the run's own message appends.
	ErrSessionBusy = errors.New("runtime: session has an active run")
	// ErrInvalidCutoff rejects a rewind whose message is unknown to the
	// session or already behind the effective truncation point.
	ErrInvalidCutoff = errors.New("runtime: cutoff message is not in the live session view")
	// ErrRewindNotWired refuses rewinds when the truncation store is nil.
	ErrRewindNotWired = errors.New("runtime: truncation store is not wired")
)

// RewindResult reports the applied cutoff.
type RewindResult struct {
	CutoffMessageID string `json:"cutoff_message_id"`
	RemainingCount  int    `json:"remaining_count"`
}

// EditSession atomically replaces the visible suffix with a new user turn and
// active run. Validation happens before the transaction; engine execution is
// launched only after marker, message, run row, and run.started all commit.
func (s *Service) EditSession(ctx context.Context, sessionID domain.SessionID, messageID, text string, options RunOptions) (domain.RunID, error) {
	mutations, ok := s.deps.Truncations.(storage.HistoryMutationStore)
	if !ok {
		return "", ErrRewindNotWired
	}
	if err := s.rejectBusySession(ctx, sessionID); err != nil {
		return "", err
	}
	stored, _, _, err := s.sessionViewCutoff(ctx, sessionID, messageID)
	if err != nil {
		return "", err
	}
	marker := storage.SessionTruncation{SessionID: sessionID, CutoffMessageID: messageID, TailMessageID: stored[len(stored)-1].ID, Reason: storage.TruncationEdit, CreatedAt: time.Now().UnixMilli()}
	return s.runWithOptions(ctx, sessionID, text, options, func(message domain.Message, run domain.Run, event domain.RunEvent) (domain.RunEvent, error) {
		committed, err := mutations.CommitSessionEdit(ctx, marker, message, run, event)
		if err != nil {
			return event, fmt.Errorf("runtime: commit session edit: %w", err)
		}
		return committed, nil
	})
}

// payloadSessionTruncated journals the audit trail of one rewind.
type payloadSessionTruncated struct {
	SessionID       string `json:"session_id"`
	CutoffMessageID string `json:"cutoff_message_id"`
	Reason          string `json:"reason"`
	ForkSessionID   string `json:"fork_session_id,omitempty"`
}

// payloadSessionForked journals the provenance of a fork on the child.
type payloadSessionForked struct {
	SessionID          string `json:"session_id"`
	ParentSessionID    string `json:"parent_session_id"`
	ForkPointMessageID string `json:"fork_point_message_id"`
}

// RewindSession voids the session view from messageID (inclusive) onward:
// a truncation marker is recorded and a session.truncated event journaled,
// while every row stays on disk (JOURNAL-REWIND-AND-FORK). The caller
// re-enters with the existing turn/start — rewind never runs a turn.
func (s *Service) RewindSession(ctx context.Context, sessionID domain.SessionID, messageID string) (RewindResult, error) {
	if s.deps.Messages == nil || s.deps.Runs == nil || s.deps.Journal == nil {
		return RewindResult{}, errors.New("runtime: service not wired")
	}
	if s.deps.Truncations == nil {
		return RewindResult{}, ErrRewindNotWired
	}
	mutations, ok := s.deps.Truncations.(storage.HistoryMutationStore)
	if !ok {
		return RewindResult{}, ErrRewindNotWired
	}
	if err := s.rejectBusySession(ctx, sessionID); err != nil {
		return RewindResult{}, err
	}
	stored, _, cutoffIdx, err := s.sessionViewCutoff(ctx, sessionID, messageID)
	if err != nil {
		return RewindResult{}, err
	}
	// Tail anchors the fold at the suffix that exists now: turns appended
	// after this rewind must stay visible, or the edit flow (rewind + a
	// fresh turn/start) would hide its own retry forever.
	marker := storage.SessionTruncation{
		SessionID:       sessionID,
		CutoffMessageID: messageID,
		TailMessageID:   stored[len(stored)-1].ID,
		Reason:          storage.TruncationRewind,
		CreatedAt:       time.Now().UnixMilli(),
	}
	runID := domain.RunID(newPrefixedID("tr_"))
	event, err := historyEvent(runID, domain.EventSessionTruncated, payloadSessionTruncated{
		SessionID:       string(sessionID),
		CutoffMessageID: messageID,
		Reason:          storage.TruncationRewind,
	})
	if err != nil {
		return RewindResult{}, err
	}
	event, err = mutations.CommitSessionRewind(ctx, marker, event)
	if err != nil {
		return RewindResult{}, fmt.Errorf("runtime: persist truncation event: %w", err)
	}
	s.publish(ctx, event)
	return RewindResult{CutoffMessageID: messageID, RemainingCount: cutoffIdx}, nil
}

// ForkResult reports the fork: the new session's id, the fork point and
// how many history rows were copied into the child.
type ForkResult struct {
	SessionID          string `json:"session_id"`
	ForkPointMessageID string `json:"fork_point_message_id"`
	CopiedCount        int    `json:"copied_count"`
}

// ForkSession copies the session history up to and including messageID into
// a brand-new session and leaves the original untouched (the fork markers
// on both sessions are provenance anchors that filter nothing). The child
// inherits the source's sandbox knobs so the branch starts on equal
// governance footing (JOURNAL-REWIND-AND-FORK §3.3).
func (s *Service) ForkSession(ctx context.Context, sessionID domain.SessionID, messageID, title string) (ForkResult, error) {
	if s.deps.Messages == nil || s.deps.Runs == nil || s.deps.Journal == nil || s.deps.Sessions == nil {
		return ForkResult{}, errors.New("runtime: service not wired")
	}
	if s.deps.Truncations == nil {
		return ForkResult{}, ErrRewindNotWired
	}
	mutations, ok := s.deps.Truncations.(storage.HistoryMutationStore)
	if !ok {
		return ForkResult{}, ErrRewindNotWired
	}
	if err := s.rejectBusySession(ctx, sessionID); err != nil {
		return ForkResult{}, err
	}
	source, err := s.deps.Sessions.GetSession(ctx, sessionID)
	if err != nil {
		return ForkResult{}, fmt.Errorf("runtime: get source session: %w", err)
	}
	stored, effective, cutoffIdx, err := s.sessionViewCutoff(ctx, sessionID, messageID)
	if err != nil {
		return ForkResult{}, err
	}
	now := time.Now().UnixMilli()
	newID := domain.SessionID(newPrefixedID("sess_"))
	if title == "" {
		title = source.Title + " (fork)"
	}
	child := domain.Session{
		ID:             newID,
		Title:          title,
		CreatedAt:      now,
		SandboxMode:    source.SandboxMode,
		ApprovalPolicy: source.ApprovalPolicy,
	}
	// Message ids are globally unique, so copies get fresh ids; the run
	// references keep pointing at the parent's runs (audit-true: the
	// content did originate there) while the child's run-driven trajectory
	// starts empty (design §3.4). Copies come from the EFFECTIVE view: rows
	// already folded out by a rewind must not resurrect in the child — the
	// child's context equals the parent's visible context at the fork point.
	copied := append([]domain.Message(nil), effective[:cutoffIdx+1]...)
	childForkPointID := ""
	for i, message := range copied {
		message.SessionID = newID
		message.ID = newMessageID()
		childForkPointID = message.ID
		copied[i] = message
	}
	// Provenance markers: the fork point on the parent (parent's message
	// id) and a back-reference on the child (the child's own copy of that
	// message). Neither filters (ApplySessionTruncation only honors
	// rewind/edit); they exist so the audit trail names the relationship.
	// Tails are recorded for the same audit symmetry but filter nothing.
	markers := []storage.SessionTruncation{{
		SessionID: sessionID, CutoffMessageID: messageID, TailMessageID: stored[len(stored)-1].ID, Reason: storage.TruncationFork,
		ForkSessionID: string(newID), CreatedAt: now,
	}, {
		SessionID: newID, CutoffMessageID: childForkPointID, TailMessageID: childForkPointID, Reason: storage.TruncationForkedFrom,
		ForkSessionID: string(sessionID), CreatedAt: now,
	}}
	parentEvent, err := historyEvent(domain.RunID(newPrefixedID("tr_")), domain.EventSessionTruncated, payloadSessionTruncated{
		SessionID:       string(sessionID),
		CutoffMessageID: messageID,
		Reason:          storage.TruncationFork,
		ForkSessionID:   string(newID),
	})
	if err != nil {
		return ForkResult{}, err
	}
	childEvent, err := historyEvent(domain.RunID(newPrefixedID("tr_")), domain.EventSessionForked, payloadSessionForked{
		SessionID:          string(newID),
		ParentSessionID:    string(sessionID),
		ForkPointMessageID: messageID,
	})
	if err != nil {
		return ForkResult{}, err
	}
	events, err := mutations.CommitSessionFork(ctx, child, copied, markers, []domain.RunEvent{parentEvent, childEvent})
	if err != nil {
		return ForkResult{}, fmt.Errorf("runtime: commit session fork: %w", err)
	}
	for _, event := range events {
		s.publish(ctx, event)
	}
	return ForkResult{SessionID: string(newID), ForkPointMessageID: messageID, CopiedCount: len(copied)}, nil
}

// rejectBusySession refuses the action while the session still has a
// non-terminal run (JOURNAL-REWIND-AND-FORK §2.4).
func (s *Service) rejectBusySession(ctx context.Context, sessionID domain.SessionID) error {
	active, err := s.deps.Runs.ListActiveRuns(ctx)
	if err != nil {
		return fmt.Errorf("runtime: list active runs: %w", err)
	}
	for _, run := range active {
		if run.SessionID == sessionID {
			return ErrSessionBusy
		}
	}
	return nil
}

// sessionViewCutoff locates the cutoff message in the session's EFFECTIVE
// view (truncation markers applied). Rows already folded out of the live
// context are not valid rewind/fork targets (ErrInvalidCutoff), and the
// returned index is view-relative so remaining-count arithmetic matches
// what the user sees. The raw stored list rides along for tail anchoring.
func (s *Service) sessionViewCutoff(ctx context.Context, sessionID domain.SessionID, messageID string) (stored, effective []domain.Message, idx int, err error) {
	stored, err = s.deps.Messages.ListMessages(ctx, sessionID)
	if err != nil {
		return nil, nil, -1, fmt.Errorf("runtime: list session messages: %w", err)
	}
	effective, err = s.effectiveSessionMessages(ctx, sessionID, stored)
	if err != nil {
		return nil, nil, -1, err
	}
	for i, message := range effective {
		if message.ID == messageID {
			return stored, effective, i, nil
		}
	}
	return nil, nil, -1, ErrInvalidCutoff
}

// effectiveSessionMessages folds the session's stored list by the UNION of
// all view-controlling markers. It is the single filter every session view
// shares (model context, session/messages, trajectory). A nil or failing
// truncation store leaves the history untouched.
func (s *Service) effectiveSessionMessages(ctx context.Context, sessionID domain.SessionID, stored []domain.Message) ([]domain.Message, error) {
	if s.deps.Truncations == nil {
		return stored, nil
	}
	markers, err := s.deps.Truncations.ListViewTruncations(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("runtime: list session truncations: %w", err)
	}
	if len(markers) == 0 {
		return stored, nil
	}
	return storage.ApplySessionTruncations(stored, markers), nil
}

func historyEvent(runID domain.RunID, typ domain.EventType, payload any) (domain.RunEvent, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return domain.RunEvent{}, fmt.Errorf("runtime: marshal history event: %w", err)
	}
	return domain.RunEvent{RunID: runID, Type: typ, CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1, Payload: data}, nil
}
