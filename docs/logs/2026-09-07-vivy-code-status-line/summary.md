# 2026-09-07 — VIVY CODE 状态行：spinner / 计时 / 右段队列与滚动指示

## What changed

B 状态行三件套（bubbletea 全屏 shell，`sdk/tui`），交付提交 `e2ad7f2`：

- **F1 braille spinner + 计时器**：busy 时状态行左侧的静态 `run…` 升级为
  `⠙ run 1m15s`。`surface.Meta` 新增 `BusySince`（零值=空闲），`live` 侧全部
  busy 切换收敛到 `setBusyLocked`（7 处调用点），`Meta()` 透出时间戳；视图
  `Model.spinFrame` 在 `Update` 中随消息自增（驱动层既有 40ms `liveTickMsg`
  心跳持续重绘，busy 与否均在自续，无需新增定时器），手写 10 帧 braille，
  不引入 `bubbles` 依赖。
- **F12 chrome 行左右分段 + 队列数**：`renderInputChrome` 由左对齐单行改为
  左右分段——左 = `err · …` / spinner+计时 / 快捷键 hints（优先级不变），
  右 = `queued N`（仅 `Queued > 0` 时显示），右对齐 pad 到整行宽，
  `lipgloss.Width()` 量宽，单行高度不变（`chromeHeight=1` 布局零抖动）。
- **F6 滚动指示 + end 回底**：右段在上滚（`!chatFollow` 且可滚）时显示
  `↓ <pct>% · end 回底`，pct 取 `chatScroll/chatMaxScroll` 并夹取 0–100；
  End 回底行为本就存在（`model.go` KeyEnd → `chatFollow=true`），本次补齐
  可见指示与发现性，不改键位。

## Files

- `sdk/tui/surface/surface.go` — `Meta.BusySince`（加法域字段）
- `sdk/tui/live/controller.go` — `busySince` + `setBusyLocked`，7 处 busy 切换收敛
- `sdk/tui/view/model.go` — `spinFrame` 字段 + `Update` 自增一行
- `sdk/tui/view/render.go` — `spinnerFrames`、`busyStatus`、`chromeRightStatus`、`renderInputChrome` 右对齐分段
- `sdk/tui/view/chrome_status_test.go` — 3 个确定性单测（新增）
- `sdk/tui/view/zpreview_test.go` — 手动预览工具扩展 busy+queued / 上滚两帧（仍 TUI_PREVIEW=1 门控）

## Explicitly not done

- compact header（`renderCompactHeader`）不动，其左中右分段维持原样；
  sidebar 的 `queue · N` 行保留未动。
- 不加 `bubbles`/新依赖；不改任何滚动键位；不新增第二个定时器。
- TODO line 64 记录的 viewport 既有债务（paused offset 为渲染行号锚点、
  clamp 全量重渲历史）不在本批次处理。

## Lane note

本批次与并行的 window-title / sidebar 配色 lane 曾同时在根树写入
`model.go`/`render.go`；该 lane 已自行以 `2768390`、`83d242e`、`fc63529`
三笔提交落地，本提交只含状态行交付物自身路径，两 lane 内容在提交态下
完整正交。
