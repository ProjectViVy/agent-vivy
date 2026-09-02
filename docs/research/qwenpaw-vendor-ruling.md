# SR-4 裁定 — QwenPaw 保持外部参考，不 vendor

> **Date:** 2026-09-02
> **Closes:** TODO §0.1 行 SR-4（vendor vs external）；REFERENCE-INDEX RI-OQ-5
> **约束：** P2-3（fsjournal 探针规格）维持 DEFERRED，本裁定不改变其状态。

## 1. 问题

QwenPaw（`agentscope-ai/QwenPaw`，Python，Apache-2.0）于 2026-08-07 完成过 P9 式核验（Scroll=SQLite 权威 + Creator Runtime=SQLite-free 双策略、atomic_store/jsonl_store/locking/session_store 实现细节，见 `REFERENCE-INDEX.md` §3.17），当时克隆在 `/tmp/QwenPaw` 且未 vendor。RI-OQ-5 问：未来是否 vendor 进 `.workspace/qwenpaw`？

## 2. 裁定：**保持外部（external-by-URL），不 vendor**

理由（按权重）：

1. **无已核验的使用场景。** QwenPaw 对 Vivy 的唯一价值是 V1+ 文件系统 journal 后端探针的**设计参考**（原子发布 / JSONL envelope / 跨进程锁 / crash-tail 恢复）。该探针的规格行 P2-3 处于 DEFERRED，其前置 MEM-1 家族同样 DEFERRED。REFERENCE-INDEX §3.17 当时的建议就是"vendor 一旦有核验过的使用场景才做"；该条件至今未满足。
2. **Apache-2.0 意味着随时可取。** 许可层面没有任何"现在抓住"的理由——上游 URL 是稳定的获取通道，license 允许未来的探针项目在需要时重新克隆、复制、改造，无期限风险。
3. **磁盘卫生与事实源新鲜度。** `.workspace/` 是参考区不是档案区：8 月核验后 claude-code、rig 等树已被清理（2026-09-02 现场核实），vendor 一个无消费者的 16 万行 Python 树只会制造第二个"索引声称在、磁盘上没有"的陈旧事实源。
4. **核验记录已保全。** 对设计有价值的全部结论（含具体文件行号）都在 `REFERENCE-INDEX.md` §3.17；将来启用探针时按 §3.17 的"Open follow-up"流程重新克隆核验一遍即可（Python 上游仍在活跃演进，重验比信旧拷贝更可靠）。

## 3. 执行动作

- 不创建 `.workspace/qwenpaw`。
- REFERENCE-INDEX §3.17 open follow-up（RI-OQ-5）标记 RESOLVED，指向本文档。
- TODO §0.1：SR-4 翻 DONE（本裁定）；P2-3 行补充"SR-4 已裁定外部获取"注记，状态不变（DEFERRED）。

## 4. 何时推翻

出现以下任一情况时重新评估（按 ASSEMBLY-OPTIONS §6 能力再入流程，先提案后动工）：

- fsjournal 探针（P2-3）立项并进入实施；
- 任何能力提案明确需要 QwenPaw 的具体实现级参考（超出 §3.17 已保全的模式级结论）。
