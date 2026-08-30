# 2026-08-30 聊天框按钮整理（chatbox-buttons）

## What changed

主页聊天框（`ui/src/components/chat/ChatInput.tsx`）成为会话操作的唯一入口：

- **新建会话移入聊天框**：顶栏右侧簇新增 Plus 按钮（新建会话 → 历史 → 审批中心），
  调 `store.createSession()`（自动选中新会话）。
- **删除原入口**：
  - `ConversationSidebar.tsx`：侧边栏 Plus 新建会话按钮删除，`onCreateSession/creating/createError`
    props 与 `_layout.tsx` 对应传参删除（`createAndOpen` 一并删除）。
  - `SessionDrawer.tsx`：会话抽屉"新建会话"按钮删除，`onCreateSession` prop 删除。
- **删除按钮（用户指定）**：
  - 语音（Mic）：删除按钮、`recording` 状态与 i18n 文案（原本只是本地状态假演示，无识别、无 RPC）。
  - 桌面伙伴（Cat）：删除按钮与 i18n 文案（原本 no-op "桌面伙伴暂未接入"）。
- **执行模式下拉接线**（智能体/计划/询问）：
  - `ChatInput` 的 `onSend` 签名扩展为 `(content, mode)`；`ChatView.submit` 将 mode
    传入 `preflight/run` 与 `turn/start`（原先硬编码 `'normal'`，链路 `api.startTurn/preflight`
    与 `store.startRun` 本就支持 mode）。
  - 智能体→`normal`、计划→`plan`（后端 `domain.RunMode` 真实支持，plan 拦截非只读工具）。
  - 询问：保留选项，选择时提示"询问模式暂未接入"且不改变当前选择（后端无对应 RunMode）。
  - `ChatView` 的 `pending` 预检暂存携带 mode，"继续"按同 mode 重跑；`regenerate` 走默认 normal。

## Already wired（本次仅验证、未改动）

审批（ShieldCheck → review/list + review/respond）与权限控制（下拉 → session/set_permission，
运行中锁定、信任模式确认弹窗）本来就在聊天框内且接真实 RPC，本次真机 smoke 复核通过。

## Kept per user instruction（未实现、保留 UI）

以下按钮无后端能力，按用户指示保留并在验收时告知：

- 附件（`attachmentUnavailable` 提示）
- AutoDream（`autodreamUnavailable` 提示）
- "＋更多"（`moreUnavailable` 提示）
- 思考模式下拉（自动/开启/关闭，纯本地状态）
- 询问模式（本次起选择时明确提示未接入，不再是静默伪选择）

已记 `docs/TODO.md` §0.1 **UI-COMPOSER**。

## What was explicitly not done

- 未动审批/权限控制的现有实现与 Review Center 面板。
- 未动 MessageBubble 占位按钮（编辑/回退/分叉）。
- 未动顶栏（会话 Sheet、待办、面具/模型切换）。
- 未修复 main 上既有的两处 e2e 失败（见 E2E-STALE），仅在本 lane 的 spec 里
  修掉了同样过期、阻碍本验证的 `画图` 断言。

## e2e

`ui/e2e/runtime.spec.ts`：删除过期的 `画图` 断言；`新会话`（原侧边栏）改为聊天框
`新建会话`（含从 /masks 返回主页再点击）；新增语音/桌面伙伴 `toHaveCount(0)` 负断言。
