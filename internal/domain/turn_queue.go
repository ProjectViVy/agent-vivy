package domain

import "time"

// QueuedTurn is one message waiting in a session's dual-track queue
// (pi parity): the steer track injects at the next turn boundary of an
// active run; the follow-up track admits after the run settles.
//
// Queue durability rides the run Journal (session_truncations marker
// precedent): turn.queued on the currently active run's journal, matched by
// turn.dequeued/turn.steered consumption events. The newest run's journal
// always materializes the full pending queue — admission re-journals the
// tail onto the next run — so restart recovery replays only the latest run.
type QueuedTurn struct {
	ID        string    `json:"id"`
	SessionID SessionID `json:"session_id"`
	// Track is "steer" or "follow_up".
	Track        string    `json:"track"`
	Text         string    `json:"text"`
	Thinking     string    `json:"thinking,omitempty"`
	Mode         string    `json:"mode,omitempty"`
	Attachments  []string  `json:"attachments,omitempty"`
	ContextPaths []string  `json:"context_paths,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	// EnqueuedOn is the run journal the turn.queued marker was written to.
	// Consumption events target the same journal while it remains appendable.
	EnqueuedOn RunID `json:"enqueued_on"`
}

const (
	QueueTrackSteer    = "steer"
	QueueTrackFollowUp = "follow_up"
)

// Queue lane modes (pi's steering_mode/follow_up_mode).
const (
	QueueModeAll        = "all"           // drain everything at the boundary/admission
	QueueModeOneAtATime = "one-at-a-time" // head only; tail stays queued
)
