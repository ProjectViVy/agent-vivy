# LOG-2 — Gateway HTTP access-log middleware

## What changed

The gateway mux (`/rpc` WebSocket, `/rpc/bootstrap`, `/healthz`, embedded
UI) previously served requests with no per-request log line; the 2026-08-30
logging normalization explicitly left this gap open as TODO LOG-2.

- New `internal/rpc/accesslog.go`: `AccessLogMiddleware` wraps the mux and
  emits one structured line per request — `method`, `path`, `status`,
  `duration_ms` — via the standard kernel logger (LOGGING.md §1/§5).
- Levels follow LOGGING.md §6: `info` normally, `warn` for 5xx, `debug`
  for `/healthz` (container healthcheck noise stays out of the info
  stream). No request or response payloads are ever logged (D-010).
- The response-writer wrapper implements `http.Hijacker` and `http.Flusher`
  passthrough. This is load-bearing for the WebSocket path: gorilla's
  `upgrader.Upgrade` hijacks the connection without calling `WriteHeader`,
  so the middleware reports the negotiated upgrade as `status=101`.
- Wired in `internal/app/app.go`: `http.Server{Handler: controlrpc.AccessLogMiddleware(logger, mux)}`.
- `docs/architecture/LOGGING.md`: access lines documented in §5/§6; the
  §7 deferred bullet removed.

## Explicitly not done

- No handler-level redaction behind the middleware (LOG-3 stays open;
  D-010 call-site discipline + tool-result `RedactSensitive` remain the
  boundary).
- No `vivy worker` file logging (LOG-1 stays open; per-worker sink design
  still needed).
- No request-id correlation on access lines; run/session ids are not in
  scope at the HTTP edge (the run id travels inside the RPC frames).
- No access-log config knob (e.g. opt-out); the middleware respects the
  global logging level, which was deemed sufficient.
