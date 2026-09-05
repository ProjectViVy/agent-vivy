# Verification

## Automated gates

- `just ci` — PASS.
  - Go formatting check, UI install/typecheck, 24 Vitest files / 201 tests, UI production build, Go vet, full Go test suite, headless compile, and every plugin/face module passed.
- `go test ./sdk/tui/...` — PASS.
- `go test -race ./sdk/tui/live ./sdk/tui/view` — PASS.
- `go test -tags vivy_headless ./internal/codeface ./cmd/vivy ./cmd/vivy-code` — PASS.
- `cd faces/tui; go test ./...` — PASS.
- `node --check studio/dsh-vivy-console/index.js` — PASS.
- `node --check studio/dsh-vivy-console/client.js` — PASS.
- `just studio` — PASS; produced `vivy-studio.exe`.
- `git diff --check` and `git -C studio diff --check` — PASS (Git emitted only the repository's LF-to-CRLF working-copy notices).

## Product paths

- Built headless `vivy.exe` and `vivy-code.exe`; a temporary Windows console harness under ignored `.workspace/` attached real `CONIN$`/`CONOUT$` handles.
  - local `vivy-code.exe` entered and remained in the fullscreen event loop for three seconds, then exited on a console Ctrl+Break — PASS.
  - `vivy tui --live --addr 127.0.0.1:8787` entered and remained in the fullscreen event loop; gateway logs recorded `/rpc/bootstrap` HTTP 200 and `/rpc` WebSocket 101 — PASS.
  - The automation host could not inject a literal Ctrl+C key event reliably; Ctrl+Break was used only to terminate these smoke processes and their application exit status was 1. Startup/connect behavior is the asserted part of this automated smoke.
- Started the required split pair with `just run` and `cd ui; pnpm dev` using `.workspace/tui-slim-smoke/home-server` as `VIVY_USER_HOME` — PASS.
  - `http://127.0.0.1:3015/` returned HTTP 200 with the application root.
  - Embedded release path `http://127.0.0.1:8787/` returned HTTP 200 with the application root.
  - Browser computer-use was unavailable in this environment, so the browser surface was checked over HTTP rather than visually; the changed product surface is the terminal, covered by the real-console smoke above.
- `go run ./sdk verify faces/tui` — PASS.
- `go run ./sdk pack --face tui --out .workspace/tui-slim-pack` — PASS.
  - generation `gen_1f7a22e8e5aabe5c`, recipe `face: tui`.
  - packed `vivy.exe` entered and remained in a real-console TUI loop before Ctrl+Break cleanup — PASS.
- Studio shell:
  - A clean worktree initially lacked generated `dsh-plugin/lib/index.js` and `client/client.js`. Running the submodule's standard install/server/client build commands supplied those ignored build products; no source fix was needed.
  - `launch-vivy-studio.ps1` then served `http://127.0.0.1:3090` — PASS.
  - `GET /vivy-console/api/code/status` returned `ready: true` and command `go run ./cmd/vivy-code` — PASS.
  - `POST /vivy-console/api/code/open` returned `ok: true` and spawned exactly the canonical `cmd.exe /k "go run ./cmd/vivy-code"` path — PASS. Smoke child processes were terminated afterward.

## Negative contract

- `vivy.exe tui --plain` — rejected as an unknown argument, exit 2.
- `vivy.exe tui --demo` — rejected as an unknown argument, exit 2.
- Repository search found no executable references to the retired flags, REPL/demo runners, duplicated view/surface wrappers, or legacy sibling-worktree launcher. Historical delivery logs retain their original record; the completed TODO row was updated with the retirement.
