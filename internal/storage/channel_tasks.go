package storage

import (
	"context"

	"agent-vivy/internal/domain"
)

// ChannelTaskCommit is the transaction boundary for one external (A2A)
// admission: it embeds the ordinary primary-run commit and adds the scoped
// receipt evidence. When NewSession is set, the transaction asserts the
// candidate session is still absent and commits it together with the
// ownership row; when nil, the named session must already be owned by the
// same scope — foreign or missing identity is storage.ErrNotFound, never an
// adoption.
type ChannelTaskCommit struct {
	PrimaryRunCommit
	Scope      domain.ChannelTaskScope
	MessageID  string
	InputHash  [32]byte
	NewSession *domain.Session
	// Admitted is the channel.task_admitted event, journaled immediately
	// after run.started inside the same transaction.
	Admitted domain.RunEvent
}

type ChannelTaskCommitResult struct {
	Receipt domain.ChannelTaskReceipt
	// Events is [run.started, channel.task_admitted] as committed, in order.
	// Replays return no events — callers re-read the original evidence.
	Events         []domain.RunEvent
	NewlyCommitted bool
}

type ChannelTaskRunQuery struct {
	Scope      domain.ChannelTaskScope
	SessionID  domain.SessionID
	AfterRunID domain.RunID
	Limit      int
}

type ChannelTaskRunPage struct {
	Runs      []domain.Run
	NextRunID domain.RunID
	HasMore   bool
}

// ChannelTaskStore is the A2A authorization/idempotency index. It is not a
// TaskStore: it owns no status, outputs or execution. The answer-side method
// arrives with A2A-03.
type ChannelTaskStore interface {
	FindChannelTaskReceipt(context.Context, domain.ChannelTaskScope, string) (domain.ChannelTaskReceipt, bool, error)
	CommitChannelTask(context.Context, ChannelTaskCommit) (ChannelTaskCommitResult, error)
	GetChannelTaskOwner(context.Context, domain.ChannelTaskScope, domain.RunID) (domain.SessionID, error)
	ListChannelTaskRuns(context.Context, ChannelTaskRunQuery) (ChannelTaskRunPage, error)
}

// ValidateChannelTaskCommit checks the cross-record invariants for the
// channel-task admission transaction before a backend opens it.
func ValidateChannelTaskCommit(commit ChannelTaskCommit) error {
	if commit.Scope.InstanceKey == "" || commit.Scope.PrincipalID == "" || commit.MessageID == "" {
		return ErrWorkInvalidMutation
	}
	if commit.NewSession != nil && commit.NewSession.ID != commit.Run.SessionID {
		return ErrWorkInvalidMutation
	}
	if commit.NewSession == nil && commit.Run.SessionID == "" {
		return ErrWorkInvalidMutation
	}
	if commit.Admitted.RunID != commit.Run.ID || commit.Admitted.Type != domain.EventChannelTaskAdmitted || commit.Admitted.PayloadVersion <= 0 {
		return ErrWorkInvalidMutation
	}
	return ValidatePrimaryRunCommit(commit.PrimaryRunCommit)
}
