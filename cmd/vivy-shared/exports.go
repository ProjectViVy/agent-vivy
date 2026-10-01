// Command vivy-shared builds the sealed DIVA shared library
// (-buildmode=c-shared): one embedded.Host per process behind a small C ABI.
//
// ABI v1 (agent-diva docs/plans/diva-next/backend-separation-contracts.md §4):
//   - every non-null return is UTF-8, NUL-terminated, library-allocated;
//     the host copies it once and calls VivyFree.
//   - no Go pointer crosses FFI: handles are table indices.
//   - envelope: {"ok":true,"value":{...}} | {"ok":false,"error":{kind,code,
//     message,data?}}.
//   - no callbacks, no dynamic unload; VivyShutdown is the only teardown.
//
// Panic recovery converts ordinary wrapper panics into a redacted internal
// error; it does not claim recovery from fatal Go runtime errors.
package main

/*
#include <stdlib.h>
#include <stdint.h>
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"agent-vivy/internal/app"
	"agent-vivy/internal/config"
	"agent-vivy/internal/embedded"
	controlrpc "agent-vivy/internal/rpc"
	// Links sdk/generation so the sealed Generation Manifest the pack overlay
	// writes into EmbeddedManifestBase64 ships inside the library; without it
	// inspect-artifact cannot verify the artifact and control actions stay
	// unsealed.
	_ "agent-vivy/sdk/generation"
)

const (
	// abiVersion must equal VIVY_ABI_VERSION in vivy_abi.h; the header is
	// the source the Rust bridge asserts against.
	abiVersion = embedded.ABIVersion

	maxInputBytes    = 4 << 20 // DN-0: input frame ≤ 4 MiB
	defaultTimeoutMs = 120_000 // DN-0: VivyCall default timeout
	maxPollEvents    = embedded.DefaultPollLimit
)

type envelope struct {
	OK    bool             `json:"ok"`
	Value json.RawMessage  `json:"value,omitempty"`
	Err   *envelopeError   `json:"error,omitempty"`
}

type envelopeError struct {
	Kind    string          `json:"kind"`
	Code    int             `json:"code,omitempty"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func okEnvelope(value any) *C.char {
	raw, err := json.Marshal(value)
	if err != nil {
		return failEnvelope("internal", 0, "marshal result")
	}
	out, err := json.Marshal(envelope{OK: true, Value: raw})
	if err != nil {
		return failEnvelope("internal", 0, "marshal envelope")
	}
	return C.CString(string(out))
}

func failEnvelope(kind string, code int, message string) *C.char {
	out, err := json.Marshal(envelope{OK: false, Err: &envelopeError{Kind: kind, Code: code, Message: message}})
	if err != nil {
		return C.CString(`{"ok":false,"error":{"kind":"internal","message":"envelope marshal"}}`)
	}
	return C.CString(string(out))
}

var (
	handles   sync.Map // uint64 -> *embedded.Host
	nextID    atomic.Uint64
	initMu    sync.Mutex
	initHeld  uint64
)

func lookup(handle C.uint64_t) (*embedded.Host, *C.char) {
	v, ok := handles.Load(uint64(handle))
	if !ok {
		return nil, failEnvelope("closed", 0, "unknown or closed handle")
	}
	return v.(*embedded.Host), nil
}

// initParams is the single VivyInit JSON input.
type initParams struct {
	ConfigPath string `json:"config_path"`
	ABIVersion uint32 `json:"abi_version"`
	// WithoutEars disables the channel Host; DIVA owns channel traffic
	// itself, so the default recipe is ears-less unless the recipe's
	// channel contract says otherwise.
	WithoutEars *bool `json:"without_ears,omitempty"`
}

//export VivyInit
func VivyInit(initJSON *C.char) (out *C.char) {
	defer func() {
		if r := recover(); r != nil {
			out = failEnvelope("internal", 0, "panic in VivyInit")
		}
	}()
	if initJSON == nil {
		return failEnvelope("invalid_input", 0, "init params required")
	}
	raw := C.GoString(initJSON)
	if len(raw) > maxInputBytes {
		return failEnvelope("invalid_input", 0, "init input exceeds frame bound")
	}
	var p initParams
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return failEnvelope("invalid_input", 0, "init params are not valid JSON")
	}
	if p.ABIVersion != abiVersion {
		return failEnvelope("incompatible_abi", 0, fmt.Sprintf("abi_version %d, want %d", p.ABIVersion, abiVersion))
	}
	if p.ConfigPath == "" {
		return failEnvelope("invalid_input", 0, "config_path required")
	}
	initMu.Lock()
	defer initMu.Unlock()
	if initHeld != 0 {
		return failEnvelope("already_initialized", 0, "a host is already initialized")
	}
	cfg, err := config.Load(p.ConfigPath)
	if err != nil {
		return failEnvelope("invalid_input", 0, "config load failed")
	}
	opts := embedded.Options{}
	ears := true
	if p.WithoutEars != nil {
		ears = !*p.WithoutEars
	}
	if !ears {
		opts.AppOptions = append(opts.AppOptions, app.WithoutEars())
	}
	host, err := embedded.Open(context.Background(), cfg, opts)
	if err != nil {
		return failEnvelope("invalid_input", 0, fmt.Sprintf("open host: %v", err))
	}
	id := nextID.Add(1)
	handles.Store(id, host)
	initHeld = id
	return okEnvelope(map[string]any{"abi_version": abiVersion, "handle": id})
}

// callRequest is the single VivyCall JSON input: a JSON-RPC method+params
// frame plus an optional per-call timeout override.
type callRequest struct {
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params"`
	TimeoutMs uint32          `json:"timeout_ms,omitempty"`
}

//export VivyCall
func VivyCall(handle C.uint64_t, requestJSON *C.char) (out *C.char) {
	defer func() {
		if r := recover(); r != nil {
			out = failEnvelope("internal", 0, "panic in VivyCall")
		}
	}()
	host, bad := lookup(handle)
	if bad != nil {
		return bad
	}
	if requestJSON == nil {
		return failEnvelope("invalid_input", 0, "request required")
	}
	raw := C.GoString(requestJSON)
	if len(raw) > maxInputBytes {
		return failEnvelope("invalid_input", 0, "request exceeds frame bound")
	}
	var req callRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil || req.Method == "" {
		return failEnvelope("invalid_input", 0, "request must be JSON {method, params?}")
	}
	timeout := time.Duration(defaultTimeoutMs) * time.Millisecond
	if req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	result, err := host.Call(ctx, req.Method, req.Params)
	if err != nil {
		return failEnvelope(classifyCallError(err), rpcErrorCode(err), err.Error())
	}
	return okEnvelope(json.RawMessage(result))
}

func classifyCallError(err error) string {
	var rpcErr *controlrpc.Error
	switch {
	case errors.Is(err, embedded.ErrClosed):
		return "closed"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.As(err, &rpcErr) && rpcErr.Code == controlrpc.CodeNotFound:
		return "invalid_input"
	case errors.As(err, &rpcErr):
		return "internal"
	default:
		return "transport_lost"
	}
}

func rpcErrorCode(err error) int {
	var rpcErr *controlrpc.Error
	if errors.As(err, &rpcErr) {
		return rpcErr.Code
	}
	return 0
}

//export VivyPollEvents
func VivyPollEvents(handle C.uint64_t, maxEvents C.uint32_t) (out *C.char) {
	defer func() {
		if r := recover(); r != nil {
			out = failEnvelope("internal", 0, "panic in VivyPollEvents")
		}
	}()
	host, bad := lookup(handle)
	if bad != nil {
		return bad
	}
	limit := int(maxEvents)
	if maxEvents == 0 || limit > maxPollEvents {
		limit = maxPollEvents
	}
	res, err := host.Poll(context.Background(), limit)
	if err != nil {
		return failEnvelope(classifyCallError(err), 0, err.Error())
	}
	return okEnvelope(res)
}

//export VivyShutdown
func VivyShutdown(handle C.uint64_t) (out *C.char) {
	defer func() {
		if r := recover(); r != nil {
			out = failEnvelope("internal", 0, "panic in VivyShutdown")
		}
	}()
	v, loaded := handles.LoadAndDelete(uint64(handle))
	if !loaded {
		return failEnvelope("closed", 0, "unknown or closed handle")
	}
	initMu.Lock()
	if initHeld == uint64(handle) {
		initHeld = 0
	}
	initMu.Unlock()
	if err := v.(*embedded.Host).Close(); err != nil {
		return failEnvelope("internal", 0, err.Error())
	}
	return okEnvelope(map[string]any{"shutdown": true})
}

//export VivyFree
func VivyFree(ptr *C.char) {
	if ptr != nil {
		C.free(unsafe.Pointer(ptr))
	}
}

func main() {}
