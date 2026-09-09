# 2026-08-30 — Super Channel contract: Eino-native A2A clarification

## Goal and background

C0's contract described A2A as a later `plugins/a2a` slice, with Task = Run. This
added one product boundary that had been asked about: Eino natively supports A2A, so
would these five chat plugins block the native route?

The conclusion was written into the contract; it is not a new architecture.

## Changes

- `docs/architecture/VIVY-CHANNEL-PACK.md` — §1 decision, §5 adopt/reject,
  §14.1 source, §15.1 layering, §18 / §19 / §20 C9 / §21.
- `docs/architecture/VIVY-PLUGIN-SPEC.md` — prohibition on `eino-ext/a2a` and
  `RegisterServerHandlers`.
- `docs/research/README.md` — one line for 14a.
- `docs/TODO.md` §10 — one record.

Locked:

- Eino core has no A2A wire protocol. In-process `AgentAsTool` / DeepAgent is not A2A;
  this slice does not touch it.
- `eino-ext/a2a` is split into two layers: reuse `models` + `transport`; do not use
  `RegisterServerHandlers(adk.Agent)` as the gateway.
- Later layering: codec → `plugins/a2a` → ChannelHost → `Service.Run` → the existing
  ADK Runner.
- The five chat plugins in this batch have zero Eino imports. C1–C8 keep their shape.

## Explicitly not done

- A2A was not implemented; the `eino-ext/a2a` dependency was not introduced; kernel /
  SDK / UI were not changed.
- The C9 capability proposal was not opened.
