# CMP-3 — 会话压缩摘要检索入口（session/compactions + 压缩历史面板）

## What changed

`session_compactions` 表此前只有写入与「取最新一条」（`LatestSessionCompaction`，
feed 装配用），没有检索面：操作者无法回看某会话的历史压缩记录。本切片补齐
存储 → RPC → UI 三段检索入口，零新表（沿用既有 `session_compactions`）。

### 存储层

- `internal/storage/contracts.go`：`CompactionStore` 增
  `ListSessionCompactions(ctx, sessionID, limit)` —— 返回该会话记录，newest-first
  （`ORDER BY created_at DESC, run_id DESC`，与 `LatestSessionCompaction` 同一排序
  决胜负）；`limit <= 0` 返回空（负 limit 在 sqlite 里语义是「无界」，必须挡住）。
- `internal/storage/sqlite/compaction.go`、`internal/storage/postgres/compaction.go`：
  同型实现（`?` vs `$n` 方言、postgres 走 `b.db.SQL`）。
- `internal/storage/conformance/suite.go`：CN-20「compactions listed by session」
  （守卫 19→20）：三记录跨两 session，断言排序（同 created_at 平手由 run_id DESC
  决出）、行字段 round-trip、limit 截断、未知 session 空集、limit=0 空集。

### RPC 层

- `internal/rpc/control.go`：`ControlDeps.Compactions`（nil → MethodNotFound，
  与 SkillRevisions/Todos 同形）；新方法 `session/compactions`：
  - `session_id` 必填（缺 → InvalidParams），未知 session → CodeNotFound；
  - `limit` 可选，默认 50，钳到 200；
  - 返回 `{compactions: [{run_id, created_at, tail_from, dropped_count, summary}]}`；
  - `summary` 是不可信生成内容（进过模型 feed），服务端原样透传，渲染防注入交给
    UI 的 React 文本节点。
- `internal/app/app.go`：组装层 `Compactions: backend` 接线。
- `internal/rpc/control_test.go`：`TestControlHandlerListsSessionCompactions` 全断言
  —— 空 `[]`（非 null）、新到旧排序、字段 round-trip、`limit=1` 截断、未知 session
  NotFound、缺参 InvalidParams、未接线 MethodNotFound。

### UI 层

- `ui/src/lib/api.ts`：`RPC_METHODS` 登记 `session/compactions`；
  `SessionCompactionRecord` 类型；`listSessionCompactions(sessionId, limit=50)`。
- `ui/src/components/settings/CompactionSettingsCard.tsx`：用量面板下新增
  「压缩历史」块——无会话 → 引导文案；有会话无记录 → 空态
  （`data-testid="compaction-history-empty"`）；有记录 → 条目列表
  （run 徽标 + 本地时间 + 折叠消息数 + 摘要 line-clamp-3，
  `data-testid="compaction-history"`）。会话切换自动加载；「立即压缩」成功后
  自动刷新；「刷新」按钮同刷历史。摘要以 React 文本节点渲染（自动转义）。
- `ui/src/i18n/en.ts`、`zh.ts`：`settings.compaction.historyTitle/historyLoading/
  historyEmpty/historyOpenSessionHint/historyRun/historyDropped` 六键双语。
- `ui/e2e/compaction-setting.spec.ts`：新增「压缩历史面板」规格——zh/en 双语下
  标题可见、空态或引导可见、原始 i18n 键回归线。

## What was explicitly not done

- 摘要正文的展开/全文查看 UI（现按 line-clamp-3 折叠展示）——如需全文检索再开切片。
- 跨 session 的压缩记录聚合/管理页（CMP-3 语义是单会话检索入口）。
- postgres 后端的 CN-20 实跑：postgres 一致性套件需要外部 DSN 才启动（既有约定），
  本切片在 sqlite 上全绿；postgres 实现与 sqlite 同型同测语义。

## Board

`docs/TODO.md` CMP-3 → DONE（§10 同日记录）。
