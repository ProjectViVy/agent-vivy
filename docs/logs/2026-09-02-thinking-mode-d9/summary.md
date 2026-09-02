# Summary — 思考模式端到端接线（UI-COMPOSER / UI-CHAT-TOOLBAR 可行部分）

## What changed

聊天框的思考模式从纯 UI 状态接成真实内核能力，按 D9（模型元数据归 provider/model
管理，不另起数据源）门控：

- **domain**：新增 `ThinkingMode`（auto/on/off）与 run 级 context 携带
  （`WithThinkingMode` / `ThinkingModeFromContext`）；`ModelInfo` 新增
  `SupportsThinking`（零值保守 false）。
- **provider**：Anthropic 目录逐模型标注 thinking 支持（Claude 3.7 Sonnet 及以后
  代际为 true）；`resolvingChatModel` 在每次调用读取 run context，仅当
  thinking=on 且目录元数据声明支持时注入 `einoclaude.WithThinking`（预算
  4096，低于协议 max_tokens 8192）。未知模型 / 非 Anthropic 后端一律不发该参数
  —— 与附件的 SupportsImages 门同构（零值默认不破坏自定义网关）。
- **runtime**：`RunOptions.Thinking`（空 = auto）经 `normalizeThinkingMode` 校验
  后写入 runCtx；无效值在持久化任何数据前拒绝（`ErrInvalidThinkingMode`）。
- **rpc**：`turn/start` 新增 `thinking` 参数（无效值 → InvalidParams）；
  `session/context` 新增 `thinking_supported`（D9 门控面，UI 由此决定选择器可见性）。
- **UI**：`ChatInput` 思考选择器仅当 `context.thinking_supported` 时渲染（D9 门
  —— 死控件一律不出现）；偏好随发送链（直接发送与排队两条路）贯通
  ChatInput → ChatView → store.startRun → `api.startTurn` → turn/start。

## Explicitly not done

- **OpenAI 兼容路径的 reasoning_effort 旋钮**：eino openai 适配器有
  `WithReasoningEffort`，但 o 系/gpt-5 的"关闭"语义不统一（minimal 仅部分模型
  合法），本代不接；元数据全部 false，选择器不出现。后续如需再立行。
- **'off' 的强制关闭**：Anthropic 缺省即不思考，auto/off 都是"不发参数"；
  对常开推理模型（o 系）off 退化为 provider 默认。
- **AutoDream / 询问模式 / 桌面伙伴 / 语音**：维持 stub（AutoDream 等 MEM-1
  DEFERRED；询问模式无内核语义）。
- **思考偏好持久化**：本轮跟随 Crush 语义为逐回合选择，不做会话级持久化。

## Filing

- TODO 行 UI-COMPOSER / UI-CHAT-TOOLBAR 注释更新（可行部分已交付，余项注明
  阻塞原因）。
