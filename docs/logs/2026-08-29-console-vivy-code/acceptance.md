# Acceptance — console Vivy Code panel

Date: 2026-08-29

| Item | How to tell | Result |
|---|---|---|
| 总控台 shows a **Vivy Code** card separate from 后端/前端 | Open Vivy 控制台 → 总控台 | PASS |
| 一键启动 does **not** open Code | Click 一键启动; no TUI window | PASS (by design) |
| ◈ 打开 Vivy Code opens a dedicated console | API open ok; Windows `cmd` titled Vivy Code on TUI worktree | PASS (API verified) |
| Host lag does not block the card | client keeps card + clipboard fallback without Studio kill | PASS |
| Source missing is fail-closed with a message | Root without `internal/tui` → card shows 未就绪 | intended |
| Packaging page untouched | 打包与版本 still never starts Code | PASS |
