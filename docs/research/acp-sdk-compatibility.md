# ACP SDK compatibility verdict — eino-contrib/acp v0.0.4

ACP-01 (G0) executable evidence for issue #1, evaluated against
`docs/superpowers/specs/2026-10-07-acp-stdio-face-design.md` §12.8.
Evidence = probe runs plus inspected SDK source lines at the pinned
revision; anything not directly executed is marked *source-inspected*.

## Pinned revision

| Item | Value |
|---|---|
| Module | `github.com/eino-contrib/acp` |
| Version | `v0.0.4` (commit `0735f2517ced1c8195f906bb70949ee6f65ceea5`, tagged 2026-06-29) |
| Sum | `h1:cH+R3wptEiLoVLq4sT7CuBD1Xyn47EHhJiHBSdyyS9E=` |
| Probe toolchain | `go1.26.8 linux/amd64` |
| Probe location | `sdk/internal/testdata/acp-compat` — isolated module (`module acpcompat`); root `go.mod` untouched |
| Protocol baseline exercised | ACP wire version 1 (initialize `protocolVersion == 1`), schema-v1.21.0 |

Probe layout: `agent_test.go` (probe agent embedding `acp.BaseAgent`
overriding only the four pilot handlers + stdlib NDJSON wire peer over
pipes), `wire_test.go` (wire-surface assertions), `transport_test.go`
(concurrency, transport boundary, diagnostics), `testdata/wire-cases.json`
(raw wire fixtures).

## Verdict summary

**v0.0.4 is not release-ready as-is.** Six of ten §12.8 rows pass, three
fail on public-API reachability or wire-error safety, and one records a
real admission-ordering race that the pilot contract must own. All
failures have small upstream fixes; none requires reimplementing the
dispatcher or vendoring schema code.

| §12.8 area | Verdict | Evidence |
|---|---|---|
| Main methods | **PASS** | Typed `initialize`/`session/new`/`session/prompt` + reverse `session/update`, `session/request_permission`, `elicitation/create` exercised against the real pin |
| Elicitation | **PASS** | `mode=form` on wire; accept/decline/cancel decode distinctly; unknown/missing `action` fails closed (zero answered/approved counters) |
| Intake cap | **FAIL** | No public setter: options are `internal/jsonrpc.ConnectionOption`; 4096-frame default unreachable |
| Worker pool | **PASS** | 8 workers drain a 200-message flood with all handlers parked; 4-occupied-prompt latch works; per-method caps must be adapter-side |
| Admission ordering | **FINDING** | Under `-race` the cancel handler can run *before* the prompt handler; only a TTL-bounded unconditional latch is loss-free |
| Shutdown | **FAIL** | `shutdownTimeout` default 30s is internal-only; a parked handler blocks `Close()` past the proposed 10s budget |
| stdio write | **PASS w/ contract** | Write deadline surfaces `context deadline exceeded`; write failure does **not** auto-teardown — adapter must close on write error |
| Wire errors | **FAIL** | `originError` leaks handler error text; panic path serializes a full goroutine stack to the wire |
| Logging | **PASS** | `acp.SetLogger(l, acp.LevelDisabled)` emits zero bytes; (default Debug logs complete raw frames — must stay disabled) |
| Link closure | **PASS** | `go list -test -deps` shows no `net/http`/WS/proxy/Hertz/Gin in the probe closure |

## Required outcomes, row by row

### Main methods — PASS

Command: `go test -run 'TestWireRoundTrip$' -count=1 -v`

Observed on the wire (via DEBUG access log, then replayed through
assertions):

- `initialize` → `result.protocolVersion == 1`.
- `session/new` with `"mcpServers":[]` → accepted; agent's test-owned
  `sessionId` returned verbatim.
- `session/prompt` → agent emitted `session/update` notification and
  `session/request_permission` before returning; the `session/update`
  envelope was observed *before* the prompt response on the wire log.
- Permission reply `{"outcome":{"outcome":"selected","optionId":"allow-once"}}`
  decoded to `RequestPermissionOutcome.Selected.OptionID == "allow-once"`
  (exact preservation). Note the double-nested `outcome` — the response
  result wraps the discriminated union inside its own `outcome` field.

### Elicitation — PASS

Command: `go test -run 'TestUnknownElicitationActionFailsClosed$' -count=1 -v`

- `UnstableCreateElicitation` emits `elicitation/create` with `"mode":"form"`,
  `message`, `requestedSchema` — matches schema-v1.21.0 despite the
  `Unstable` API name.
- Replies `{"action":"accept","content":{...}}`, `{"action":"decline"}`,
  `{"action":"cancel"}` decode into distinct response variants.
- `{"action":"approve_all",...}` → response decode error
  (`unknown discriminator value: approve_all`), no variant set.
- Missing `action` → decode error (`missing discriminator action`).
- In both failure cases the probe's answered/approved counters stayed 0 —
  fail-closed confirmed.

### Intake cap — FAIL (public surface)

Source-inspected: `internal/jsonrpc/connection.go:113`
`c.maxPendingDispatch = n` and the option type `jsonrpc.ConnectionOption`
— `conn.NewAgentConnectionFromTransport(agent, transport, opts
...jsonrpc.ConnectionOption)` references an internal package, so no option
value can be constructed outside the SDK module. Defaults
(source-inspected): `requestWorkers=8`, `maxPendingDispatch=4096`,
`shutdownTimeout=30s`.

Probe confirmation (`flood-shared-queue`): 8 workers parked on prompts +
200 queued `session/cancel` notifications all delivered, conn stayed
healthy — the 4096 queue exists and works, but 64 pending frames cannot
be enforced before handlers with the public API.

Smallest upstream change: export option constructors from `conn` (e.g.
`conn.WithMaxPendingDispatch(n)`, `conn.WithRequestWorkers(n)`,
`conn.WithShutdownTimeout(d)`, `conn.WithRequestTimeout(d)`) aliasing the
existing internal options — spec-preferred "publishing aliases for
existing connection options".

### Worker pool — PASS (adapter-side cap)

Command: `go test -run 'Test(PromptCancelAdmissionOrder|TransportBoundaries)$'`
+ `flood-shared-queue` evidence above. The pilot's "4 active prompts"
bound is an adapter admission decision — the SDK exposes no per-method
concurrency knob (all handlers share the 8-worker pool); implementing it
in the adapter is straightforward and keeps the SDK contract unchanged.

### Admission ordering — FINDING (contract decision required)

Command: `go test -race -run TestPromptCancelAdmissionOrder -count=20 -timeout=2m`

First probe model (latch gated on `turnActive` inside the prompt handler)
was loss-free in normal runs but **lost real cancels under `-race`**:
the `session/cancel` notification handler executed before the
`session/prompt` handler entered, so at cancel-dispatch time the session
showed no in-flight turn — indistinguishable from an idle cancel at the
SDK boundary.

Final model — unconditional latch with bounded TTL (mirroring the spec's
proposed 5s pending-cancel bound): `SessionCancel` always records
`latched=true, latchedAt=now`; prompt admission consumes the latch iff
fresh. Verified:

- prompt parked pre-slot + cancel → prompt resolves `cancelled` (1 and 4
  occupied sessions, 20 race iterations clean);
- cancel between turns then immediate prompt → cancelled (the latch is
  prompt-agnostic within TTL — recorded contract consequence);
- idle cancel aged past TTL then prompt → `end_turn` (hazard bounded).

The strict spec wording "idle cancel never applies to a later prompt" is
*unimplementable at the SDK handler boundary* — the two wire cases are
indistinguishable there. The TTL latch bounds the misapplication window
to the latch TTL; the contract must name that bound explicitly (spec §10
proposes 5s).

### Shutdown — FAIL (public surface)

`shutdown-vs-10s-budget`: `conn.Close()` with a parked prompt handler was
still blocked at 2s; per source it waits up to `shutdownTimeout` (30s
internal default) before abandoning — **over the proposed 10s launch
budget and not publicly tunable**. `shutdown-cooperative`: idle `Close()`
latency ~38µs. Additional caveat from source (connection.go:455-483):
when the timeout is hit, `Close` abandons the wait but the waiter
goroutine itself leaks until handlers exit.

Smallest upstream change: same option-alias export
(`conn.WithShutdownTimeout`); plus the adapter must drive prompt
cancellation through handler ctx so handlers return promptly.

### stdio write — PASS with adapter contract

`blocked-stdout` (real `os.Pipe`, nobody drains the read end): after the
~64KiB kernel buffer fills, `conn.SessionUpdate` with a 500ms caller
deadline surfaced `context deadline exceeded` — no fabricated success.
`broken-pipe`: after closing the read end, `SessionUpdate` surfaced
`write |1: broken pipe`, **and the connection stayed open** — a write
failure is not connection-fatal on its own; the adapter must treat any
write error/timeout as fatal and close the stream itself (matches spec
§12.9's "never emit a replacement success").

### Wire errors — FAIL

Command: `go test -run 'TestSDKWireErrorsAndLogging$' -count=1 -v` with
canaries `SK-CANARY-9f8e7d6c5b` and `/home/ci/secrets/vault-CANARY.json`.

| Case | Wire error payload | Verdict |
|---|---|---|
| Decode failure (`"prompt":"SK-CANARY-…"` wrong type) | `{"code":-32602,"message":"prompt is required"}` | clean — no canary, no `originError` |
| Handler returns `fmt.Errorf("upstream %s from %s", secret, path)` | `{"code":-32603,"message":"internal error","data":{"error":"internal error","originError":"upstream SK-CANARY-… from /home/ci/…/vault-CANARY.json"}}` | **leak** — raw cause on the wire |
| Handler panics with canary value | `-32603` + `originError` containing panic text **and a full goroutine stack** (file paths, function names, line numbers) | **leak** — stack on the wire |

`internal/jsonrpc/connection.go:737,748-750` wrap handler errors and panic
details into `ErrInternalError`, whose `internalErrorData{OriginError}`
marshals verbatim (errors.go:23). Adapter mitigation only covers errors it
returns itself (always return a sanitized `*acp.RPCError`); the panic path
is unreachable from agent code.

Smallest upstream change: stop attaching `originError` to response data on
the generic/panic path, or add a connection-level error sanitizer hook the
adapter supplies. Spec requires covering decode/panic/response paths, not
just plugin-returned errors — an upstream fix (or reviewed patch) is the
smallest honest resolution.

### Logging — PASS

`acp.SetLogger(captureLogger, acp.LevelDisabled)` before `Start`: zero
bytes reached the sink across all failure cases — no frame, cause, or
stack leak through SDK diagnostics. Caution recorded: the **default Debug
level logs complete raw frames** (`[ACP/stdio][send|recv]` lines in probe
output) including user payload text — the launcher must keep
`LevelDisabled` installed before any I/O.

### Link closure — PASS

Command: `go list -test -deps .` in the isolated module → 138 packages;
`grep` for `net/http|websocket|hertz|gin|proxy|httpserver|wsserver|wsutil`
over the list → zero matches. The probe closure pulls only
`eino-contrib/acp{,/conn,/transport,/transport/stdio}` +
`internal/{connspi,log,methodmeta,jsonrpc,safe}`. The SDK module *declares*
HTTP/WS code in-tree, but it stays outside the selected import graph.

## Dependency decision

- **Not releasable as-is** (matches spec §12.8's own caveat).
- Smallest upstream patch set, in priority order:
  1. Sanitize wire error data — no `originError`/stack on generic and
     panic paths (privacy-blocking).
  2. Public aliases for `WithMaxPendingDispatch`, `WithShutdownTimeout`,
     `WithRequestTimeout`, `WithRequestWorkers` (bounds enforceability).
  3. *(contract, optional)* a documented admission-ordering guarantee;
     today the pilot must own the TTL-latch semantics.
- Alternatives if upstream will not merge in the pilot window: a fork
  carrying exactly the same three diffs with an exact pin — still far
  smaller than a private dispatcher. No in-Vivy JSON-RPC layer is
  proposed or built by the probe.

## Fixture command record

```text
cd sdk/internal/testdata/acp-compat
go test -run 'Test(WireRoundTrip|UnknownElicitationActionFailsClosed)$' -count=1 -v   # PASS
go test -run 'Test(PromptCancelAdmissionOrder|TransportBoundaries|SDKWireErrorsAndLogging)$' -count=1 -v   # PASS
go test -race -run TestPromptCancelAdmissionOrder -count=20 -timeout=2m               # PASS (19.2s)
go test -count=1 -v ./...                                                            # PASS (5.1s)
go list -test -deps . | wc -l                                                        # 138
go list -test -deps . | grep -E 'net/http|websocket|hertz|gin|proxy|httpserver|wsserver|wsutil'  # 0 matches
```
