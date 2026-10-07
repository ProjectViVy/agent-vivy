package domain

// Channel-task records are the core's authorization and idempotency index
// for external (A2A) submissions. They own no task status, outputs or
// execution — only who submitted what, under which instance scope, and which
// native Session/Run the admission committed. See design §6–6.1.
type ChannelTaskScope struct {
	// InstanceKey is the stable Module/provider/instance tuple; credential
	// revision and Generation hash are deliberately excluded.
	InstanceKey string
	PrincipalID string
}

// ChannelTaskInput carries one accepted external message into native
// admission. Its scope is constructed by the Host, never decoded from a
// client payload.
type ChannelTaskInput struct {
	Scope     ChannelTaskScope
	MessageID string
	SessionID SessionID
	RunID     RunID
	// QuestionID targets a captured pending ask_user interaction instead
	// of a fresh admission: the message carries the remote ordinary
	// answer. Empty means submit; set means answer dispatch.
	QuestionID string
	Parts      []string
}

// ChannelTaskReceipt is the durable result identity for one accepted
// external message: retry/reconnect resolution reads this row, not run
// state. DeletedAt marks revocation without discarding the deduplication
// key; the row outlives Session/Run deletion on purpose.
type ChannelTaskReceipt struct {
	Scope       ChannelTaskScope
	MessageID   string
	Operation   string
	InputHash   [32]byte
	SessionID   SessionID
	RunID       RunID
	QuestionID  string
	AcceptedSeq EventSeq
	CreatedAt   int64
	DeletedAt   *int64

	// Replayed is transient (never persisted): true when this Submit call
	// resolved to an already-committed receipt instead of committing anew.
	Replayed bool
}
