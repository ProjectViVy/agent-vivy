# CH-C9 — A2A / NeuroLink (DEFERRED Note, Not a Work Order)

> **2026-10-07 A2A design — G0 adopted:** [Issue #2 design](../../superpowers/specs/2026-10-07-a2a-server-design.md)
> is now the single authority for the A2A half of this note: official `a2a-go/v2`
> transport reuse through a custom RequestHandler, a native ChannelHost TaskHost,
> snapshot-convergence reconnect (option A — exact replay unselected), minimum
> deployment (loopback + reverse proxy), remote ordinary answers included, and
> opt-in Generation membership per issue #2. The EinoExt codec choice below is
> historical, not the adopted implementation dependency. Implementation remains
> unscheduled pending G1. NeuroLink is unchanged and stays deferred here.

## 1. Identity

| | |
|---|---|
| ID | CH-C9 |
| Status | **DEFERRED** — Each requires an independent capability proposal |
| Contract | §15, §15.1; Evolution Stage H |
| Eino | A2A codec/transport = official `a2a-go/v2` (see adopted design); `RegisterServerHandlers`/any second-executor wiring remains prohibited |

**Do not claim implementation** before proposal authorization.

## 2. Goal (Future)

Both are heavyweight plugins on ChannelHost, not a new kernel, Face, or ACP.

**NeuroLink:** Local WS server; grant `channel.listen`; the Host owns the bind, defaulting to loopback; do not add a telegram-style required-fields card.

**A2A:** Northbound interoperability; grant `channel.a2a`; HTTP+JSON off by default; `taskId` = `run_id`; requests enter `Service.Run`.

Layering:

```text
A2A JSON-RPC     ← official a2a-go/v2 transport + custom RequestHandler (2026-10-07 design)
    ↓
plugins/a2a-server ← independent go.mod; protocol adapter only
    ↓
ChannelHost      ← existing
    ↓
Service.Run      ← existing ADK Runner
```

## 3. Current State

Envelope slots were finalized in C2. The Listen surface was declared in C3. The five chat plugins must not use the `channel.a2a` / `channel.listen` grants.

## 4–8. Prohibitions (Even if Work Starts in the Future)

- Use `RegisterServerHandlers(adk.Agent)` as the Vivy gateway.
- A second TaskStore.
- Have Listen bind `:8787` `/rpc`.
- Import `eino-ext/a2a` into the default body (historical assumption — superseded by the `a2a-go/v2` choice in the 2026-10-07 design).
- List "Add NeuroLink" on the settings page when the plugin is not compiled into the body.
- Treat remote returns as trusted tool output.

## 9. Risk

eino-ext/a2a is still alpha, and its declared eino version does not align with Vivy's pin. Historical note only — the 2026-10-07 design adopts the official `a2a-go/v2` SDK instead; the drop-in example-server prohibition applies equally to the SDK's own `NewHandler`/`AgentExecutor`/`TaskStore` path.

## 10. Handoff

Open a slice PLAN only after writing an independent capability proposal for each. Do not start writing code directly from this note.
