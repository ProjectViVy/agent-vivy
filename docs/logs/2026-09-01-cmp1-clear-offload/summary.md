# CMP-1 — reduction clear 转存 Backend（文件级恢复）

## What changed

eino reduction 的 clear 阶段此前在 Vivy 里只跑 clear-only（`Backend: nil`）：
被清的老工具结果变成内存占位，内容彻底丢弃。本切片把转存半边接上：

- `runtime.EngineConfig` 新增 `OffloadBackend *EinoFilesystemBackend`
  （internal/runtime/engine.go）。选具体类型而非 eino 接口：typed-nil
  指针装箱成非 nil 接口会让 eino 误判 offload 模式、随后每次写都在 nil
  receiver 上失败——在 `buildCompactionHandlers` 里做 nil 检查后转接口，
  typed-nil 被结构性排除。
- `buildCompactionHandlers` 新参 `offload`，写入 eino reduction
  `Config.Backend`；`ReadFileToolName: tools.ReadFileName`（占位文案点名
  Vivy 的 read_file）；`GenClearOffloadFilePath: genClearOffloadPath`。
- `genClearOffloadPath`（internal/runtime/compaction_middleware.go）：工作区
  相对、正斜杠路径 `compaction/clear/<call-id>`。eino 默认路径是
  `filepath.Join(RootDir, "clear", callID)`（RootDir 默认 `/tmp`，Windows
  反斜杠进占位文案）；自定义后占位文本跨平台可被 read_file 直开。
- `safeOffloadCallID`：provider 发的 call id 直接成为文件名。eino 默认实现
  不做净化——恶意/异常 id（`..\x`、`../../x`）会试图逃逸 run workspace（下游
  safeWorkspacePath 虽 fail-closed，但会让整个 clear 报错）。此处白名单
  `[a-zA-Z0-9_-]`≤128，越界或为空回退 `uuid.NewString()`（与 eino 默认语义
  一致）。go.mod 中 google/uuid 由 indirect 转直接。
- app 装配（internal/app/compaction.go `buildEngineConfig` + app.go 两处
  调用）：复用既有 `fileBackend`（与 AgentsMDBackend 同一 run workspace
  后端），启动与 settings-save 重载两条路径一致。

## 行为

- 有 workspace（`runtime.workspace_root` 配置）时：clear 触发 → 老工具结果
  内容写入 `<run-workspace>/compaction/clear/<call-id>`，占位文案
  `<persisted-output>Tool result saved to: compaction/clear/<call-id> …
  Use read_file to view.`，模型同 run 内可 read_file 取回。
- 无 workspace（`fileBackend` 为 nil，如直接 runtime 测试座架）：与旧版
  逐字节一致（无 offload 的内存占位）。

## What was explicitly not done

- Offload 文件的跨 run 生命周期/清理：run workspace 本身已有隔离与清理
  语义，转存只在 run 内有意义，未加额外 TTL/清理钩子。
- summarization 摘要结果的 offload（eino summarization 无此 seam）。
- 压缩设置 UI 覆盖层（CMP-2 遗留，同属另开切片）。
