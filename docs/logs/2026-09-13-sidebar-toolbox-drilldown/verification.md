# Verification

## Commands and results

| Command | Result |
| --- | --- |
| `pnpm typecheck` (in `ui/`) | Pass |
| `pnpm test` (in `ui/`) | 35 files / 316 tests pass |
| `pnpm build` (in `ui/`) | Pass (chunk-size warning pre-existing) |
| Browser smoke at `http://127.0.0.1:3015` | Pass — see below |

`just ci` (full kernel + UI gate) was **not** run: this change is UI-only and
the environment only exercised the UI lane. Backend packages were not touched.

## Browser smoke (Playwright + system Chrome)

1. Open `http://127.0.0.1:3015`, skip welcome wizard
2. Root sidebar text: 新建会话 / 聊天 / 工具箱 / VIVY / 会话 / 设置
3. Click 工具箱 → 返回 / 工具箱 / 定时任务 / MCP / Skill / 中控台 / 审批中心
4. Click 返回 → back to root
5. Click VIVY → 返回 / VIVY / 人格 / 面具 / 进化 / 记忆 / 记事本
6. Navigate 工具箱 → MCP → drill-down stays on toolbox menu (path-synced)

Screenshots in this folder:

- `smoke-root.png`
- `smoke-toolbox.png`
- `smoke-vivy.png`
- `smoke-mcp.png`

## Not verified in this pass

- Session folder assign/expand on a multi-session real Journal (smoke used the
  live single “新会话”)
- Mobile sheet sidebar interaction beyond shared `ConversationSidebar` markup
