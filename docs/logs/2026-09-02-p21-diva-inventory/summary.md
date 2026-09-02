# Summary — P2-1：Diva 全量能力清单（Keep/Adapt/Defer/Drop）

## What changed

- `docs/research/diva-capability-inventory.md`（新）：对 `morediva/agent-diva`
  （Rust，17 crates）做现场盘点，产出 68 行 × 10 域的能力清单：
  **Keep 29（已交付 28 + 待提案 1：消息编辑/回退/分叉 = UI-CHAT-ACT）· Adapt 15 ·
  Defer 18 · Drop 6**。
  - 逐行附 Diva 证据路径（README / AGENTS-ARCH.md / LAPUTA.md / crates /
    `providers.yaml` 47 预设 grep / `#[tauri::command]` 176+1 命令 grep）与 Vivy
    2026-09-02 现状对照（已交付判定以 TODO §10 + `docs/logs/` 为准）。
  - 取代 `AGENT-VIVY-ASSEMBLY-OPTIONS.md` §5 的 V0 占位清单（该文档 §5 保留为
    V0 决策记录，不再作为全量清单入口）。
  - §12 重申 ASSEMBLY-OPTIONS §6 再入规则：**tag ≠ 实施授权**，Defer→实施 /
    Adapt→实施必须先走能力提案，且不得从 Diva 的 crate 边界 / Tauri 命令名 /
    旧 schema 出发。
- `docs/TODO.md`：P2-1 行翻 DONE + §10 记录。
- `docs/research/OPEN-ITEMS.md`：P2-1 行翻 DONE。

## Method note

盘点由只读探索代理完成（25 次工具调用，逐文件取证）；无法核实的条目（wtf 工具、
neuro_link 通道、core/ 内部模块）显式标注"未核实"并按保守 tag 处理（Drop / Defer）。
agent-diva 的 TODOLIST.md 远景（Workbench/PEN/Mirror、A2A 等）识别为**计划 backlog
而非已交付能力**，不入清单。

## What was explicitly not done

- 不改变任何 TODO 优先级；不为清单中的 Keep-未交付行自动立项。
- 1.10（消息编辑/回退/分叉）维持 UI-CHAT-ACT 既有排期（设计片先行）。

## Scope

docs-only；零代码。
