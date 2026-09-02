# Verification — SR-4 QwenPaw vendor 裁定

日期：2026-09-02。

```text
ls .workspace/
  → 无 qwenpaw 目录（裁定落地即"零磁盘动作"，无需 vendor）

just ci   （与本日 P3 许可审查 / P2-1 清单两片 docs 合并过门）
  → CI-EXIT:0
```

事实源核对：`REFERENCE-INDEX.md` §3.17（2026-08-07 的 P9 式核验记录：Apache-2.0、
Scroll/Creator 双策略、atomic_store/jsonl_store/locking/session_store 细节、shadow-Git
checkpoint 非 Eino）——裁定引用的全部技术结论均出自该既有核验，未做新上游抓取。
