# Notes — why this shape

## The observed failures

Both child runs came from one ordinary research task on the daily host:

- `child_49e8138cc55297f3` — `http request: host "api.github.com" is not allowlisted`.
  The default `http_request` allowlist is localhost only; `web_fetch` is the tool that
  should have been used. The failure was fatal to the child, so the model never got to
  switch tools.
- `child_82d80bd25d6fb855` — `web fetch: Get "https://...": proxyconnect tcp: public
  fetch: target address is private or local`. `HTTPS_PROXY=http://127.0.0.1:7897` was
  in the environment; the public-only dialer treated the proxy itself as an SSRF
  target.

In both cases the parent stayed alive (a child failure arrives as a recoverable
`child_failed` tool result) but the child's work was lost and the parent saw only
`sub-agent failed: The child task could not be completed. Please retry.`

## Reference implementation

`.workspace/pi/packages/agent/src/agent-loop.ts` — `runToolCall` never rejects for a
tool failure: unknown tool, argument validation, a blocked call and a thrown error all
return `isError: true`, the error result is appended to context, and the loop
continues unless the provider stops/aborts or a call explicitly terminates. Its
subagent extension returns `isError: true` with
`errorMessage || stderr || finalOutput || "(no output)"` and does not force a
conclusion turn. Vivy now matches that model on a model-driven leg, with two Vivy-only
additions: the typed §5 reason vocabulary, and refusals recorded with
`effects=not_executed`.

## Eino capability check (v0.9.21)

`compose.ToolNode` (`compose/tool_node.go`) aborts on any non-interrupt tool error;
`UnknownToolsHandler` covers only missing tool names; there is no continue-on-error
switch. A `ToolMiddleware` could swallow errors but would lose per-spec classification
and the nudge/durable-operation ordering. The existing adapter seam
(`internal/runtime/tooladapter.go`, `invokeOutcome`) is therefore the minimal
compatible conversion point. No new Eino import outside `internal/runtime`; no second
runtime.

## Owner decisions

- Fatal stays limited to **runtime-integrity faults**.
- "Failure as a tool result" applies to **all model-driven legs** (parent and child).

## Design points worth remembering

- A redirect hop into a private target is a plain error (`network_failed`,
  `effects=unknown`), not a refusal: the initial request already ran, so claiming
  `not_executed` would lie about effects.
- `PrepareDownload` keeps syntax-only URL validation. Resolving DNS at approval time
  would let a transient resolver failure kill the run before a human ever decided.
- The proxy endpoint is the only private dial target allowed, and targets are still
  resolved locally before the request. Trusting every loopback dial whenever a proxy
  exists would turn the proxy into an SSRF bypass.
- `ErrHookDenied` (a hook *decision*) is a refusal; `ErrHookBlocked` (spawn, timeout,
  malformed envelope, empty or invalid rewrite) stays fatal — a broken hook chain is a
  runtime fault, not a governance answer.
- Work-admission refusals use a small `workAdmissionError` that unwraps to
  `ErrWorkRunNotAdmitted`, so the model sees the site's own wording without the shared
  sentinel text appended.

## Residual risks

- The proxy is trusted for the final hop: once a request is handed to the configured
  proxy, Vivy cannot observe which address it actually dialed. The local preflight
  resolution bounds the target, but a proxy that ignores it is outside this contract.
- `http_request` still uses its own dialer (`safeDialContext`) and host allowlist; a
  non-loopback LAN proxy is still refused there. Its allowlist already bounds targets.
- No retry/backoff on `network_failed`. The model may retry immediately, which is
  correct for a transient reset and wasteful for a hard outage.
- Restart recovery after `SQLITE_BUSY` can retry a terminal append and then log
  `run already has a terminal event` repeatedly (`child_2c0394b98e6c04c9`). Tracked as
  `RUN-RESTART-DUPLICATE-TERMINAL` in `docs/TODO.md` §0.1; deliberately not fixed in
  this lane.
