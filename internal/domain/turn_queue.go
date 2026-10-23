package domain

import "time"

// QueueControlJournalID identifies Session-owned removal tombstones that must
// outlive a sealed Run. Storage deletes this stream with its owning Session.
func QueueControlJournalID(sessionID SessionID) RunID {
	return RunID("queue-control/" + string(sessionID))
}

// QueuedTurn is one message waiting in a session's dual-track queue
// (pi parity): the steer track injects at the next turn boundary of an
// active run; the follow-up track admits after the run settles.
//
// Queue durability rides the run Journal (session_truncations marker
// precedent): turn.queued on the currently active run's journal, matched by
// turn.dequeued/turn.steered consumption events. The newest run's journal
// materializes the full pending queue atomically in run.started. Idle removal
// tombstones use the Session control stream when the carrier is already sealed.
type QueuedTurn struct {
	ID        string    `json:"id"`
	SessionID SessionID `json:"session_id"`
	// Track is "steer" or "follow_up".
	Track string `json:"track"`
	Text  string `json:"text"`
	// RequestRef names the private immutable input snapshot. Journal and Face
	// projections carry the reference, never image bytes or file contents.
	RequestRef string             `json:"request_ref,omitempty"`
	Request    *QueuedTurnOptions `json:"-"`
	CreatedAt  time.Time          `json:"created_at"`
	// EnqueuedOn is the run journal the turn.queued marker was written to.
	// Consumption events target the same journal while it remains appendable.
	EnqueuedOn RunID `json:"enqueued_on"`
}

// QueuedTurnOptions is the serializable user input of a future Run. Host-only
// callbacks and automatic Goal authority are deliberately absent.
type QueuedTurnOptions struct {
	Mode                 RunMode           `json:"mode,omitempty"`
	Profile              PolicyProfile     `json:"policy_profile,omitempty"`
	Face                 Face              `json:"face,omitempty"`
	Thinking             ThinkingMode      `json:"thinking,omitempty"`
	CollaborationMode    CollaborationMode `json:"collaboration_mode,omitempty"`
	CollaborationVersion int               `json:"collaboration_version,omitempty"`
	Provenance           *Provenance       `json:"provenance,omitempty"`
	Attachments          []Attachment      `json:"attachments,omitempty"`
	FileContexts         []FileContext     `json:"file_contexts,omitempty"`
	Continuity           *ContinuityInput  `json:"continuity,omitempty"`
}

const (
	QueueTrackSteer    = "steer"
	QueueTrackFollowUp = "follow_up"
)

// Queue lane modes (pi's steering_mode/follow_up_mode).
const (
	QueueModeAll        = "all"           // all steer messages at a boundary; separate follow-up Runs
	QueueModeOneAtATime = "one-at-a-time" // head only; tail stays queued
)
