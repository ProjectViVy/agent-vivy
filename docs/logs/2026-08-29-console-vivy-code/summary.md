# Vivy Console — Vivy Code panel on 总控台

Date: 2026-08-29
Status: complete (Studio overlay; not kernel)

## Outcome

The Studio **总控台** gains a third card **Vivy Code（TUI 开发面板）** that
opens the Crush-style TUI shell in a dedicated OS console. It is explicitly
**not** part of 一键启动 / 停止 / 重启 and does not share the backend or
frontend process trees or log pipes.

## Delivered

- `studio/dsh-vivy-console/index.js`
  - `GET /vivy-console/api/code/status`
  - `POST /vivy-console/api/code/open` `{ mode: "demo" | "plain" }`
  - Source root: `VIVY_CODE_ROOT` → workspace → `../agent-vivy-tui-crush`
  - Windows: `start "Vivy Code" cmd /k go run ./cmd/vivy tui --demo`
- `studio/dsh-vivy-console/client.js` — third status card on 总控台; banner
  clarifying one-click is backend+frontend only; clipboard fallback if host
  `/code/*` is missing (no need to kill Studio)
- README + package description
- Profile install synced under
  `data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-console/`

## Explicitly not done

- Merging TUI into the headless backend lifecycle or Vite lifecycle
- Embedding TUI inside the Studio browser tab
- Packaging `faces/tui` into a species generation
- Forcing a Studio restart for this feature (operator may refresh the tab)
