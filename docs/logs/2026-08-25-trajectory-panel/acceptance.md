# Acceptance

A later agent or human working in this repo should be able to verify by eye at
`http://127.0.0.1:3015/dashboard` (split pair, `just dev`):

1. 中控台页签为 **概览 / Token / 轨迹** —— 不再出现「审计」Tab、审计卡片、
   顶部审计入口；`ui/src/components/audit/` 不存在。
2. 轨迹 Tab 内可见三块，与 DeepSeek Harness 轨迹视图同构：
   - 工具栏：「实际时长」切换、全部回合折叠、全部调用折叠、右侧「搜索」框；
   - 三泳道时间轴（Input/Model/Tools，44px 标签栏 + 50px 绘图区）：助理条带
     呈 TTFT/解码渐变分段；在图上按下拖动可框选一段（账本仅高亮区间内记录，
     区间外淡化）；悬停任一条带显示 KIND · 起止时间 · Total · TTFT · Decoding
     提示；Escape 清除选区。
   - 账本：按回合分组的记录行（SYSTEM/USER/CONTEXT/ASSISTANT/TOOL/SUBTOOL/
     COMPACTED 类型徽标，ASSISTANT 行带 `Request #N` 跳转钮），错误行红色
     `text → result`；折叠/展开全部回合与调用后，账本变为 `… 已折叠 · N 条
     记录` 摘要行；搜索关键词可过滤账本并在时间轴淡化未命中条带。
   - 点击行或 `Request #N` 打开右侧详情：请求级有摘要/用量/时序
     （Status/Provider/Model/工具调用/重试、Token 明细、TTFT/解码/总时长），
     记录级有输入/输出/思考。
3. 语言切换（设置 → 语言）后轨迹面板文案随之中英切换，无裸 i18n key。
4. `just ci` 是后续改动的门槛；新增轨迹数据/投影纯函数改动应先跑
   `pnpm test`（`trajectory-utils.test.ts` 覆盖投影、折叠、格式化与数据不变式）。

## 已知遗留（本次明确不做）

- 轨迹面板为纯演示数据（`vivy.demo` 之外的静态常量），接真实运行轨迹需
  内核提供日志/回放 RPC，另立任务（见 `docs/TODO.md` §0.1 UI-TRAJ）。
- 面板右侧详情为固定 320px 宽、不支持拖拽缩放与键盘缩放（DSH 原版支持），
  后续可按需补充。

## Leftover findings

- 全新 checkout 的 `just ci` 需先 `pnpm build` 生成 `ui/dist`（Go 侧
  `ui/embed.go` 的 `go:embed all:dist` 目标），详见
  `docs/TODO.md` §0.1 UI-CI-BOOTSTRAP。