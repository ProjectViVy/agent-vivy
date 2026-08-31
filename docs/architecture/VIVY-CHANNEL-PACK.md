# Vivy Channel Pack — 超级通道合同

> 状态：**方向已采纳**（2026-08-30 讨论拍板：超级通道 + 本批全部插件化）。
> 未改内核、未改 `sdk/plugin` 落地缝之前，本文是合同正本，不是实现。
> 服从 `SELF-EVOLVING-GATEWAY.md`、`VIVY-ASSEMBLY.md`、`VIVY-PLUGIN-SPEC.md`、**`VIVY-STUDIO.md`**、PRD §5.0 / D-016。
> 日期：2026-08-30
>
> 对照证据（只读，不是依赖）：
> - `.workspace/picoclaw` 的 channel 系统（Go 适配器样本）
> - agent-diva 2026-08-18～08-23：通道退役、能力信封、A2A 研究、NeuroLink 预留
> - `github.com/cloudwego/eino-ext/a2a`（A2A 协议/编解码对照；示例服务器不是产品路径）
> - DeepSeek Harness 的「能力缝 / 登记即效果」
> - 本仓库 ADR-015 的 pack overlay（`internal/generated/plugins/zz_register.go`）

相关：

- `VIVY-ASSEMBLY.md` — 按所是命名；本批通道适配器走 `plugins/` + `seam: channel`，不新开 `channels:` 配方键
- `VIVY-PLUGIN-SPEC.md` — 用户层 `plugins/<name>/`；本文扩 `seam: channel`
- `SELF-EVOLVING-GATEWAY.md` — 装插件 = 造新版本；默认 `Register()` 为空
- `VIVY-GATEWAY-AND-STUDIO.md` — NG-10 模型可见≡入账；NG-11 拒绝第二种身体
- `ACP-REMOTE-CONTROL-PROPOSAL.md` — 遥控已有回合；不是耳朵
- `VIVY-FACE-PACK.md` — 本机嘴（web / tui / headless）；channel 不得替代 face
- `VIVY-CHANNEL-EVOLUTION.md` — 本 EPIC 演进架构树；子 AGENT PLAN 在 `docs/plans/channel-epic/`
- `.workspace/picoclaw/pkg/channels/README.zh.md` — 协议适配器与 Manager 的分工

---

## 0. 一句话

> **超级通道是内核里的世界入口，不是五个 bot 的集合。**
> Host、规范信封、可选能力矩阵永远在物种里。
> telegram / discord / 飞书 / 钉钉 / QQ 全部是 `seam: channel` 插件。
> 真卸 = 配方删行再 `pack`。停用 = yaml。默认身体没有耳朵。

这是把 DSH「登记即效果」译成 Vivy 冷拔插的方式：偷能力缝和可检查的登记，不偷热挂、不偷把 Journal 当插件、不偷外挂 `telegram.exe`。

---

## 1. 拍板记录

| 日期 | 决定 |
|---|---|
| 2026-08-25 | 写出厂 `channels/` 冷拔插提案。C0 未采纳。 |
| 2026-08-27 | 前端移植 Diva 七平台配置 UI（仅 localStorage）。`UI-CHANNELS-BE`。 |
| 2026-08-30 | **采纳超级通道。** Host + 信封 + 能力矩阵是合同脊柱。A2A / NeuroLink 后切、同信封。 |
| 2026-08-30 | **本批全部插件化。** 五个适配器进 `plugins/<name>/`，走已落地的 `Register()` overlay。不新开 `channels/` 目录，不新开 `RegisterChannels()`。 |
| 2026-08-30 | **Eino 原生 A2A = 偷协议，不偷示例服务器。** 后切 `plugins/a2a` 用 `eino-ext/a2a` 的 models/transport；禁止 `RegisterServerHandlers(adk.Agent)` 当网关。循环仍是 `Service.Run`。 |

本批五个名字：`telegram`、`discord`、`feishu`、`dingtalk`、`qq`。

有意例外：`VIVY-ASSEMBLY.md` 写「出厂代码禁止放进 `plugins/`」。本批通道适配器**不是内核器官**，是可选耳朵，因此进 `plugins/` 并按 seam 打标签。若日后第一方器官变多，可把目录迁到 `channels/<name>/`，**ABI 不变**。

---

## 2. 三扇门（不要合成一个 seam）

```text
世界先说话                         人看见的嘴                    遥控已有回合
───────────────                    ──────────                    ──────────
ChannelHost / WorldPort            FaceHost                      ACP（另案）
  plugins/telegram …               web / tui / headless          观察、取消、审批
  以后：neurolink                  一代一张主脸                  不是新耳朵
  以后：a2a
```

- **Face** 是嘴。channel 不得替代本机 UI。
- **ACP** 是控制面。它转向已有 Run，不发明第二套入站世界。
- **ChannelHost** 是耳朵。聊天平台、NeuroLink、A2A 都从这扇门进。

A2A 不是第二套循环。远程任务映射到已有 Vivy `Run`。NeuroLink 不是 Telegram 的同行配置卡；它是后切的重量级插件，Listen 面由 Host 拥有。

---

## 3. 要解决的感觉

允许纯 Go 写 Telegram / 飞书适配器。不允许作者觉得自己在改网关，也不允许改一份 yaml 就让身体长出新耳朵。

判定「像在做 channel 插件」的标准：

1. 工作目录只有 `plugins/<name>/`。日常不打开 `internal/`。
2. 世界只通过 `sdk/plugin` 的 channel 契约进来。Host / Journal / `Service.Run` 是内核，适配器看不见。
3. 身份是清单里的名字和 `seam: channel`，不是某个 `.go` 被 `engine.go` 引用。
4. 加入 / 拿掉一个 channel，作者改的是**配方**，不是 blank import 列表。`pack` 生成 `Register()`。
5. 跑起来之前，它只是源。跑起来之后，它已经是某一代 EXE 的一块肉。默认提交进物种的注册表仍是空的（ADR-015）。

picoclaw 用 `init()` + Gateway blank import 把二十一个协议焊进同一个进程。那是热树上的行，卸不干净。Vivy 的叠加发生在 `vivy-sdk pack`，产物是一整代 EXE。

---

## 4. 四层

```text
L0  内核 Host（永不插件化）
    准入 · 会话映射 · 入账 · 派发 Run · 出站 · 能力发现
    可选 listen 面（不是 :8787）· 后切子进程监督

L1  规范信封（超级通道的真正合同）
    身份 / 线程 / 类型化 parts / 流式 / 投递 / 预留 task
    Face 之外的世界入口都归一到这里

L2  适配器 ABI（sdk/plugin，seam: channel）
    必选：Start / Stop / Send / PublishInbound
    可选：Host 类型断言发现，适配器不持有 Service

L3  插件器官 plugins/<name>/
    本批五个 + 以后 neurolink / a2a
    配方 plugins: 点名才编进这一代；默认 Register() = nil
```

| 层 | 住哪 | 改它的感觉 | 怎么出现在活身体里 |
|---|---|---|---|
| 内核 Host | `internal/`（实现时再定包名） | 在改 Vivy | 永远编进来 |
| 通道插件 | `plugins/<name>/`，`seam: channel` | 在做 Telegram / 飞书 | 配方 `plugins:` + pack |
| 配置信封 | `config.yaml` 的 `channels:` | 在调旋钮 | 只能点名 **inspect 已列出** 的名字 |

`pluginhost` 今天把一切 `Adapt` 成 `tools.Tool`。channel **不得**走这条路。`Seam()` 第一次成为分流依据：工具进工具表，channel 进 Host。

---

## 5. 从 DSH / picoclaw / Diva 偷什么，拒绝什么

### 5.1 偷

| 来源 | 想法 | Vivy 形态 |
|---|---|---|
| DSH | 按所是命名 | inspect 标签是 channel（来自 seam），不是 tool |
| DSH | 能力缝 = Definition + Provider + Consumer | 清单 + 适配器 + **只有 ChannelHost 消费** |
| DSH | 登记是效果 | grant、子进程柄、webhook 路径进 inspect；卸有定义 |
| DSH | 模型可见 ≡ 已入账 | 入站必须有 `channel.inbound`，不能伪装成本机 UI 的 `user` 行 |
| DSH | 两套事件面 | typing / placeholder 走活面；出处走 Journal |
| DSH | 可杀的世界 | 后切：同二进制 `vivy channel --name telegram`（不是第二种身体） |
| picoclaw | `Channel` + 可选能力接口 | Start/Stop/Send；typing / editor / media / listen 由 Host 发现 |
| picoclaw | 结构化 Inbound / Outbound / MediaPart | 抄字段，不抄 Manager 进内核包 |
| picoclaw | session 维度 `chat` / `topic` / `sender` | 映射到已有 Vivy Session，不再搞一套 JSONL |
| picoclaw | 共享 webhook mux | **Host 拥有**；适配器只是 Handler |
| ADR-015 | 活注册表为空；pack overlay 才链进去 | 同一份 `internal/generated/plugins/zz_register.go` |
| Diva 2026-08-23 | 频道信封必须能被 A2A 复用 | L1 预留 `task_id` / parts / stream，禁止私有 metadata 当主合同 |
| Diva 2026-08-23 | A2A 是北向适配器 | 后切 `plugins/a2a`；Task = 已有 Run |
| Diva 2026-08-23 | NeuroLink 是重量级预留 | 后切 `plugins/neurolink`；不进 telegram 式 ready/missing_fields 卡 |
| Eino ADK | `ChatModelAgent` + `Runner`（Vivy 已 pin） | 物种循环。ChannelHost 与后切 A2A 都喂 `Service.Run`，不另起 Runner |
| Eino ADK | `AgentAsTool` / DeepAgent | 进程内多智能体，不是 A2A 协议。本批不碰 |
| eino-ext/a2a | Agent Card、JSON-RPC、Task、parts、Stream | 后切 `plugins/a2a` 的 codec 偷 `models` + `transport` |
| Diva 2026-08-18 | 未验证通道退役 | slack / whatsapp / matrix / irc / mattermost / nextcloud 不进本批 |

### 5.2 拒绝

| 想法 | 原因 |
|---|---|
| 现有 `tool` / `tool-world` seam 硬塞 Telegram | 那是模型的手。channel 是世界先说话 |
| 五个适配器写进 `internal/` | 本批全部插件化；内核只留 Host |
| 本批新开 `channels/` + `RegisterChannels()` | 并行第二套 overlay。ABI 复用已落地的 `Register()` |
| 改 `config.yaml` 就出现 Discord | 配置不是插件系统。身体里没有的名字，启动失败 |
| 内核 `Config` 长 `TelegramSettings` | 非核心属性写回核心，每加一个协议内核胖一圈 |
| `.dll` / Go `plugin` / 外挂 `telegram.exe` | NG-11；Windows 一键；密钥边界 |
| 插件自己 `net.Listen` | 插件不是进程；Listen 面由 Host 开 |
| 把 Host / Journal / Session 映射做成插件 | 环境不能是种群成员；那是新种，不是新一代 |
| 21 个协议编进默认身体 | 体积、崩溃域、PRD §5.0.1 个人网关 |
| 微信 QR、WhatsApp native、Delta Chat 外挂 RPC | 非官方客户端、TTY、第二种身体 |
| 让 Telegram 用户直接 `approval/respond` | 新的 HITL 主体，属于 ACP 提案，不绑第一刀 |
| import `github.com/sipeed/picoclaw` | import-lint 禁止 `.workspace`；依赖树会炸开。MIT 允许改写适配器 |
| 空 `allow_from` 放行 | Diva GUI 合同；个人网关 fail-closed |
| 钉钉倒退成 webhook 文本机器人 | Diva / picoclaw 已是 Stream；Octos webhook 不是基线 |
| Discord `voice.go` / `pion/webrtc` | 本批明确不做 |
| 把网页 UI 做成 `seam: channel` | Face 是另一扇门 |
| `eino-ext/a2a.RegisterServerHandlers(adk.Agent)` 当 Vivy A2A 网关 | 第二套循环：自建 Hertz、默认内存 TaskStore、直接 `Runner.Run/Resume`，跳过 Journal / Policy / 审批 / 沙箱；Listen 不归 Host。协议能偷，这条服务器绑法不能偷 |

---

## 6. 冷拔插的三层卸载（不要混成一个开关）

```text
1. 换代拔插（真冷）
   配方删 telegram → pack → 下一代 EXE 没有这块肉，也没有 telego

2. 运行时停用（卡还在槽里）
   config enabled: false → Host 不 Start / 调用 Stop
   代码仍在身体里；inspect 仍列出 compiled-in

3. 实例杀死（DSH unload 在 Go 里能诚实落地的部分）
   杀掉 `vivy channel --name telegram` 子进程
   分类失败 channel_lost；物种与 Journal 还活着
   第一刀可以同进程；契约必须已经按进程边界写
```

「卸得干净」只适用于 (1)。判据见 §13。

`enabled: false` 不是卸载。活着的 EXE **不**动态加载任何 channel 代码——与 Kind B 相同。

---

## 7. ChannelHost（内核，永不插件化）

Host 列入「内核永不插件化」名单，与 Journal writer、Policy、Secret resolver、inspect、`vivy worker` 监督并列。

独占、适配器不许做的：

1. **准入。** `allow_from` 空 = 拒绝 `Start`（fail-closed）。禁止 picoclaw / Diva GUI「空名单等于对全世界说话」。`"*"` 第一刀不允许。
2. **会话映射。** `(channel, chat_id[, topic_id])` → 已有或新建 Vivy `Session`。本机 UI Session 与 channel Session 默认不合流。
3. **入账。** 先 `channel.inbound`，再 `Message(role=user)` 带出处。调用 `Service.Run`。
4. **出站。** 把终态（及后续若做的增量）投回适配器 `Send`。活面的 typing / placeholder 不进 Journal。
5. **密钥。** 只解析信封里的 `token_env`（或对称的 `*_env`）。值永不写配置、永不写 Journal。
6. **能力发现。** Host 对适配器做类型断言；可选接口不存在就降级，不得要求五个包实现全套。
7. **监督。** 后切子进程的生命周期；崩溃 = `channel_lost`，不是物种死亡。
8. **listen 面（若有，不在本批五个插件）。** 独立 listen，默认 loopback；**禁止**挂在 `:8787` `/rpc` 上。适配器只声明 path + `http.Handler`。NeuroLink 走这条面。

Host **不**认识 `parse_mode`、飞书 encrypt 算法、Telegram forum 的 markdown。那些是插件的 `settings`。

---

## 8. 规范信封（L1，第一刀就定形）

字段预留，行为后切。禁止 `Metadata map[string]string` 当主合同。picoclaw 已把 Peer / MessageID 提成一等字段，直接继承。

v1 五个插件可以只填文本；Host 必须已经认识这些槽：

| 槽 | v1 五个聊天插件 | 以后 NeuroLink | 以后 A2A |
|---|---|---|---|
| `channel` + `chat_id` + `sender` | 必填 | pipe 的 chat/sender | `channel=a2a` + contextId |
| `message_id` / `reply_to` / `topic_id` | 有就填 | 有就填 | A2A message id |
| `parts[]`（text / media-ref / structured） | 先只 text | text + 以后媒体 | text + artifact |
| 活面：typing / delta / placeholder / finalize | 有能力再接 | delta/reply 是协议核心 | StreamResponse |
| `run_id` / `task_id` | Host 写入，适配器只回传 | 同上 | A2A taskId = Vivy run_id |
| delivery / edit / delete | 可选接口 | 可选 | Task 状态 |

能力矩阵（Host 发现，inspect 列出 compiled-in vs advertised vs enabled）：

```text
必选     Start Stop Send
交互     Typing  Edit  Delete  Reaction  Placeholder  Stream
介质     MediaSender
入站面   WebhookHandler / ListenHandler   ← Host 拥有 Listen
可靠     HealthChecker  错误分类（rate-limit / temporary）
重量级   TaskLifecycle（A2A）  PipeServer（NeuroLink）
```

五个包第一刀：必选 + 该平台 picoclaw 里已经稳、且不挡文本闭环的可选能力。Stream / 媒体 / 群 / 审批卡片全部后切，但 **Host 必须已经认识这些接口**。否则不是超级通道，只是五个 bot。

---

## 9. 通道插件（L3）

### 9.1 目录（本批）

```text
plugins/
  hello-fs/                 # 已有；seam: tool-world
  telegram/                 # 本批；独立 go.mod
    vivy-plugin.json
    plugin.go               # New() plugin.Plugin，且实现 Channel
    settings.go             # 非核心字段的类型与解码
    plugin_test.go
    README.md
    go.mod                  # 只依赖 sdk/plugin + 该平台 SDK
  discord/
  feishu/
  dingtalk/
  qq/

internal/generated/plugins/zz_register.go
  // 提交进物种的永远是空的（ADR-015）
  func Register() []plugin.Plugin { return nil }
```

独立 Go module **对本批是硬要求**：默认 `go build ./cmd/vivy` 与 `just ci` 的 `go test ./...` 的 import 图到不了 `github.com/mymmrac/telego` 等。`pack` overlay 生成的 `Register()` 才 import `plugins/telegram`。`hello-fs` 可以留在物种模块里，因为它没有肥 SDK。

一旦有人在 `internal/` 随手 import telegram，冷拔插作废。

### 9.2 清单 `vivy-plugin.json`

```json
{
  "apiVersion": "vivy.plugin/v0",
  "name": "telegram",
  "version": "0.1.0",
  "seam": "channel",
  "module": ".",
  "grants": ["channel.poll", "secret.read"],
  "channel": {
    "transport": "poll",
    "max_message_runes": 4096
  }
}
```

| 字段 | 规则 |
|---|---|
| `name` | 与目录名一致；本代配方内唯一 |
| `seam` | 必须是 `channel`。禁止写成 `tool` / `tool-world` |
| `grants` | 本包上限，pack 进这一代后冻结。运行时不能靠配置放宽 |
| `channel.transport` | 本批只许 `poll`（出站长轮询或出站 WS 客户端）。`webhook` / `listen` 是后切，且需要对应 grant |
| `tools` | **禁止**出现。channel 不是模型工具 |

`vivy-sdk verify` 对 `seam: channel` 走 channel 规则（零个 tool，必须实现 Channel），对 `tool` / `tool-world` 仍要求至少一个 tool。

### 9.3 代码契约（按进程边界写）

作者只 import `agent-vivy/sdk/plugin`。公开面在现有 `SeamTool` 之外增加，而不是另起一个 SDK：

```go
const SeamChannel Seam = "channel"

const (
    GrantChannelPoll    Grant = "channel.poll"     // 出站长轮询 / WS 客户端
    GrantChannelWebhook Grant = "channel.webhook"  // 仅声明 path；Listen 是 Host 的
    GrantChannelListen  Grant = "channel.listen"   // NeuroLink 等；Listen 仍是 Host 的
    GrantChannelA2A     Grant = "channel.a2a"      // 后切
    GrantSecretRead     Grant = "secret.read"      // 只读 env_key 的值
)

type Channel interface {
    Name() string
    Seam() Seam // 必须 SeamChannel
    Grants() []Grant
    Start(ctx context.Context, env ChannelEnv) error
    Stop(ctx context.Context) error
    Send(ctx context.Context, msg OutboundMessage) (ids []string, err error)
}

type ChannelEnv interface {
    Secret(envKey string) (string, error) // 失败闭合；值不进日志
    HTTP() *http.Client                   // 出站；无 Listen
    Settings() json.RawMessage            // C4 新增；opaque settings 以 JSON 传插件
    PublishInbound(ctx context.Context, msg InboundMessage) error
    Media() MediaStore                    // 第一刀可为 no-op
}
```

`Register()` 仍返回 `[]plugin.Plugin`。`seam: channel` 的插件：

- 必须实现 `Channel`
- `Tools()` 必须为空（若 `Plugin` 仍带该方法）
- Host 按 `Seam()` 分流，不 Adapt 成 tool

现有 `hello-fs` 的 `Plugin` 形状本切片可以不拆。不要为了 channel 先 churn tool 插件 ABI。

`PLUGIN-SPEC` 禁止「插件里起长期后台服务抢端口」。channel 的例外只此一条：持有 `channel.poll` 时，`Start` 可以跑**出站**长轮询或出站 WS 客户端。`net.Listen` / `http.ListenAndServe` 仍然禁止。

`verify` 对 `seam: channel`：

| 禁止 | 原因 |
|---|---|
| `net.Listen` / `http.ListenAndServe` / `ListenAndServeTLS` | Listen 是 Host 的 |
| `os.Open` / `exec.Command` 绕过 Env | 与现有插件相同 |
| import `internal/`、import eino | 与现有插件相同 |
| 清单带 `tools` | 认错 Consumer |
| grant 含 `channel.webhook` / `channel.listen` / `channel.a2a` 而本批配方启用 | 本批五个插件不允许 |
| import picoclaw 或 `.workspace` | 改写，不依赖 |

可选能力（typing、MessageEditor、Placeholder、MediaSender、WebhookHandler、StreamingCapable、TaskLifecycle、PipeServer）仍用接口，由 Host 类型断言。适配器不持有 `*runtime.Service`。

即使 v1 Telegram 与 Host **同进程**调用，Inbound 也只走 `Env.PublishInbound`。这样后切把包挪到 `vivy channel` 子进程时，不改适配器。

---

## 10. 配方与 pack

不为本批增加 `channels:` 配方键。点名走已有的 `plugins:`：

```yaml
apiVersion: vivy.generation/v0
loop: eino
world: sandbox
providers:
  - openai
tools:
  - notes
  - filesystem
  - execute
  - ask-user
plugins:
  - plugins/hello-fs          # seam: tool-world
  - plugins/telegram          # seam: channel
  - plugins/dingtalk          # seam: channel
```

规则：

- 没写进配方的插件，这一代不存在。不扫描 `plugins/`。
- `pack` 生成作者不准手改的同一份 `Register()`：

```go
// Code generated by vivy-sdk pack. DO NOT EDIT.
func Register() []plugin.Plugin {
    return []plugin.Plugin{
        hellofs.New(),
        telegram.New(),
        dingtalk.New(),
    }
}
```

- `inspect` / `generation.json` 按 seam 分类列出：name、version、seam、grants、transport、source_ref、tree_hash。telegram 打印成 channel，不是 tool。
- 主线提交的物种身体是全量本体：`internal/generated/plugins/zz_register.go`
  注册所有第一方通道插件，`just run`、内嵌 UI 二进制和 Docker 开箱即有
  全部耳朵。`pack` 在构建期用 `-overlay` 把同一份文件替换成 `--with`
  选出的更窄组合，用来装配精简代；提交的本体保持全量。

卸通道插件 = 从 `plugins:` 删一行，再 pack。要 eval / promote。不是删运行时 allowlist。

开发手感：

```text
vivy-sdk verify plugins/telegram
vivy-sdk pack --with telegram --with dingtalk
vivy-sdk inspect-artifact dist/...
```

---

## 11. 配置：信封固定，settings 不透明

配置合法，当且仅当它旋转 **这一代已经编进来的** 名字。它不能长出新器官。

```yaml
channels:
  telegram:
    enabled: true
    allow_from: ["telegram:123456"]
    token_env: TELEGRAM_BOT_TOKEN
    settings:                    # 内核对这块不透明
      parse_mode: html
      proxy: "http://127.0.0.1:7890"
```

| 字段 | 谁解码 | 规则 |
|---|---|---|
| 名字（map key） | Host | 必须出现在这一代 `Register()` 里且 `seam: channel`，否则启动失败 |
| `enabled` | Host | `false` = 不 Start。inspect 仍显示 compiled-in |
| `allow_from` | Host | 空或缺省 = 拒绝 Start。`"*"` 第一刀不允许 |
| `token_env` | Host → `SecretResolver` | 必须匹配 `^[A-Z][A-Z0-9_]*$`。没有字面 token |
| `settings` | **该 channel 插件** | 未知字段 fail-closed。新字段若这代 lib 不认识，要升版本再 pack |

因此「可选配置文件更新」只覆盖：

- 已经编译进解码器的非核心字段（proxy、parse_mode、占位文案）
- 今晚关耳朵（`enabled: false`）
- 收紧 / 改写 `allow_from`、换 `token_env` 的名字

不覆盖：加 Discord、改 transport 为 webhook、给 Host 加新事件类型、放宽 grants。

这与远程 MCP、provider `env_key` 同一条哲学：**配置是旋钮和远程依赖，不是插件系统。**

内核 `config.go` 只增加 channel **信封**（名字、enabled、allow_from、token_env、opaque settings）。禁止 `TelegramSettings`、`FeishuSettings` 进 `internal/config`。

前端现状（`UI-CHANNELS-BE`）：七平台表单写 localStorage，`allow_from` 文案是「留空不限制」，平台列表含 email / neuro-link。接后端时必须改成：

- 只能开关 **compiled-in** 的名字
- 空 `allow_from` = 拒绝启动
- email / neuro-link 从可添加列表拿掉，直到有对应插件

---

## 12. 入站路径与账本

```text
平台 Update
  → 适配器规范化 InboundMessage / SenderInfo / parts
  → ChannelEnv.PublishInbound
  → Host：allow_from（空 = 丢弃并记审计，不 Start 的 channel 根本走不到这里）
  → Journal  channel.inbound
        {channel, peer, message_id, content_digest, bytes}
        永不写 token、原始密钥、无限附件
  → SessionMap.Ensure(channel, chat_id[, topic]) → Session（带来源）
  → Message(role=user, source=channel, …)
  → Service.Run
  → 活面：typing / placeholder（不进 Journal）
  → run 终态 → Host → adapter.Send
```

NG-10：新的模型可见输入必须有新事件。今天把 Telegram 文本写成普通 `user` 行，回放会像本机 UI 说的。

需要改的合同（采纳后，另 PR，不在本文偷运实现）：

- 新 `EventType`：`channel.inbound`（及可选的 `channel.started` / `channel.stopped` / `channel.lost`，后两者可先活面）
- `domain.Message` 增加出处：`source`（`ui` \| channel 名）、`peer`（bounded）
- `run.started` 保持 `additionalProperties: false`；来源不塞进旧 payload
- SQLite / Postgres 迁移与 conformance

第一刀不做媒体、不做群触发、不做增量流式编辑。Telegram forum 若碰到，按 picoclaw 把 `topic_id` 拼进映射键，避免串上下文。

HITL：审批 / 提问仍只由内核定输赢。第一刀双写到本机 UI；channel 只投递「有一个待审批」类文本（可选，可再砍）。Telegram 用户直接决定审批 = ACP 远程主体，不在本合同。

---

## 13. 控制面与网络

`:8787` 继续是 loopback JSON-RPC。Channel 的 `poll` 是「联系 provider」（与 MCP、模型 HTTP 同类），不把本机变成托管入口。

`channel.webhook` / `channel.listen` 与 `0.0.0.0` 是产品决定，要显式重访 PRD §5.0.1 / D-016。本批五个插件没有 webhook / listen。若后切 NeuroLink 或 webhook：

- Host 开第二条 listen，默认 loopback
- 公网或隧道必须配置显式打开
- 适配器不得自己 Bind

RPC 不急着做 `channels/list` 的完整 CRUD。第一刀：`species/inspect` 已能列出 compiled-in；配置仍是文件。后切再加只读 RPC，避免做成运行时装载器。设置页换读写层属于 `UI-CHANNELS-BE`，依赖 Host 落地。

---

## 14. 初批五个插件

产品要的不是 picoclaw 那 21 个协议，而是一套 **可过夜的国内耳朵 + 两只国际耳朵**。
全部从 `.workspace/picoclaw` **改写**适配器（不 import 其模块），接到同一套 Host。
默认提交的 `vivy.exe` 仍然 `Register() = nil`。下面是 **可点名进配方的插件**，不是焊进日常身体。

每个适配器第一刀都是：私聊（或单聊）文本 in / 文本 out、`allow_from` fail-closed、`token_env`、无媒体、无群触发、无 HITL 代批。
群 / 媒体 / 占位编辑 / 流式是该包的后切，不挡 Host。

### 14.1 波次（按传输和产品风险，不是按知名度）

| 波次 | 插件 | picoclaw 源 | 传输（代码为准） | 认证 | 为何排这里 |
|---|---|---|---|---|---|
| **0** | 无协议 | — | — | — | Host + 账本 + 空注册表 + SDK seam。没有这个，移植只是抄 bot |
| **A** | `plugins/telegram` | `pkg/channels/telegram` | 出站 long-poll | Bot token | 最小适配器，用来钉死 ABI；CI 用假更新，不碰真实 Bot |
| **A** | `plugins/dingtalk` | `pkg/channels/dingtalk` | 出站 Stream WS | `client_id` / `client_secret` | **第一只国内耳朵**。无需公网、无 QR、无 32-bit stub。回包依赖入站带来的 `session_webhook` |
| **B** | `plugins/feishu` | `pkg/channels/feishu` | 出站飞书 SDK WS | `app_id` / `app_secret` | 国内协作主力。文档仍写 webhook，**实现是 WS**。64-bit only；`encrypt_key` 放 lib `settings` |
| **B** | `plugins/qq` | `pkg/channels/qq` | 官方 Bot WS | `app_id` / `app_secret` | 国内社群。是 **QQ 开放平台机器人**，不是个人号 |
| **B** | `plugins/discord` | `pkg/channels/discord` | Gateway WS | Bot token | 国际社区。**禁止**移植 `voice.go` / `pion/webrtc` / TTS 探测 |
| **后切** | `plugins/neurolink` | Diva `neuro_link.rs`（合同，不是代码源） | Host listen + 本机 WS | 本机绑定 | 重量级管道。不进本批，不进设置页可添加列表 |
| **后切** | `plugins/a2a` | eino-ext/a2a 的 models/transport（codec）；Diva A2A 研究包（北向适配器） | HTTP+JSON（默认关） | Bearer / 技能白名单 | Task = Run。不把 `RegisterServerHandlers` 当网关。不进本批 |
| **不做进本批** | wecom / weixin / onebot / email | — | — | — | 绑定面、个人号、第二种身体、邮箱另案 |

Slack / LINE / Matrix 沿用 Diva 2026-08-18 退役，不进本批。

### 14.2 推荐的两代配方（示例，不是默认身体）

开发/评测用的「国际校对身体」（点名才存在）：

```yaml
plugins:
  - plugins/telegram
  - plugins/discord
```

国内过夜用的「办公身体」:

```yaml
plugins:
  - plugins/dingtalk
  - plugins/feishu
  - plugins/qq
```

住户可以只要其中一行。禁止为了省事把五个焊进默认 `just run` 的 EXE。
`inspect` 必须能看出这一代编了哪些、缺 telego 还是缺 lark SDK。

### 14.3 每包第一刀范围

| 包 | 做 | 明确不做（该包后切） |
|---|---|---|
| telegram | 私聊文本、long-poll、proxy/`base_url` 可放 settings | webhook、群、媒体、命令菜单、MarkdownV2 全套 |
| dingtalk | 单聊文本、Stream 模式、保存并使用 session webhook | 卡片、媒体；不改成 webhook 文本机器人 |
| feishu | 单聊文本、WS 事件、`is_lark` 域名开关 | 32-bit、表情、文档里的公网 webhook 模式 |
| qq | 单聊/频道文本（以官方 API 能稳收的为准） | 大文件 base64、语音、个人号、OneBot |
| discord | DM / 文本频道文本、Message Content Intent | `voice.go`、WebRTC、slash command 全家桶、TTS |

### 14.4 改写规则（相对 picoclaw）

- 只偷：`Start`/`Stop`/`Send`、InboundContext/SenderInfo、错误分类、该平台的 token 用法。
- 不偷：`init()` blank import 进 Gateway、空 `allow_from` 放行、各 channel 自建 HTTP、内核里的 `TelegramSettings` 类型。
- 每个包自己的 `settings.go` 解码 opaque yaml。Host 只看见信封。
- 一个包一个 `vivy-plugin.json`。配方点谁，`pack` 才 import 谁。飞书 SDK 不得出现在只含 telegram 的那代 `go.mod` 闭包里。

---

## 15. A2A / NeuroLink 预留（本批不实现）

两者都是 ChannelHost 上的重量级插件，不是新内核，也不是 Face / ACP。

**NeuroLink**

- 本机 WebSocket **server**，给第三方管道（桌面伙伴等）
- grant：`channel.listen`；Host 拥有 bind，默认 loopback
- 协议帧可后定；入站仍走 `PublishInbound`，chat/sender 映射成 channel Session
- 不进 `channel_statuses` 那种 telegram 式必填字段卡
- 设置页在本插件编进身体之前不得列出「添加 NeuroLink」

**A2A**

- 北向智能体互操作。基线 A2A v1.0 HTTP+JSON，默认关闭
- grant：`channel.a2a`
- `taskId` = 已有 Vivy `run_id`；不要第二套 TaskStore 语义
- Agent Card 是能力声明，不是权限系统；真实权限仍是 Policy / Approval
- 请求必须进 `Service.Run`，不能绕过沙箱和审批
- 出站 URL 防 SSRF；远程返回当不可信输入

信封在本批就已经为它们留了 `task_id` / parts / stream。禁止五个聊天插件先把这些做成私有 metadata。

### 15.1 与 Eino 原生 A2A 的关系

Eino **核心**（Vivy pin `github.com/cloudwego/eino v0.9.13` 的 `adk.ChatModelAgent` + `Runner`）没有 A2A 线协议。进程内多智能体是 `AgentAsTool` / DeepAgent，与通道无关，本批不碰。

Eino **原生 A2A** 在扩展包 `github.com/cloudwego/eino-ext/a2a`（只读对照，不是本批依赖；观察到 `v0.0.1-alpha.13`）。拆两层，不要合成一条「整包原生」：

| 层 | 是什么 | Vivy |
|---|---|---|
| 协议 / 编解码 | Agent Card、JSON-RPC、Task、Message parts、Stream（`models` + `transport`） | **后切 `plugins/a2a` 的 codec 偷这里** |
| 示例服务器 | `extension/eino.RegisterServerHandlers(adk.Agent)`：自建 Hertz + 默认内存 TaskStore，直接 `adk.NewRunner().Run/Resume` | **禁止当产品路径** |

正确叠法（后切 C9，本批不实现）：

```text
A2A JSON-RPC / Agent Card          ← eino-ext/a2a 的 models + transport
        ↓
plugins/a2a  (seam: channel)       ← 独立 go.mod；只做编解码 + 能力声明
        ↓
ChannelHost                        ← allow_from、session、channel.inbound、Journal
        ↓
Service.Run → 已有 ADK Runner      ← Eino 原生循环（现在就在 internal/runtime）
        ↓
事件回 Host → 插件 Send            ← StreamResponse / Task 状态
```

禁止把 `RegisterServerHandlers` 挂到 Vivy 的 `adk.Agent` 上。那会：另起 TaskStore、跳过 Journal / Policy / 审批 / 沙箱、第二张 Hertz 听面、把 alpha 依赖和 Hertz 拖进默认身体。该模块声明的 eino 版本与 Vivy pin 也不对齐，不能 drop-in。`VIVY-PLUGIN-SPEC.md` 已禁止插件 import `github.com/cloudwego/eino*`——A2A 协议栈若进身体，只许作为独立 `plugins/a2a` 的 go.mod，且不得让默认 `just ci` 闭包看见它。

同 ACP：`eino-ext/acp` 也是 `AgentEvent` 直出协议。Vivy ACP 提案已拒绝第二套运行时；A2A 同样。

本批五个聊天插件零 Eino import。C1–C8 不必为 A2A 改形状。C9 才写独立能力提案。

---

## 16. 「卸得干净」的验收

从一代身体拿掉 telegram 之后，对 **新 EXE**（不是对还在跑的旧进程）必须同时成立：

1. `Register()` 不含 telegram
2. `vivy-sdk inspect-artifact` 的配方与依赖图不含 telegram / telego
3. 启动后没有 telegram 子进程，也不发起 Telegram HTTP
4. 配置里若仍写 `channels.telegram`，启动失败（名字不在身体里），而不是默默忽略
5. 旧 Journal 的 `channel.inbound` **仍在**（账本是遗传物质；换代删器官不删历史）

`enabled: false` 的验收是另一套：二进制里仍有 telegram，inspect 列出 compiled-in + disabled，进程不 poll。

默认 `just ci` 路径：`go test ./...` 不编译五个通道模块，物种 `go.mod` 不出现 telego / discordgo / lark / 钉钉 / botgo。

---

## 17. 和现有插件、MCP、worker、Face 的边界

```text
Kind A  Skill 文本            不编译
Kind B  能力源码              工具 / provider / tool-world / **channel**
Kind C  世代 EXE              pack 的唯一装载动作
配置     MCP 地址、env_key、channel 信封
worker   同二进制子 run        不是插件通道；后切 channel 子进程抄它的 argv 模式
face     本机嘴                另一份提案；不得用 channel 替代
ACP      遥控                  另一份提案；不是入站世界
```

channel 是 Kind B 的新 seam，不是 Kind A，不是 MCP，不是第二种 EXE。

`config.tools.enabled` 管出厂工具。ADR-015：pack 进去的用户工具绕过这张表。channel 对称：配方决定肉，信封 `enabled` 决定醒着。没有 `channels.allow` 去拉外置进程。

---

## 18. 关键决定

1. **Host 是内核，适配器是插件。** 把 Host 插件化会使 `channel.inbound` 随适配器漂移，Journal 不再是遗传物质。
2. **本批五个全部进 `plugins/`，`seam: channel`。** 不新开 `channels/` 与 `RegisterChannels()`。inspect 按 seam 打标签。
3. **默认注册表为空。** 与 ADR-015 同构。日常 `just run` / 默认 `vivy.exe` 没有耳朵。
4. **配置不能发明身体里没有的名字。** 冷拔插的根。
5. **内核只解码信封，settings 由插件解码。** 否则每加一个协议就改 `internal/config`。
6. **本批 transport = poll（含出站 WS 客户端）。** webhook / listen 重访 local-first。
7. **契约按进程边界写，第一刀可同进程。** 崩溃域后切，ABI 先对。
8. **空 `allow_from` fail-closed。** 个人网关默认不对全世界说话。设置页文案必须跟着改。
9. **不 import picoclaw。** 改写适配器，偷消息模型和能力接口。
10. **HITL 主体仍是本机。** channel 当审批人另案。
11. **独立 `go.mod` 对本批是硬要求。** 肥 SDK 不得进入默认 `just ci` 闭包。
12. **信封第一刀定形。** A2A / NeuroLink 后切插件，不后切合同槽。
13. **Face / ACP 仍独立。** 网页 UI 不是 `seam: channel`。
14. **Eino 原生 A2A = 偷协议，不偷 `RegisterServerHandlers`。** 循环仍是 `Service.Run`；codec 后切才碰 `eino-ext/a2a`。

---

## 19. 非目标（本合同）

- 本文件合并时改 `sdk/plugin` 或加事件类型（那是采纳后的实现 PR）
- 热挂 / 热卸 / 市场扫描
- 完整 picoclaw 协议矩阵、媒体流水线、群触发、流式占位编辑
- 公网 webhook、Discord voice、微信个人号、OneBot 外挂桥、email
- 企业微信扫码绑定面
- 用 channel 替代本机 UI
- 实现 A2A 或 NeuroLink
- 把 `eino-ext/a2a` 的 Hertz 示例服务器 / `RegisterServerHandlers(adk.Agent)` 当 Vivy 网关
- 把五个 SDK 写进默认 `go.mod`

---

## 20. 落地切片（实现时）

顺序就是依赖。每一刀都应能单独评测；未做的不出现在默认 EXE。

| 切片 | 做什么 | 成功 |
|---|---|---|
| C0 合同 | 本文采纳；ASSEMBLY / PLUGIN-SPEC / GATEWAY 交叉引用 | 文档一致，无代码。**本切片** |
| C1 账本 | `channel.inbound` 事件 schema、Message 出处、存储迁移、conformance | `just ci`；无适配器 |
| C2 SDK + 空注册表 + 信封配置 | `SeamChannel`、grants、`ChannelEnv`、verify 禁 Listen / 禁 tools、pack 能 overlay 非 tool 插件 | 无协议 deps；pack 空列表仍为空身体 |
| C3 Host + 假 channel 插件 TCK | 能力发现、fail-closed、PublishInbound → 入账 → Run → Send | 无真实协议 |
| C4 `plugins/telegram` | 私聊文本 polling；独立 module；fail-closed allow_from | 候选能收发文本；默认 EXE 仍无 telego |
| C5 inspect / UI | inspect 列出 compiled-in vs enabled；设置页只展示身体里有的名字；空 allow_from 文案改正 | 住户能看见这代有没有耳朵 |
| C6 `plugins/dingtalk` | Stream；session webhook 只进 lib settings / 运行时表 | 国内单聊文本闭环 |
| C7 `plugins/feishu` + `qq` + `discord` | 三个独立包、三次 pack 点名；Discord 不含 voice | 配方可组成「办公身体」或「国际身体」 |
| C8 同二进制子进程 | `vivy channel --name <id>`；Host 监督 | 杀一只耳朵不断账本 |
| C9 NeuroLink / A2A | 各需独立能力提案 + 绑定/鉴权面。A2A：codec 用 eino-ext/a2a 的 models/transport，执行走 ChannelHost → `Service.Run` | 无提案则本切片不开；禁止示例服务器绑 ADK |

C0 是文档 PR。C1 起才动内核。C4 之前禁止把 `telego` 写进物种默认 `go.mod`。C4/C6/C7 每个包一次 pack 评测，禁止「一次 PR 链进五个 SDK」。

排期、WBS、活动图与甘特的权威在 `docs/TODO.md` §0.2（2026-08-30 拍板）。本文切片是依赖，不是日历。演进树：`VIVY-CHANNEL-EVOLUTION.md`。子 AGENT 领取：`docs/plans/channel-epic/`。

---

## 21. 已关闭 / 仍开放的问题

已关闭：

1. 出厂模块是否独立 `go.mod` — **是**（本批硬要求）。
2. 本批是否新开 `channels/` — **否**。走 `plugins/` + 现有 `Register()`。
3. 五个适配器是否进内核 — **否**。全部插件化。
4. 超级通道边界 — Face / ACP 独立；A2A / NeuroLink 是 Host 上的后切插件。
5. 第一刀 ABI 厚度 — 信封和可选接口第一刀定形；五个包只实现文本必选。
6. 空 `allow_from` — fail-closed。
7. Eino 原生 A2A — 协议/编解码可复用；`RegisterServerHandlers(adk.Agent)` 不是产品路径。循环仍是 `Service.Run`。本批五个插件不 import Eino。

仍开放（不挡 C0，挡后续实现或产品）：

1. **channel Session 与本机 UI Session 能否显式链接。** 第一刀否。需要产品句子再开。
2. **`"*"` 作为 allow_from。** 第一刀否。若将来要公开 bot，必须显式且进 inspect。
3. **何时把 `plugins/telegram` 迁到 `channels/telegram`。** 第一方器官变多再开；ABI 不变。
4. **设置页 email / neuro-link 卡片何时删除。** 接 `UI-CHANNELS-BE` 时删；C0 只改合同。

---

## 22. PR Plan（实现时）

### PR 1 — 采纳合同

- 文件：本文；`VIVY-ASSEMBLY.md`；`VIVY-PLUGIN-SPEC.md`；`SELF-EVOLVING-GATEWAY.md`；`VIVY-GATEWAY-AND-STUDIO.md`
- 依赖：无
- 无运行时代码
- **本 PR**

### PR 2 — 账本出处

- 文件：`internal/domain`、`schemas/events/`、`internal/storage/{sqlite,postgres,conformance}`
- 依赖：PR 1
- 增加 `channel.inbound` 与 Message 出处；旧路径 `source=ui`

### PR 3 — SDK + 空注册表 + 信封配置

- 文件：`sdk/plugin`、`sdk/internal` verify/pack（支持独立 module 的 channel 插件）、`internal/config`、`internal/pluginhost` 分流
- 依赖：PR 2
- `pack` 能把 `seam: channel` 叠进现有 `Register()`；默认身体仍为空

### PR 4 — ChannelHost 最小闭环（假插件）

- 文件：`internal/` 下 Host、Session 映射、测试用假 channel 插件
- 依赖：PR 3
- 单测：PublishInbound → 入账 → Run；空 allow_from 拒绝

### PR 5 — `plugins/telegram`

- 文件：`plugins/telegram/`（独立 go.mod）、pack 配方示例
- 依赖：PR 4
- 候选 EXE 对真实 Bot API 的手工/隔离测试；`just ci` 默认路径仍不链 telego

### PR 6 — inspect 与设置页

- 文件：inspect RPC、`ui/` 设置（替换 localStorage 读写层；拿掉未编入的平台；改正 allow_from 文案）
- 依赖：PR 5
- 开发验证走 `http://127.0.0.1:3015`

### PR 7 — `plugins/dingtalk`

- 文件：`plugins/dingtalk/`
- 依赖：PR 4（可与 PR 5 并行，但 Host 必须先合）
- session webhook 留在适配器

### PR 8 — `plugins/feishu`

- 文件：`plugins/feishu/`（仅 64-bit 实现，32-bit 编译失败要有明确错误）
- 依赖：PR 4
- WS 事件，不实现公网 webhook

### PR 9 — `plugins/qq`

- 文件：`plugins/qq/`
- 依赖：PR 4
- 官方机器人 API，文本 only

### PR 10 — `plugins/discord`（无 voice）

- 文件：`plugins/discord/`（不复制 `voice.go`）
- 依赖：PR 4
- 该插件 `go.mod` 不得出现 `pion/webrtc`

### PR 11 — 同二进制 channel 子进程

- 文件：`cmd/vivy` argv、Host 监督、分类失败
- 依赖：至少一个真实适配器（PR 5 或 PR 7）
- 契约不变，适配器搬家

NeuroLink / A2A 各需独立提案，不在本 plan。

---

## 23. 一句话（再写一遍）

> **配方是那棵树。Channel 插件是树上的一行。**
> 活进程不长出新行。新行只出现在下一代 EXE 上。
> Host 是环境；插件是肉；yaml 是旋钮。
> 改 yaml 不等于换代；换代才能让 telego 从二进制里消失。
> 五个耳朵全部是插件。默认身体是聋的。
