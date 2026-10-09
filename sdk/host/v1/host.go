// Package host is the public Go embedding surface of one sealed VIVY
// runtime (agent-diva DIVA Next contract W3-1). A native desktop host
// opens a sealed generation from its manifest, drives the trusted
// in-process control plane with Call, receives notifications through a
// blocking Next reader, and owns a bounded, context-aware Close.
//
// Ownership: exactly one live Host owns a process and its data root at a
// time. A failed Open releases the claim; a timed-out Close retains it
// because runtime resources may still be live.
package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"agent-vivy/internal/app"
	"agent-vivy/internal/config"
	"agent-vivy/internal/embedded"
	"agent-vivy/internal/logging"
	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/sdk/generation"
)

// Bounds from W3-1.
const (
	// MaxRequestBytes bounds one encoded Call frame (method + params).
	MaxRequestBytes = 4 << 20
	// MaxBatchLimit is the largest event batch one Next call may return;
	// limit 0 requests this default.
	MaxBatchLimit = 500
	// CloseBudget is the initial desktop teardown budget applied to Close
	// when the caller's context carries no deadline.
	CloseBudget = 5 * time.Second
)

// Error kinds and their wire codes (W3-1). Kind "rpc" preserves the
// upstream JSON-RPC code verbatim instead of mapping to a host code.
const (
	KindInvalidInput           = "invalid_input"
	KindNotReady               = "not_ready"
	KindClosed                 = "closed"
	KindAlreadyInitialized     = "already_initialized"
	KindTimeout                = "timeout"
	KindCancelled              = "cancelled"
	KindTransportLost          = "transport_lost"
	KindInternal               = "internal"
	KindIncompatibleGeneration = "incompatible_generation"
	KindRPC                    = "rpc"

	CodeInvalidInput           = -32602
	CodeNotReady               = -32080
	CodeClosed                 = -32081
	CodeAlreadyInitialized     = -32082
	CodeTimeout                = -32083
	CodeCancelled              = -32084
	CodeTransportLost          = -32085
	CodeInternal               = -32086
	CodeIncompatibleGeneration = -32087
)

// Options selects the runtime variant to open. ConfigPath is mandatory and
// must be absolute; WithoutEars composes the runtime without channel
// hosts when the generation's channel contract allows it.
type Options struct {
	ConfigPath  string
	WithoutEars bool
}

// Notification is one server-initiated control-plane message, exactly as
// the control peer delivered it.
type Notification struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// EventBatch is one drain of the notification queue. Gap is sticky: it is
// true when events were dropped due to queue overflow since the last
// reported gap.
type EventBatch struct {
	Events []Notification `json:"events"`
	Gap    bool           `json:"gap"`
}

// Error is the single normalized error the host returns. Kind classifies
// the failure; Code is the wire code (or the preserved RPC code when Kind
// is "rpc"); Data carries safe RPC error payload when present.
type Error struct {
	Kind    string          `json:"kind"`
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("host: %s (code %d): %s", e.Kind, e.Code, e.Message)
}

// Host is the single process-resident runtime owner. It is safe for
// concurrent use: Call may run from many goroutines, Next admits exactly
// one reader at a time, and Close is idempotent.
type Host struct {
	inner          *embedded.Host
	previousLogger *slog.Logger
	ownedLogger    *slog.Logger

	reader      atomic.Bool
	releaseOnce sync.Once
	release     func()
	logCloser   io.Closer
}

var (
	ownerMu             sync.Mutex
	ownerHeld           bool
	openEmbeddedRuntime = embedded.Open
)

func hostError(kind string, code int, format string, args ...any) *Error {
	return &Error{Kind: kind, Code: code, Message: fmt.Sprintf(format, args...)}
}

func restoreDefaultLogger(owned, previous *slog.Logger) {
	if owned != nil && slog.Default() == owned {
		slog.SetDefault(previous)
	}
}

// Open claims the single runtime owner slot, reads and validates the
// absolute config path inside VIVY, requires a sealed embedded generation
// (missing or mismatched manifests are startup errors), and composes the
// embedded runtime. Any failure releases the owner claim.
func Open(ctx context.Context, options Options) (*Host, error) {
	if strings.TrimSpace(options.ConfigPath) == "" {
		return nil, hostError(KindInvalidInput, CodeInvalidInput, "config path is required")
	}
	if !filepath.IsAbs(options.ConfigPath) {
		return nil, hostError(KindInvalidInput, CodeInvalidInput, "config path must be absolute: %q", options.ConfigPath)
	}
	manifest, err := generation.EmbeddedManifest()
	if err != nil {
		return nil, hostError(KindIncompatibleGeneration, CodeIncompatibleGeneration, "sealed generation manifest unavailable: %v", err)
	}
	generationID, _, err := generation.InspectManifestProvenance(manifest)
	if err != nil {
		return nil, hostError(KindIncompatibleGeneration, CodeIncompatibleGeneration, "sealed generation manifest invalid: %v", err)
	}

	ownerMu.Lock()
	if ownerHeld {
		ownerMu.Unlock()
		return nil, hostError(KindAlreadyInitialized, CodeAlreadyInitialized, "a VIVY runtime is already open in this process")
	}
	ownerHeld = true
	ownerMu.Unlock()
	release := func() {
		ownerMu.Lock()
		ownerHeld = false
		ownerMu.Unlock()
	}

	cfg, err := config.Load(options.ConfigPath)
	if err != nil {
		release()
		return nil, hostError(KindInvalidInput, CodeInvalidInput, "load config: %v", err)
	}
	logger, effective, logCloser, err := logging.Setup(logging.Options{
		Level:         cfg.Logging.Level,
		Format:        cfg.Logging.Format,
		ConsoleFormat: cfg.Logging.ConsoleFormat,
		Dir:           cfg.LogDirectory(),
		RetentionDays: cfg.Logging.RetentionDays,
		Stdout:        cfg.Logging.Stdout,
	})
	if err != nil {
		release()
		return nil, hostError(KindInternal, CodeInternal, "init logging: %v", err)
	}
	previousLogger := slog.Default()
	slog.SetDefault(logger)

	appOptions := []app.AppOption{app.WithLogger(logger)}
	if options.WithoutEars {
		appOptions = append(appOptions, app.WithoutEars())
	}
	inner, err := openEmbeddedRuntime(ctx, cfg, embedded.Options{AppOptions: appOptions})
	if err != nil {
		_ = logCloser.Close()
		restoreDefaultLogger(logger, previousLogger)
		release()
		return nil, mapError(fmt.Errorf("open runtime: %w", err))
	}
	logger.Info("vivy host logging initialized",
		"level", effective.Level, "format", effective.Format, "console", effective.Console)
	logger.Info("vivy host opened", "generation", generationID)
	return &Host{
		inner: inner, release: release, logCloser: logCloser,
		previousLogger: previousLogger, ownedLogger: logger,
	}, nil
}

// Call issues one control-plane request and returns the raw result.
// Requests larger than MaxRequestBytes are rejected before dispatch. When
// ctx expires after admission the request may already have mutated state;
// the error reports the timeout, never a guessed outcome.
func (h *Host) Call(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	if len(method)+len(params) > MaxRequestBytes {
		return nil, hostError(KindInvalidInput, CodeInvalidInput, "request exceeds %d bytes", MaxRequestBytes)
	}
	result, err := h.inner.Call(ctx, method, params)
	if err != nil {
		return nil, mapError(err)
	}
	return result, nil
}

// Next blocks until at least one notification or a sticky gap is pending,
// the host closes, or ctx is done; it then drains up to limit events.
// limit 0 defaults to MaxBatchLimit; a limit outside 1..MaxBatchLimit is an
// input error. Exactly one reader may call Next at a time.
func (h *Host) Next(ctx context.Context, limit int) (EventBatch, error) {
	if limit < 0 || limit > MaxBatchLimit {
		return EventBatch{}, hostError(KindInvalidInput, CodeInvalidInput, "limit must be within 0..%d", MaxBatchLimit)
	}
	if limit == 0 {
		limit = MaxBatchLimit
	}
	if !h.reader.CompareAndSwap(false, true) {
		return EventBatch{}, hostError(KindInvalidInput, CodeInvalidInput, "Next admits exactly one reader at a time")
	}
	defer h.reader.Store(false)
	res, err := h.inner.Next(ctx, limit)
	if err != nil {
		return EventBatch{}, mapError(err)
	}
	out := EventBatch{Gap: res.Gap}
	if len(res.Events) > 0 {
		out.Events = make([]Notification, len(res.Events))
		for i, n := range res.Events {
			out.Events[i] = Notification{Method: n.Method, Params: n.Params}
		}
	}
	return out, nil
}

// Close unwinds the peer and the runtime once. A caller without a deadline
// gets the default CloseBudget; on expiry the returned timeout names the
// component still unwinding and the process claim is retained, so the
// runtime cannot be reopened over live resources. Teardown continues in
// the background and a later Close observes its completion.
func (h *Host) Close(ctx context.Context) error {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, CloseBudget)
		defer cancel()
	}
	err := h.inner.CloseContext(ctx)
	if err == nil {
		h.releaseOnce.Do(func() {
			restoreDefaultLogger(h.ownedLogger, h.previousLogger)
			// The sinks belong to this host's lifetime: closing them on
			// Windows releases the rotated log file so profile dirs can be
			// removed; a later Open builds fresh sinks.
			if h.logCloser != nil {
				_ = h.logCloser.Close()
			}
			h.release()
		})
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return mapError(err)
	}
	return &Error{Kind: KindInternal, Code: CodeInternal, Message: err.Error()}
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	var herr *Error
	if errors.As(err, &herr) {
		return err
	}
	var rpcErr *controlrpc.Error
	if errors.As(err, &rpcErr) {
		return &Error{Kind: KindRPC, Code: rpcErr.Code, Message: rpcErr.Message, Data: rpcErr.Data}
	}
	switch {
	case errors.Is(err, embedded.ErrClosed):
		return hostError(KindClosed, CodeClosed, "host is closed")
	case errors.Is(err, context.Canceled):
		return hostError(KindCancelled, CodeCancelled, "operation cancelled")
	case errors.Is(err, context.DeadlineExceeded):
		return hostError(KindTimeout, CodeTimeout, "operation timed out: %v", err)
	case errors.Is(err, controlrpc.ErrPeerClosed):
		return hostError(KindTransportLost, CodeTransportLost, "control transport lost")
	default:
		return &Error{Kind: KindInternal, Code: CodeInternal, Message: err.Error()}
	}
}
