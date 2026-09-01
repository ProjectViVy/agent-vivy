# Acceptance — CMP-1 clear offload

## 人工如何确认

1. 启动开发栈（`just run` + `cd ui; pnpm dev`，开
   <http://127.0.0.1:3015>），确认 `config.yaml` 配了
   `runtime.workspace_root` 与启用了 `runtime.compaction`。
2. 与 Vivy 连续对话，让它执行十几个产生大输出（几 KB）的工具调用
   （例如连续读大文件），直到上下文越过压缩触发线——聊天流出现
   「上下文已压缩」（context.compacted）事件。
3. 之后在同一会话里问早期某个工具调用的详细内容：模型应能调用
   `read_file`（路径形如 `compaction/clear/<call-id>`，压缩占位文本里
   写明）取回原始输出并作答——这是本切片的核心验收：被清结果可恢复。
4. 打开该 run 的 workspace 目录（`<workspace_root>/<runID>/`）：
   `compaction/clear/` 下应有与被清工具调用数一致的文件，内容为原始
   工具输出。
5. 未配 `workspace_root` 的部署：压缩行为与之前一致（占位不转存），
   无报错、无行为差异。

## 回归面

- 压缩触发/摘要链路（CMP-2 的 failover、usage 事件）不受影响——既有
  compaction 契约测试全绿。
- 无 workspace 场景逐字节保持旧行为（nil backend → 无 offload 占位）。
