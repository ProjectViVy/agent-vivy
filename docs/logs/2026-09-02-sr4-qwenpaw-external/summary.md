# Summary — SR-4：QwenPaw 保持外部，不 vendor

## What changed

- `docs/research/qwenpaw-vendor-ruling.md`（新）：裁定 QwenPaw 保持 external-by-URL。
  理由：无已核验使用场景（fsjournal 探针 P2-3 DEFERRED）、Apache-2.0 随时可取、
  避免"索引声称在/磁盘没有"的陈旧事实源；§3.17 核验结论已保全，重验优于信旧拷贝。
  附推翻条件（探针立项或提案需要实现级参考）。
- `docs/research/REFERENCE-INDEX.md`：§3.17 open follow-up（RI-OQ-5）标 RESOLVED；
  §6 加行。
- `docs/research/OPEN-ITEMS.md` / `docs/TODO.md`：SR-4 翻 DONE；P2-3 行补注裁定，
  状态维持 DEFERRED；TODO §10 记录。

## What was explicitly not done

- 不创建 `.workspace/qwenpaw`；不做 fsjournal 探针（P2-3 维持 DEFERRED，前置 MEM-1
  亦 DEFERRED）。
- channel 全家停驶拍板不受影响。

## Scope

docs-only；零代码。
