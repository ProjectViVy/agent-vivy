# Verification

- `go test ./internal/runtime -run 'TestMapper(ConcatenatesStreamingToolCallIdentityAndArguments|StreamingToolCallFlushesPreambleAndFencesNextRound)' -count=1` — PASS.
- `just ci` — PASS: formatting, UI typecheck, 24 Vitest files / 201 tests, UI build, Go vet, full Go suite (including `internal/runtime`), headless compile, and all plugin/face modules.
- Split development path — PASS:
  - `just run` served the control plane from an isolated `.workspace` home.
  - `cd ui; pnpm dev` served Vite.
  - `GET http://127.0.0.1:3015/` returned 200 with the app root.
  - `GET http://127.0.0.1:8787/rpc/bootstrap` returned 200.
- `git diff --check` — PASS.

## Reproduction evidence

The pre-fix private VIVY CODE instance recorded an empty `tool.requested`, followed by correctly identified `tool.started` and `tool.finished` events for `list_dir`. Its log recorded a final fragment containing invalid standalone JSON (`}`), proving the mapper had retained only the last chunk. The new regression reproduces this fragmentation deterministically without a live provider.
