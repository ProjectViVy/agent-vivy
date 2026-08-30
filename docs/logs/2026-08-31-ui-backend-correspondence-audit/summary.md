# UI ↔ 后端对应关系审查

## 变更

- 新增 `docs/research/ui-backend-correspondence-2026-08-31.md`。
- 完成非通道 Web UI 与真实 JSON-RPC/runtime/产品契约的双向扫描。
- 登记后端已有但 UI 错配或遗漏的 10 项发现；对后端尚不存在的演示面明确排除。

## 范围

覆盖聊天运行链路、Skills、Dashboard、Lifecycle、Review Center/Run Inspector、上下文压缩、主导航，以及 Provider/MCP/Token/Session 等对应性核验。未修改产品代码、通道或 Studio。

## 未完成

本交付是审查报告，不实施报告中的 UI 修复。浏览器 runtime 当前无可用实例，无法完成可视截图 smoke；HTTP split 进程已启动并返回 200。
