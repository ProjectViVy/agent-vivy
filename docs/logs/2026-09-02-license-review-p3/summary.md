# Summary — 参考项目许可审查（P3-1 claude-code / P3-2 rig）

## What changed

- `docs/research/license-review-2026-09-02.md`（新）：两项上游许可的一手核验与裁定。
  - **P3-1 claude-code**：上游 `anthropics/claude-code` LICENSE.md 全文 = "© Anthropic PBC. All rights reserved. Use is subject to Anthropic's Commercial Terms of Service."；GitHub 许可证检测 None。裁定专有，禁止任何源码/资产复用；Defer 立场从"缺席推断"升级为"上游明文证实"。
  - **P3-2 rig**：上游 `0xPlaygrounds/rig` LICENSE = 标准 MIT；2026-08-06 曾把标准 MIT 版权行误读为"自定义许可"，疑云解除。Vivy intent 维持 Drop（Rust；Eino 已覆盖同缝），SystemV 探针时为许可安全候选。
- `docs/research/REFERENCE-INDEX.md`：§3.3 / §3.15 条目改写（上游核验证据 + 本地副本已清理事实）；RI-OQ-1/RI-OQ-2 标记 RESOLVED。
- `docs/research/OPEN-ITEMS.md`：P3-1 / P3-2 行翻 DONE。
- `docs/TODO.md`：P3-1 / P3-2 行翻 DONE + §10 记录。

## What was explicitly not done

- 无代码改动；不改变任何引用/复用行为（两项目本就零复用）。
- P3-3（Defer 项目 vs Crush 重评）、P3-4（FSL-1.1-MIT 深度复用，DEFERRED）不在本片。

## Scope

docs-only。现场核实 `.workspace/` 已无 claude-code / rig 本地副本，审查全部改对上游取证。
