# Tool failures as model-visible results, proxy-aware web tools, bounded child cause

Date: 2026-10-24 · Branch: `fix/tool-failure-continuation` (from `stable` @ `9697dd0e`)

## What changed

Two child runs died during one research task and the parent only saw
`sub-agent failed: The child task could not be completed. Please retry.`:

- `child_49e8138cc55297f3` — `http request: host "api.github.com" is not allowlisted`
  (the default `http_request` allowlist is localhost only).
- `child_82d80bd25d6fb855` — `web fetch: ... proxyconnect tcp: public fetch: target
  address is private or local`. The host exports `HTTPS_PROXY=http://127.0.0.1:7897`;
  the public-only dialer saw the proxy's own loopback address and refused it.

Neither child got a second model turn: Eino's `ToolsNode` escalates every
non-interrupt tool error into a run-fatal `NodeRunError`, and Vivy only
soft-converted a short typed allowlist.

Three fixes, one delivery:

1. **Tool failure is data on a model-driven leg.** `classifyModelLegToolFailure`
   (`internal/runtime/tool_failure.go`) keeps the typed allowlist, then converts any
   remaining invocation error into a bounded, redacted `tool.finished` result:
   `recoverable/network_failed` for `*url.Error` and `net.Error` (including a per-call
   timeout whose parent is still live), `recoverable/tool_error` otherwise, both with
   `effects=unknown`. Only control flow (parent cancellation/deadline, native Eino
   interrupts), explicit governance refusals and runtime-integrity faults stay fatal.
   Per-call governance boundaries became refusals with `effects=not_executed`: a tool
   outside the run selection, a tool-hook `deny` (new `ErrHookDenied`), and a work
   action the run is not admitted for (new `ErrWorkRunNotAdmitted`). The
   `http_request` host-allowlist denial is now wrapped with `ErrPolicyDenied`, so the
   model is told the host is not allowlisted instead of losing the run.
2. **Proxy-aware public network.** New `internal/runtime/web_network.go` owns the
   outbound policy for `web_fetch` and `download`: it snapshots
   `httpproxy.FromEnvironment()`, allows exactly the configured proxy endpoints as
   dial targets, and still resolves every target locally — private/local targets are
   refused before a request is sent, and each redirect hop is revalidated. A
   credential-bearing or non-HTTP(S) URL is now a typed `*tools.ArgError`, so it is
   `invalid_arguments` in both the normal and the preflight path.
3. **Bounded child cause.** `child.failed` carries an optional `detail`
   (≤1024 bytes, redacted) except when the cause is already a provider-specific
   message; `ChildRunDetails` composes it as `message + "\nCause: " + detail`, so a
   fatal child hands the parent a readable cause instead of only a generic sentence.

New reason vocabulary lives in `internal/domain/tool_operation.go` and is accepted by
`internal/storage/tool_operations.go`; there is no second list. `schemas/events/payloads/child.failed.json`
gained the optional `detail` field (additive, no version bump).

`docs/architecture/NUDGE-DESIGN.md` §5 now records the full classification table, the
pi `isError` tool-result model this follows, and the remaining fatal set.

## Explicitly not done

- No retry/backoff policy for `network_failed`; the model decides whether to retry.
- `http_request` still uses its own dialer and host allowlist; a non-loopback LAN
  proxy remains refused there.
- A fatal child's partial transcript is not extracted as the parent's result.
- The separately observed restart-recovery defect (`SQLITE_BUSY`, then repeated
  `run already has a terminal event`, `child_2c0394b98e6c04c9`) is recorded on the
  open board, not fixed here.
