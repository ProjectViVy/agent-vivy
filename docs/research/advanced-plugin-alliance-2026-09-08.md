# Vivy 进阶插件联盟调研：编译期自由装配与内核边界

> 日期：2026-09-08
> 状态：调研结论 / 架构提案，尚未构成实现授权
> 范围：Vivy 物种内核；不含 Vivy Studio 壳层实现
> 目标：评估“一切皆插件”，统一 `internal` 与 `pluggable` 的装配语义，同时守住单一 `Service.Run` / Journal / policy 路径。

## 0. 执行摘要

### 结论

方向可行，但必须把原命题精确化为：

> **一切产品能力都是可声明、可依赖、可替换的装配单元；并非一切内核不变量都可由普通插件接管。**

推荐采用“**统一元模型、分级端口、生成式装配、运行时冻结**”：

1. `internal` 与 `pluggable` 共用 Module Descriptor、依赖图、生命周期、Generation provenance 和 inspect；
2. 二者不共用权限。`internal` 可实现特权端口，`pluggable` 只能实现公开 SDK 端口并经 Grant/Host；
3. 编译前可自由选择能力实现；`vivy-sdk pack` 解析依赖、生成强类型 wiring，再由 Go 编译链接；
4. 进程启动后插件图冻结，不做 Go 动态加载、热卸载或第二运行时；
5. Eino 是受控编排内核，不是插件管理器。Eino 组件只在 `internal/runtime`、`internal/provider` 适配；
6. `Service.Run`、Journal authority、policy/approval、domain event schema、composition compiler 等必须耦合在 kernel；它们的后端实现可有 internal provider，但权威语义不能外放。

### 建议拍板

| 决策 | 建议 |
|---|---|
| 总体模型 | `Module + Port + Generated Assembly` |
| 装配时机 | pack/Go build 前解析；启动时只初始化已编入模块 |
| 运行时动态插件 | 不做 |
| 公共插件执行 | 可信源码编进同一 EXE；不可信能力走 MCP/sidecar，不冒充 Go 插件 |
| API 形态 | 元数据统一、能力接口分开；拒绝一个万能 `Plugin` 接口 |
| 冲突语义 | 默认 fail-closed；禁止 last-writer-wins |
| Eino | internal loop/provider adapter，复用 ADK/Compose/Middleware；不泄漏到 SDK |
| 第一批迁移 | Tool / ToolWorld / Channel / Face 进入联盟，但保持现有 ABI |
| 第二批端口 | Provider、pre-tool middleware、run observer、context source、skill source |
| UI | Face 保持整脸；另建细粒度 `ui/slot`，不混为一类 |

## 1. 当前 Vivy：已经有插件骨架，但还不是“联盟”

### 1.1 已成立的事实

当前插件是“源码治理单元 + 编译期 pack”，而非运行时动态库：

- `vivy-plugin.json` 由 SDK 验证；
- `vivy-sdk pack` 生成注册文件并编译新 EXE；
- 未进入配方的源码不属于该 Generation；
- 插件不能 import `internal` 或 Eino；
- 同进程插件崩溃会影响整个 EXE。

证据：`docs/architecture/VIVY-PLUGIN-SPEC.md:16-28,165-177,227-264`。

现有源码已经支持五种 seam：`tool`、`tool-world`、`provider`、`channel`、`face`，以及 11 个 Grant；架构文档仍有“四类 seam”和旧 Grant 词表，已发生文档漂移。证据：`sdk/plugin/plugin.go:12-35,37-88` 对比 `docs/architecture/VIVY-PLUGIN-SPEC.md:77-87,98-117`。

真正有消费者的公共能力是：

| 能力 | 当前消费者 | 状态 |
|---|---|---|
| Tool / ToolWorld | `internal/pluginhost.Adapt` → `internal/tools.Tool` | 已成立 |
| Channel | `ChannelHost` | 已成立 |
| Face | `FaceHost` / control-plane client | 已成立 |
| Diagnostic observer | file mutation diagnostic bridge | 已成立的窄能力 |
| LSP status provider | control-plane status source | 已成立的窄能力 |
| Provider seam | 无专用消费者 | 只有枚举/清单概念，尚未成立 |

`pluginhost.Adapt` 只显式跳过 Channel，然后把其他 `plugin.Plugin.Tools()` 适配为工具；这证明 `SeamProvider` 目前没有独立运行语义。证据：`internal/pluginhost/host.go:23-45`。

### 1.2 当前装配的局限

1. **统一接口过窄**：`Plugin` 只返回 `Tools()`，Channel/Face 需要旁路 ABI；继续加能力会把接口变成 God object。
2. **分类与信任混在一起**：`seam` 表达能力类型，却不能表达 internal/pluggable 的信任等级、依赖、冲突、cardinality 和生命周期。
3. **生成注册表只覆盖用户插件**：出厂 loop/world/provider/tool 仍由 `internal/app` 手工 wiring；无法实现真正的编译期自由组合。
4. **缺少依赖图**：没有 Definition/Provider/Consumer、缺失依赖、循环依赖和多实现选择的统一规则。
5. **缺少 owner-scoped lifecycle**：工具是值，Channel/Face 有各自启停；后台资源、cleanup 和失败回滚没有统一语义。
6. **默认物种体与窄 Generation 语义不同**：提交的 `internal/generated/plugins/zz_register.go` 手工编入全部第一方 Channel，pack 才替换为窄注册表。证据：`internal/generated/plugins/zz_register.go:1-28`。
7. **运行时仍有动态能力开关**：builtin tools 可通过 settings 重建 Eino engine，plugin tools 始终追加；MCP server 也可运行时替换。它们是“已编入能力的激活/配置”，不是新代码装载。证据：`internal/app/app.go:335-358,394-471,614-619`。

### 1.3 与旧架构文档的关系

`VIVY-ASSEMBLY.md` 当前明确规定“只有用户自定义叫插件”，出厂能力按 loop/world/provider/tool 命名。证据：`docs/architecture/VIVY-ASSEMBLY.md:30-38,42-74,140-157`。

本报告提出的是更高一级的**内部元模型统一**：

- 产品/UI 仍应称“工具、模型出口、世界、通道、脸”；
- 架构和 pack 层把它们统一视为 Module；
- “internal/pluggable”是来源与信任分类，不要求产品界面把所有东西显示成“插件”。

因此这不是简单补文档，而是对现有命名合同的方向性扩展；必须经架构拍板后才能改合同。

## 2. 参考项目结论

## 2.1 Eino：复用编排，不复刻插件系统

Vivy 锁定 Eino `v0.9.13`，本地 `.workspace/eino` 已是更高的 alpha 版本，所以 API 判断以 Go module cache 的 `v0.9.13` 为准。[1]

Eino v0.9.13 提供：

- Component interfaces：ChatModel、Tool、Retriever、Embedding、Loader、Transformer、Indexer；
- Compose：Graph、Chain、Workflow、Parallel、Branch、Lambda 和 `Compile`；
- ADK：Agent、Runner、Interrupt、Resume、Checkpoint；
- Agent middleware、Tool middleware 和 callbacks；
- EinoExt 的 OpenAI、Claude、MCP 等具体 adapter。

本地证据：`go.mod:13-19`，以及 `C:/Users/Administrator/go/pkg/mod/github.com/cloudwego/eino@v0.9.13/components/types.go:17-86`、`compose/chain.go:157-536`、`compose/graph.go:296-467`、`adk/handler.go:139-265`、`adk/runner.go:50-149`。

Eino没有通用插件发现、版本、信任、Grant、Factory registry。`schema.Register` 是序列化类型注册；callbacks 不保证跨 Handler 的全局顺序，也不是 durable event bus。因此不能把 Eino 当成 Vivy 插件管理器。[1]

Vivy 已正确复用 Eino 的 ADK、dynamic tool search、Skill、AgentsMD、reduction、summarization；工具经 `toolAdapter` 才进入 Eino，并在这里统一执行参数校验、policy、hook、approval 和结果边界。证据：`internal/runtime/engine.go:9-15,118-220`、`internal/runtime/tooladapter.go:24-35,79-180`。

结论：新增插件系统应在 Eino 之上做 Vivy-owned assembly compiler；不要重写 Eino 已有 Compose/ADK/Middleware，也不要向 SDK 暴露 Eino 类型。

## 2.2 Hermes：学习 facade/provider profile，拒绝多套 registry

Hermes 当前不是一个插件系统，而是通用 Python plugin、model provider、memory、context engine、MCP、skills、dashboard、gateway hooks 等多套扩展机制并存。[2]

值得吸收：

- provider profile 只声明 endpoint/auth/request 差异，核心持有 client、凭证轮换和 streaming；
- `PluginContext` 作为 Host facade，不直接交出 kernel 对象；
- deferred tools 只是模型可见性层，真实调用仍经过工具 registry、hooks 和 approval；
- profile 隔离 config、session、skill 和 subprocess home；
- plugin/MCP failure 可局部 unavailable。

本地证据：`C:/Users/Administrator/Desktop/morediva/.workspace/hermes-agent/providers/base.py:1-10`、`hermes_cli/plugins.py:286-354`、`tools/tool_search.py:150-209`、`hermes_cli/profiles.py:37-52`。

不应照搬：

- 多个 discovery loader；
- 全局 registry；
- last-writer-wins / first-writer-wins / silent ignore 混合冲突策略；
- 非事务式 `register(ctx)`；
- 把 skills、MCP、代码 plugin、UI plugin 混成同一信任等级。

Hermes 的核心经验可归纳为：插件实现能力，kernel 决定能力何时可用、谁能调用、是否审批，以及结果怎样进入 session state。[2]

## 2.3 DeepSeek Harness / Cordis：学习语义，不复制热加载

Cordis 的最小底座是根 Context、Reflect service registration、Registry、Fiber/effect lifecycle、Events 和 Loader；产品能力通过 Definition、Provider、Consumer、`inject` 与 owner-scoped effects 进入系统。[3]

本地证据：

- `.../.workspace/deepseek-harness/deepseek-harness/vendor/cordis/src/context.ts:9-84`
- `vendor/cordis/src/registry.ts:91-145,189-337`
- `vendor/cordis/src/fiber.ts:139-154,212-332,402-441,641-695`
- `vendor/cordis/src/events.ts:24-32,125-301`
- `docs/user/develop/practice/index.md:5-49`

最值得迁移到 Go 编译期模型的语义：

| Cordis | Vivy 对应 |
|---|---|
| Service Definition / Provider / Consumer | typed Port contract / Provider factory / generated consumer wiring |
| `inject` | pack 阶段依赖 DAG |
| Fiber owner | Module owner + Cleanup stack |
| effect/disposer | `Start` 成功后登记、逆序 `Close` |
| PENDING | 编译期缺依赖直接失败；仅显式 optional 才允许 absent |
| profile/bundle/patch | generation recipe layering |
| event dispatch mode | 类型化 middleware/event phase |
| rollback | startup 部分失败时撤销本 Module 全部贡献 |

不迁移 runtime dynamic import、反射式 `ctx.foo`、依赖变化热重载、Node VM 或无 payload 合同的通用事件总线。

DSH 还揭示一个重要规则：只有 Provider 没有 Consumer，不算完整 seam。Vivy 当前 `SeamProvider` 正是这个状态。

## 2.4 其他旁证

OpenFang 的 Channel/Provider/Tool 仍有大量静态 module 和 `match` 分发，但其 WASM fuel、epoch timeout、capability-checked host function 值得未来不可信执行层参考。[4]

ZeroClaw 用 feature flags 和宏维护 provider slot 的单一清单，并把配置、遍历和 factory dispatch 一起生成；Go 对应物应是 generation codegen，而不是多个手写 switch。其 WASM Channel 在当前快照仍有 placeholder，说明“manifest 声明 capability”不能替代 conformance test。[5]

OpenClaw 的 manifest-first、metadata snapshot、owner-tagged registry 和 rollback 很成熟，但 native plugin 与 Gateway 同进程同权限，不适合作为 Vivy 第三方默认信任模型。[6]

Pi 的小型 typed lifecycle、逐 extension 错误收集和 stale-context invalidation 有参考价值；其 extension 默认拥有宿主进程权限，必须配合外部 sandbox 才能形成安全边界。[7]

综合结论：

- ZeroClaw：借编译期 single source of truth；
- Cordis：借依赖和 owner lifecycle；
- Hermes：借 capability facade 和 provider profile；
- OpenClaw：借 manifest snapshot 与原子回滚；
- OpenFang：只为未来 sidecar/WASM 借资源计量；
- Pi：借简单生命周期，不借权限模型。

## 3. 推荐架构：统一 Module，分级 Port

## 3.1 四层模型

```text
Generation Recipe
      │
      ▼
Assembly Compiler / Verifier           ← kernel，唯一
      │  resolve DAG + trust + grants + conflicts
      ▼
Generated Typed Wiring                 ← Go source，禁止手改
      │
      ├── internal modules             ← 特权实现
      └── pluggable modules            ← 公开 SDK + Host grants
      │
      ▼
Frozen Runtime Assembly
      │
      ▼
Single Service.Run / Journal / Policy  ← 唯一权威路径
      │
      ▼
Eino adapter / providers               ← 仅 internal/runtime + internal/provider
```

关键点：统一的是**治理和装配元模型**，不是把所有能力压进一个 Go interface。

## 3.2 `internal` 与 `pluggable`

| 维度 | internal | pluggable |
|---|---|---|
| 来源 | Vivy 仓库/受控第一方模块 | 用户或生态源码模块 |
| 选择 | generation recipe | generation recipe |
| 描述 | 同一 Module Descriptor | 同一 Module Descriptor |
| 生命周期 | 同一 owner/cleanup 模型 | 同一 owner/cleanup 模型 |
| 可实现端口 | public + privileged core-provider ports | public ports only |
| import | 按内部 package firewall；Eino 仍限 runtime/provider | `sdk/plugin`、公开 port contract、自有依赖；禁止 internal/Eino |
| world access | 仍优先走窄接口 | 只能走 Grant-filtered Host facade |
| 信任 | 构建时受信 | 经审核的同进程源码；不是安全沙箱 |
| 失败 | required 默认启动失败 | manifest 明确 required/optional；禁止隐式降级 |

模块不能靠 manifest 自称 `internal`。信任等级由 assembly compiler 根据 source catalog / recipe lane 赋予并写入 Generation；否则用户插件可自我提权。

## 3.3 Module Descriptor

建议从单一 `seam` 升级为多 contribution 描述：

```yaml
apiVersion: vivy.module/v1
id: builtin/eino-loop
version: 1.0.0
source: internal
provides:
  - port: core/loop-driver@v1
    implementation: eino
requires:
  - port: core/chat-model@v1
  - port: std/tool-catalog@v1
optional:
  - port: std/context-source@v1
conflicts: []
grants: []
lifecycle: process
```

规则：

- `source/trust` 的有效值由 pack 赋予，不信任模块自报；
- 一个 Module 可提供多个相关 Port，但每个 Port 仍有独立 typed contract；
- `requires` 默认 required；optional 必须显式；
- Port catalog 定义 cardinality、scope、允许的 trust、failure policy；
- Descriptor 纯数据，无初始化副作用；
- v0 `seam` 可在过渡期转换为一个标准 Port。

## 3.4 Port 分层

### A. kernel-closed ports

只有 kernel 或 internal module 可提供/消费：

- `core/loop-driver@v1`
- `core/chat-model@v1`
- `core/storage-engine@v1`
- `core/checkpoint-store@v1`
- `core/credential-resolver@v1`
- `core/sandbox-backend@v1`

“closed”不代表实现硬编码；它表示只能被受控 internal module 替换，且必须满足 kernel conformance。

### B. public standard ports

可由 internal 或 pluggable 实现：

- `std/tool@v1`
- `std/tool-world@v1`
- `std/channel@v1`
- `std/face@v1`
- `std/provider-profile@v1`
- `std/context-source@v1`
- `std/skill-source@v1`
- `std/middleware/pre-tool@v1`
- `std/observer/run@v1`
- `std/observer/diagnostic@v1`
- `std/ui/slot@v1`（未来）

### C. extension ports

使用 `x/<author>/<port>@vN`，但不提供 `map[string]any` 服务定位器：

- producer 与 consumer 必须共享一个可验证的 Go contract package；
- codegen 生成静态 import 与类型赋值，让 Go 编译器做最终类型检查；
- kernel 只记录 ID、版本、owner 和 provenance，不自动把 extension port 暴露给模型、RPC 或 Journal；
- 无 consumer 的 extension provider 在 pack 阶段报错，而不是静默存在。

## 3.5 为什么不要万能 `Plugin` 接口

一个包含 Tool、Channel、Provider、Storage、UI、Hook、Lifecycle 的接口会产生：

- 大量无意义空方法；
- 不同生命周期互相污染；
- public API 被 internal 特权能力拖宽；
- 每加一种能力都破坏全部实现；
- 无法表达一个模块提供多个独立 contribution。

推荐：Descriptor/owner 统一，Port interface 分开，generated binder 负责将每个模块的 typed contribution 放入 Assembly。

## 4. Assembly Compiler

## 4.1 三种“编译”必须区分

1. **Assembly compile**：`vivy-sdk pack` 解析 recipe、manifest、依赖和权限；
2. **Go compile/link**：生成静态 imports/wiring，构建 EXE；
3. **Eino Compile**：进程启动时对已构造 Graph/Chain/Workflow 做运行图编译。

Eino Compile 不是 Go 插件加载，也不能代替前两步。

## 4.2 管道

```text
generation.yml + module manifests + source catalog
  → normalize v0 seam to v1 ports
  → validate identity/version/provenance/hash
  → assign trust class
  → resolve provides/requires/optional/conflicts
  → enforce port cardinality and trust policy
  → detect missing dependency / duplicate / cycle
  → validate grants and import quarantine
  → generate typed zz_assembly.go
  → go build / conformance tests
  → embed immutable Generation manifest
  → startup initialize and freeze
```

必须保持：

- 不扫描目录自动加入；recipe 点名才存在；
- default conflict = error；不做 last-writer-wins；
- single port 的多实现只能由 recipe 显式选择；
- optional 依赖缺失必须形成可 inspect 的状态；
- Generated file 不可手改；
- artifact 可列出 module、port、implementation、version、trust、grants、source hash 和依赖边。

## 4.3 启动生命周期

推荐阶段：

```text
Describe → Construct → Start → Ready → Frozen → Stop → Close
```

语义：

- `Describe` 必须纯函数；
- `Construct` 按 DAG 顺序创建 typed contribution，不启动后台工作；
- `Start` 才允许占用资源；每一步立即登记 owner cleanup；
- required 模块失败：逆序关闭已启动模块并中止启动；
- optional 模块失败：只有 manifest 和 Port policy 明确允许 unavailable 才继续；
- `Frozen` 后禁止新增/删除代码模块；
- `Stop/Close` 逆 DAG、幂等、有 deadline；
- Run/session scope 的资源由 kernel 创建子 owner，但不能另建运行时 registry。

不做 Cordis 热更新，但保留其最重要的 owner-scoped cleanup 语义。

## 4.4 失败与冲突矩阵

| 情况 | 结果 |
|---|---|
| required port 缺失 | pack fail |
| single port 多 provider 未显式选择 | pack fail |
| 依赖循环 | pack fail，并打印 cycle |
| pluggable 请求 closed port | verify fail |
| Grant 超出该 Port 允许上限 | verify fail |
| Eino import 出现在非 runtime/provider | import gate fail |
| 模块构造失败 | startup fail + rollback |
| optional 模块启动失败 | 标记 unavailable；仅在合同允许时继续 |
| tool/schema 名冲突 | pack fail |
| extension port 无 consumer | pack fail 或显式 `allowUnused`；默认 fail |
| module panic | 当前同进程模型会伤及 EXE；必须在文档/inspect 明示 |
| 不可信第三方代码 | 不编入；改走 MCP/sidecar |

## 5. Vivy 现有能力的“插件联盟”地图

## 5.1 第一梯队：直接纳入，保留现有 ABI

| 联盟成员 | 现状 | 动作 |
|---|---|---|
| Tool | builtin + pluginhost 已有统一 domain Tool | 包一层 Module Descriptor；不改 Eino adapter |
| ToolWorld | LSP、文件/进程世界能力已有 Grant Env | 纳入 `std/tool-world`，细分可选 observer ports |
| Channel | Channel ABI + ChannelHost 已成立 | Channel 为 module contribution；Host 留 kernel |
| Face | 整体 Face ABI + FaceHost 已成立 | Face 为 exclusive public port；Host/RPC 留 kernel |
| Diagnostic observer | 已有窄 optional interface | 提升为 `std/observer/diagnostic` |
| LSP status | 已有窄 provider interface | 提升为只读 status port |

这一步应是兼容迁移：现有插件无需一次性重写，v0 manifest 由 pack 转译。

## 5.2 第二梯队：适合加入，但必须先补 Consumer

| 候选 | 可插件化实现 | 必须保留的宿主控制 |
|---|---|---|
| Model provider | provider profile、request quirks、model catalog enricher | credential resolver、route freeze、streaming accounting、raw model ID 规则 |
| Loop driver | Eino ADK/Compose loop 或未来替代实现 | `Service.Run`、Journal mapping、budget、cancel、terminal |
| World backend | sandbox/local/fs/exec/http/fetch/download | workspace identity、Grant、network policy、audit |
| Context source | AGENTS.md、always skill、retrieval context | token budget、session/log boundary、redaction |
| Compaction strategy | Eino reduction/summarization 参数与策略 | durable compaction event、checkpoint compatibility、token accounting |
| Tool middleware | pre/post-tool typed contribution | phase ordering、policy re-evaluation、approval不可绕过 |
| Run observer | audit、telemetry、channel projection | Journal 顺序、redaction、backpressure；observer 不得成为 authority |
| Skill source | filesystem/marketplace/custom source | trust scan、install governance、session mount truth |
| Title generator | provider/model strategy chain | session identity、durable update、usage attribution |
| Search/media backend | network search、image/audio/video provider | routing、secret、timeout、result caps |
| MCP adapter | server-to-tool/resource/prompt adapter | transport lifecycle、approval、schema/size、remote side effect |
| UI slot | 声明式 card/tab/action contribution | RPC auth、state mutation policy、renderer isolation |

注意：MCP endpoint/config 本身仍是外部依赖，不等于本地代码插件；可以插件化的是“将 MCP 能力投影进 Vivy 的 adapter/provider”。

## 5.3 第三梯队：internal-only provider

| 候选 | 可替换部分 | 不可替换语义 |
|---|---|---|
| Storage backend | SQLite / Postgres engine implementation | Journal append contract、transaction boundaries、terminal uniqueness |
| Checkpoint store | blob backend / encoding implementation | Vivy envelope、engine version、checksum、fail-closed recovery |
| Credential backend | env/OS vault/未来 secret store | secret 不持久化、不记录、最小暴露 |
| Sandbox implementation | OS-specific executor | policy、workspace scope、deny rules、audit |
| Worker transport | process transport / future remote worker adapter | parent authority、budget、approval route、event validation |
| Memory/index backend | retrieval/index provider | namespace、authorization、durability、provenance |

这些可成为 internal Module，但不能成为普通 pluggable port。

## 5.4 暂不进入联盟

- 任意事件总线 listener；先定义 typed event/phase；
- 任意 RPC route；先定义 UI slot/action contract；
- 任意 Journal reader/writer；只能开放窄查询或 append-intent API；
- 任意 policy evaluator replacement；可开放 policy rule contribution，但最终裁决器留 kernel；
- 任意长期后台 service；除非有明确 Port、owner、deadline、cleanup 和资源预算；
- arbitrary Eino Graph/Lambda；仅 internal recipe 可以装配，不能成为公共 SDK ABI。

## 6. 必须耦合的内容

“必须耦合”指语义和权威必须由 kernel 统一拥有；不等于每个后端实现都必须写死。

| 必须耦合项 | 原因 | 可替换边界 |
|---|---|---|
| Module/Port catalog 与 Assembly compiler | 若它也可被普通插件替换，系统无法定义插件是否合法 | 无；kernel 自举底座 |
| Generation identity/provenance/hash | artifact 必须可重建、可 inspect | hash 实现可内部维护，不开放 |
| Domain IDs、event schema、run state machine | 所有模块必须共享同一语言 | 只能版本演进 |
| 单一 `Service.Run` / `RunWithOptions` | 防止第二 agent runtime 和多套终止语义 | LoopDriver 可替换，Service 不替换 |
| Journal authority | durability-before-visibility、顺序、exactly-one-terminal | backend 可 internal 替换 |
| policy / approval / Grant enforcement | 插件不能自授予或绕过安全决策 | 可贡献规则，最终裁决不可替换 |
| tool dispatch envelope | schema、argument safety、policy、hook rewrite、approval 必须同路 | tool implementation 可替换 |
| checkpoint envelope/recovery protocol | 版本、checksum、迁移与 resume 必须一致 | store backend 可替换 |
| workspace/session/tenant identity | 文件、记忆、通道、模型上下文都依赖相同 scope | adapter 可替换，身份权威不可 |
| credential resolution/redaction | secret 不得进入 manifest、Journal、error | secret backend internal-only |
| ChannelHost / FaceHost | transport/UI 只能通过宿主进入 run/RPC | Channel/Face adapter 可替换 |
| RPC protocol 与 mutation authorization | UI/plugin 不得绕过 control plane | renderer/slot 可替换 |
| budget/cancellation/worker supervision | 必须跨模型、工具、child run 统一计量 | transport 可 internal 替换 |
| Eino quarantine | 避免框架类型污染产品契约 | Eino adapter/版本可替换 |

现有 `Service` 明确保证“先持久化、后发布”，并维护 active/pending/recovery/budget/terminal 状态。证据：`internal/runtime/service.go:144-205,2542-2612`。这些都不能下放给 Loop 插件或 Eino callback。

工具策略也必须保持单一路径：参数校验 → policy → pre-hook → 重写后重新校验/重新 policy → approval → execution。证据：`internal/runtime/tooladapter.go:79-180`。

## 7. Eino capability check

本设计触及 agent loop、model/tool orchestration、middleware、streaming、checkpoint、MCP 和 context，因此必须先列出 Eino 可复用面。

| 需求 | 锁定 Eino/EinoExt 能力 | Vivy 决策 |
|---|---|---|
| Agent loop | `adk.NewChatModelAgent`、`adk.Runner` | Eino loop 作为 internal module，复用 |
| Tool orchestration | `components/tool`、`compose.ToolsNode` | 复用；Domain Tool 经现有 adapter |
| Dynamic tool visibility | `adk/middlewares/dynamictool/toolsearch` | 已复用；不是 plugin registry |
| Skill injection | `adk/middlewares/skill` | 已复用；Skill source 可做 Vivy port |
| Project instructions | agentsmd middleware | 已复用；source 可装配，boundary 留 Vivy |
| Compaction | reduction + summarization middleware | 已复用；策略可装配，durability 留 Vivy |
| Middleware | `TypedChatModelAgentMiddleware`、`ToolMiddleware` | internal 直接组合；public 经 Vivy typed port |
| Graph composition | Graph/Chain/Workflow/Branch/Parallel | internal recipe target；不公开 Eino ABI |
| Callbacks | callbacks Handler | 只做观测；不做 Journal authority |
| Checkpoint/resume | `CheckPointStore`、Interrupt、Resume | 复用 Eino机制；Vivy envelope/store authority 保留 |
| MCP tools | EinoExt `mcp.GetTools` | 只做 schema/tool adapter；transport/governance 留 Vivy |
| RAG | Retriever/Embedding/Loader/Transformer/Indexer interfaces | 未来优先适配这些接口，不重写同类编排 |

自定义 Vivy Assembly 的正当 gap：Eino 没有 module discovery、dependency graph、version/trust/grant、Generation provenance、public SDK firewall 和 kernel authority contract。迁移边界也明确：若未来 Eino 提供稳定组件 registry，可替换 `internal/runtime` 内 factory adapter，但不能替换 Vivy manifest、trust、Service.Run 和 Journal。

## 8. 分阶段落地

### P0：合同拍板，不写功能

1. 接受或拒绝“架构层一切皆 Module、产品层按真名显示”；
2. 冻结 `internal` / `pluggable` 定义；
3. 冻结 Port naming、cardinality、scope、failure policy；
4. 明确 v0 seam → v1 port 兼容策略；
5. 更新 `VIVY-ASSEMBLY.md` 与 `VIVY-PLUGIN-SPEC.md` 的五 seam/Grant 漂移。

验收：没有代码迁移；合同能回答谁提供、谁消费、谁掌权、失败怎么办。

### P1：Assembly compiler 最小闭环

- Module Descriptor v1；
- source catalog 和 trust assignment；
- provides/requires/conflicts DAG；
- duplicate/missing/cycle diagnostics；
- generated typed wiring；
- Generation inspect；
- owner-scoped startup cleanup；
- v0 manifest compatibility adapter。

先只承载 Tool/ToolWorld/Channel/Face，不改变行为。

### P2：第一方 internal 装配

- 把当前 app wiring 映射为 internal modules；
- 先描述、不急于物理搬目录；
- 提取 Eino LoopDriver、World、Provider、builtin Tool factories；
- 每次只迁一个 Port，并证明仍经过单一 Service.Run。

### P3：高级公共端口

按风险由低到高：

1. observer/diagnostic、run observer；
2. context source、skill source；
3. provider profile / chat model consumer；
4. pre/post-tool middleware；
5. UI slot。

每个 Port 必须同时交付 Definition、Provider、Consumer、failure model 和 conformance suite；禁止只加枚举。

### P4：internal-only 后端

- storage engine；
- checkpoint store；
- sandbox backend；
- credential backend；
- memory/index backend；
- worker transport。

要求 contract test 证明替换实现不改变 kernel authority。

### P5：不可信生态（按需）

只有出现明确市场需求再做：

- sidecar JSON-RPC/MCP；或
- WASM/WIT + fuel/timeout/capability host。

不把它和第一版编译期 Go 插件混做一个项目。

## 9. 验收矩阵

| 验收项 | 必须结果 |
|---|---|
| 自由装配 | recipe 删除某 Module 后，产物不包含其 import/实现 |
| 强类型 | Port 类型错误由生成代码/Go 编译失败暴露 |
| 依赖 | missing/duplicate/cycle 在 pack 阶段确定性失败 |
| 权限 | pluggable 不能提供 closed port，不能自称 internal |
| 架构统一 | 所有用户/Channel/worker 入口最终进入同一个 Service.Run |
| Eino | 只有 runtime/provider import Eino；公开 SDK 无 Eino 类型 |
| durability | Journal 仍先于 event visibility |
| tool governance | builtin/plugin/MCP tool 经过同一 policy/approval pipeline |
| 生命周期 | 部分启动失败能按 owner 逆序清理 |
| provenance | inspect 显示 Module、Port、版本、trust、Grant、hash、依赖 |
| runtime freeze | 运行中不能装载新 Go module；配置只能激活已编入能力 |
| 兼容 | v0 Tool/Channel/Face 插件可通过 adapter 继续 pack |

## 10. 主要风险

| 风险 | 控制 |
|---|---|
| “一切皆插件”演化为第二运行时 | kernel-owned Service/Journal/Policy 明文不可替换 |
| 过度 DI / `map[string]any` | standard Port typed；extension Port 共享 contract package；generated wiring |
| public SDK 膨胀 | 每个 Port 独立版本；internal port 不进 public SDK |
| manifest 自我提权 | trust 由 source catalog/recipe 赋予 |
| 同进程插件被误认为 sandbox | 文档与 inspect 明示；不可信代码走 sidecar/WASM |
| Eino 类型泄漏 | 保留 import quarantine 与 adapter |
| 编译期和运行时开关混淆 | Generation 决定“存在”；settings 决定已编入能力的“激活” |
| 一次大搬家 | 先 metadata/wiring，再逐 Port 迁移；目录移动最后做 |
| 旧插件断裂 | v0 compatibility adapter + conformance tests |
| 文档再次漂移 | Port catalog 生成 manifest schema、inspect schema 和文档表 |

## 11. 最终判断

Vivy 可以做到高级版“一切皆插件”，但正确形态不是 Cordis 的运行时热树，也不是 Hermes 的多 registry，也不是把所有能力塞进一个 `Plugin` interface。

正确形态是：

> **Kernel 是不可替换的物理定律；internal/pluggable 是遵守同一装配协议、拥有不同权限的器官；Eino 是循环器官内部的编排引擎；Generation 是唯一组成真相。**

第一步不应重构 `engine.go`，而应先定义 Module/Port/Assembly Compiler 合同；随后用现有 Tool/ToolWorld/Channel/Face 做零行为迁移，证明这套元模型成立，再开放 Provider、middleware、context 和 UI slot。

本轮只完成调研与架构提案；未授权、未实施源码或测试改动。

## Sources

[1] https://github.com/cloudwego/eino/tree/v0.9.13 — cloudwego/eino v0.9.13
[2] https://github.com/NousResearch/hermes-agent/tree/cf328723d43d101a99fa27b9f358d0f336f4e17f — NousResearch/hermes-agent cf328723
[3] https://github.com/deepseek-ai/deepseek-harness/tree/cd5ef8148158c3a752a658978873241fdf8e2bbc — deepseek-ai/deepseek-harness cd5ef814
[4] https://github.com/RightNow-AI/openfang/tree/acf2587e46be174c10200489c9a2d23a39a98aeb — RightNow-AI/openfang acf2587e
[5] https://github.com/zeroclaw-labs/zeroclaw/tree/d91e08eaefc4750fbd59a4d98f0adcfe0e785b41 — zeroclaw-labs/zeroclaw d91e08ea
[6] https://github.com/openclaw/openclaw — openclaw/openclaw
[7] https://github.com/earendil-works/pi/tree/6564d9471702727141e20b305d17679e06373e57 — earendil-works/pi 6564d947
