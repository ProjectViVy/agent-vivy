# RB-1 回退调研：回退怎么做、Vivy 承诺不承诺"代码回退"

- 日期：2026-08-31
- 立项：`docs/TODO.md` §0.1 RB-1（2026-08-31 用户拍板，VC-0 决策清单追加项 3）
- 调研范围：checkpoint 桥（FR-8 会话恢复）/ 文件版本 history（VC-3 编辑前存档）/ git 语义回退——三者边界与组合；产出 = Vivy 是否承诺"代码回退"能力及落点（主线/插件/code face）
- 参照源：本仓库 `internal/`（rb1-rollback 工作树快照）；`.workspace/crush/`（Charm Crush，FSL-1.1-MIT，仅行为参照、代码不得拷入）

---

## 1. 结论一句话

**Vivy 现在不支持任何代码回退，但这在当前产品形态下是"正确的不支持"——文件工具只写每 run 隔离 scratch，根本碰不到用户的真实代码；等 VC code face 把工具面接到真实工作区时，文件级回退成为安全网刚需，届时以主线内核的"编辑前存档 version chain + 单文件/会话级恢复"落地（不是 git 语义回退）。建议现在只拍板设计方向，实现挂在 VC-1/VC-3 的存储设计里顺路交付，不单独立项抢跑。**

---

## 2. 三种"回退"的语义边界

| 语义 | 是什么 | 能解决 | 不能解决 | Vivy 现状 |
|---|---|---|---|---|
| **checkpoint 桥（FR-8）** | eino ADK 运行态快照（opaque bytes + 引擎版本信封 + 校验和），恢复被审批/提问中断或重启打断的 run | 会话/运行状态恢复：pending 审批重新可决、resume 继续跑 | 不含文件语义——checkpoint 恢复的 run 继续用磁盘上现在的文件，动过的文件不会被还原 | 已实现（`internal/runtime/checkpoint.go` + 重启恢复） |
| **文件版本 history** | 每次写/改前把旧内容存档成版本链；回退 = 把文件恢复到链上某个版本 | 代码回退的正解：单文件"撤销这次改动"、会话级"撤销这个会话动过的所有文件" | 对 execute/bash 类副作用无效；对"恢复到 git commit"这种仓库级语义无表达力 | **无任何实现**（无表、无存档、无恢复 RPC） |
| **git 语义回退** | commit / branch / reset / stage 等仓库级操作 | 精确、成熟的仓库级历史 | 依赖工作区是 git 仓库；用户没 commit 的中间态没有粒度；把"让 agent 跑 git"当回退 = 把安全网建立在工具上 | 无（且主线刻意不装 shell/复合命令） |

**判定：Vivy 应承诺的是"文件级回退"（版本链），不承诺 git 语义回退**——git 归用户的 git，agent 只对自己动过的文件负责。checkpoint 桥与文件级回退是互补关系（一个恢复运行态、一个恢复文件态），组合点见 §5。

---

## 3. Vivy 现状盘点（逐条核实）

### 3.1 checkpoint 桥 = 纯运行态，无文件语义

- `internal/runtime/checkpoint.go`：`VersionedCheckpointStore` 把 eino runner 的 opaque checkpoint 字节包上 `{engine_version, checksum_sha256, created_at}` 信封（4 字节长度头 + JSON 头 + payload），落 `storage.BlobStore`，读时 fail-closed 校验引擎版本 + 校验和。**信封里没有也不可能有文件路径/内容。**
- `internal/runtime/checkpointadapter.go`：仅把它适配成 `adk.CheckPointStore/Deleter`。
- 用途（`internal/runtime/service.go`）：审批/提问中断挂起 run；重启恢复（recovery）把"还能读到可验 checkpoint 的非终止 run"重建为 pending，等下一次决定 resume。engine.go:224 `Resume` = `runner.ResumeWithParams`。
- **结论：FR-8 的"恢复"是运行态恢复，与代码回退无关。回退调研不要求改它。**

### 3.2 文件写路径 = 每 run 隔离 scratch，旧内容无处存档

- 工具面（write_file / patch）统一走 `EinoFilesystemBackend`（`internal/runtime/filesystem_backend.go`），路径解析**无条件**经过 `WorkspaceManager.Ensure(runID)` → `<Runtime.WorkspaceRoot>/<runID>/` 的私有 scratch（`resolve()`，filesystem_backend.go:627）；`safeWorkspacePath` 再做逃逸/软链/保护名拦截。**即使在 `danger_full_access` 下，文件工具也出不了这个 scratch**（危险模式只是跳过 sandbox 校验，backend 的 containment 照旧）。
- 默认配置 `workspace_root: data/workspaces`（config.example.yaml:60）——scratch 在数据目录里，不是用户项目。
- `WriteFile`（:296）：读旧内容 → 与 `ProposalPreconditionHash` 比对（审批后目标被外部改过则拒绝，"proposal stale"）→ `atomicWrite` 原子替换 → 产出 `boundedDiff`。**旧内容只在两处留下残影，均不可恢复**：
  1. `boundedDiff`（:768）：单个 `@@` hunk，`-old`/`+new` 全文嵌入，超 32KB 截断——它在 tool result payload 里随 Journal tool 事件落库，但格式有损、不可直接回写；
  2. 审批提案（`PrepareWriteFile`/`PreparePatchFile`）：`reviews` 表存 `precondition_hash` + `preview`——hash 只能判"变了没有"，preview 是有损 diff。
- `PatchFile`（:346）：精确唯一替换后委托 `WriteFile`，同样无存档。
- download（`internal/runtime/download.go`）写文件同样直落 scratch。
- **scratch 生命周期**：无任何清理/回收代码（runtime/app 下无对 workspace 的 RemoveAll），按 run ID 确定性重建/复用。所以"run 级回退"等价于"扔掉/重建这个 run 的 scratch"——这在当前形态里是平凡操作，不需要版本链。

### 3.3 能碰真实文件系统的只有 `execute`（危险模式），它天然没有回退

- `internal/runtime/command_backend.go:151`：仅 `danger_full_access` 且命中白名单才放行；复合命令（管道/`&&`/git）当前根本跑不了，故不存在"bash 搞坏了工作区"的回退问题。**VC-1 bash 化会打开这个口子——那是本调研结论真正挂靠的时间点（见 §5.1）。**

### 3.4 存储层 = 有版本原语、无文件版本表

- sqlite/postgres 26 张表族（approvals/blob/compaction/crons/journal/lease/messages/notes/questions/reviews/runs/sessions/skill_revisions/snapshot/studio/todos/token_usage…）：**没有 file/version 类表**。
- 现成版本原语：`storage.BlobStore` = generation 追加 + 指针翻转（D-030，sqlite `blob.go`：`checkpoint_generations(id, generation, blob)` + `checkpoints` 指针行）。**API 只暴露 latest 读取**，没有列历史代数的查询——若复用它承载文件版本需要扩接口（List generations / 按代读取）。

### 3.5 审计（Journal）有"发生了什么"、没有"恢复什么"

~40 种 RunEvent 事件溯源已经把每次写动的 diff 残影、precondition_hash 记进了事件流；回退能力的增量不是"多记点日志"，而是**可机读、可回写的旧内容存档 + 恢复动作本身走审批**。

---

## 4. Crush 参照（行为对齐目标，代码不拷）

- **版本链**（`internal/history/file.go`）：per-session 文件版本表（session_id, path, content, version 递增，全文非增量）。每个写工具（edit/write/multiedit/lsp_replace_symbol/lsp_rename）经 `commitFileChange`（edit.go:246）挂链：首次见到的文件先 `Create(path, oldContent)` 存旧内容；若用户手工改过（链上 latest ≠ 磁盘内容）先插一版中间态；然后 `CreateVersion(newContent)`。记录失败只记日志、不阻塞写（best-effort）。
- **stale-read 防护**（`internal/filetracker`）：记 (session, path, last_read_at)；编辑前若磁盘 mtime 晚于 last read 直接拒绝（"read before edit"）。与版本链同点：都是 per-session 语义。
- **消费方 = 只有展示**（本调研的关键发现）：TUI 侧 `loadSessionFiles`（ui/model/session.go:112）把版本链聚合为"本会话改了哪些文件"面板（first→latest diff 统计 + pubsub 实时更新）。**全库检索 restore/revert/rollback：Crush 没有任何恢复/撤销消费方。** 版本链是"先埋数据、等消费者"的基建——对标结论：Crush 自己也还没做代码回退，Vivy 若做即直接领先。
- 迁移启示：Crush 全文存 TEXT 无大小上限、无保留策略；Vivy 设计时应带 1MB 级单版本上限 + 每 (session, path) 保留 N 版 + 会话清理钩子（对齐 Vivy 既有 maxFileBytes 1MB 的工具面预算）。

---

## 5. 建议：承诺范围、落点与挂靠

### 5.1 是否承诺"代码回退" → 承诺文件级，现在拍方向、随 VC 顺路交付

1. **当前主线产品（今天）**：文件工具只写 scratch，用户代码不受影响；回退不构成当下缺口。不立项抢跑。
2. **VC-1（bash 化 + 工具面对齐）**：bash/复合命令打开真实执行面后，"写坏文件"从不可能变成可能。此时**文件级回退（L1）成为安全网前提**：write/patch/bash 影响到的文件在动前存档。
3. **VC-3（LSP/编辑深化 + VC-1 已并轨的 stale-read filetracker）**：version chain 与 filetracker 本来就要求合并一次存储设计（研究 §8.4 已记）；恢复 RPC 与 UI 是同一张表的读侧，顺路交付（L2）。

### 5.2 落点：主线内核，不是插件、不是 code face 专属

- 版本链寄生在存储层 + 文件工具执行路径 + 审批流（恢复动作本身要过 HITL）——三处都是主线核心，**插件化没有意义**。
- code face 只是第一个重消费者；主线 web face 的 write/patch 同表同链（scratch 模式下 scratch 的版本链价值低，可在 code face 真实工作区模式启用全量存档、主线维持轻量或按文件开关——见 O4）。
- 治理对齐：Journal 增加 `file_version.archived` / `file.restored` 类事件（先落库再推送，走既有 RunEvent 管线）；恢复动作 = 一次 write_file 语义的提案，走审批策略（auto 时也要 precondition hash 防覆盖并发写）。

### 5.3 分层设计（写进 VC-1/VC-3 验收的最低承诺）

| 层 | 内容 | 状态 |
|---|---|---|
| **L1 存档 + 单文件恢复** | write/patch 前把旧内容（sha256 去重）写版本链；`files/versions` 只读 RPC；`files/restore(version)` 走审批提案（precondition hash 防并发覆盖） | VC-1 内随 filetracker 存储设计一并落 |
| **L2 会话级回退** | 按 session 聚合版本链，倒序恢复全部动过的文件（stale 冲突单列拒绝并续走）；UI 一键"撤销本会话改动" | VC-3（与 LSP 编辑深化同期） |
| **L3 git 语义** | 不做。code face 在检测到 git 仓库时提示用户走 git；agent 不承诺 reset/commit 级回退 | 明确不做（产品口径） |

### 5.4 存储形态取舍（设计决定项，实现前定稿）

- **方案 A（建议）：新 `file_versions` 表**（sqlite + postgres 双后端）：`(id, session_id, run_id, path, version, content_hash, content BLOB, created_at)` + `(session_id, path, version)` 唯一。理由：BlobStore API 只有 latest 语义、且 checkpoint 信封字节是 eino 专属格式；文件版本需要按 path/会话聚合查询与保留策略，独立表更直。存档放 BLOB 列即可（Vivy 已有 SQLite/PG 双实现先例）。
- 方案 B：扩展 BlobStore 加 ListGenerations/GetGeneration。省一张表，但把文件语义塞进 checkpoint 命名空间，查询别扭，且 PG/SQLite 两处都要扩。
- 与 filetracker 的合并存储（研究 §8.4 既定）：`file_reads(session_id, path, read_at)` 同批设计，read 工具记录、写前校验——两个 CRUD 一次迁移做掉。

---

## 6. 决策点（回写 TODO 时挂 Open，由用户拍板）

| # | 决策 | 建议 |
|---|---|---|
| O1 | 承诺范围：只 L1 / L1+L2 | L1 随 VC-1，L2 随 VC-3（分两步，不做一步到位） |
| O2 | 保留策略：每 (session,path) 保 N 版 + 单版上限 | N=20、单版 1MB（对齐工具面 maxFileBytes），会话删除时级联清理 |
| O3 | 存储：新表（A）vs BlobStore 扩展（B） | A |
| O4 | 存档范围：code face 真实工作区全量、主线 scratch 轻量（只记 hash+diff 残影） vs 两面同链 | 两面同链、同一套 RPC（实现简单、审计一致；scratch 上链的成本就是几 KB/版） |
| O5 | 恢复动作的治理：恒审批 vs 跟随审批策略 | 跟随策略 + precondition hash 强制；danger_full_access 下也保留 hash 校验（防覆盖并发写） |
| O6 | bash 写坏的文件（非 write/patch 路径）是否纳入 L1 | 第一版不做文件级捕捉（bash 影响面靠沙箱+白名单约束）；记录为已知边界 |

---

## 7. 回写动作

- `docs/TODO.md` RB-1 行：结论回写（本文件 + §5 建议待用户确认 O1..O6 后转 DONE 或按拍板结果改写）。
- VC-1 行备注追加：存档/filetracker 合并存储设计 + L1 为 bash 化安全网前提。
- VC-3 行备注追加：L2 会话级回退与恢复 RPC/UI 挂靠。
