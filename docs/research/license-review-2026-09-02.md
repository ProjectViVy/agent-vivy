# Reference License Review — claude-code (P3-1) 与 rig (P3-2)

> **Date:** 2026-09-02
> **Closes:** TODO §0.1 rows P3-1、P3-2；REFERENCE-INDEX open questions RI-OQ-1、RI-OQ-2
> **Method:** 上游一手验证（GitHub API + raw LICENSE 全文），非二手转述；本地 `.workspace/` 树状态现场核实。

## 0. 前提核实：两个本地副本均已不存在

`REFERENCE-INDEX.md`（2026-08-15 版）把 claude-code 记为 `.workspace/claude-code/`、rig 记为 `.workspace/rig/`。2026-09-02 现场 `ls .workspace/` 核实：两棵树都已被清理，`.workspace/` 现存仅 agent-wiki-library / caveman / crush / deepseek-harness / eino / headroom / oh-dsh / smoke 八个目录。因此本次审查全部改为对**上游仓库**取证；索引条目同步更新（见 §3）。

## 1. P3-1 — claude-code 上游 LICENSE：专有，禁止源码复用

- **上游仓库：** `anthropics/claude-code`（本地旧副本 README 声称的 `claude-code-best/claude-code` 为镜像）。
- **上游 LICENSE.md 全文（2026-09-02 经 GitHub raw/contents API 取得）：**
  > © Anthropic PBC. All rights reserved. Use is subject to Anthropic's [Commercial Terms of Service](https://www.anthropic.com/legal/commercial-terms).
- **GitHub 许可证检测：** `license: None`（API `repos/anthropics/claude-code` 的 license 字段为 null——GitHub 无法把它归类为任何 OSI 许可证）。
- **裁定：** 专有软件（all-rights-reserved + 商业 ToS 约束）。**禁止任何形式的源码复制或衍生**。这与 2026-08-06 的既有立场（"无 LICENSE 文件，按 all-rights-reserved 处理，仅作交互 UX 参考"）一致，且从"缺席推断"升级为"上游明文证实"。
- **允许的接触面（不变）：** 阅读 AGENTS.md/CLAUDE.md 层面的 UX 语汇与交互模式描述；不复用任何源文件、prompt 资产、schema。
- **Vivy intent 维持 Defer，理由从"license 未证实"变为"license 已证实为专有"。**

## 2. P3-2 — rig 上游 LICENSE：标准 MIT，"自定义许可"疑云解除

- **上游仓库：** `0xPlaygrounds/rig`（Playgrounds Analytics 的 Rust LLM 框架）。
- **上游 LICENSE 全文（2026-09-02 经 raw.githubusercontent 取得）：** 标准 MIT 正文，版权行为
  > Copyright (c) 2024, Playgrounds Analytics Inc.
  其后为逐字的标准 MIT 授权条款（use/copy/modify/merge/publish/distribute/sublicense/sell + 保留版权声明 + AS-IS 免责）。
- **历史疑点的根源：** 2026-08-06 的索引把"Copyright (c) 2024, Playgrounds Analytics Inc."这行**标准 MIT 版权声明**误读成了自定义许可标记。经全文核对，不存在任何 BSL / source-available / 附加限制条款。
- **裁定：** rig 为 **MIT**，许可层面可复用。但 Vivy intent 维持 **Drop**——它是 Rust 框架，与 V0 的 Go+Eino 选型不合，许可障碍解除≠架构理由改变。若未来 SystemV 探针需要 Rust 组件，rig 现在是"许可安全"的候选参考。
- **RI-OQ-2 关闭。**

## 3. REFERENCE-INDEX 同步

- §3.3 claude-code：补上游专有许可证据与日期，注明本地副本已清理。
- §3.15 rig：更正 License 结论为 MIT（误读更正），注明本地副本已清理。
- §6 RI-OQ-1 / RI-OQ-2：标记 RESOLVED（2026-09-02，见本文档）。

## 4. 对 Vivy 的净影响

零代码影响。两项审查均不改变任何实现路径；产出是"哪些参考项目碰不得、哪些解禁"的确定性。保留既有的聚合格约束（REFERENCE-INDEX §4）不变：GPL/AGPL/无许可证项目依旧禁止源码复用。
