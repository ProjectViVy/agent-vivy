# A2A SDK probe — official client vs custom RequestHandler

Evidence for plan A2A-00, task .1 (issue #2). Probe module:
`sdk/testdata/a2a-probe/` — own `go.mod`, nested module boundary: `go list ./sdk/...`
does not include it, so the probe can never ship inside the Vivy binary.

- SDK: `github.com/a2aproject/a2a-go/v2 v2.6.0` (unchanged since design pin)
- Go: `go1.26.4` directive, toolchain go1.26.8
- Verification commands:

  ```
  cd sdk/testdata/a2a-probe
  go mod tidy            # GOPROXY=https://goproxy.cn,direct on this box
  go vet ./...
  go test ./... -count=1 -v
  ```

  Result: 26 PASS, 0 FAIL, 0 SKIP (2026-10-07).

## Fixture of record

`TestA2ACustomHandlerOfficialClient` (`handler_test.go`) mounts a custom
`a2asrv.RequestHandler` (`probeHandler`, in `probe_support_test.go`) through
`a2asrv.NewJSONRPCHandler` and drives it with `a2aclient.NewFromCard`.

Subtests: compile (all 11 RequestHandler methods), send, stream, interrupt
(input-required + auth-required close the stream cleanly), terminal,
validation, middleware, errors, limits.

`TestA2AJSONRPCWireCompatibility` (`compatibility_test.go`) drives raw POSTs:
malformed JSON → -32700, wrong `jsonrpc` field → -32600, unknown method →
-32601, non-POST → JSONRPC error body, service params reach the handler,
`params.tenant` passes through verbatim.

`TestA2AEventSurfaceContract` confirms the v1.0 event surface carries no
durable resume cursor or sequence field — relevant to the G0 replay decision
(A2A-R1 stays conditional on owner choosing exact replay).

## Observed event sequences

`SendStreamingMessage` on a scripted `Task → statusUpdate(working) →
artifactUpdate → statusUpdate(completed)` arrives in order; first item is the
`Task`. `input_required`/`auth_required` status updates arrive then the stream
closes cleanly (iterator ends, no error). Mid-stream errors arrive as
JSONRPC error frames inside the SSE data stream and decode to typed `*a2a.Error`
(errors.Is works against `a2a.Err*` sentinels).

## Adapter findings (what the real server must own)

1. **Interceptors on a custom handler need `InterceptedHandler` directly.**
   `a2asrv.WithCallInterceptors` is a `RequestHandlerOption` bound to the stock
   `NewHandler(executor, ...)` pipeline — it dereferences the internal
   `defaultRequestHandler` and cannot be passed to `NewJSONRPCHandler`
   (which takes `TransportOption`). A custom handler composes interceptors via
   `&a2asrv.InterceptedHandler{Handler: h, Interceptors: [...]}` then feeds
   that to `NewJSONRPCHandler`. Verified working in `newGuardedServer`.

2. **CallContext is attached twice, asymmetrically.**
   The transport does `NewCallContext(ctx, NewServiceParams(req.Header))` in
   `ServeHTTP` — so even a bare handler sees `ServiceParams` (A2A-* headers,
   lowercased keys). But `CallContext.Tenant()` is only populated by
   `InterceptedHandler.attachMethodCallContext` copying `req.Tenant`. A bare
   handler must read `req.Tenant` itself; an intercepted one can use either.

3. **Streaming is gated by the agent card.** The official client silently
   downgrades `SendStreamingMessage` to `SendMessage` when
   `card.Capabilities.Streaming` is false. VIVY's card must declare
   `Streaming: true` or streaming callers get a single non-streaming result.

4. **v1.0 JSONRPC method names are PascalCase.** The v2 transport dispatches
   `SendMessage`, `SendStreamingMessage`, `GetTask`, `ListTasks`,
   `CancelTask`, `SubscribeToTask`, `{Get,Create,List,Delete}TaskPushNotificationConfig`,
   `GetExtendedAgentCard`. The spec-0.3 names (`message/send`, `tasks/get`, …)
   live only in `a2acompat/a2av0` — a separate compat layer we do not mount.
   Non-SDK v0.3 clients will not interop without that layer (G0 decision if
   ever needed; v1.0 clients use the PascalCase names).

5. **Version enforcement is adapter-side.** The transport does not reject
   `A2A-Version != 1.0`; the probe guard interceptor does. Same for required
   extensions (`callCtx.Extensions().RequestedURIs()`).

6. **No size limits in the typed path.** Non-streaming responses are decoded
   with an unbounded `json.Decoder`. SSE client-side frames are capped at
   `internal/sse.MaxSSETokenSize` = 10 MiB; oversize surfaces as a stream error,
   never partial delivery (verified by `limits` subtest). Server-side request
   limits are likewise the host's job (VIVY HTTP seam).

7. **Auth never enters the SDK layer.** Host middleware validates the bearer,
   strips it, and attaches a private context value; the handler observed the
   marker and an empty `authorization` service param. A 401 JSONRPC body before
   the transport yields a client-side error without invoking the handler.

8. **Panic surface.** `TransportConfig.PanicHandler` + `KeepAliveInterval` are
   the only `TransportOption`s; handler panics propagate unless configured —
   the real adapter should set a panic handler that maps to
   `ErrInternalError`.

9. **Unused-but-required methods.** Push-config and extended-card methods are
   mandatory on the interface; returning `ErrPushNotificationNotSupported` /
   `ErrExtendedCardNotConfigured` is a valid, wire-correct answer.

## Forbidden-path compliance

The probe uses only `a2asrv.RequestHandler` + `NewJSONRPCHandler` +
`InterceptedHandler` — no `NewHandler`, `AgentExecutor`, `TaskStore`, event
queue, or execution engine. No second runner exists anywhere in the probe.

## Open items for A2A-02+

- SSE keep-alive interval (`WithTransportKeepAlive`) exists; whether VIVY needs
  it under the deployment decision (loopback vs multi-principal) is a G0-owned
  question, not measured here.
- `SubscribeToTask` re-attach semantics (A2A-R1 exact replay) remain unprobed
  by design — conditional on owner choosing B.
