# MCP 传输升级提案：stdio 接入与上游 OAuth（Eino/EinoExt 上游优先）

- 日期：2026-09-07
- 定位：提案 + 派单依据。切片实施时按 `parallel-worktree-isolation` 另起 worktree，本提案只入文档。
- 用户拍板（2026-09-07）：
  1. MCP 能力用上游 Eino/EinoExt（及其依赖的 mcp-go）现成组件实现，**不自研协议栈**。
  2. 上游缺口走 **PR 途径**回赠上游，不 fork、不 replace pin。
- 影响面：`internal/config`、`internal/runtime/mcp_backend.go`（装配分支 + 治理钩子）、MCP 状态投影；Journal/Policy/HITL 契约零改动（OAuth 状态投影除外）。

## 1. Eino 能力检查（必做项，引用实据）

| 能力 | 上游来源（仓库锁定版本） | 结论 |
|---|---|---|
| stdio 传输 | `github.com/mark3labs/mcp-go v1.0.0`（go.mod 直接依赖）：`client.NewStdioMCPClient[WithOptions]`；`transport.WithCommandFunc`（自定义 spawn 钩子）、`WithCommandLogger`、`WithCommandStderrWriter`；stderr 有界环形缓冲（永不堵死子进程）；Close 带进程退出等待 | 直接采用 |
| OAuth 2.1（Streamable HTTP/SSE） | 同仓库 `client.NewOAuthStreamableHttpClient` + `transport.OAuthConfig`；`OAuthHandler` 内置动态客户端注册（`RegisterClient`）、PKCE、refresh 轮换（RFC 8707 resource 参数）；`TokenStore` 接口；`OAuthAuthorizationRequiredError` 哨兵错误 | 直接采用（切片 2） |
| 工具发现/schema 投影 | `github.com/cloudwego/eino-ext/components/tool/mcp v0.0.9` 的 `GetTools(Config{Cli: client.MCPClient})`——**传输无关**；Vivy 已在 `internal/runtime/mcp_backend.go:279` 消费 | 零改动 |
| 前向路径（观察，不切换） | `eino-ext/components/tool/mcp/officialmcp v0.1.1`（基于官方 `modelcontextprotocol/go-sdk` v1.6.1）：`ClientSession` 接口 + `session.Session` 透明重连、custom transport factories（#946 已合入）、`ResultPolicy` 可配 isError 语义；仍 tools-only | 持续观察 |

自研对照：不用上游则需自写 JSON-RPC over stdio 帧协议、子进程生命周期管理、OAuth 2.1 + DCR + PKCE + 刷新轮换——与 AGENTS.md 架构决策顺序第 2/3 条冲突，明确不做。

## 2. 现状盘点（派单时"不要重做"）

- `MCPBackend` 已用官方 `client.NewStreamableHttpClient` 替换自研协议栈（`docs/research/eino-boundary-audit-2026-09-05.md` §5.1，2026-09-06 完成）；initialize、typed tools/resources/prompts、会话生命周期全走 mcp-go client。
- Eino 工具"只投影不挂载"原则已落地；`mcp_list_tools`/`mcp_call` + `PrepareMCPCall` 审批路径不变。
- 治理既有资产：512KiB raw / 256KiB content / 32 页 bounds（HTTP transport 层）、8s operation timeout（ctx 级，传输无关）、`ReplaceServers` 配置热更、关闭扇出。
- TUI 侧栏 MCP 状态已区分 configured/initialized（TUI-SIDEBAR-N1-OPEN）。
- 配置面：`config.MCPServer{Name,Endpoint,AuthEnv}`（`internal/config/config.go:330`）+ runtime `MCPServerConfig`（`internal/runtime/mcp_backend.go:33`）同构镜像。

## 3. 切片 1：stdio 传输（先行，无 OAuth 依赖）

全部为薄适配，协议层零自研：

1. `internal/config`：`MCPServer` 增 `command`/`args`/`env`（值经环境变量间接，D-010）/`cwd`；校验 `endpoint` 与 `command` 二选一互斥；新字段 parse/validate 表驱动测试。
2. `internal/runtime/mcp_backend.go`：`MCPServerConfig` 镜像新字段；`newServer`（:516）加分支——有 `command` → `client.NewStdioMCPClientWithOptions`，否则维持 Streamable HTTP。下游握手、`mcp_list_tools`/`mcp_call`、resources/prompts、`ReplaceServers` 热更、close 全部不动（同一 `client.Client` 接口）。
3. spawn 治理（经 `WithCommandFunc`，Vivy 拥有）：最小 env 注入（仅白名单系统变量 + 配置项，不透传全量父环境）、cwd 钳制、命令 denylist 对齐 bash 工具规则、spawn 事件结构化日志（server 名/命令，不含密钥）。
4. stdio 响应预算：512KiB raw 上界落投影层（`boundedMCPTransport` 是 HTTP 层，覆盖不到 stdio）。
5. 进程死亡 fail-closed：exited 状态入 MCP 状态投影；不自动重启（监督重拉后续再议）。
6. 不订阅 notification / continuous listening（维持现状）。

验收：`just ci`；config 校验用例；stdio backend 集成测试用 mcp-go server 包起本地子进程（确定性、无网络）；热更/关闭生命周期测试覆盖 stdio 分支；真实路径冒烟用一个本地 stdio server，记入该轮 `verification.md`。

## 4. 切片 2：OAuth（后行，独立裁决点）

1. `MCPServerConfig` 增 oauth 块（client_id、client_secret_env、scopes）；Vivy 实现 `TokenStore`（token 落实例目录受限文件；永不进 Journal、日志、事件 payload——D-010）。
2. `newServer` 对配置了 OAuth 的 server 换 `client.NewOAuthStreamableHttpClient`；`OAuthAuthorizationRequiredError` → 状态投影第三态 needs-auth（侧栏 configured/initialized 已有区分，加一态）。
3. 交互式授权 control-plane RPC + 状态机（浏览器完成授权，回调落 127.0.0.1 临时端口）。
4. 启动前裁决点：远程/多客户端形态下浏览器回调的 localhost 可达性（限制⑦）——是否先限本地 face 支持。

## 5. 上游限制与补位（2026-09-07 源码核实）

| # | 限制 | 补位 |
|---|---|---|
| ① | stdio 默认 spawn 透传全量父环境（`os.Environ()`） | cmdFunc 最小 env |
| ② | stdio 无 cwd 选项 | cmdFunc 设 `cmd.Dir` |
| ③ | 无自动重启：进程死即传输死 | fail-closed + 状态展示 |
| ④ | Windows `.cmd/.bat` 拉起未保证 | cmdFunc 兜底 `cmd /c`（实施时验证） |
| ⑤ | stdio 线上无字节上界 | 投影层预算 |
| ⑥ | OAuth 仅覆盖 Streamable HTTP/SSE | 规范如此（stdio 无 OAuth），无需补 |
| ⑦ | OAuth 交互回调假设后端 localhost 可达 | 切片 2 裁决点 |
| ⑧ | TokenStore 仅接口 | Vivy 实现 |
| ⑨ | Eino 组件 tools-only、`IsError`→Go error | 维持现有边界（audit §5.1 记录） |

## 6. 上游 PR 途径（不 fork）

| 事项 | 仓库 / 流程 | 时机 |
|---|---|---|
| stdio `WithWorkingDir` 选项；Windows batch spawn 处理；最小 env 选项 | mark3labs/mcp-go：MIT，fork → branch → tests → PR 到 main，无 CLA/DCO 门槛；2026-09-07 撞车检查为空地（无同类 issue/PR） | 切片 1 实施时同步起草 |
| officialmcp 补 resources/prompts | cloudwego/eino-ext：Apache-2.0，git-flow（**PR 目标 `develop` 分支**），AngularJS commit 规范，gofmt + golangci-lint，功能类**先 issue 后 PR**，需签 CLA | 决定走 officialmcp 路线时 |
| 日常缺陷修复 | 上游 PR，本地用钩子先绕 | 撞到再提 |

fork 触发条件（当前 9 条限制无一满足）：上游拒绝修 + 钩子完全绕不过。

## 7. 明确不做

- 不 fork、不 replace pin mcp-go 或 eino-ext。
- 不切 officialmcp/go-sdk 双 SDK 并行栈（锁定 mcp-go v1.0.0 + eino-ext/tool/mcp v0.0.9 不动）。
- 不自研 MCP 协议栈；不做 notification 订阅。
- 移除边界不变：Eino MCP 组件覆盖 resources/prompts 且保留 isError 语义后，方可移除对应 mcp-go typed plumbing（audit §5.1）。
