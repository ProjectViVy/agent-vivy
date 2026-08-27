# Verification — Vivy Console (dsh-vivy-console)

Date: 2026-08-27

## Commands run

| Step | Command | Result |
| --- | --- | --- |
| Baseline commit | `git commit` of uncommitted Studio bundle state | `0b97a1c` — 457 files |
| Syntax checks | `node --check studio/dsh-vivy-console/{index.js,hook.js,client.js}` | OK ×3 |
| Profile manifest | mirror of `launch-vivy-studio.ps1` manifest logic | `dsh-vivy-console` in deps+bundles |
| Profile install | `pnpm install --dir data/studio-home/profiles/vivy-studio` | Done; `node_modules/dsh-vivy-console/client.js` present; stale `dsh-vivy-debugger` removed |
| Gate | `just ci` (root) | ✅ exit 0 — Go vet/fmt, `go test`, headless compile, 57 UI tests passed, Vite build ok |
| Bundle resolve | `dsh plugin --profile vivy-studio list` | ✅ `dsh-vivy-console@file:...` present; `dsh-vivy-debugger` absent |

## Pending (next turn)

- Studio restart via detached `restart-studio.ps1` (in-tree processes die
  with the listener), then browser smoke at `http://127.0.0.1:3090`:
  1. Conversation view ring shows 「Vivy 控制台」 after 上下文; `.vdbg-fab`
     (old floating button) is gone.
  2. 网关 pane: start mock gateway → status green, logs stream.
  3. VIVY WEB pane: proxied iframe renders the gateway UI; RPC capture shows
     `initialize` traffic; console capture shows app logs; evaluate box
     answers `document.title`.
  4. 生命周期 pane: `list generations` renders; `release` without
     confirmation is refused; a pack/eval round-trip completes.
  5. Air gap: `data/vivy.db`, `data/demo/`, `data/workspaces/` untouched;
     gateway data only under `data/studio-home/vivy-console/`.
