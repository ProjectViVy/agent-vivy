# VC-0 验收（人怎么确认它生效）

1. **面具即 face**：打开 http://127.0.0.1:3015（split 模式），把输入区面具
   切到"程序员"，发一条消息 → 这次 run 以 code face 运行：Journal 的
   `run.started` payload `face:"code"`；切回默认/研究员/作家面具 →
   `face:"web"`。
2. **面具卡片可见升级**：程序员面具的能力列表多出"以 code face 运行"
   （英文界面为 "Runs with the code face"）。
3. **预检回显**：浏览器 DevTools → Network → `preflight/run` 请求体带
   `face` 字段，响应 `face` 与所选面具一致（程序员 = code，其它 = web）。
4. **非法 face 被拒**：对 `preflight/run` 直接发 `face:"shell"` → RPC 错误
   -32602 `runtime: invalid face`，UI 不发起 run。
5. **恢复不丢 face**：构造一次工具审批挂起（approval 挂起的 run），审批
   恢复后后续事件 payload 的 face 与挂起前一致。
6. **code face prompt 框定**：程序员面具下让模型引用代码位置，回复倾向
   path:line；`composeRunPreamble` 单测断言 code face 前导含
   "Code mode is active"、web face 前导不含。
