package tools

import (
	"context"
	"encoding/json"

	"agent-vivy/internal/domain"
)

type runIDContextKey struct{}
type proposalDataContextKey struct{}
type sessionIDContextKey struct{}
type workspaceIDContextKey struct{}

// WithRunID binds the durable run identity for tools that need a run-scoped
// capability such as the isolated filesystem. The runtime owns the source of
// truth; tools only consume the value.
func WithRunID(ctx context.Context, runID domain.RunID) context.Context {
	return context.WithValue(ctx, runIDContextKey{}, runID)
}

// RunIDFromContext returns the run identity supplied by the runtime.
func RunIDFromContext(ctx context.Context) domain.RunID {
	runID, _ := ctx.Value(runIDContextKey{}).(domain.RunID)
	return runID
}

// WithProposalData carries the exact opaque proposal payload into a resumed
// tool call. It is empty for question interactions and legacy approvals.
func WithProposalData(ctx context.Context, data json.RawMessage) context.Context {
	return context.WithValue(ctx, proposalDataContextKey{}, append(json.RawMessage(nil), data...))
}

func ProposalDataFromContext(ctx context.Context) json.RawMessage {
	data, _ := ctx.Value(proposalDataContextKey{}).(json.RawMessage)
	return append(json.RawMessage(nil), data...)
}

type proposalPreconditionKey struct{}

// WithProposalPrecondition carries the approved target hash into the resumed
// tool call. Mutation backends use it to fail closed if the target changed
// while the human review was open.
func WithProposalPrecondition(ctx context.Context, hash string) context.Context {
	return context.WithValue(ctx, proposalPreconditionKey{}, hash)
}

func ProposalPreconditionFromContext(ctx context.Context) string {
	value, _ := ctx.Value(proposalPreconditionKey{}).(string)
	return value
}

type proposalStaleReporterKey struct{}

// WithProposalStaleReporter lets a mutation backend report a failed
// precondition without importing the runtime package.
func WithProposalStaleReporter(ctx context.Context, report func(string)) context.Context {
	return context.WithValue(ctx, proposalStaleReporterKey{}, report)
}

func ReportProposalStale(ctx context.Context, reason string) {
	if report, ok := ctx.Value(proposalStaleReporterKey{}).(func(string)); ok && report != nil {
		report(reason)
	}
}

func WithSessionID(ctx context.Context, sessionID domain.SessionID) context.Context {
	return context.WithValue(ctx, sessionIDContextKey{}, sessionID)
}

func SessionIDFromContext(ctx context.Context) domain.SessionID {
	sessionID, _ := ctx.Value(sessionIDContextKey{}).(domain.SessionID)
	return sessionID
}

// WithWorkspaceID binds the allocator-issued workspace identity. It is kept
// separate from RunID so Sources and workspace-backed tools cannot infer a
// filesystem identity from a durable run identifier.
func WithWorkspaceID(ctx context.Context, workspaceID string) context.Context {
	return context.WithValue(ctx, workspaceIDContextKey{}, workspaceID)
}

func WorkspaceIDFromContext(ctx context.Context) string {
	workspaceID, _ := ctx.Value(workspaceIDContextKey{}).(string)
	return workspaceID
}
