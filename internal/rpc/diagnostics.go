package rpc

import (
	"context"
	"errors"

	"agent-vivy/internal/logging"
)

// DiagnosticsService is the bounded log-read/GUI-capture seam owned by
// internal/logging (D5). Nil disables diagnostics/log and
// diagnostics/gui/append.
type DiagnosticsService interface {
	Read(ctx context.Context, q logging.DiagnosticQuery) (logging.DiagnosticPage, error)
	AppendGUI(ctx context.Context, batch logging.GuiLogBatch) (logging.GuiLogAck, error)
}

// diagnosticsLogs serves diagnostics/logs: bounded, redacted reads over
// the owned runtime/gui log families. Query validation failures map to
// InvalidParams; gap pages carry gap:true, never fabricated continuity.
func (h *controlHandler) diagnosticsLogs(ctx context.Context, request Request) (any, *Error) {
	if h.deps.Diagnostics == nil {
		return nil, &Error{Code: MethodNotFound, Message: "diagnostics is not configured"}
	}
	var query logging.DiagnosticQuery
	if err := decodeParams(request, &query); err != nil {
		return nil, err
	}
	page, err := h.deps.Diagnostics.Read(ctx, query)
	if err != nil {
		if errors.Is(err, logging.ErrDiagnosticQuery) {
			return nil, &Error{Code: InvalidParams, Message: err.Error()}
		}
		return nil, internalError(err)
	}
	return page, nil
}

// diagnosticsGUIAppend serves diagnostics/gui/append: bounded sanitized
// capture into the owned GUI daily family. Write failures are RPC errors;
// a partial write reports its accepted prefix in the error message.
func (h *controlHandler) diagnosticsGUIAppend(ctx context.Context, request Request) (any, *Error) {
	if h.deps.Diagnostics == nil {
		return nil, &Error{Code: MethodNotFound, Message: "diagnostics is not configured"}
	}
	var batch logging.GuiLogBatch
	if err := decodeParams(request, &batch); err != nil {
		return nil, err
	}
	ack, err := h.deps.Diagnostics.AppendGUI(ctx, batch)
	if err != nil {
		if errors.Is(err, logging.ErrDiagnosticQuery) {
			return nil, &Error{Code: InvalidParams, Message: err.Error()}
		}
		return nil, internalError(err)
	}
	return ack, nil
}
