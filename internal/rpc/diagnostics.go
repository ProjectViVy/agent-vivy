package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-vivy/internal/buildinfo"
	"agent-vivy/internal/logging"
)

// DiagnosticsService is the bounded log-read/GUI-capture seam owned by
// internal/logging (D5). Nil disables diagnostics/log and
// diagnostics/gui/append.
type DiagnosticsService interface {
	Read(ctx context.Context, q logging.DiagnosticQuery) (logging.DiagnosticPage, error)
	AppendGUI(ctx context.Context, batch logging.GuiLogBatch) (logging.GuiLogAck, error)
}

// diagnosticsLogs serves diagnostics/logs: bounded reads over
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

// diagnosticsBundleMaxRecords bounds the log tail folded into a bug report.
const diagnosticsBundleMaxRecords = 200

type diagnosticsBundleParams struct {
	SessionID string `json:"session_id"`
}

// diagnosticsBundle serves diagnostics/bundle: a bug-report markdown file
// in the instance exports directory (VCP C2 /bug) holding the build id, the
// session reference, and a bounded runtime-log tail. The handler
// writes the file itself because the report's scope is a diagnostics
// artifact, not a session transcript.
func (h *controlHandler) diagnosticsBundle(ctx context.Context, request Request) (any, *Error) {
	if h.deps.Diagnostics == nil {
		return nil, &Error{Code: MethodNotFound, Message: "diagnostics is not configured"}
	}
	if h.deps.DiagnosticsBundleDir == "" {
		return nil, &Error{Code: MethodNotFound, Message: "diagnostics bundle directory is not configured"}
	}
	var params diagnosticsBundleParams
	if err := json.Unmarshal(request.Params, &params); err != nil {
		return nil, &Error{Code: InvalidParams, Message: err.Error()}
	}
	page, err := h.deps.Diagnostics.Read(ctx, logging.DiagnosticQuery{
		Source: logging.DiagnosticSourceRuntime,
		Limit:  diagnosticsBundleMaxRecords,
	})
	if err != nil {
		return nil, internalError(err)
	}
	now := time.Now().UTC()
	var b strings.Builder
	b.WriteString("# Vivy bug report\n\n")
	fmt.Fprintf(&b, "- version: %s\n", buildinfo.Version)
	if params.SessionID != "" {
		fmt.Fprintf(&b, "- session_id: %s\n", params.SessionID)
	}
	fmt.Fprintf(&b, "- generated_at: %s\n", now.Format(time.RFC3339))
	if page.Gap {
		b.WriteString("- log_gap: true\n")
	}
	b.WriteString("\n## Runtime log tail\n\n")
	for _, record := range page.Records {
		at := "-"
		if record.At != nil {
			at = time.UnixMilli(*record.At).UTC().Format("15:04:05.000")
		}
		fmt.Fprintf(&b, "%s %-5s %s %s\n", at, record.Level, record.Component, record.Message)
	}
	if len(page.Records) == 0 {
		b.WriteString("(no runtime log records)\n")
	}
	name := fmt.Sprintf("bug-%s.md", now.Format("20060102-150405"))
	if sessionID := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, params.SessionID); sessionID != "" {
		name = fmt.Sprintf("bug-%s-%s.md", sessionID, now.Format("20060102-150405"))
	}
	if err := os.MkdirAll(h.deps.DiagnosticsBundleDir, 0o700); err != nil {
		return nil, internalError(err)
	}
	path := filepath.Join(h.deps.DiagnosticsBundleDir, name)
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return nil, internalError(err)
	}
	return map[string]any{"path": path, "records": len(page.Records)}, nil
}
