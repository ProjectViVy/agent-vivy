# VC-2 死循环检测（tool loop guard）

日期：2026-09-01　分支：`feat/vc1a-bash-tool`（worktree `agent-vivy-vc0`）　任务：VC-2 第 2 项（研究 §5 VC-2.2）

## 范围

对齐 Crush 的 `StopWhen` 死循环防护：**最近 10 步已完成的工具调用中，同一
"调用+结果"签名重复 >5 次即终止 run**。签名 = 工具名 + 规范化参数 JSON +
结果文本 + 错误文本（64 位 FNV 哈希，结果不驻留内存）。

- **落点**：`eventMapper` 新增 `loopWindow`（每 run 一份；审批恢复后窗口重置
  ——恢复是人工决策点，重新计数合理且有注释）。工具结果完成时在
  `toolResultEventsParts` 记录签名，超限返回 `errLoopDetected` 哨兵。
- **参数规范化**：请求时把解码后的 `map[string]any` 重新 marshal（Go 按 key
  排序）存到 `openToolCall.argsJSON`，模型重排 JSON key 的同参调用仍算重复。
- **终止路径**：`terminalEvent` 新增 `errLoopDetected` 分类，新 cause 类别
  `loop_detected`，用户可见消息有界（FR-11：不泄露签名/引擎内部）。行为与
  `ErrExceedMaxIterations`（MA-4）、预算熔断一致。
- **不做**：不新增配置项（窗口/上限编译期常量，与 Crush 10/>5 一致）；
  不改动 engine.go（守 GATEWAY 文档约束）；不触碰 UI（`failure.ts` 对非
  `provider_error` 类别直接展示服务端消息，`loop_detected` 自动生效）。

## 法律与对齐

Crush 为 FSL-1.1-MIT：本片只做行为对齐（10 步窗口 >5 次判停），零代码复制；
哈希/窗口/规范化实现全部自写。

## 验证

见 `verification.md`。
