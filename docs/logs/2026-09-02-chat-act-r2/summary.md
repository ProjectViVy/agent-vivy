# UI-CHAT-ACT R2 — session/fork 内核 + UI 编辑/回退/分叉接线

## What changed

R1（80fe3b6）落了内核 `session/rewind`（截断标记、读时折叠、控制器）。
R2 按设计 `docs/architecture/JOURNAL-REWIND-AND-FORK.md` 补齐另一半：
内核 `session/fork` + UI 三键全部接线，UI-CHAT-ACT 主体闭账。

### 内核（fork = 复制历史到新会话，原会话不动）

- **尾锚修正（e2e 离线回放抓出 R1 设计缺陷）**：初版折叠按"cutoff 之后全部
  作废"的开放区间实现，rewind 后追加的新回合也被折叠出去——编辑流
  （rewind + 重发）的重试会永久消失，3015 上表现就是重发后视图为空。
  修正：`session_truncations` 增加 `tail_message_id`（migration020 尚未随
  任何已发布 DB 落地，原地扩展），折叠改为隐藏**闭区间 `[cutoff, tail]`**
  （标记时刻会话末条消息）；rewind 之后追加的回合在 tail 之外，保持可见。
  设计文档 §2.1/§2.2/§8-Q1 已回签（未知 cutoff/tail fail-open 不变）。
  CN-21、rewind_service、control route 三层都加了"rewind 后追加回合可见"
  回归断言；CN-21 另加 cutoff==tail 用例（回退最后一条时两锚同 id，必须
  各自独立解析，否则折叠静默失效——e2e 第二轮抓出的第二个折叠缺陷）。
- **有效视图修正（e2e 离线回放抓出）**：fork 最初按原始 stored 列表复制
  `≤ fork 点` 的消息行——编辑流（rewind + 重发）之后再分叉，被 rewind 折出
  上下文的原文会在子会话复活。修正：rewind/fork 截点定位与 fork 复制全部
  改按**有效视图**（截断标记折叠后）——子会话上下文 = 原会话 fork 点处的
  可见上下文；对已折叠消息请求 rewind/fork → `ErrInvalidCutoff`（与
  ErrInvalidCutoff 注释早已声称的"already behind the effective truncation
  point"对齐）；`remaining_count` 同步改为视图相对计数。设计文档 §3.3
  第 3 步已回签。回归：`TestRewindAndForkRespectEffectiveView`（隐藏消息
  双负例 + 复制不复活 + 视图相对 remaining）+ e2e 子会话复活守卫断言。
- **并集折叠修正（e2e 第三轮抓出）**：视图最初取"最新一条 rewind/edit 标记
  胜出"——但编辑流本身就是 rewind + 新回合 + 可能再次 rewind，第二次回退
  （对重试消息）会在 latest-wins 下顶掉第一次的标记，把已编辑掉的原文
  复活（e2e reload 后 `hello vivy` 重现暴露）。修正为**并集折叠**：视图 =
  存储列表减去该会话全部 rewind/edit 闭区间 `[cutoff, tail]` 的并集；
  `TruncationStore` 读取语义拆分——`LatestSessionTruncation`（审计读，
  任意原因最新）不变；新增 `ListViewTruncations`（视图读，仅 rewind/edit
  按插入序全量返回）+ `storage.ApplySessionTruncations`（区间并集遮罩），
  单标记 `ApplySessionTruncation` 变为薄封装。`effectiveSessionMessages`
  与 `session/messages` 折叠点（control.go）均改走并集。CN-21 加"晚写的
  fork 锚不进视图折叠、连续 rewind 累积、标记间追加行的非连续并集"断言；
  runtime 加 `TestSuccessiveRewindsAccumulate`。设计文档 §2.1/§2.2 回签。
- `internal/runtime/rewind_service.go`：
  - `ForkSession(ctx, sessionID, messageID, title)`：busy 门禁（同会话有活动
    run → ErrSessionBusy）→ `sessionViewCutoff` 在有效视图中定位截点（缺 →
    ErrInvalidCutoff）→ 新会话（`sess_` 前缀；标题缺省 = 源标题 + " (fork)"，
    本地化文案由 UI 传入）→ 复制有效视图 `[:cutoff+1]`（含截点）到子会话 →
    双向存证标记：父会话 `fork`（`fork_session_id` 指向子）、子会话
    `forked-from`（指回父，截点记子侧副本 id）→ 经 `RecordExternalRunEvent`
    在 `tr_` 合成 run 上记 `session.truncated`（reason=fork）与
    `session.forked`（parent_session_id + 截点）。
  - 设计偏离（已回签设计文档）：副本消息铸造全新 `msg_` id（messages.id 是
    全局唯一主键，不是 (session_id,id)）；子会话 forked-from 标记的截点指向
    子侧副本 id；标题回退用 " (fork)" 后缀（区域中立），UI 传本地化标题。
  - `RewindSession` 重构共用 `rejectBusySession` / `sessionViewCutoff` 助手。
- `internal/storage/contracts.go`：新增 `TruncationFork`/`TruncationForkedFrom`
  原因常量；`ApplySessionTruncation` 判定反转 —— 只有 rewind/edit 过滤，
  fork/forked-from 是纯出处锚点永不折叠，未知未来原因同样不折叠（fail-safe）。
- `internal/domain/event.go`：`EventSessionForked`（`session.forked`）入词表。
- `internal/rpc/control.go`：`session/fork` 方法（`{session_id, message_id,
  title?}` → `{session_id, fork_point_message_id, copied_count}`），错误映射
  与 rewind 一致（busy → -32009，缺截点 → -32004）；capabilities 追加
  `session.fork`；`listMessages` 接上折叠（`Truncations` 注入）。
- `internal/app/app.go`：两处装配点注入 `Truncations`。
- 测试：`rewind_service_test.go`（ForkSession 全流程：复制数/继承
  SandboxMode+ApprovalPolicy/双向标记/`session.forked` 事件/负例 busy+缺截点；
  Rewind 共用助手后回归），`conformance/suite.go` CN-21（最新胜出折叠 +
  fork/forked-from 直通 + 陈旧截点 fail-open），`control_test.go`
  TestSessionForkRoute（端到端 RPC 含信封断言）+ TestSessionRewindRoute，
  `domain` 词表守卫 38。

### UI

- `ui/src/lib/api.ts`：`rewindSession`/`forkSession` fetchers + 方法表注册。
- `ui/src/lib/store.ts`：`rewindSession`（RPC 后重拉 context + 消息视图）、
  `forkSession`（RPC 后重拉会话列表，返回新 id）。
- `ui/src/components/chat/MessageBubble.tsx`：用户气泡「编辑」启用 —— 气泡内
  就地 Textarea + 保存（`chat.editSave`）与取消；助手气泡「回到这里」「从此
  分叉」启用，均带 AlertDialog 确认（`rewindConfirm*`/`forkConfirm*`），运行
  期间照旧禁用。旧的 `chat.pending` 占位按钮删除。
- `ui/src/components/chat/ChatView.tsx`：`handleEdit` = rewind + `submit(新文本)`
  （内核不做自动重发，重发语义在 UI 层）；`handleRewind` = rewind 后把仍在
  上下文里的最近一条用户输入预填输入框；`handleFork` = fork + 跳转新会话；
  动作失败经 RecoverableError 呈现（重试仅清错）。
- `ui/src/components/chat/ChatInput.tsx`：`draftPreset` 属性（seq 变化时写入
  草稿并聚焦，ref 防同一 seq 重复应用）承载回退预填。
- `ui/src/i18n/en.ts`/`zh.ts`：`chat.editSave`/`editCancel`/`rewindConfirm*`/
  `forkConfirm*` 双语。

## What was explicitly not done

- 工具消息气泡上的回退/分叉（按钮只在助手文本气泡；工具结果折叠展示不变）。
- 队列消息、运行中会话的动作（busy 门禁内核侧已拒，UI 照旧禁用）。
- R3 打磨项（分叉标题本地化传递、子会话轨迹空态文案等）视预算另行处理。
- `initialize()` boot 窗口竞态（落定前点「新建会话」会被尾部自动选中覆盖，
  e2e 回放暴露）：本轮以规格等待信号绕开，产品侧修复另立 `UI-INIT-RACE`。

## Board

`docs/TODO.md` UI-CHAT-ACT 行翻 DONE（R1 内核 + R2 接线/回放全交付；
编辑/回退/分叉三键实装），§10 记账。
