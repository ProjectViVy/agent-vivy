package tools

import (
	"context"
	"encoding/json"

	"agent-vivy/internal/domain"
)

type runIDContextKey struct{}
type toolCallIDContextKey struct{}
type proposalDataContextKey struct{}
type sessionIDContextKey struct{}
type workspaceIDContextKey struct{}
type imagesSupportedContextKey struct{}

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

// WithToolCallID binds the model-issued stable call identity for tools whose
// durable operation must be idempotent across retries and recovery.
func WithToolCallID(ctx context.Context, callID string) context.Context {
	return context.WithValue(ctx, toolCallIDContextKey{}, callID)
}

func ToolCallIDFromContext(ctx context.Context) string {
	callID, _ := ctx.Value(toolCallIDContextKey{}).(string)
	return callID
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

// HasProposalData reports a reviewed-call binding even when its private data
// is missing. File backends must not treat that loss as an unreviewed call.
func HasProposalData(ctx context.Context) bool {
	_, ok := ctx.Value(proposalDataContextKey{}).(json.RawMessage)
	return ok
}

// WithoutProposalData keeps a run's approval out of sibling and fresh calls.
func WithoutProposalData(ctx context.Context) context.Context {
	return context.WithValue(ctx, proposalDataContextKey{}, false)
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

// WithImagesSupported binds whether the active model accepts image parts. The
// runtime owns the capability truth; tools only consume it.
func WithImagesSupported(ctx context.Context, supported bool) context.Context {
	return context.WithValue(ctx, imagesSupportedContextKey{}, supported)
}

// ImagesSupported reports whether the active model accepts image input. A
// context without the binding keeps the status-quo allow for unknown routes.
func ImagesSupported(ctx context.Context) bool {
	supported, ok := ctx.Value(imagesSupportedContextKey{}).(bool)
	return !ok || supported
}
