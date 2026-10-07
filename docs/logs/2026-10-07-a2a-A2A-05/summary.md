# A2A-05 Summary — Host HTTP lifecycle, authentication and discovery view

## .1 feat(channelhost): authenticate dedicated task HTTP ingress — 137b7817

- `internal/config`: typed `channels.<name>.http` settings (listen
  address, public_base_url, principal {id, token_env}, protective
  bounds). Strict validation: unknown keys rejected, bounds checked,
  token resolved through env reference only — raw token values never
  appear in config, logs or Inspect.
- `internal/channelhost/http.go`: dedicated ingress mounts `POST /a2a`
  plus `GET /.well-known/agent-card.json` only when the channel owns an
  `channel.a2a` grant binding and a task surface is present.
- Auth: constant-time bearer comparison, per-principal allowlisting,
  safe correlation errors (no oracle on unknown/invalid credentials).
  `A2A-Version` preserved; `Authorization` and untrusted forwarding/
  identity headers stripped before dispatch — `X-Forwarded-*` is never
  treated as authority. Body bounds enforced before decode.
- Private `TaskPrincipal` binding injected from the authenticated
  principal; request never supplies its own identity.

## .2 feat(channelhost): govern task listener lifecycle and limits — 1c4f8f0a

- Listener state machine absent → inactive → starting → serving →
  draining → stopped/failed, owned by the Host; app startup failures
  close listeners in reverse ownership order.
- Bounded writer path preserving `Flusher`/`ResponseController` for
  SSE; no whole-stream `WriteTimeout` (keepalive + reauthorization
  before writes instead). 1 MiB complete-frame limit enforced before
  partial emission.
- Rate limiting: discovery route gets a bounded public bucket; RPC
  routes get authenticated per-principal buckets. No wildcard CORS.
- Token rotation: credential change revokes prior streams on listener
  restart while preserving principal ownership.
- Inspect distinguishes compiled / granted / wired / started states and
  reports effective bounds; credentials and filesystem paths redacted.
