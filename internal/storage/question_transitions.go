package storage

import (
	"context"

	"agent-vivy/internal/domain"
)

// QuestionTransitionCommit is the atomic settlement of one pending
// ask_user interaction: the question row CAS and the runtime-built
// journal event commit inside one transaction (design A2A-03, design
// section 6). The caller supplies the captured pending Question ID and
// the owning Run ID; both must match the stored row.
type QuestionTransitionCommit struct {
	RunID      domain.RunID
	QuestionID string
	Outcome    domain.QuestionStatus // answered | cancelled | expired
	Answer     string
	Actor      string
	Reason     string
	At         int64
	// Event is the versioned journal envelope for the transition
	// (user.question_answered / cancelled / expired). Its seq is assigned
	// inside the transaction.
	Event domain.RunEvent
}

// QuestionTransitionResult reports the committed transition.
type QuestionTransitionResult struct {
	Changed bool
	Events  []domain.RunEvent
}

// QuestionTransitionStore settles a pending question atomically with its
// journal event. It is the shared native discipline both the local
// Review/Face path and channel callers route through — one transition
// implementation, two native callers.
type QuestionTransitionStore interface {
	// CommitQuestionTransition CASes the captured pending question to
	// Outcome and appends Event, atomically. Returns ErrNotFound for a
	// missing question or run mismatch, ErrConflict when the question is
	// no longer pending or already expired, and ErrRunClosed when the run
	// already carries a committed terminal event.
	CommitQuestionTransition(context.Context, QuestionTransitionCommit) (QuestionTransitionResult, error)
}

// ChannelTaskAnswerCommit is the external (A2A) face of the same
// transition: everything in QuestionTransitionCommit plus the scoped
// remote-identity receipt that must commit in the same transaction. The
// receipt records operation=answer and the captured Question ID so a
// losing retry can never answer a later question.
type ChannelTaskAnswerCommit struct {
	Scope     domain.ChannelTaskScope
	MessageID string
	InputHash [32]byte
	// Transition carries the captured QuestionID/RunID and the built
	// answered envelope; Session and Run IDs on the receipt are taken
	// from the stored question row, never from the caller.
	Transition QuestionTransitionCommit
}
