# VC-0 verification record

Date: 2026-08-31. All commands ran in worktree `agent-vivy-vc0` (branch
`feat/vc0-face-skeleton`).

## Kernel

- `go build ./...` — passed (a new worktree first needs a gitignored `ui/dist/.keep`
  to satisfy go:embed).
- `go test ./...` — passed: runtime (about 54s, including the three new face tests
  `face_test.go` / `face_service_test.go` / prompt-face cases), rpc, app,
  and domain all passed.
- `go vet ./...` — no warnings.

## UI

- `pnpm typecheck` (tsc --noEmit) — passed.
- `pnpm test` — 22 files and 177 cases all passed (including the new
  `src/components/masks/mask-catalog.test.ts`: programmer → code, other masks leave face unspecified, 2 cases).
- `pnpm build` — passed.

## Real-path smoke test (http://127.0.0.1:3015)

8787 was occupied by another process, so this smoke test started the worktree backend with
`VIVY_ADDR=127.0.0.1:8791` and Vite with `VIVY_BACKEND_ADDR=http://127.0.0.1:8791 pnpm dev`; all requests
went through the 3015 same-origin proxy (the same contract used by the browser).

Script: `.workspace/smoke-face.mjs` (gitignored draft, bootstrap → WebSocket
JSON-RPC). Result: SMOKE PASS 4/4:

1. `preflight/run` with `face:"code"` → response `face:"code"`.
2. Without `face` → response `face:"web"` (explicit server normalization).
3. `face:"shell"` → RPC -32602 `runtime: invalid face\nface must be web,
   tui, or code`.
4. `turn/start` with `face:"code"` → Journal `run.started` payload
   `face:"code"`, `mode:"normal"`; terminal `run.failed` (the new worktree had no
   provider key, as expected; this does not affect the face evidence).

## just ci

`just ci` (fmt-check → vet → test → headless-compile → ui-ci) — passed, exit 0.
UI-ci excerpt: vitest 22 files and 177 cases all passed (including the new mask-catalog face cases),
`vite build` passed.
