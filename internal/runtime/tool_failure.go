package runtime

import (
	"context"
	"errors"
	"io/fs"

	"github.com/cloudwego/eino/compose"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/mcphost"
	"agent-vivy/internal/tools"
)

// toolFailure records one classified, model-correctable tool invocation
// outcome (NUDGE-DESIGN §4/§5). Absence of a record means ordinary
// success. Status is "recoverable" or "refused". Reason is a stable
// vocabulary word. Diagnostic is redacted, bounded text — never raw
// credentials. Effects is "not_executed", "none" or "unknown".
type toolFailure struct {
	Status     string
	Reason     string
	Diagnostic string
	Effects    string
}

// Vocabulary of §5 statuses, reasons and effects. Failure identity is
// metadata, never a string-prefix heuristic.
const (
	toolFailureStatusRecoverable = "recoverable"
	toolFailureStatusRefused     = "refused"

	toolFailureReasonInvalidArguments = "invalid_arguments"
	toolFailureReasonNotFound         = "not_found"
	toolFailureReasonPolicyDenied     = "policy_denied"
	toolFailureReasonUserDenied       = "user_denied"
	toolFailureReasonCommandFailed    = "command_failed"
	toolFailureReasonRemoteTool       = "remote_tool_error"

	toolEffectsNotExecuted = "not_executed"
	toolEffectsNone        = "none"
	toolEffectsUnknown     = "unknown"
)

// toolFailureDiagnosticMaxBytes bounds the diagnostic text retained on
// the failure record and projected into tool.finished.error.
const toolFailureDiagnosticMaxBytes = 2048

// classifyToolFailure applies the §5 allowlist at the invocation origin:
// only failures raised by the tool's own InvokableRun are eligible.
// Cancellation, deadlines and native interrupts are never failures —
// they propagate before classification. Errors outside the allowlist
// (transport, storage, policy engine, unknown) keep their fatal cause.
func classifyToolFailure(ctx context.Context, spec domain.ToolSpec, err error) (toolFailure, bool) {
	if err == nil {
		return toolFailure{}, false
	}
	// Parent cancellation/deadline outranks any error text.
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return toolFailure{}, false
	}
	// Native Eino interrupts (approval/question) keep their lifecycle.
	if _, interrupted := compose.IsInterruptRerunError(err); interrupted {
		return toolFailure{}, false
	}
	var argErr *tools.ArgError
	if errors.As(err, &argErr) {
		return toolFailure{
			Status:     toolFailureStatusRecoverable,
			Reason:     toolFailureReasonInvalidArguments,
			Diagnostic: boundToolFailureDiagnostic(err.Error()),
			Effects:    toolEffectsUnknown,
		}, true
	}
	// fs.ErrNotExist is recoverable only from readonly invocations; a
	// write that loses its path stays fatal.
	if spec.Readonly && errors.Is(err, fs.ErrNotExist) {
		return toolFailure{
			Status:     toolFailureStatusRecoverable,
			Reason:     toolFailureReasonNotFound,
			Diagnostic: boundToolFailureDiagnostic(err.Error()),
			Effects:    toolEffectsNone,
		}, true
	}
	var execErr *mcphost.ToolExecutionError
	if errors.As(err, &execErr) {
		return toolFailure{
			Status:     toolFailureStatusRecoverable,
			Reason:     toolFailureReasonRemoteTool,
			Diagnostic: boundToolFailureDiagnostic(execErr.Text),
			Effects:    toolEffectsUnknown,
		}, true
	}
	return toolFailure{}, false
}

// boundToolFailureDiagnostic redacts secrets and clamps the diagnostic
// before it is retained or projected.
func boundToolFailureDiagnostic(text string) string {
	return truncateUTF8(tools.RedactSensitive(text), toolFailureDiagnosticMaxBytes)
}

// refusalFailure builds the §4 record for a per-call refusal: the
// invocation never ran, so effects are always not_executed.
func refusalFailure(reason, diagnostic string) toolFailure {
	if reason == "" {
		reason = toolFailureReasonPolicyDenied
	}
	return toolFailure{
		Status:     toolFailureStatusRefused,
		Reason:     reason,
		Diagnostic: boundToolFailureDiagnostic(diagnostic),
		Effects:    toolEffectsNotExecuted,
	}
}

// markInvocationFailure publishes the typed record for the current tool
// call on the leg's side channel. A missing state means the caller runs
// outside a model-driven leg (governed shell) — nothing is published
// and the existing behavior is preserved. Inside a leg, a missing call
// ID is the §5 invariant failure and aborts the run.
func markInvocationFailure(ctx context.Context, failure toolFailure) error {
	state := nudgeStateFromContext(ctx)
	if state == nil {
		return nil
	}
	return state.MarkFailure(compose.GetToolCallID(ctx), failure)
}
