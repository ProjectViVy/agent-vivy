# Acceptance

## Operator

1. Start the split pair (`just run` + `cd ui; pnpm dev`) and open
   `http://127.0.0.1:3015`.
2. Reproduce a failing provider turn (the MiniMax second turn is the original
   case) or point the provider at a loopback endpoint that returns 400.
3. Read the failed call from `.workspace/dev/home/vivy.db`:
   - the `model.request` row is version 5 and shows per-message roles, ordered
     block kinds, every call/result id, complete counts and tool declarations;
   - the paired `model.call.finished` row has `status:"failed"` and a `wire`
     object with the HTTP status, message roles, content forms, tool count and
     body bytes;
   - neither row contains prompt text or a content digest.
4. Confirm that a bare `400 (2013)` produces exactly one provider attempt. A
   retry is visible only when the error carries a tool-order phrase, the input
   contains a real tool result, and the invariants pass; the
   `auto_retry.started` reason carries a structural fingerprint.
5. Confirm that a malformed input we can prove is unsendable fails with
   `ModelRequestInvalid` and leaves both the request and finished rows, without
   charging a model call.

## Developer

- `GOTOOLCHAIN=go1.26.4 go test ./internal/runtime/... ./internal/provider/... ./internal/app/... -count=1`
  passes; `go vet` on runtime and provider is clean.
- Removing the `wire` sink or reverting the request mapping makes the golden
  and audit tests fail, so the HTTP shape cannot silently drift.
- A journal reader can reconstruct the rejected call's shape without ever
  holding its content.

## Explicit non-goals

No UI change, no new RPC, no storage migration, and no automatic prompt
rewriting. Assistant-text normalization waits for captured wire evidence.
