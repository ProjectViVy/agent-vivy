# Acceptance

A developer on this tree can compile and start Vivy again.

1. From the repo root, `go build ./...` succeeds (no `undefined: ModelResolver` / `NewResolvingChatModel` / `TakeOrganismLease`).
2. `just ci` is green.
3. Two processes opening the same SQLite Journal: the second `TakeOrganismLease` fails with `storage.ErrLeaseHeld` rather than silently sharing the file.
4. Settings → Model (or a frozen `OPENAI_API_KEY` ENV session) supplies the live `ModelSpec`; the openai Ref does not read the process environment for the API key.
