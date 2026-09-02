# JOURNAL-REWIND-AND-FORK — 会话内消息编辑 / 回退 / 分叉 设计提案

> **Status:** proposal（设计片；实现未开始）
> **Date:** 2026-09-02
> **Closes toward:** TODO §0.1 行 UI-CHAT-ACT（`MessageBubble.tsx` 中编辑/回退/分叉三个禁用占位）
> **Terrain map:** 本提案基于 2026-09-02 对 `internal/storage`、`internal/runtime`、`internal/rpc` 的逐文件现场盘点（引用处 file:line 均为当日事实）。
> **Related:** `VIVY-ASSEMBLY.md`（事件合同）、`hitl-review-center.md`、RB-1 调研（文件级回退——**另一根轴**，见 §1.3）。

## 1. 问题与语义

### 1.1 用户故事

聊天流里用户对第 N 条自己的消息不满意时：

| 动作 | 语义 | 分支后旧消息 |
|---|---|---|
| **编辑（edit）** | 修改第 N 条内容，从这条重跑；N 之后的一切（含 N）退出上下文 | 留档不可见 |
| **回退（rewind）** | 从第 N 条（含）起整体作废，回到第 N-1 条末尾重发/重生成 | 留档不可见 |
| **分叉（fork）** | 以"到第 N 条为止"的历史为底新建一个会话，原会话原样保留 | 原会话全部可见 |

三者共享同一个内核原语：**"在消息 M 处作废其后历史"**。编辑/回退 = 原语 + 重发；分叉 = 原语的旁支形态。

### 1.2 硬约束（盘点结论）

1. **Journal 是追加式事实源**（`Journal.Append` + `ErrRunClosed`，contracts.go:57-64）：全仓唯一成块删除是 `DeleteSession` 的整会话事务（sqlite/sessions.go:107-114）。消息表无删除，run_events 无删除。
2. **模型上下文读的是 messages 表**，不是事件流（`runMessages` → `ListMessages`，service.go:1151-1179）；事件流服务订阅/恢复/checkpoint。**messages 表 = 模型的事实源**，run_events = 审计/订阅事实源——两根轴都必须被尊重。
3. **压缩已建立"读时折叠"先例**：`CompactSession` 不删任何东西，落 `SessionCompaction{TailFrom}` 行（compaction_service.go:168-178）+ `context.compacted` 事件，折叠发生在读取时（`foldSessionHistory`，service.go:1186-1217）。截断天然属于同一机制家族。
4. **会话内无 per-session 活跃 run 闸**（`s.active` 按 runID 键控，service.go:380；`RunWithOptions` 无条件起跑）——截断必须自带忙检查。
5. 子 run 语义（`domain.Run.Kind/ParentID/RootID/Depth`，domain/run.go:85-94）存在，但**子 run 重启后不重执行**（service.go:629-631）——fork 若复用 child 语义会继承这个恢复不对称，**不采用**（见 §3.4）。

### 1.3 非目标

- **文件回退**：RB-1 已拍板文件级版本链走 VC-3 存储，本提案不碰。
- **物理删除**：不提供任何 DELETE FROM messages/run_events 的截断实现（审计轴不可断）。
- **多会话批量操作 / 跨会话重排**：只做单会话内单点截断。
- **重试/重新生成本身**：已交付（重新生成 = 重发上一条用户输入开新回合，2026-08-26）。

## 2. 核心原语：逻辑截断标记（不删行）

### 2.1 存储（migration 020，sqlite + postgres + conformance 同型）

```sql
CREATE TABLE session_truncations (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,   -- pg: BIGSERIAL
    session_id    TEXT NOT NULL,
    cutoff_message_id TEXT NOT NULL,   -- 作废区间起点（含）
    tail_message_id   TEXT NOT NULL DEFAULT '',  -- 标记时刻会话末条消息 id（含）
    reason        TEXT NOT NULL,       -- 'rewind' | 'edit' | 'fork'
    fork_session_id TEXT,              -- reason='fork' 时指向新会话
    created_at    INTEGER NOT NULL
);
CREATE INDEX idx_session_truncations_session ON session_truncations(session_id, id);
```

- **有效截断 = 该 session 全部 rewind/edit 标记的并集**（R2 修正，e2e 回放抓出：编辑流 = rewind + 新回合 + 可能再次 rewind，若"最新一条标记胜出"，第二次回退会把第一次已作废的区间复活；视图 = 存储列表减去各闭区间 `[cutoff, tail]` 的并集。fork/forked-from 锚行不进视图折叠，也不得顶掉 rewind 标记——视图读取走 `ListViewTruncations`（仅 rewind/edit），审计读取走 `LatestSessionTruncation`（任意原因最新）。旧行永远保留——作废的作废本身也是事实）。
- 消息表行**原样保留**：被截区间内的消息在读取层被过滤，不是被删。审计轴完整：run_events、被截消息、截断标记三者都可回放。
- **尾锚（tail anchor）**：截断隐藏的是**闭区间 `[cutoff, tail]`**——标记时刻会话里已存在的最后一条消息。rewind 之后追加的新回合（id 在 tail 之外）保持可见：编辑流 = rewind + 全新 `turn/start`，若无尾锚，折叠会把重试本身永久藏掉（R2 e2e 离线回放抓出的设计缺陷，初版"cutoff 之后全部作废"的开放区间语义已废弃）。未知 cutoff/tail 一律 fail-open（不折叠）。
- 大小预算：标记行只存 id 不存内容，无 1MB 类上限问题；每会话标记数天然有界（每次人工动作一条）。
- conformance 套件新增组（守卫 20→21+）：`latest` 决胜、跨会话隔离、空会话零标记、标记对 `ListMessages`/`runMessages` 的过滤效应、**rewind 后追加回合的可见性**。

### 2.2 读取层折叠（单一过滤点）

`ListMessages` 的**运行时消费方**统一改为"取该 session 全部 rewind/edit 标记，按闭区间 `[cutoff, tail]` **并集**剔除"（尾锚语义见 §2.1）：

- `runMessages`（模型上下文，service.go:1151-1179）——截断后模型看到的历史以 cutoff 为界；
- `session/messages` RPC（UI 列表，control.go:908）——UI 同样看到截断后视图（旧消息在数据库里，但产品视图以有效截断为准）；
- `trajectory/session`（轨迹投影）——同一过滤，保持三视图一致。

**与压缩的复合顺序**：截断先于压缩折叠——cutoff 之后的消息先被剔除，`SessionCompaction.TailFrom` 已落在被剔除区间的压缩行随之失效（其摘要对应的尾部已不在上下文；不删行，折叠函数自然跳过）。foldSessionHistory 内按 `truncation → compaction` 两段实现，单一函数内可测。

**不折叠的读取方**：`run/log`（单 run 事件回放）与重启恢复（service.go:585-667）**不过滤**——被截 run 的事件流仍然完整可审计，恢复逻辑照旧处理其终态；恢复重建的是"非终止 run"，与截断正交（§2.4 冲突面除外）。

### 2.3 事件与审计

截断动作本身入事件流（沿用压缩的 `RecordExternalRunEvent` 模式，compaction_service.go:779，合成 run id `tr_<seq>`）：

```json
{ "type": "session.truncated", "payload_version": 1,
  "payload": { "session_id": "...", "cutoff_message_id": "...",
               "reason": "rewind|edit|fork", "fork_session_id": "..." } }
```

**不走审批**：截断是用户对**自己会话视图**的产品动作，不触发工具/副作用；与 `session/delete` 同级（后者也无审批）。D-010 语义不受影响（不新增敏感载荷）。

### 2.4 忙检查与中断

`Service.RewindSession`/`ForkSession` 入口处：列出该 session 的活跃 run（`ListActiveRuns` 按 session 过滤），非空即返回 `ErrSessionBusy`（新错误，runtimeError 映射 Conflict/InvalidParams——映射点 §4 定）。UI 侧 `actionsDisabled` 已在 run 进行中禁用按钮（MessageBubble.tsx:138），双保险。**不提供"截断并取消"复合动作**——取消已有专门 UI（`turn/interrupt`），保持单一关切。

## 3. 三个动作的实现形态

### 3.1 回退（rewind）

`session/rewind` `{ session_id, message_id }`：

1. busy 检查（§2.4）；
2. 校验 message 属于该 session 且未已在有效截断区间内；
3. 写 truncation 标记（reason='rewind'）+ `session.truncated` 事件；
4. 返回截断后视图游标。UI 收到后把输入框预填被回退的用户消息文本（本地行为），由用户重发走既有 `turn/start`——**内核不做"自动重发"**，编辑与回退都终结于显式的新回合。

### 3.2 编辑（edit）

= `session/rewind`（cutoff=被编辑消息）+ 既有 `turn/start`（新内容）。被编辑消息本体退出上下文，新内容作为新 user 消息追加。**UI 无需新 RPC**——两次既有调用组合。语义上"编辑后旧内容不可见但留档"，与 1.1 表一致。

### 3.3 分叉（fork）

`session/fork` `{ session_id, message_id, title? }`：

1. busy 检查（原会话）；
2. 新建 session（`SessionStore.CreateSession`，title 默认 `原标题 · 分叉`）；
3. **复制** ≤ message_id 的**有效视图**消息行到新 session（含附件引用；附件 data_url 本就内联在消息行）。复制而非引用：fork 后两个会话各自独立演化，避免跨会话读穿透；成本 = 一次有界 INSERT（会话历史已是 MB 级预算）。R2 修正（e2e 回放抓出）：截点定位与复制均按**有效视图**（截断标记折叠后的行）而非原始 stored 列表——已被 rewind 折出的行不得在子会话复活，子会话上下文 = 原会话 fork 点处的可见上下文；对已折叠消息请求 rewind/fork → `ErrInvalidCutoff`；
4. 在**原会话**写 truncation 标记？——**否**。fork 原会话不变！标记写在原会话仅为记录 fork 事实，但 reason='fork' 的标记**不改变原会话视图**（cutoff_message_id 记为 fork 点、过滤规则对 reason='fork' 跳过过滤、只作审计与防重放锚）。另在**新会话**写 `fork_session_id` 反向溯源行（reason='forked-from'，同样不过滤）。R2 补签（e2e 回放抓出）：锚行**也不得遮蔽**视图规则——视图读取（`LatestViewTruncation`，只取 rewind/edit 最新）与审计读取（`LatestSessionTruncation`，任意原因最新）分离；若视图读取也取"最新行"，晚写的 fork 锚会顶掉更早的 rewind 标记令折叠复原；
5. 新会话写入 `session.forked` 事件（payload 带 parent_session_id + fork 点），返回新 session_id；UI 跳转新会话。

**不采用 child-run 语义的原因**：fork 的产物是一个用户可见的**会话**（可继续多回合、有自己的 compaction/待办/审批），而 child run 是单回合执行单元且重启不重执行——二者生命周期不同构。

### 3.4 分叉之后

- 新会话完全独立：独立 compaction 行、独立 truncation 序列、独立 token 统计；
- 原会话 `session/compactions`、轨迹、消息均不变；
- `ListRunsBySession`（runs 表）不迁移——被 fork 的历史 run 仍属原会话，新会话的回合从 fork 后第一 turn 开始，轨迹面板自然为"新会话新轨迹"。

## 4. RPC 面（新增 2 个方法，dispatch switch control.go:475-658）

| Method | Params | 返回 | 错误 |
|---|---|---|---|
| `session/rewind` | `{session_id, message_id}` | `{cutoff_message_id, remaining_count}` | InvalidParams / NotFound（message 不属于会话）/ Conflict（会话忙 / 已截断点无效） |
| `session/fork` | `{session_id, message_id, title?}` | `{session_id}` | 同上 |

`runtimeError` 新增 `ErrSessionBusy`、`ErrInvalidCutoff` 映射。能力清单（capabilities 列表，control.go:477-502）登记两方法。

## 5. UI 接线（MessageBubble 占位启用）

- **编辑**：气泡进入本地编辑态（textarea）→ 确认后 `rewind(M)` + `startTurn(新文本)`；取消即回。排队中（queuedMessages 非空或 runBusy）按钮保持禁用（现状继承）。
- **回退**：确认对话框（"此后的 N 条消息将退出上下文（留档）"）→ `rewind(M)` → 输入框预填原文本。
- **分叉**：确认对话框 → `fork(M)` → 跳转新会话。
- store：`rewindSession`/`forkSession` 动作 + 截断后 `listMessages` 重读（既有刷新路径复用）；welcome 后 `session/messages` 自动获得过滤视图，无前端额外状态。

## 6. 测试计划

1. **存储 conformance**：标记 CRUD + latest 决胜 + 过滤效应（sqlite/postgres 双后端）。
2. **runtime**：rewind 后 `runMessages` 不含 cutoff 后消息；截断后 compaction 失效路径；busy 负例（活跃 run 时 rewind → ErrSessionBusy）；fork 后新会话上下文 = 复制历史 + 原 session 视图不变；`session.truncated` 事件落 journal。
3. **RPC**：两方法正/负例（NotFound/Conflict/InvalidParams）；`session/messages` 过滤生效。
4. **端到端**：e2e 建会话 → 两回合 → 编辑第一条 → 断言视图只剩编辑后链 + 新回合照常完成。
5. **恢复正交性**：截断会话含非终止 run 时恢复行为不变（§2.2 不折叠读取方）。

## 7. 实施切分建议（实现未排期，按价值顺序）

- **R1**：migration 020 + `SessionTruncationStore` + 折叠 + `session/rewind` RPC + busy 闸（纯内核，UI 只启用"回退"占位）。
- **R2**：`session/fork` + UI 编辑/分叉接线 + e2e。
- **R3（可选）**：轨迹/压缩复合细节打磨 + 文档收口。

每片独立过 `just ci` + conformance + 单提交。

## 8. 公开问题（实现前需拍板）

1. ~~`cutoff_message_id` 用消息 id 还是 `(created_at, id)` 复合决胜~~ **已拍板（R2）**：折叠按**列表位置**匹配 id（`ListMessages` 次序即权威），不比 created_at；追加 **`tail_message_id` 尾锚**限定作废闭区间（§2.1），位置匹配天然避开同 created_at 并列问题，conformance CN-21 钉死。
2. fork 复制的附件大消息是否设条数上限——倾向沿用会话历史既有预算，不新设。
3. `session.truncated` 事件是否要进 `session/context` 的折叠摘要提示——倾向不进（截断是用户动作，不是上下文预算事件）。
