# MCP 传输升级提案：stdio 接入与上游 OAuth（Eino/EinoExt 上游优先）

- 日期：2026-09-07
- 定位：提案、决策记录与切片 1 验收依据。stdio 切片已于 2026-09-08 落地；OAuth 仍是后续独立切片。
- 用户拍板（2026-09-07）：
  1. MCP 能力用上游 Eino/EinoExt（及其依赖的 mcp-go）现成组件实现，**不自研协议栈**。
  2. 上游缺口走 **PR 途径**回赠上游，不 fork、不 replace pin。
- 影响面：`internal/config`、`internal/app/settings`、`internal/app/app.go`、`internal/runtime/mcp_backend.go`、MCP RPC/浏览器管理面、MCP 状态投影；Journal/Policy/HITL 契约零改动（OAuth 状态投影除外）。侧栏仍复用现有 `configured/initialized/error` 状态，但既有 MCP snapshot 加法增加 `transport` 与 `env_missing` 字段，供 TUI 显示 stdio 类型和缺失 child key。

## 1. Eino 能力检查（必做项，引用实据）

| 能力 | 上游来源（仓库锁定版本） | 结论 |
|---|---|---|
| stdio 传输 | `github.com/mark3labs/mcp-go v1.0.0`（go.mod 直接依赖）：`transport.NewStdioWithOptions` 构造 transport、`client.NewClient` 构造官方 client；`transport.WithCommandFunc`（自定义 spawn 钩子）、`WithCommandLogger`、`WithCommandStderrWriter`；stderr 有界环形缓冲；Close 带进程退出等待 | 直接采用；不用会 eager-start 的 `client.NewStdioMCPClientWithOptions` |
| OAuth 2.1（Streamable HTTP/SSE） | 同仓库 `client.NewOAuthStreamableHttpClient` + `transport.OAuthConfig`；`OAuthHandler` 内置动态客户端注册（`RegisterClient`）、PKCE、refresh 轮换（RFC 8707 resource 参数）；`TokenStore` 接口；`OAuthAuthorizationRequiredError` 哨兵错误 | 直接采用（切片 2） |
| 工具发现/schema 投影 | `github.com/cloudwego/eino-ext/components/tool/mcp v0.0.9` 的 `GetTools(Config{Cli: client.MCPClient})`——**传输无关**；Vivy 已在 `internal/runtime/mcp_backend.go:279` 消费 | 零改动 |
| 前向路径（观察，不切换） | `eino-ext/components/tool/mcp/officialmcp v0.1.1`（基于官方 `modelcontextprotocol/go-sdk` v1.6.1）：`ClientSession` 接口 + `session.Session` 透明重连、custom transport factories（#946 已合入）、`ResultPolicy` 可配 isError 语义；仍 tools-only | 持续观察 |

自研对照：不用上游则需自写 JSON-RPC over stdio 帧协议、子进程生命周期管理、OAuth 2.1 + DCR + PKCE + 刷新轮换——与 AGENTS.md 架构决策顺序第 2/3 条冲突，明确不做。

## 2. 现状盘点（派单时"不要重做"）

- `MCPBackend` 已用官方 `client.NewStreamableHttpClient` 替换自研协议栈（`docs/research/eino-boundary-audit-2026-09-05.md` §5.1，2026-09-06 完成）；initialize、typed tools/resources/prompts、会话生命周期全走 mcp-go client。
- Eino 工具"只投影不挂载"原则已落地；`mcp_list_tools`/`mcp_call` + `PrepareMCPCall` 审批路径不变。
- 治理既有资产：512KiB raw / 256KiB content / 32 页 bounds（HTTP transport 层）、8s operation timeout（ctx 级，传输无关）、`ReplaceServers` 配置热更、关闭扇出。
- TUI 侧栏 MCP 状态已区分 configured/initialized（TUI-SIDEBAR-N1-OPEN）。
- 配置面：`config.MCPServer{Name,Endpoint,Command,Args,EnvFrom,Cwd,AuthEnv}`（`internal/config/config.go`）+ settings/runtime 同构镜像；`env_from` 是 CHILD→HOST 环境名引用，不保存环境值。

## 3. 切片 1：stdio 传输（先行，无 OAuth 依赖）

全部为薄适配，协议层零自研：

1. `internal/config` 与 `internal/app/settings`：`MCPServer` 增 `command`/`args`/`env_from`/`cwd`；`endpoint` 与 `command` 二选一；PATH 名或绝对路径命令授权，危险 basename denylist，**不新增 execute allowlist**；argv 按行/映射行编辑。新字段 parse/validate/round-trip 测试已加入。
2. `internal/runtime/mcp_backend.go`：`MCPServerConfig` 镜像新字段；`newServer` 有 `command` 时用 `transport.NewStdioWithOptions` + `client.NewClient`，否则维持 Streamable HTTP。transport 只在首次操作的 `initialize` 中 `Start`，保留 EinoExt `GetTools`、`mcp_list_tools`/`mcp_call`、resources/prompts 与治理路径。
3. spawn 治理（经 `WithCommandFunc`，Vivy 拥有）：最小 env 注入（系统变量 + `env_from` 的 CHILD→HOST 引用，不透传全量父环境）、cwd 解析到 `runtime.workspace_root` 内、命令 denylist、结构化 spawn 日志（server 名/命令 basename，不含密钥）；Windows `.cmd/.bat` 通过 `ComSpec` 兜底。
4. 响应预算：stdio 沿用 decoded/projected bounds 与 operation context timeout；mcp-go v1.0.0 的 stdio `ReadString('\n')` 接缝没有 raw-frame 上界，本片不伪造 512KiB raw 保证，raw-frame 限制登记 TODO。
5. 进程死亡 fail-closed：下一次操作识别 transport/process-closed，复用既有 `error` 状态；stdio 不自动重启，只有配置替换才会得到新 client。
6. settings/app/RPC/browser import-export/i18n 已贯通；TUI/sidebar 沿用现有 sidebar route 与三态，并加法传递 `transport`/`env_missing`，不新增独立 UI/surface 协议。
7. 不订阅 notification / continuous listening（维持现状）；Windows 子进程树治理也不在本片保证范围，登记 TODO。

验收：`just ci`；config/settings 校验用例；stdio backend 集成测试用当前 Go test binary 作为确定性本地子进程（无网络）；RPC/UI import-export 测试；热更/关闭生命周期由现有 backend seam 覆盖。统一门禁和 split-browser/真实路径冒烟记录在 `docs/logs/2026-09-08-mcp-stdio/verification.md`。

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
| ⑤ | stdio 线上无 raw-frame 字节上界 | 本片仅保留 decoded/projected bounds；raw-frame 上界需上游或 transport seam，登记 TODO |
| ⑥ | OAuth 仅覆盖 Streamable HTTP/SSE | 规范如此（stdio 无 OAuth），无需补 |
| ⑦ | OAuth 交互回调假设后端 localhost 可达 | 切片 2 裁决点 |
| ⑧ | TokenStore 仅接口 | Vivy 实现 |
| ⑨ | Eino 组件 tools-only、`IsError`→Go error | 维持现有边界（audit §5.1 记录） |
| ⑩ | mcp-go v1.0.0 没有子进程树退出回调；Close/WaitDelay 只覆盖当前 child | 本片记录 Windows process-tree 治理 TODO，不宣称完整树回收 |

## 6. 上游 PR 途径（不 fork）

| 事项 | 仓库 / 流程 | 时机 |
|---|---|---|
| stdio `WithWorkingDir` 选项；Windows batch spawn 处理；最小 env 选项 | mark3labs/mcp-go：MIT，fork → branch → tests → PR 到 main，无 CLA/DCO 门槛；2026-09-07 撞车检查为空地（无同类 issue/PR） | 本片先用 Vivy `WithCommandFunc` 绕接；是否起草上游 PR 留待 raw-frame/process-tree 证据充分后单独裁决 |
| officialmcp 补 resources/prompts | cloudwego/eino-ext：Apache-2.0，git-flow（**PR 目标 `develop` 分支**），AngularJS commit 规范，gofmt + golangci-lint，功能类**先 issue 后 PR**，需签 CLA | 决定走 officialmcp 路线时 |
| 日常缺陷修复 | 上游 PR，本地用钩子先绕 | 撞到再提 |

fork 触发条件（当前 9 条限制无一满足）：上游拒绝修 + 钩子完全绕不过。

## 7. 明确不做

- 不 fork、不 replace pin mcp-go 或 eino-ext。
- 不切 officialmcp/go-sdk 双 SDK 并行栈（锁定 mcp-go v1.0.0 + eino-ext/tool/mcp v0.0.9 不动）。
- 不自研 MCP 协议栈；不做 notification 订阅。
- 不把 stdio 当插件装载面；它是显式配置的 MCP 依赖。配置只授权 PATH 名/绝对路径命令与安全 denylist，不另设 execute allowlist。
- 不把未实现的 raw-frame 预算或 Windows process-tree 回收写成已交付能力；两项进入 `docs/TODO.md` §0.1。
- 移除边界不变：Eino MCP 组件覆盖 resources/prompts 且保留 isError 语义后，方可移除对应 mcp-go typed plumbing（audit §5.1）。
