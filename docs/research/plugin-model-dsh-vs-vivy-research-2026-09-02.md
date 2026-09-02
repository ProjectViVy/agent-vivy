# 插件模型深调：DSH Cordis vs Vivy seam 模型（2026-09-02）

> 触发：维护者观察——"vivy 侧的插件能力似乎没做好，限定了类别；想象中应完全类似 DSH 那样完全可自定义。"
> 本文是插件模型专项深调。战略级全能力对比已有前置文档：
> `docs/research/DSH-VS-AGENT-VIVY-CAPABILITY-GAP.md`（其结论：多数差距是**有意差距**）。
> 本文只回答一件事：**插件这一层，差距到底在哪、哪些是设计使然、哪些值得收窄。**
> 结论先行：维护者的感觉对了一半——Vivy 插件确实被类别限定（seam 闭集 + 封闭 grants + 单一 Plugin 接口），
> 但这不是"没做好"，是 v0 拍板的编译期治理模型；DSH 的"完全自定义"以
> **插件全信任 + 运行时热组合** 为前提，与 Vivy 单二进制 / provenance / generation 审计哲学相反。
> 真正值得收窄的是**扩展点数量**（事件、provider-consumer），不是装载时机。

---

## 1. DSH 侧事实：Cordis，"一切皆插件"

来源：`.workspace/deepseek-harness/deepseek-harness/`（working clone，0.1.2-alpha.1，领先于 upstream mirror）。
以下 `<D>` = 该 clone 根。

### 1.1 插件是什么：纯代码，无类别限制

插件就是一个实现了 Service 的 JS/TS 模块，三种形态（`<D>/docs/cordis-primer.md:9`、
`<D>/docs/cordis-tutorial/01-first-plugin.md:55-75`）：

```ts
// 1. 函数插件（规范形态）
export const name = 'hello'
export const inject = ['tools']          // 依赖的服务 key
export function apply(ctx: Context, config: Config) { ... }
// 2. 对象插件 { name, apply(ctx) }
// 3. 类插件 class MyService extends Service
```

- 可选模块导出仅四个：`name` / `inject` / `apply` / `Config`（Schemastery schema）。
- **没有任何类别/类型枚举**。一个插件可以是工具、LLM adapter、沙箱后端、存储后端、
  UI 特性，乃至整个子系统。分类是**事实性的**（"工具管线事件归 `ctx.tools`，模型流式归
  `ctx.llm`"，`cordis-primer.md:56`），不是校验器强制的。

### 1.2 生命周期：声明式组合，运行时挂载

- **发现即声明**：插件是 YAML 组合文件（`cordis.patch.yml`）里的行；bundle patch →
  profile patch → `--patch` 覆盖层逐层叠写，每行 last-write-wins
  （`<D>/packages/bundle/base/cordis.patch.yml`）。
- Loader（`<D>/vendor/loader/src/config/entry.ts`）import 模块、经
  `ctx.registry.plugin(...)` 挂进 Fiber；条目并发启动，顺序只由 `inject` 依赖表达。
- 行内 `!!js` 标量可在激活时对 loader 上下文求值（config 插值 / `disabled` 条件）。
- **注册即可逆**：一切经 `ctx.effect()` / `ctx.on()` 注册的东西自动持有 disposer，
  卸载/HMR 按序回卷（`cordis-primer.md:13-16`）。
- 状态机 `pending|loading|active|failed|unloading`（`<D>/packages/host/plugin-inventory/src/types.ts:7-13`）；
  `name`/`inject`/`group` 变更强制 dispose+重启、失败回滚；config 变更热补丁。
- 安装：`dsh plugin add <pkg>` 转发 pnpm；`--patch ./overlay.yml` 挂本地插件。
- **模型动态插件**：挂 `tool-cordis` + `cordis-host-runner` 后，模型自己可用
  `cordis_define / cordis_run / cordis_stop / cordis_undefine` 在内存中定义运行插件
  （进程内、版本化、不落盘），`cordis_inspect_*` 只读自省
  （`<D>/packages/extensions/tool-cordis/README.md:40-56`）。

### 1.3 扩展面：~55 个 ctx 服务 key + 类型化事件

服务 key（各包 README 收割）：`tools, llm, sessions, sessionQuery, sessionPersistence,
sessionProjections, sessionTitle, fs, shell, subprocess, sandbox, sandboxPolicy, terminals,
codeRuntime, jobs, workflowEngine, subagents, agents, agentPresets, commands, skills,
systemPrompt, attachments, userQuestions, approval, credentials, settings, storage,
compaction, web, webServer, webhookRuntime, lsp, tokenMeter, goals, planMode,
permissionPresets, theme, locale, layout, slots(客户端), cordisInspect, ...`

关键能力形态：

| 形态 | API | 出处 |
|---|---|---|
| 模型工具 | `ctx.tools.register(defineTool({name, parameters, execute}))` + `tools/pre-execute|execute|post-execute` 瀑布中间件 | `<D>/packages/core/tools/src/index.ts:142-183` |
| 提示注入 | `ctx.systemPrompt` 有序 section、动态上下文、`{{var}}` | `<D>/packages/core/system-prompt/src/index.ts` |
| 人用命令 | `ctx.commands` 注册 CommandDefinition，UI 直执行、不过模型 | `<D>/docs/subsystems/commands.md:19-46` |
| 技能提供者 | `ctx.skills` 注册 SkillProvider {list, get} | `<D>/docs/subsystems/skills.md:30-77` |
| 子代理 | `ctx.subagents` 命名 spawn 注册表 | `<D>/packages/subagent/subagent/README.md:32` |
| UI 槽位 | `ctx.slots.register/inject` 类型化 React 槽位树 | `<D>/docs/subsystems/slots.md:44-64` |
| 基建替换 | LLM adapter、沙箱后端、存储、凭据（Service-Definition/Provider/Consumer 三件套） | `<D>/docs/user/develop/practice/index.md:9-49` |

事件五分派模式：`emit`（观察）/ `waterfall`（around 中间件、可短路）/ `parallel` /
`serial` / `bail`（首个非 undefined 胜）。分派模式是事件公共契约的一部分
（`cordis-primer.md:15-27`）。

### 1.4 信任模型：插件全信任，权限在工具执行层

- 常规插件**无沙箱**："preset 与它点名的插件同等特权……等同 shell 访问"
  （`<D>/packages/preset/agent-presets/README.md:12`）。
- 模型动态插件有 `node:vm` 沙箱 + `vmTimeoutMs`（默认 5000），浏览器半边需人工审批——
  但文档明说"沙箱隔离全局变量，**不是安全边界**，当作 bash 访问对待"
  （`tool-cordis/README.md:60`）。
- 真正的权限闸门是 `tools/pre-execute` allow/deny/ask 瀑布、approval 子系统、
  进程沙箱（`read-only | workspace-write | danger-full-access`）。
  **插件装载本身不过权限**。

---

## 2. Vivy 侧事实：编译期 seam 治理模型

来源：`sdk/`、`internal/pluginhost/`、`internal/channelhost/`、`docs/architecture/`。

### 2.1 插件是什么：治理单元 = 清单 + Go 源码 + 世代

```
plugins/<name>/
  vivy-plugin.json   身份 + 契约（verify/pack 读）
  plugin.go          实现 sdk/plugin.Plugin
```

清单是"给 SDK 编译器的配方卡，不是给运行时装载器的"
（`VIVY-PLUGIN-SPEC.md:87`）。schema：`sdk/internal/manifest.go:21-44`
（apiVersion（仅 `vivy.plugin/v0`）/ name / version / seam / module / grants / tools /
channel{transport, max_message_runes}）。

pack 流（五步，`.agents/skills/vivy-plugin-five/SKILL.md`）：
`vivy-sdk verify` → `vivy-sdk pack --with <name>` → 生成新 `zz_register.go` →
`go build -overlay` 出新代 EXE + `generation.json`（tree_hash、逐插件 provenance）。
**安装 = pack；卸载 = 去掉配方行重新 pack。运行中的进程永远不读 plugins/ 目录。**

### 2.2 类别限定：是的，三重闭集

1. **seam 四值枚举**（`sdk/plugin/plugin.go:14-31`）：`tool` / `tool-world` /
   `provider` / `channel`。规格明文：禁止 `journal`、`policy`、`sdk`、`studio`
   （`VIVY-PLUGIN-SPEC.md:82`："只许……禁止……"）。校验在
   `sdk/internal/manifest.go:73-76`。
2. **grants 八词封闭词汇表**（`plugin.go:35-76`）：`fs.read / fs.write /
   channel.poll / channel.webhook / channel.listen / channel.a2a / secret.read /
   proc.spawn`；seam 级再限制（channel 族 grants 仅 `seam: channel`；
   `proc.spawn` 仅 `tool-world`，`manifest.go:90-95`）。
3. **单一 Plugin 接口**：`Plugin { Name, Seam, Grants, Tools }`
   （`plugin.go:95-100`）——一个插件能贡献的**只有工具列表**，加上两个旁路通道：
   - `seam: channel` 时实现 `plugin.Channel` ABI（`sdk/plugin/channel.go:14-30`，
     另有 11 个可选能力接口 + 2 预留槽，`channel.go:138-225`）；
   - tool-world 可选实现 `DiagnosticObserver`（`plugin.go:119-122`）。

注意：`SeamProvider` 在词汇表里合法，但**内核零消费者**（grep 证实仅定义处出现）——
是个名义 seam。今天所有非 channel 插件实质上都是"工具包"。

### 2.3 插件不能做什么（verify 直接 fail，`sdk/internal/inspect.go:58-219`）

- import 内核（`agent-vivy/internal/...`）、Eino、pion、`.workspace` 路径；
- 直调 `os.Open/Create/StartProcess`、`exec.Command`（必须走 `Env`）；
- 开任何 listen socket；`go:embed` 可执行后缀；`package main`。
- Env 面刻意极小：`Secret / 出站 HTTP / Settings / PublishInbound / Media / Spawn`——
  "no Journal, no Policy, no raw OS, no Eino"（`VIVY-PLUGIN-SPEC.md:139`）。
- 无 UI、无命令、无事件/钩子注册、无热重载。内核确有 ToolHookChain
  （`internal/runtime/hooks.go:58`，`internal/app/app.go:312` 装配），但来源是
  **config 脚本**，不是插件可注册点。

### 2.4 设计意图（为什么长这样）

- "允许纯 Go 代码。不允许作者觉得自己在改内核"（`VIVY-PLUGIN-SPEC.md:28` 一带）。
- "插件崩溃 = 这一代 EXE 崩溃。隔离不在进程，在下一代"（`:263`）。
- `SELF-EVOLVING-GATEWAY.md:366-388` **显式拒绝**全部运行时装载：WASM、DLL/Go
  plugin、外部 stdio exe、目录扫描/插件市场，全部"拒绝"。只收两种形态：
  Skill 文本（Kind A，不编译）、Go 源码+清单（Kind B，进配方等 pack）。
- 三分法：Kind A Skill（行为文本）/ Kind B Plugin（能力源码）/ Kind C
  Generation（配方 + 打出的 EXE，唯一"安装"动作）（`SELF-EVOLVING-GATEWAY.md:236-256`）。
- AGENTS.md 硬规则 `no-plugin-via-engine-import`：装插件只走 `vivy-sdk pack`。

---

## 3. 逐维对比

| 维度 | DSH (Cordis) | Vivy | 定性 |
|---|---|---|---|
| 插件形态 | JS/TS 模块（代码即插件） | Go 源码 + JSON 清单 | 范式差，非优劣 |
| 类别限制 | 无（事实分类） | seam 四值闭集 + grants 八词闭集 | **真实差距**（用户感知点） |
| 装载时机 | 运行时（YAML 组合、HMR、热卸载） | 编译期（pack 出新代 EXE） | **有意差距**（NG-11，provenance/审计需要） |
| 扩展点数量 | ~55 ctx 服务 key + 五模式类型化事件 | 1 个接口（Tools）+ channel ABI + 1 观察者 | **真实差距，最大头** |
| 中间件/事件 | waterfall around、bail、可短路；工具管线 pre/execute/post | 无（ToolHookChain 是 config 脚本专属） | 真实差距 |
| UI/命令/技能注册 | 插件可注册 slots/commands/SkillProvider | 无；技能是 Kind A 纯文本，与插件体系正交 | 真实差距（部分是哲学：UI 不属物种身体） |
| 基建替换（LLM adapter/存储/沙箱后端） | Provider 三件套自由注册 | SeamProvider 有名无实；provider 是配置不是插件 | 真实差距（名义 seam 已预留） |
| 模型自写插件 | tool-cordis（vm 沙箱、人工审批、非安全边界） | 拒绝（S8"无门自改写"待封） | **有意差距**（NG-11） |
| 信任/沙箱 | 插件全信任；权限闸在工具执行层 | 源码级静态检查 + grants fail-closed + 路径沙箱 + 写入入 file_versions 账 | 不同范式；Vivy 更严在装载前，DSH 更松但运行时闸更细 |
| 失败语义 | 单插件 failed，其余存活 | 插件崩溃 = 这一代 EXE 崩溃，修复在下一代 | 范式差（`VIVY-PLUGIN-SPEC.md:263`） |
| 安装/卸载 | pnpm add / patch 行删除，即时 | re-pack 新代 EXE | 有意差距（世代审计） |

一句话：**DSH 把"组合性"做成产品（runtime plugin OS），Vivy 把"治理性"做成产品
（compile-time generation）。用户的想象 = DSH 范式；Vivy v0 拍板 = 反 DSH 范式。**

---

## 4. 差距定性：哪些不该追，哪些值得收窄

### 不该追（有意差距，动它伤物种哲学）

1. **运行时热装载**。`SELF-EVOLVING-GATEWAY.md` 已三拒（WASM/DLL/stdio exe）。
   编译期 pack 是 provenance（tree_hash、逐插件来源）与"卸载 = re-pack"审计语义的根基；
   改成运行时装载等于放弃 Generation 模型。
2. **插件进 Journal/Policy**。内核永不插件化（`SELF-EVOLVING-GATEWAY.md:159-169`）。
   DSH 也只是把权限做成插件可挂的事件，并未把存储引擎交出去——这条两边其实同向。
3. **模型自写插件**。DSH 自己声明 vm 沙箱非安全边界；Vivy S8 将其列为拒收。
   若将来要做，必须走 Kind A 文本化或独立 gate，不属于本文主张。

### 值得收窄（真实差距，且与治理模型兼容）

按性价比排序：

1. **SeamProvider 落地**（已有钩位，零新概念）：让 `seam: provider` 插件可注册
   LLM adapter / embedding / 存储后端实现，消费方仍走内核定义的接口。
   这正是 DSH Service-Definition/Provider/Consumer 三件套的 Go 对应物，
   且把"provider 是配置"升级为"provider 可插件供给"而不动摇编译期装载。
2. **事件/钩子 seam**（最大扩展面差距）：新增 `seam: hook`（或 tool-world 可选接口），
   允许插件挂 `pre-execute / post-execute / around` 式工具中间件与 run 生命周期观察者。
   内核已有 `ToolHookChain` 底座（`internal/runtime/hooks.go`），只差把"钩子来源"
   从 config 脚本扩到插件——权限闸仍在内核（每个 hook 也要过 grants 校验）。
3. **命令/UI 不动**：UI 与人用命令是 Studio/前端面，不属物种身体；
   Vivy 的双事件面架构有意把 UI 挡在外面。保持。
4. **grants 词汇按需扩**（如 `net.listen` 仍禁、但可议 `timer`）：闭集正确，
   但词汇表应随扩展点增长，而不是让能力挤进 tool 语义。

### 若拍板"就要 DSH 式完全自定义"

那不是改插件系统，是换产品哲学：接受插件全信任、放弃单二进制世代审计、
补运行时组合层（YAML patch / pnpm 等价物）、重写 channel/tools 装载路径。
成本量级 ≥ 一个 CH 通道 EPIC，且与 `prd-agent-vivy-v0.md` 的单 organism 契约冲突。
不建议。折中路线是上文 §4 第 1、2 条（编译期不变，扩缝）。

---

## 5. 结论

1. 用户感知属实：Vivy 插件被**三重闭集**限定（seam 枚举、grants 词汇、单一 Plugin
   接口），扩展面约为 DSH 的 1/50（1 个接口 + channel ABI + 1 观察者 vs ~55 服务 key
   + 事件系统）。
2. 但"没做好"不成立：闭集是 v0 显式拍板（spec 白纸黑字"只许/禁止"），配套的
   verify 静态检查、grants fail-closed、generation 审计是 DSH 没有的强项；
   DSH 的开放以"插件等同 shell 访问、无安全边界"为代价。
3. 真正的行动项不是"改成 DSH"，而是**在编译期范式内扩缝**：SeamProvider 落地 +
   hook/事件 seam。两项都不破坏 Kind A/B/C 分法和 generation 审计。
   已挂 `docs/TODO.md` §0.1 **PLG-1**，待拍板。
