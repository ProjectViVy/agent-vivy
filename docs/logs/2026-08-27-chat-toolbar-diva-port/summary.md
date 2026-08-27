# 聊天框上方功能栏移植（Agent-DIVA → Vivy，仅 UI）

Date: 2026-08-27
Status: complete

## Outcome

对照 `agent-diva/agent-diva-gui/src/components/ChatView.vue` 的
`chat-input-toolbar`（真实聊天框上方功能栏），把内容与交互移植到
Vivy `ui/src/components/chat/ChatInput.tsx`。按用户确认的范围：**完全对齐
DIVA 排布（替换现有工具栏）+ 依赖后端的按钮保留「暂未接入」提示条**
（仅 UI，无任何后端改动）。

## 移植后的工具栏（从左到右）

1. **执行模式选择**（下拉，`side="top"` 向上弹出）：智能体（Zap）/
   计划（Settings2）/ 询问（Brain），菜单项 = 图标 + 标题 + 说明 + 当前项
   勾选，选中后触发器图标与文字即时更新（对照 DIVA `modeOptions` +
   `mode-menu`）。
2. **附件**（Paperclip）：点击显示「附件功能暂未接入」提示条（stub）。
3. **思考模式选择**（下拉，`side="bottom"`）：自动（Lightbulb 轮廓）/
   开启（Lightbulb 实心 `fill="currentColor"`）/ 关闭（LightbulbOff）
   （对照 DIVA `ThinkingToggle`）。
4. **AutoDream 触发**（GitBranch）：点击显示「AutoDream 暂未接入」提示条。
5. **桌面伙伴**（Cat）：点击显示「桌面伙伴暂未接入」提示条。
6. **权限模式选择**（下拉，向上弹出）：谨慎（Shield）/ 智能（Sparkles）/
   信任（CheckCircle），菜单项结构同模式下拉（对照 DIVA `permissionOptions`）。
7. 分隔线（`h-4 w-px bg-border`）。
8. **右侧群组**（`ml-auto`，对照 DIVA `.chat-corner-actions`）：
   - **历史**（Clock）：点击打开右侧「会话」抽屉（复用现有
     `SessionDrawer`，DIVA 中该按钮切换会话侧栏）。
   - **审批中心**（ShieldCheck）：点击打开审批中心 Sheet；待审批数
     显示为 DIVA 式**数字角标**（红底白字圆角 pill，替代原红点），
     `aria-expanded` 反映打开状态。

## Delivered

- `ui/src/components/chat/ChatInput.tsx` — 工具栏整行重写，逐项对应
  DIVA；三个下拉用 Radix `DropdownMenu`（`ui/components/dropdown-menu`），
  触发器 `asChild`（`aria-expanded`/`aria-haspopup` 由 Radix 提供）；
  图标按钮沿用 Vivy 的 `rounded-lg p-1.5 text-muted-foreground
  hover:bg-accent` 与 `title`/`aria-label`；视觉沿用 Vivy Tailwind token，
  不引入 DIVA 的 CSS 变量。删除原「画图（Palette）」「智能（Sparkles）」
  按钮与布尔 agentMode 切换（升级为三态下拉）。textarea 与 footer
  （上下文环 / 提示条 / 更多 / 语音 / 发送·停止）保持原样。
- `ui/src/lib/store.ts` — 新增 `sessionDrawerOpen` 状态与
  `setSessionDrawerOpen`（仿 `reviewCenterOpen` 的既有模式）。
- `ui/src/routes/_layout.tsx` — 会话 Sheet 由本地 `sessionOpen` 改用 store
  的 `sessionDrawerOpen`，使 ChatInput 的历史按钮可打开同一抽屉
  （创建/选择会话后的关闭逻辑同步迁移）。
- `ui/src/i18n/zh.ts` / `en.ts` — `chatInput` 词条按 zh 权威结构同步：
  新增 `planMode`/`askMode`/`agentModeDesc`/`planModeDesc`/`askModeDesc`/
  `thinkingMode`/`thinkingModeAuto`/`thinkingModeOn`/`thinkingModeOff`/
  `autodreamTrigger`/`autodreamUnavailable`/`openMate`/`mateUnavailable`/
  `permissionCautious`/`permissionSmart`/`permissionTrusted` 及三个
  `*Desc`；删除失效键 `switchedToNormal`/`switchedToAgent`/`draw`/
  `drawUnavailable`/`smart`/`smartStrategy`/`branch`/`branchUnavailable`/
  `historyHint`。

## 语义差异（与 Agent-DIVA）

- **模式 / 思考 / 权限选择为纯 UI 状态**：不改变发送语义
  （`ChatView.submit` 仍走 `preflight(sessionId, text, 'normal')` →
  `startRun`），也不作为隐藏的后端开关；DIVA 中这些值随 send 事件上报，
  等 Vivy 内核支持对应执行模式后再接入。
- **附件 / AutoDream / 伙伴 / 语音为 stub**：DIVA 中分别做真实上传、
  触发 AutoDream、打开桌面伙伴、录音；Vivy 这些能力未接入，按用户确认
  保留 `showNotice` 提示条，不伪装操作。
- **历史按钮**：DIVA 切换会话侧栏（含列表选择），Vivy 复用
  `SessionDrawer`（右上角会话按钮同源），交互闭环且仍是纯 UI。

## Explicitly not done

- 未新增任何 Go 后端、RPC 或 `api.ts` 传输改动；不触碰 Journal 语义。
- 未实现附件真实选取/上传预览、AutoDream 触发、桌面伙伴、语音录音
  （保持「暂未接入」提示条）。
- 模式/思考/权限选择未做本地持久化（DIVA 将 permissionMode 存
  localStorage `agent-diva.permissionMode`；本轮为会话内状态，后续如需
  持久化应落 `vivy.ui.*` key）。
- footer 行（上下文环、提示条、Plus、Mic、发送/停止）不在本次范围，
  保持原样（其结构已与 DIVA footer 对应）。
- 未新增组件级单测（ui 暂无 @testing-library 基建，按小改动不为此
  前置搭建自动化的原则，以 typecheck + 既有 vitest + 真实路径冒烟覆盖）。