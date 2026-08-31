# VC-1e — AGENTS.md 注入（eino agentsmd 直采）+ `vivy init`

日期：2026-08-31 ｜ 分支：`feat/vc1a-bash-tool`（vc0 worktree）｜ 轨道：VIVY-CODE VC-1

## What changed

**D6 上下文文件注入（只读 AGENTS.md → run preamble）**

- 采用 eino v0.9.13 原生 `adk/middlewares/agentsmd` 中间件（D6 免费直通，零自研注入逻辑）。
  - `internal/runtime/agentsmd.go`：`AgentsMDFileName = "AGENTS.md"`（D6 拍板：不引 CLAUDE.md/VIVY.md 多文件优先级）、累计 64 KiB 注入预算、`AgentsMDBackend` 类型别名（app 层不触碰 eino 类型，D-007）。
  - `internal/runtime/engine.go`：`EngineConfig.AgentsMDBackend`；middleware 注册在 compaction handlers **之后**（官方推荐顺序：注入内容瞬态、不参与摘要压缩）。
  - Backend = 既有 `EinoFilesystemBackend`（结构化满足 `agentsmd.Backend` 的单一 `Read` 方法），按请求 ctx 的 run ID 解析到**本次 run 的私有 workspace** 内的 `AGENTS.md`——逐 run 隔离，不跨 run 泄漏。
- 行为：缺文件 → 中间件 warning + 跳过（run 不受影响）；文件存在 → 每次模型调用在第一条真实 user 消息前注入一条 user 消息（带 idempotency Extra 标记，同一 run 只注入一次）；**瞬态**——从不写入 Journal / 消息存储。
- 配套修复：`safeWorkspacePath` 的 "path does not exist" 错误现在 wrap `os.ErrNotExist`——agentsmd loader 以 `errors.Is(err, os.ErrNotExist)` 区分"文件不存在（跳过）"与"其他读取错误（致命）"，原通用错误会让无 AGENTS.md 的 run 直接失败。消息文本不变，仅补全 sentinel 链。
- app 装配：`buildEngineConfig` 新增 `agentsMDBackend` 参数，startup 与 settings-save 重载两条路径都接到 `fileBackend`（workspace root 未配置时自然不注入）。

**`vivy init` 生成 AGENTS.md**

- `cmd/vivy/init.go`：新子命令 `vivy init`（main.go dispatch 在 worker/tui 之前）。
- 行为对齐研究 §8.4 initialize 要点（行为对齐，零代码拷贝，FSL-1.1-MIT）：
  - **空目录拒绝**（仅含 `.git` 等隐藏条目视为空）——init 描述既有项目；
  - **拒绝覆盖** 已存在的 AGENTS.md（exit 1，原文件原样保留）；
  - **探测既有规则文件**（`.cursorrules`、`.cursor/rules`、`.github/copilot-instructions.md`），命中时在生成的 AGENTS.md 末尾列出"保持同步或引用"提示；
  - 模板核心原则："只记录非显而易见的知识——agent 读得了代码，猜不到意图"，四节：Project overview / Build, test, verify / Conventions the code does not show / Known pitfalls。

## Scope / not done

- **stale-read 防护（filetracker/file_versions）**：RB-1 结论挂靠，与文件版本 history 合并为一次存储设计，等用户 O1..O6 批准，未实现（同 VC-1d）。
- 无全局（用户级）AGENTS.md：D6 拍板只覆盖工作区文件，Crush 的 `~/.config/crush/CRUSH.md` 全局层未纳入（未拍板不擅自加）。
- 注入无配置开关：无 AGENTS.md 时中间件为 no-op（一次 Debug 日志），常规 run 零成本；不加未拍板的 knob。
- `vivy init` 不做代码分析生成内容（Crush 用模型生成摘要）；V0 交付的是结构化模板 + 规则文件探测，"由 run 生成内容"走正常对话即可。

## FSL compliance

Crush 为 FSL-1.1-MIT：仅行为/协议对齐（initialize 的空目录拒绝、规则文件探测、"只记非显而易见知识"原则），模板文本与实现全部自写，无源码拷入。

## Files

- `internal/runtime/agentsmd.go`（新）：D6 常量、`AgentsMDBackend` 别名、middleware 构造。
- `internal/runtime/engine.go`：EngineConfig 字段 + handler 装配（compaction 之后）。
- `internal/runtime/filesystem_backend.go`：ErrNotExist sentinel wrap（一行 + 注释）。
- `internal/app/compaction.go` / `internal/app/app.go`：`buildEngineConfig` 双路径接线。
- `cmd/vivy/init.go`（新）+ `cmd/vivy/main.go`：`vivy init` 子命令。
- `config.example.yaml`：workspace_root 注释记录注入行为。
- 测试：`internal/runtime/agentsmd_test.go`（5 个）、`cmd/vivy/init_test.go`（4 个）。
