package runtime

import (
	"context"
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

// payloadSessionTruncated journals the audit trail of one rewind.
type payloadSessionTruncated struct {
	SessionID       string `json:"session_id"`
	CutoffMessageID string `json:"cutoff_message_id"`
	Reason          string `json:"reason"`
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
	active, err := s.deps.Runs.ListActiveRuns(ctx)
	if err != nil {
		return RewindResult{}, fmt.Errorf("runtime: list active runs: %w", err)
	}
	for _, run := range active {
		if run.SessionID == sessionID {
			return RewindResult{}, ErrSessionBusy
		}
	}
	stored, err := s.deps.Messages.ListMessages(ctx, sessionID)
	if err != nil {
		return RewindResult{}, fmt.Errorf("runtime: list session messages: %w", err)
	}
	cutoffIdx := -1
	for i, message := range stored {
		if message.ID == messageID {
			cutoffIdx = i
			break
		}
	}
	if cutoffIdx < 0 {
		return RewindResult{}, ErrInvalidCutoff
	}
	marker := storage.SessionTruncation{
		SessionID:       sessionID,
		CutoffMessageID: messageID,
		Reason:          storage.TruncationRewind,
		CreatedAt:       time.Now().UnixMilli(),
	}
	if err := s.deps.Truncations.RecordSessionTruncation(ctx, marker); err != nil {
		return RewindResult{}, fmt.Errorf("runtime: record session truncation: %w", err)
	}
	runID := domain.RunID(newPrefixedID("tr_"))
	if _, err := s.RecordExternalRunEvent(ctx, runID, domain.EventSessionTruncated, payloadSessionTruncated{
		SessionID:       string(sessionID),
		CutoffMessageID: messageID,
		Reason:          storage.TruncationRewind,
	}); err != nil {
		return RewindResult{}, fmt.Errorf("runtime: persist truncation event: %w", err)
	}
	return RewindResult{CutoffMessageID: messageID, RemainingCount: cutoffIdx}, nil
}

// effectiveSessionMessages applies the session's newest truncation marker
// to a raw ListMessages slice. It is the single filter every session view
// shares (model context, session/messages, trajectory). A nil or failing
// truncation store leaves the history untouched.
func (s *Service) effectiveSessionMessages(ctx context.Context, sessionID domain.SessionID, stored []domain.Message) []domain.Message {
	if s.deps.Truncations == nil {
		return stored
	}
	marker, ok, err := s.deps.Truncations.LatestSessionTruncation(ctx, sessionID)
	if err != nil || !ok {
		return stored
	}
	return storage.ApplySessionTruncation(stored, marker)
}
