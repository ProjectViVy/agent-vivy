# Acceptance

How a human can tell LOG-2 works:

1. Start the split dev pair (`just run` + `cd ui; pnpm dev`) or the
   release binary and open `http://127.0.0.1:3015`.
2. Watch the server console/log file (`<data_dir>/logs/vivy.log.*`).
   Every page load now shows one `http request` line per hit:
   `method=GET path=/rpc/bootstrap status=200 duration_ms=...`.
3. Open a chat and send a message — the WebSocket dial logs
   `path=/rpc status=101` once per connection upgrade (not per frame).
4. Health checks (`GET /healthz`, e.g. the Docker healthcheck or a
   manual curl) do **not** produce info-level lines — drop the level to
   `debug` to see them.
5. Trigger a 5xx (if available) and confirm the line comes out at
   `level=WARN`.
6. Nothing in the access lines ever echoes request bodies, tokens, or
   provider payloads — path and timing only.
