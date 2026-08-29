# 验收视角（2026-08-30，上下文压缩真实生效）

在 `just dev`（或 `just run` + `ui/pnpm dev`）后打开 `http://127.0.0.1:3015`：

1. **聊天框上下文环是真实的**：输入框左侧的环显示「上下文占用：X% · 已用 / 模型窗口 tokens」，悬停可见字节明细（feed 字节 / 字节上限）与「已达压缩阈值 / 已压缩」标记。数字来自服务端 `session/context`（feed 装配口径 + provider 模型窗口），不再是客户端硬编码 256KB 估算。
2. **超支自动压缩**：让会话上下文超过「最大 tokens × 阈值%」（或字节上限）后发消息，运行中自动压缩：先确定性清理旧工具调用/结果占位（保留最近 N 轮），仍超标则用同一模型生成摘要替换历史。`context.compacted` 事件入 Journal（`run/log` 可见 mode/before_tokens/after_tokens），环回落并出现「已压缩」。
3. **设置页配置真实生效**：设置 → 通用 →「上下文压缩」卡：修改启用/最大 tokens/阈值/保留最近消息并「保存配置」，写入 `settings.yaml`；以后每次运行即按新阈值压缩，无需重启（空闲时立即重建引擎，运行中延迟到下一轮）。
4. **立即压缩真实可用**：点「立即压缩」：会话很快出现「压缩完成：before → after tokens」；刷新页面后占用降低且标注「已应用会话压缩摘要」；后续对话的模型可见历史为「摘要 + 最近消息尾部」（Journal/消息列表仍保留完整追加历史）。
5. **不再是演示**：`DivaSettingsPreview` 里的演示「上下文压缩」卡已被真实卡片取代（原演示占位仅剩说明文字）；「执行压缩预览/恢复预览默认值」按钮消失。

## 验收式测试（自动化锚点）

- `go test ./internal/runtime/ -run 'TestEngineSummarizationCompaction|TestEngineReductionRunsBeforeSummarization|TestServiceContextStatusAndCompactSession|TestScheduleEngineReload'` —— 引擎压缩管线、会话级持久压缩、热重建全部通过。
- `go test ./internal/rpc/ -run TestContextCompactionRPC` —— `settings/update` 持久化 compaction、`settings/get` 回显、`session/context`、`context/compact` 通过。