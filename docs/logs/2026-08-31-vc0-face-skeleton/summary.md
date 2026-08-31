# VC-0: face 骨架（kernel + UI + schemas）

日期：2026-08-31。分支：`feat/vc0-face-skeleton`（worktree `agent-vivy-vc0`）。

## 改了什么

引入 per-run 维度 `face`（web | tui | code），与 RunMode 同层：face 是入口装配
归因（哪个入口在服务这次 run），不是进程模式。VC-0 只做装配骨架与 code face
的 prompt 框定，不改工具面。

- 内核
  - `internal/domain/face.go`：`Face` 枚举与 `Valid()`。
  - `internal/runtime/face.go`：`normalizeFace`（空 → web；未知 → `ErrInvalidFace`）、
    `withFace`/`runFace`（context 未绑定时回落 web）。
  - `internal/runtime/service.go`：`RunOptions.Face`；`run.started`、
    `tool.approval_required`、`user.question_required` 事件 payload 增加
    `face` 字段（非 omitempty，统一显式写 web）；挂起/恢复链
    （approvalDetails/questionDetails → rebuildPending/rebuildPendingQuestion →
    resumeRun）逐环传递 face，中断恢复不丢失。
  - `internal/runtime/prompt.go`：`composeRunPreamble` 按 face 注入 code 模式前导
    （直接操作本 run workspace 文件；未经用户明确要求不做任何版本控制操作；
    代码引用尽量 path:line）。静态 Instruction 不带 face（引擎级单例）。
  - `internal/runtime/preflight.go`：预检校验并回显 face。
  - `internal/rpc/control.go`：`preflight/run`、`turn/start` 接受 `face` 参数；
    响应回显；`ErrInvalidFace` 映射 RPC -32602。
- schemas：`schemas/events/payloads/{run.started,tool.approval_required,user.question_required}.json`
  增加 `face` enum 字段（旧 Journal 无该字段视为 web）。
- UI（VC-0c）
  - `ui/src/lib/api.ts`：`Face` 类型；`preflight`/`startTurn` 透传 `face`。
  - `ui/src/components/masks/mask-catalog.ts`：`faceForMaskId` —— programmer
    面具 → code face，其余面具不指定（服务端默认 web）。
  - `ui/src/lib/store.ts` / `ui/src/components/chat/ChatView.tsx`：startRun、
    预检、pending 续跑、重新生成全部携带当前面具的 face。
  - i18n：programmer 面具能力列表加"以 code face 运行"/"Runs with the code face"。
- 契约文档：`docs/dev/NEW_UI_ARCHITECTURE.md` 同步 Face 类型与 preflight 字段。

## 设计取舍

- **不建 face→工具面过滤表。** D1 决议：bash/job/grep/glob/multiedit 是主线
  共享工具，所有 face 通用；等 VC-3 插件工具带来真实差异化的面时再引入切换
  机制（届时挂 `withSelectedTools` 同层）。
- **Masks 仍是 UI 本地人格选择**，不写内核；programmer → code 是纯 UI 映射。
- `tui` 仅保留枚举位，尚无入口装配。

## 明确未做

- face 专属工具面/权限差异（VC-3）。
- tui 入口。
- 消息流里向用户展示 face 徽标（可选，未排期）。
