# CH-C9 — A2A / NeuroLink (DEFERRED Note, Not a Work Order)

## 1. Identity

| | |
|---|---|
| ID | CH-C9 |
| Status | **DEFERRED** — Each requires an independent capability proposal |
| Contract | §15, §15.1; Evolution Stage H |
| Eino | Borrow models/transport from `eino-ext/a2a`; `RegisterServerHandlers` prohibited |

**Do not claim implementation** before proposal authorization.

## 2. Goal (Future)

Both are heavyweight plugins on ChannelHost, not a new kernel, Face, or ACP.

**NeuroLink:** Local WS server; grant `channel.listen`; the Host owns the bind, defaulting to loopback; do not add a telegram-style required-fields card.

**A2A:** Northbound interoperability; grant `channel.a2a`; HTTP+JSON off by default; `taskId` = `run_id`; requests enter `Service.Run`.

Layering:

```text
A2A JSON-RPC     ← eino-ext/a2a models + transport
    ↓
plugins/a2a      ← independent go.mod; encoding and decoding only
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
- Import `eino-ext/a2a` into the default body.
- List "Add NeuroLink" on the settings page when the plugin is not compiled into the body.
- Treat remote returns as trusted tool output.

## 9. Risk

eino-ext/a2a is still alpha, and its declared eino version does not align with Vivy's pin. Use it only as a codec reference, not as a drop-in example server.

## 10. Handoff

Open a slice PLAN only after writing an independent capability proposal for each. Do not start writing code directly from this note.
