# Acceptance

## 人工验收

1. 打开 `http://127.0.0.1:3015`，进入 中控台/Dashboard → "会话"（Overview）
   Tab：三个数字来自真实后端——会话数与 Settings/侧栏看到的会话一致；
   新建/删除会话后刷新数字变化；有待审批/提问时 Review Center 的 pending
   数与第三格一致。
2. "近期活动"卡不再出现（该卡原本显示编造的"日报已生成/技能变更待处理/
   定时任务完成"演示条目）。
3. 断开后端（或停掉 `just run`）再打开 Overview：显示错误横幅 + 重试
   按钮，不再是假的 12/2/1。
4. Token Tab 行为不变（本就真实）；Trajectory Tab 仍为演示轨迹（已知，
   由 UI-TRAJECTORY-DEMO 行跟踪）。
5. localStorage 中不再写入 `vivy.demo.dashboard`。

## 判定标准

- `just ci` 绿（tsc/eslint/vitest/build 无断裂）。
- `just ui-e2e` 全套绿（无 dashboard 专属 spec，全套回归兜底）。
- 分离开发对（`just run` + `pnpm dev`）浏览器实测 Overview 数字与
  RPC 返回一致。
