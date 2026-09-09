# 2026-08-30 — Recoverable frontend errors

Date: 2026-08-30
Scope: Vivy UI (`ui/`), not Studio.

## What changed

Previously, conversation failures collapsed control-plane rejection, raw engine errors,
and a missing model key into one red string, while startup failures required a full-page
refresh. This change brings failure classification into a shared layer and connects it
to the startup page and chat area.

- `ui/src/lib/failure.ts`: split unreachable control plane, missing API Key, and ordinary
  run failure into three categories, each with a recovery action.
- `RpcClient.connect`: turn `/rpc/bootstrap` network failures (`Failed to fetch` /
  `ECONNREFUSED`) into actionable copy instead of passing the browser `TypeError`
  through unchanged.
- Store: `retryInitialize` clears the RPC client and reconnects without a full-page
  refresh; `payload.message` from `run.failed` enters `runError`, and reopening a
  historical failed run carries it too.
- The startup page and chat area use the same `RecoverableError`: control-plane failure
  offers Retry, a missing key offers "Open model settings", and the draft remains after
  a send failure.

## Explicitly not done

- Do not change backend `run.failed` classification (the engine may still wrap `API key
  missing` in NodeRunError text).
- Do not replace every Settings/demo page with `RecoverableError`.
- Do not automatically reconnect the control plane; retry remains a user action.
