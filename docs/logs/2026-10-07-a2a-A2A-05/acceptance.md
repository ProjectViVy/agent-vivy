# A2A-05 Acceptance

- Dedicated Host-owned HTTP lifecycle per design §10/§11: real loopback
  bind, graceful drain, honest failed/stopped states; the Module never
  owns `net.Listen`/`ListenAndServe`.
- Authentication is bearer-token, constant-time, per-principal
  allowlisted; credential material stays in env references only.
- Card/discovery view safe without remote principal; task operations
  require the authenticated binding.
- G0 minimum deployment contract honored: single principal, loopback +
  reverse proxy; no TLS/multi-principal machinery added.
- No default endpoint added to ordinary Generations (opt-in per issue
  #2); protective bounds are not throughput claims.
