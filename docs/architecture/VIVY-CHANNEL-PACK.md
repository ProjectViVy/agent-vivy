# Vivy Channel Pack — 出厂 channel 的冷拔插

> 状态：**提案**。未采纳前不改内核、不改 `sdk/plugin` 的已落地缝。
> 服从 `SELF-EVOLVING-GATEWAY.md`、`VIVY-ASSEMBLY.md`、`VIVY-PLUGIN-SPEC.md`、**`VIVY-STUDIO.md`**、PRD §5.0 / D-016。
> 日期：2026-08-25
>
> 对照证据（只读，不是依赖）：`.workspace/picoclaw` 的 channel 系统；
> DeepSeek Harness 的「能力缝 / 登记即效果」；本仓库 ADR-015 的 pack overlay。
> 多通道网关在 V0 是非目标（PRD §4、`AGENT-VIVY-DIRECTION.md`）。本文是 **V1 之后**
> 的能力提案，不是把 picoclaw 焊进日常 `vivy.exe`。

相关：

- `VIVY-ASSEMBLY.md` — 按所是命名；出厂单元不住 `plugins/`
- `VIVY-PLUGIN-SPEC.md` — 只约束用户层 `plugins/<name>/`
- `SELF-EVOLVING-GATEWAY.md` — 装插件 = 造新版本；默认 `Register()` 为空
- `VIVY-GATEWAY-AND-STUDIO.md` — NG-10 模型可见≡入账；NG-11 拒绝第二种身体
- `ACP-REMOTE-CONTROL-PROPOSAL.md` — 远程主体与 HITL；channel 用户当审批人走那条路，不走本提案第一刀
- `VIVY-FACE-PACK.md` — 本机嘴（web / tui / headless）；channel 不得替代 face
- `.workspace/picoclaw/pkg/channels/README.zh.md` — 协议适配器与 Manager 的分工

---

## 0. 一句话

> **Channel 是配方上的可编译器官，不是工具，也不是内核。**
> Host 留在物种里；协议和非核心字段放进出厂模块；配置只能旋转已经编进这代身体的旋钮。
> 真卸 = 配方删行再 `pack`。停用 = yaml。杀实例 = 同二进制子进程（后切）。

这是把 DSH「一切皆插件」译成 Vivy 冷拔插的方式：偷能力缝和可检查的登记，不偷热挂、不偷把 Journal 当插件、不偷外挂 `telegram.exe`。

---

## 1. 要解决的感觉

允许纯 Go 写 Telegram / 飞书适配器。不允许作者觉得自己在改网关，也不允许改一份 yaml 就让身体长出新耳朵。

判定「像在做 channel 包」的标准：

1. 出厂工作目录只有 `channels/<name>/`。用户自写的才进 `plugins/<name>/`（`seam: channel`）。日常不打开 `internal/`。
2. 世界只通过 `sdk/plugin` 的 channel 契约进来。Host / Journal / `Service.Run` 是内核，适配器看不见。
3. 身份是清单里的名字和 seam，不是某个 `.go` 被 `engine.go` 引用。
4. 加入 / 拿掉一个 channel，作者改的是**配方**，不是 blank import 列表。`pack` 生成 `RegisterChannels()`。
5. 跑起来之前，它只是源。跑起来之后，它已经是某一代 EXE 的一块肉。默认提交进物种的注册表仍是空的（与 ADR-015 同构）。

picoclaw 用 `init()` + Gateway blank import 把二十一个协议焊进同一个进程。那是热树上的行，卸不干净。Vivy 的叠加发生在 `vivy-sdk pack`，产物是一整代 EXE。

---

## 2. 从 DSH / picoclaw 偷什么，拒绝什么

### 2.1 偷

| 来源 | 想法 | Vivy 形态 |
|---|---|---|
| DSH | 按所是命名 | 配方键 `channels:`，inspect 标签是 channel，不是 plugin |
| DSH | 能力缝 = Definition + Provider + Consumer | 清单 + 适配器 + **只有 ChannelHost 消费** |
| DSH | 登记是效果 | grant、子进程柄、webhook 路径进 inspect；卸有定义 |
| DSH | 模型可见 ≡ 已入账 | 入站必须有 `channel.inbound`，不能伪装成本机 UI 的 `user` 行 |
| DSH | 两套事件面 | typing / placeholder 走活面；出处走 Journal |
| DSH | 可杀的世界 | 后切：同二进制 `vivy channel --name telegram`（不是第二种身体） |
| picoclaw | `Channel` + 可选能力接口 | Start/Stop/Send；typing / editor / media / webhook 由 Host 发现 |
| picoclaw | 结构化 Inbound / Outbound / MediaPart | 抄字段，不抄 Manager 进内核包 |
| picoclaw | session 维度 `chat` / `topic` / `sender` | 映射到已有 Vivy Session，不再搞一套 JSONL |
| picoclaw | 共享 webhook mux | **Host 拥有**；适配器只是 Handler |
| ADR-015 | 活注册表为空；pack overlay 才链进去 | `internal/generated/channels/zz_register.go` 同构 |

### 2.2 拒绝

| 想法 | 原因 |
|---|---|
| 现有 `tool` / `tool-world` seam 硬塞 Telegram | 那是模型的手。channel 是世界先说话 |
| 出厂 Telegram 放进 `plugins/` | 冒充用户层，命名脏了（`VIVY-ASSEMBLY.md`） |
| 改 `config.yaml` 就出现 Discord | 配置不是插件系统。身体里没有的名字，启动失败 |
| 内核 `Config` 长 `TelegramSettings` | 非核心属性写回核心，每加一个协议内核胖一圈 |
| `.dll` / Go `plugin` / 外挂 `telegram.exe` | NG-11；Windows 一键；密钥边界 |
| 插件自己 `net.Listen` | 插件不是进程；webhook 面由 Host 开 |
| 把 Host / Journal / Session 映射做成插件 | 环境不能是种群成员；那是新种，不是新一代 |
| 21 个协议编进默认身体 | 体积、崩溃域、PRD §5.0.1 个人网关 |
| 微信 QR、WhatsApp native、Delta Chat 外挂 RPC | 非官方客户端、TTY、第二种身体 |
| 让 Telegram 用户直接 `approval/respond` | 新的 HITL 主体，属于 ACP 提案，不绑第一刀 |
| import `github.com/sipeed/picoclaw` | import-lint 禁止 `.workspace`；依赖树会炸开。MIT 允许改写适配器 |

---

## 3. 三层，名词分开

```text
vivy.exe  内核（永不可插件化）
  Journal · Policy · SecretResolver · HITL · inspect
  ChannelHost          ← 新的一等对象；对标 picoclaw Manager+Bus 的职责，不是抄它的包
       │ 准入、会话映射、入账、出站、监督、可选 webhook 面
       │
       ├─ 出厂模块 channels/telegram     配方键 channels:
       └─ 用户模块 plugins/acme-matrix   配方键 plugins: ，seam: channel
              ▲
              │  都只实现 SDK Channel 契约
              │  默认世代不 import；pack overlay 才链进 RegisterChannels()
```

| 层 | 住哪 | 改它的感觉 | 怎么出现在活身体里 |
|---|---|---|---|
| 内核 Host | `internal/`（实现时再定包名） | 在改 Vivy | 永远编进来 |
| 出厂 channel 包 | `channels/<name>/`（独立模块或默认可不链的树） | 在做 Telegram / 飞书 | 配方 `channels:` + pack |
| 用户 channel 包 | `plugins/<name>/` | 在做插件 | 配方 `plugins:` + pack，seam 必须是 `channel` |
| 配置信封 | `config.yaml` 的 `channels:` | 在调旋钮 | 只能点名 **inspect 已列出** 的名字 |

出厂代码禁止放进 `plugins/`。用户代码禁止放进 `channels/`。两边 ABI 相同，目录和口头禅不同。

---

## 4. 冷拔插的三层卸载（不要混成一个开关）

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

「卸得干净」只适用于 (1)。判据见 §12。

`enabled: false` 不是卸载。活着的 EXE **不**动态加载任何 channel 代码——与 Kind B 相同。

---

## 5. ChannelHost（内核，永不插件化）

Host 列入「内核永不插件化」名单，与 Journal writer、Policy、Secret resolver、inspect、`vivy worker` 监督并列。

独占、适配器不许做的：

1. **准入。** `allow_from` 空 = 拒绝 `Start`（fail-closed）。禁止 picoclaw「空名单等于对全世界说话」。
2. **会话映射。** `(channel, chat_id[, topic_id])` → 已有或新建 Vivy `Session`。本机 UI Session 与 channel Session 默认不合流。
3. **入账。** 先 `channel.inbound`，再 `Message(role=user)` 带出处。调用 `Service.Run`。
4. **出站。** 把终态（及后续若做的增量）投回适配器 `Send`。活面的 typing / placeholder 不进 Journal。
5. **密钥。** 只解析信封里的 `token_env`（环境变量名）。值永不写配置、永不写 Journal。
6. **监督。** 后切子进程的生命周期；崩溃 = `channel_lost`，不是物种死亡。
7. **webhook 面（若有，不在第一刀）。** 独立 listen，默认 loopback；**禁止**挂在 `:8787` `/rpc` 上。适配器只声明 path + `http.Handler`。

Host **不**认识 `parse_mode`、飞书 encrypt 算法、Telegram forum 的 markdown。那些是 lib 的 `settings`。

---

## 6. 出厂模块（external lib）

协议实现和非核心属性住在这里。内核 `internal/config` **不**为每个协议长结构体。

### 6.1 目录（目标）

```text
channels/                         # 出厂；默认世代不 import
  go.mod                          # 可选：独立模块，避免物种默认 go.mod 拖入 telego
  telegram/
    vivy-channel.json             # pack / verify 必读
    plugin.go                     # New() plugin.Channel
    settings.go                   # 非核心字段的类型与解码
    plugin_test.go
    README.md
  feishu/
    ...

internal/generated/channels/zz_register.go
  // 提交进物种的永远是空的（ADR-015 同构）
  func Register() []plugin.Channel { return nil }
```

独立 Go module 是推荐形态：默认 `go build ./cmd/vivy` 的 import 图到不了 `github.com/mymmrac/telego`。`pack` overlay 生成的 `RegisterChannels()` 才 import `channels/telegram`。若第一刀为了少一个 module 而同仓库同 module，必须用「空注册表 + 无 import」保证默认二进制同样干净；一旦有人在 `internal/` 随手 import telegram，冷拔插作废。

### 6.2 清单 `vivy-channel.json`

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
| `name` | 与目录名一致；本代配方内唯一（出厂 `channels:` 与用户 `plugins:` 共用一个名字空间） |
| `seam` | 必须是 `channel`。出厂包禁止写成 `tool` |
| `grants` | 本包上限，pack 进这一代后冻结。运行时不能靠配置放宽 |
| `channel.transport` | 第一刀只许 `poll`（出站长轮询或出站 WS 客户端）。`webhook` 是后切，且需要 grant `channel.webhook` |
| `tools` | **禁止**出现在 channel 清单里。channel 不是模型工具 |

用户自写的 channel 仍用 `vivy-plugin.json`，`seam: channel`，同样没有 `tools` 列表。`vivy-sdk verify` 对 `seam: channel` 走 channel 规则（至少零个 tool），对 `tool` / `tool-world` 仍要求至少一个 tool。

### 6.3 代码契约（按进程边界写）

作者只 import `agent-vivy/sdk/plugin`。公开面在现有 `SeamTool` 之外增加，而不是另起一个 SDK：

```go
const SeamChannel Seam = "channel"

const (
    GrantChannelPoll    Grant = "channel.poll"     // 出站长轮询 / WS 客户端
    GrantChannelWebhook Grant = "channel.webhook"  // 仅声明 path；Listen 是 Host 的
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
    PublishInbound(ctx context.Context, msg InboundMessage) error
    Media() MediaStore                    // 第一刀可为 no-op
}
```

`pluginhost` 今天把一切 `Adapt` 成 `tools.Tool`。channel 不得走这条路。Host 才是 Consumer。`Seam()` 第一次成为分流依据：工具进工具表，channel 进 Host。

`verify` 对 `seam: channel`：

| 禁止 | 原因 |
|---|---|
| `net.Listen` / `http.ListenAndServe` / `ListenAndServeTLS` | Listen 是 Host 的 |
| `os.Open` / `exec.Command` 绕过 Env | 与现有插件相同 |
| import `internal/`、import eino | 与现有插件相同 |
| 清单带 `tools` | 认错 Consumer |
| grant 含 `channel.webhook` 而第一刀配方启用 | 第一刀不允许 webhook |

可选能力（typing、MessageEditor、Placeholder、MediaSender、WebhookHandler）仍用接口，由 Host 类型断言。适配器不持有 `*runtime.Service`。

即使 v1 Telegram 与 Host **同进程**调用，Inbound 也只走 `Env.PublishInbound`。这样 v1.5 把包挪到 `vivy channel` 子进程时，不改适配器。

---

## 7. 配方与 pack

`vivy.generation.yml`（采纳后）增加一等键：

```yaml
apiVersion: vivy.generation/v0
loop: eino
world: sandbox
providers:
  - openai
  - mock
tools:
  - notes
  - filesystem
  - execute
  - ask-user
channels:                 # 出厂器官；没写就不存在于这一代
  - telegram
plugins:
  - plugins/hello-fs
  - plugins/acme-matrix   # seam 必须是 channel，否则 verify 失败
```

规则：

- 没写进配方的出厂 channel，这一代不存在。不扫描 `channels/`。
- 用户 channel 仍走 `plugins:`，靠清单 seam 分流，不靠第二个目录扫描。
- `pack` 生成作者不准手改的：

```go
// Code generated by vivy-sdk pack. DO NOT EDIT.
func RegisterChannels() []plugin.Channel {
    return []plugin.Channel{
        telegram.New(),
        acmematrix.New(),
    }
}
```

- `domain.AssemblyRecipe` 增加 `Channels []string`。`inspect` / `generation.json` 列出每个 channel 的 name、version、seam、grants、transport、source_ref、tree_hash。
- 默认提交的物种身体：`RegisterChannels()` 返回 nil，配方 `channels:` 为空。`just run` 的开发二进制没有耳朵。

卸出厂 channel = 从 `channels:` 删一行，再 pack。卸用户 channel = 从 `plugins:` 删一行。都要 eval / promote。不是删运行时 allowlist。

---

## 8. 配置：信封固定，settings 不透明

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
| 名字（map key） | Host | 必须出现在这一代 `RegisterChannels()` 里，否则启动失败 |
| `enabled` | Host | `false` = 不 Start。inspect 仍显示 compiled-in |
| `allow_from` | Host | 空或缺省 = 拒绝 Start。`"*"` 若将来允许，必须显式且记入 inspect |
| `token_env` | Host → `SecretResolver` | 必须匹配 `^[A-Z][A-Z0-9_]*$`。没有字面 token |
| `settings` | **该 channel 包** | 未知字段 fail-closed。新字段若这代 lib 不认识，要升版本再 pack |

因此「可选配置文件更新」只覆盖：

- 已经编译进解码器的非核心字段（proxy、parse_mode、占位文案）
- 今晚关耳朵（`enabled: false`）
- 收紧 / 改写 `allow_from`、换 `token_env` 的名字

不覆盖：加 Discord、改 transport 为 webhook、给 Host 加新事件类型、放宽 grants。

这与远程 MCP、provider `env_key` 同一条哲学：**配置是旋钮和远程依赖，不是插件系统。**

内核 `config.go` 只增加 channel **信封**（名字、enabled、allow_from、token_env、opaque settings）。禁止 `TelegramSettings`、`FeishuSettings` 进 `internal/config`。

---

## 9. 入站路径与账本

```text
平台 Update
  → 适配器规范化 InboundMessage / SenderInfo
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

HITL：审批 / 提问仍只由内核定输赢。第一刀双写到本机 UI；channel 只投递「有一个待审批」类文本（可选，可再砍）。Telegram 用户直接决定审批 = ACP 远程主体，不在本提案。

---

## 10. 控制面与网络

`:8787` 继续是 loopback JSON-RPC。Channel 的 `poll` 是「联系 provider」（与 MCP、模型 HTTP 同类），不把本机变成托管入口。

`channel.webhook` 与 `0.0.0.0` 是产品决定，要显式重访 PRD §5.0.1 / D-016。第一刀没有 webhook。若后切：

- Host 开第二条 listen，默认 loopback
- 公网或隧道必须配置显式打开
- 适配器不得自己 Bind

UI 设置页里的 Diva 通道演示数据（`diva-preview-data.ts`）不是本层。采纳后应改为 inspect 这一代 **编进来的** channel + 配置信封，而不是写死 Telegram/Discord/飞书三张卡片。

RPC 不急着做 `channels/list` 的完整 CRUD。第一刀：`species/inspect` 已能列出 compiled-in；配置仍是文件。后切再加只读 RPC，避免做成运行时装载器。

---

## 11. 初期出厂配方（国内 + Telegram + Discord）

产品初期要的不是 picoclaw 那 21 个协议，而是一套 **可过夜的国内耳朵 + 两只国际耳朵**。
全部从 `.workspace/picoclaw` **改写**适配器（不 import 其模块），接到同一套 Host。
默认提交的 `vivy.exe` 仍然 `channels: []`。下面是 **可点名进配方的出厂包**，不是焊进日常身体。

每个适配器第一刀都是：私聊（或单聊）文本 in / 文本 out、`allow_from` fail-closed、`token_env`、无媒体、无群触发、无 HITL 代批。
群 / 媒体 / 占位编辑 / 流式是该包的后切，不挡 Host。

### 11.1 波次（按传输和产品风险，不是按知名度）

picoclaw 文档把飞书/企微标成「较难」，把 Telegram/Discord 标成「简单」。Vivy 另外一条轴：**出站拉更新 vs 扫码 vs 外挂进程**。大陆验收不能只拿 Telegram：它经常要代理。

| 波次 | 包 | picoclaw 源 | 传输（代码为准） | 认证 | 为何排这里 |
|---|---|---|---|---|---|
| **0** | 无协议 | — | — | — | Host + 账本 + 空注册表。没有这个，移植只是抄 bot |
| **A** | `telegram` | `pkg/channels/telegram` | 出站 long-poll | Bot token | 最小适配器，用来钉死 ABI；CI 用假更新，不碰真实 Bot |
| **A** | `dingtalk` | `pkg/channels/dingtalk` | 出站 Stream WS | `client_id` / `client_secret` | **第一只国内耳朵**。无需公网、无 QR、无 32-bit stub。注意：回包依赖入站带来的 `session_webhook` |
| **B** | `feishu` | `pkg/channels/feishu` | 出站飞书 SDK WS（`larkws.Client`） | `app_id` / `app_secret` | 国内协作主力。文档仍写 webhook，**实现是 WS**。64-bit only；`encrypt_key` 放 lib `settings` |
| **B** | `qq` | `pkg/channels/qq` | 官方 Bot WS | `app_id` / `app_secret` | 国内社群。是 **QQ 开放平台机器人**，不是个人号 |
| **B** | `discord` | `pkg/channels/discord` | Gateway WS | Bot token | 国际社区。**禁止**移植 `voice.go` / `pion/webrtc` |
| **C** | `wecom` | `pkg/channels/wecom` | 出站 AI Bot WS | 扫码得到 `bot_id` + `secret` | 传输合格，**绑定不合格**：要 TTY/UI 扫码。进初期配方前必须先有 Vivy 侧的绑定面（设置页或一次性 CLI），密钥仍只经 `env_key` / 信封 |
| **不做进初期出厂** | `weixin` | `pkg/channels/weixin` | iLink 长轮询 | 个人微信扫码 | 个人号、非 Bot 合同、可能要 ffmpeg。另开提案 |
| **不做进初期出厂** | `onebot` | `pkg/channels/onebot` | 连本地 NapCat/Lagrange | `ws_url` | 外挂协议进程 = 第二种身体。用户以后可写 `plugins/onebot` |

Slack / LINE / Matrix 不在本初期集合里。

### 11.2 推荐的两代配方（示例，不是默认身体）

开发/评测用的「国际校对身体」（点名才存在）：

```yaml
channels:
  - telegram
  - discord
```

国内过夜用的「办公身体」：

```yaml
channels:
  - dingtalk
  - feishu
  - qq
```

住户可以只要其中一行。禁止为了省事把 A+B+C 焊进默认 `just run` 的 EXE。
`inspect` 必须能看出这一代编了哪些、缺 telego 还是缺 lark SDK。

### 11.3 每包第一刀范围

| 包 | 做 | 明确不做（该包后切） |
|---|---|---|
| telegram | 私聊文本、long-poll、proxy/`base_url` 可放 settings | webhook、群、媒体、命令菜单、MarkdownV2 全套 |
| dingtalk | 单聊文本、Stream 模式、保存并使用 session webhook | 卡片、媒体 |
| feishu | 单聊文本、WS 事件、`is_lark` 域名开关 | 32-bit、表情、文档里的公网 webhook 模式 |
| qq | 单聊/频道文本（以官方 API 能稳收的为准） | 大文件 base64、语音 |
| discord | DM / 文本频道文本、Message Content Intent | `voice.go`、WebRTC、slash command 全家桶 |
| wecom | 仅在绑定面就绪后：单聊文本 WS | 流式编辑、媒体、把 QR 打进物种 TTY 当唯一绑定 |

### 11.4 改写规则（相对 picoclaw）

- 只偷：`Start`/`Stop`/`Send`、InboundContext/SenderInfo、错误分类、该平台的 token 用法。
- 不偷：`init()` blank import 进 Gateway、空 `allow_from` 放行、各 channel 自建 HTTP、内核里的 `TelegramSettings` 类型。
- 每个包自己的 `settings.go` 解码 opaque yaml。Host 只看见信封。
- 一个包一个 `vivy-channel.json`。配方点谁，`pack` 才 import 谁。飞书 SDK 不得出现在只含 telegram 的那代 `go.mod` 闭包里（独立 module 或空注册表保证）。

---

## 12. 「卸得干净」的验收

从一代身体拿掉 telegram 之后，对 **新 EXE**（不是对还在跑的旧进程）必须同时成立：

1. `RegisterChannels()` 不含 telegram
2. `vivy-sdk inspect-artifact` 的配方与依赖图不含 telegram / telego
3. 启动后没有 telegram 子进程，也不发起 Telegram HTTP
4. 配置里若仍写 `channels.telegram`，启动失败（名字不在身体里），而不是默默忽略
5. 旧 Journal 的 `channel.inbound` **仍在**（账本是遗传物质；换代删器官不删历史）

`enabled: false` 的验收是另一套：二进制里仍有 telegram，inspect 列出 compiled-in + disabled，进程不 poll。

---

## 13. 和现有插件、MCP、worker 的边界

```text
Kind A  Skill 文本            不编译
Kind B  能力源码              工具 / provider / tool-world / **channel**
Kind C  世代 EXE              pack 的唯一装载动作
配置     MCP 地址、env_key、channel 信封
worker   同二进制子 run        不是插件通道；后切 channel 子进程抄它的 argv 模式
```

channel 是 Kind B 的新 seam，不是 Kind A，不是 MCP，不是第二种 EXE。

`config.tools.enabled` 管出厂工具。ADR-015：pack 进去的用户工具绕过这张表。channel 对称：配方决定肉，信封 `enabled` 决定醒着。没有 `channels.allow` 去拉外置进程。

---

## 14. 关键决定

1. **Host 是内核，适配器是肉。** 把 Host 插件化会使 `channel.inbound` 随适配器漂移，Journal 不再是遗传物质。
2. **出厂走 `channels/`，用户走 `plugins/` + `seam: channel`。** 同一 ABI，不同目录。禁止出厂冒充用户插件。
3. **默认注册表为空。** 与 ADR-015 同构。日常 `just run` / 默认 `vivy.exe` 没有耳朵。
4. **配置不能发明身体里没有的名字。** 冷拔插的根。
5. **内核只解码信封，settings 由 lib 解码。** 否则每加一个协议就改 `internal/config`。
6. **第一刀 transport = poll。** webhook 重访 local-first。
7. **契约按进程边界写，第一刀可同进程。** 崩溃域后切，ABI 先对。
8. **空 `allow_from` fail-closed。** 个人网关默认不对全世界说话。
9. **不 import picoclaw。** 改写适配器，偷消息模型和能力接口。
10. **HITL 主体仍是本机。** channel 当审批人另案。

---

## 15. 非目标（本提案）

- V0 交付、把多通道写进当前 `just ci` 必过路径
- 热挂 / 热卸 / 市场扫描
- 完整 picoclaw 协议矩阵、媒体流水线、群触发、流式占位编辑
- 公网 webhook、Discord voice、微信个人号、OneBot 外挂桥
- 企业微信扫码绑定面（波次 C 的前置；未做之前 `wecom` 不进配方）
- 用 channel 替代本机 UI
- 在本文件合并时改 `sdk/plugin` 或加事件类型（那是采纳后的实现 PR）

---

## 16. 落地切片（采纳后）

顺序就是依赖。每一刀都应能单独评测；未做的不出现在默认 EXE。

| 切片 | 做什么 | 成功 |
|---|---|---|
| C0 合同 | 本文采纳；`VIVY-ASSEMBLY.md` 增加 `channel` 行；内核永不插件化名单加上 Host；PLUGIN-SPEC 声明 `seam: channel` 的分流 | 文档一致，无代码 |
| C1 账本 | `channel.inbound` 事件 schema、Message 出处、存储迁移、conformance | `just ci`；无适配器 |
| C2 Host + SDK | `SeamChannel`、grants、`ChannelEnv`、空 `zz_register.go`、verify 禁 Listen、信封配置 | 无协议 deps；pack 空列表仍为空身体 |
| C3 出厂 telegram | `channels/telegram` 私聊文本 polling；pack overlay；fail-closed allow_from | 候选能收发文本；默认 EXE 仍无 telego |
| C4 inspect / UI | inspect 列出 compiled-in vs enabled；设置页丢掉 Diva 演示列表 | 住户能看见这代有没有耳朵 |
| C5 钉钉 | `channels/dingtalk` Stream；session webhook 只进 lib settings / 运行时表，不进 Journal 明文滥用 | 国内单聊文本闭环 |
| C6 飞书 + QQ + Discord 文本 | 三个独立包、三次 pack 点名；Discord 不含 voice | 配方可组成「办公身体」或「国际身体」 |
| C7 同二进制子进程 | `vivy channel --name <id>`；Host 监督 | 杀一只耳朵不断账本 |
| C8 企微（可选） | 绑定面 + `channels/wecom` | 无扫码面则本切片不做 |

C0 是文档 PR。C1 起才动内核。C3 之前禁止把 `telego` 写进物种默认 `go.mod` 的必经 import。C5/C6 每个包一次 pack 评测，禁止「一次 PR 链进五个 SDK」。

---

## 17. 尚未关闭的问题（采纳前要人拍板）

1. **出厂模块是否独立 `go.mod`。** 推荐独立，保证默认二进制无协议 deps。同 module + 空注册表是较小的第一刀，但更易被误 import。
2. **C5 是否与 C3 同船。** 推荐 C3 同进程、C5 紧随，避免第一只耳朵把物种打死。若第一刀就必须可杀，C3+C5 合并，成本更高。
3. **channel Session 与本机 UI Session 能否显式链接。** 第一刀否。需要产品句子再开。
4. **`"*"` 作为 allow_from。** 第一刀否。若将来要公开 bot，必须显式且进 inspect。

---

## 18. PR Plan（采纳后）

### PR 1 — 采纳合同

- 文件：本文状态改为方向采纳；`VIVY-ASSEMBLY.md` 增加 channel 行；`VIVY-PLUGIN-SPEC.md` 交叉引用 seam 分流；`SELF-EVOLVING-GATEWAY.md` 内核名单加上 ChannelHost
- 依赖：无
- 无运行时代码

### PR 2 — 账本出处

- 文件：`internal/domain`、`schemas/events/`、`internal/storage/{sqlite,postgres,conformance}`
- 依赖：PR 1
- 增加 `channel.inbound` 与 Message 出处；旧路径 `source=ui`

### PR 3 — SDK + 空注册表 + 信封配置

- 文件：`sdk/plugin`、`sdk/internal` verify/pack、`internal/generated/channels`、`internal/config`、`internal/pluginhost` 分流（工具仍 Adapt，channel 不进工具表）
- 依赖：PR 2
- `pack` 能生成 `RegisterChannels()`；默认身体仍为空

### PR 4 — ChannelHost 最小闭环（无真实协议）

- 文件：`internal/` 下 Host、Session 映射、假适配器测试
- 依赖：PR 3
- 单测：PublishInbound → 入账 → Run；空 allow_from 拒绝

### PR 5 — 出厂 `channels/telegram`

- 文件：`channels/telegram/`、pack 配方示例
- 依赖：PR 4
- 候选 EXE 对真实 Bot API 的手工/隔离测试；`just ci` 默认路径仍不链 telego

### PR 6 — inspect 与设置页

- 文件：inspect RPC、`ui/` 设置（替换 Diva 演示数据）
- 依赖：PR 5
- 开发验证走 `http://127.0.0.1:3015`

### PR 7 — 出厂 `channels/dingtalk`

- 文件：`channels/dingtalk/`
- 依赖：PR 4（可与 PR 5 并行，但 Host 必须先合）
- 国内第一只耳朵；session webhook 留在适配器

### PR 8 — 出厂 `channels/feishu`

- 文件：`channels/feishu/`（仅 64-bit 实现，32-bit 编译失败要有明确错误）
- 依赖：PR 4
- WS 事件，不实现公网 webhook

### PR 9 — 出厂 `channels/qq`

- 文件：`channels/qq/`
- 依赖：PR 4
- 官方机器人 API，文本 only

### PR 10 — 出厂 `channels/discord`（无 voice）

- 文件：`channels/discord/`（不复制 `voice.go`）
- 依赖：PR 4
- `go.mod` 不得出现 `pion/webrtc`

### PR 11 — 同二进制 channel 子进程

- 文件：`cmd/vivy` argv、Host 监督、分类失败
- 依赖：至少一个真实适配器（PR 5 或 PR 7）
- 契约不变，适配器搬家

### PR 12 — 企业微信（可选，绑定面先行）

- 文件：设置页或一次性绑定 CLI + `channels/wecom/`
- 依赖：PR 6（要有非 TTY 的密钥落入信封的路径）
- 无绑定面则本 PR 不开

---

## 19. 一句话（再写一遍）

> **配方是那棵树。Channel 是树上的一行。**
> 活进程不长出新行。新行只出现在下一代 EXE 上。
> Host 是环境；lib 是肉；yaml 是旋钮。
> 改 yaml 不等于换代；换代才能让 telego 从二进制里消失。
