# UI-INIT-RACE 修复交付总结

## 交付主题
修复应用初次加载窗口期内用户显式创建/切换会话时，因 `initialize()` 异步快照与尾部自动选中覆盖导致的操作丢失（UI-INIT-RACE 闭环）。

## 改动范围

1. **会话合并与意图保护（`ui/src/lib/store.ts`）**：
   - `initialize()` 异步拉取后端快照（`api.listSessions()`）返回后，去重合并等待期内用户在内存中已创建或更新的会话（`freshlyCreated` + `inFlightMap`），避免覆盖刚建的会话导致抽屉丢失新项。
   - 若合并后已有会话（用户在等待期已建），不再次调用 `api.createSession('')` 冗余创建空白兜底会话。
   - 检查 `get().activeSessionId`：若当前 store 内已有合法激活的会话（存在于合并列表中），坚决不回写覆盖，也不触发冗余的 `selectSession`；仅在当前未选择有效会话时，才遵循 localStorage 记忆值或列表首项进行初始化选定。

2. **发送丢弃保护与告警（`ui/src/lib/store.ts`、`ui/src/i18n/{zh,en}.ts`）**：
   - `startRun` 与 `editSession` 在检测到 `get().activeSessionId !== sessionId` 时，不再静默 `return`，而是设置 `runError` 并抛出 `Error(t('errors.sessionMismatch'))`。
   - 调用方 `ChatInput` 的 `try...catch` 捕获该异常，保留用户输入的草稿文本与待发送附件（不再执行 `setValue('')`），同时 `ChatView` 渲染 `RecoverableError` 提示条，杜绝静默吞消息与草稿丢失。
   - 补充 `errors.sessionMismatch` 中英文国际化文案。

3. **单测覆盖（`ui/src/lib/store.test.ts`）**：
   - 新增 4 个针对性单元测试，覆盖初始化挂起期间创建会话防覆盖、合并列表去重、防冗余创建兜底会话、以及会话不匹配时的异常抛出与 `runError` 状态记录。

4. **审计与台账（`docs/TODO.md`）**：
   - 更新 §0.1 中的 `UI-INIT-RACE` 条目为 `DONE 2026-09-04`，记录闭环详情。

## 明确未做（Out of Scope）
- 不修改服务端的会话创建与列表 RPC 协议。
- 不影响会话运行中的队列管理逻辑（运行中消息仍按既定 Crush 规范正常入队）。
