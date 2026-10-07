package acp

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	acp "github.com/eino-contrib/acp"
)

// controlCallTimeout bounds a single Control request the adapter issues on
// behalf of a wire request (spec §10: 10 s host control call).
const controlCallTimeout = 10 * time.Second

// controlAllowlist is the §12.2 method set the adapter may call. Anything
// else fails before reaching the host.
var controlAllowlist = map[string]struct{}{
	"initialize":       {},
	"session/create":   {},
	"turn/start":       {},
	"run/subscribe":    {},
	"run/unsubscribe":  {},
	"run/cancel":       {},
	"run/get":          {},
	"review/get":       {},
	"approval/respond": {},
	"question/respond": {},
	"review/respond":   {},
}

// call issues one allowlisted Control request with the bounded child
// timeout. It never holds adapter locks.
func (a *agent) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if _, ok := controlAllowlist[method]; !ok {
		return nil, acp.NewRPCError(-32603, "control method outside adapter allowlist",
			errorReason("REQUIRED_CAPABILITY_UNAVAILABLE"))
	}
	child, cancel := context.WithTimeout(ctx, controlCallTimeout)
	defer cancel()
	return a.host.Call(child, method, params)
}

// errorReason packs the stable adapter reason vocabulary into error data;
// reason strings live in data, never in custom methods or messages.
func errorReason(reason string) map[string]string {
	return map[string]string{"reason": reason}
}

func rpcError(code int, message, reason string) *acp.RPCError {
	return acp.NewRPCError(code, message, errorReason(reason))
}

// rpcCodeOf extracts the numeric JSON-RPC code from any error in the chain
// exposing RPCErrorCode; it never inspects message strings.
func rpcCodeOf(err error) (int, bool) {
	var coded interface{ RPCErrorCode() int }
	if errors.As(err, &coded) {
		return coded.RPCErrorCode(), true
	}
	return 0, false
}

// safeRPCError implements the §12.7 mapping: Control-side errors become the
// adapter's fixed vocabulary with a stable reason and no raw cause, path or
// stack on the wire.
func safeRPCError(err error) *acp.RPCError {
	if err == nil {
		return nil
	}
	// Deliberate adapter errors are already safe.
	var own *acp.RPCError
	if errors.As(err, &own) {
		return own
	}
	if code, ok := rpcCodeOf(err); ok {
		switch code {
		case -32602:
			return rpcError(-32602, "invalid input", "INVALID_INPUT")
		case -32004:
			return rpcError(-32002, "resource not found", "RESOURCE_NOT_FOUND")
		case -32009:
			// Non-stale conflicts (stale approval/question is consumed by
			// the interaction call sites, not here) are run admission
			// conflicts to the client.
			return rpcError(-32001, "run admission conflict", "RUN_CONFLICT")
		case -32601:
			// An allowlisted method missing on the host is a startup
			// capability failure, never a client-visible method gap.
			return rpcError(-32603, "required capability unavailable", "REQUIRED_CAPABILITY_UNAVAILABLE")
		}
	}
	return rpcError(-32603, "internal failure", "INTERNAL_FAILURE")
}
