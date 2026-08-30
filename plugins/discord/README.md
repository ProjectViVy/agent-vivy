# plugins/discord — Vivy 的 Discord 耳朵（Gateway 文本收发，DM + 服务器文字频道）

`discord` 是一个 seam-channel 插件（VIVY-CHANNEL-PACK.md §9），由内核
ChannelHost 启动和消费，不是模型工具。目录结构和 Start/Stop/Send 骨架
沿用 `plugins/telegram` 钉下的形状模板，生命周期沿用
`plugins/dingtalk` / `plugins/feishu` / `plugins/qq` 的监督重拨模式。

## **本插件只对接 Discord 官方 Bot**

- **不是用户账号（self-bot）**：不登录任何个人 Discord 账号，不模拟
  客户端协议，不使用用户 token。唯一凭据是在
  [discord.com/developers](https://discord.com/developers/applications)
  开发者门户申请的 **Bot Token**（`Bot ` 前缀由插件补上）。
- **不做 webhook 服务**：没有 webhook 回调、没有监听端口。传输是
  **出站 Gateway WebSocket**（manifest `transport: "poll"` + grant
  `channel.poll`，WS 计入 poll，合同 §14.3）+ 官方 REST 发消息接口
  （`POST /channels/{id}/messages`）。
- **无 voice / 无 pion / 无 TTS**：完全不接 Discord 语音（voice.go、
  pion/webrtc、WebRTC、TTS 一律不碰——SDK 验证器从 CH-C7c 起**拒绝
  任何插件 import `github.com/pion/*`**，测试夹具
  `sdk/internal/testdata/bad-pion-import` 钉住这条规则）。
- **无 slash 全家桶**：不注册任何 application command / interaction
  处理器——slash 指令在 discordgo 里是独立的 interaction 事件，本插件
  只注册 `MESSAGE_CREATE`，interaction 根本到不了插件；消息侧的指令
  类型负载（type 20/23）也被形状过滤器丢弃。
- SDK 用 **上游 `github.com/bwmarrin/discordgo v0.29.0` 原版**。参考
  实现 picoclaw 钉的是同一版本但带了一个 fork replace
  （`yeongaori/discordgo-fork`）；本插件**不带**这个 replace，物种只
  链接上游模块。

代码与文档同样声明：见 `plugin.go` 包注释（no voice/no pion/no TTS/no
slash 全部写在最前面）。

## Message Content Intent（运维前置条件）

Discord 从 2022 年起把消息内容列为**特权 intent**：

- 插件在每次 IDENTIFY 时请求 `IntentsGuildMessages |
  IntentsDirectMessages | IntentsMessageContent`；
- **机器人拥有者必须先在开发者门户（Bot 页）勾选
  "MESSAGE CONTENT INTENT"**，否则网关会以关闭码 4014 拒绝会话——
  第一次 `Open` 直接失败，Start fail-closed（不会留下一个"看似已
  启动、实际收不到内容"的聋耳朵）；
- 不开 intent 时 DM 的文本仍会推送（DM 豁免），但服务器文字频道的
  `content` 会恒为空——插件按"空内容不发布"的规则丢弃，等于失聪。

## 第一刀范围（做什么 / 不做什么）

**做：**

- **DM（私信）+ 服务器文字频道**的纯文本收（`MESSAGE_CREATE`，
  Message Content Intent，见上）
- 纯文本发：`POST /channels/{id}/messages`，content **原样**发送——
  Discord 原生渲染 markdown，插件**不剥离也不追加**任何格式；不使用
  embed、不强转 markdown
- 回复寻址：ChatID 即**频道 ID**（DM 场景就是 DM 频道 ID，可直接作为
  发送目标——与被动回复型平台不同，Discord 允许 bot 主动向可见频道
  发消息，因此**没有被动回复窗口**，重启后也能回话）
- `ReplyTo` 槽位：入站 type-19 回复消息带的 `referenced_message.id`
  会被填进信封（仅记录，不做线程行为）

**不做（后切）：**

- **语音全套**（voice.go / pion / WebRTC / TTS / 语音转写）—— Species
  层面禁令，不是"暂缓"
- slash 指令、按钮/组件、modal、context menu、interaction 任何形式
- 群组触发词/@ 过滤策略（服务器频道里所有可读文本都会进 Host 的
  allow_from 白名单闸门）、embed、媒体/附件、reaction、typing 指示、
  消息编辑/撤回、论坛帖、线程管理

## 权限与策略边界

| 事项 | 归属 |
|---|---|
| `allow_from` 发信人白名单 | **Host（内核）** 强制，插件不做自己的白名单 |
| 密钥解析 | Host 的 `Secret`；Discord 只有**一把** bot token，settings 侧 `token_env` 与信封 `token_env` 成对钉住（C3/C4 模式），两边必须一致否则 fail-closed |
| channel settings | 插件严格解码（未知字段 fail-closed），内核不认识 `settings` 里的键 |
| 监听端口 | 没有。Gateway 是纯出站 WebSocket；发送是纯 REST 出站 |

发信人写成 `discord:<用户ID>`（`MESSAGE_CREATE` 的 `author.id`），
ChatID 即频道 ID。与配置里的 `allow_from` 条目**精确匹配**（无通配，
`"*"` 不允许）。

形状过滤器（发布前本地丢弃，白名单之外的第二道闸）：

- **bot 作者直接丢**（echo guard）：网关会把 bot 自己发出的消息
  （以及其他 bot 的消息）也推成 `MESSAGE_CREATE`，不丢会自激循环
- 指令类型消息（`CHAT_INPUT_COMMAND` 20 / `CONTEXT_MENU_COMMAND` 23）、
  系统消息（入服提示、pin 提示等）丢
- 空 content（纯图片/附件/embed/sticker）丢——媒体收是后切
- 缺 sender / message id / channel id 的丢（Host dispatch 反正会丢）

**不去重**：Discord 网关**不重投**已下发的事件（与 QQ 不同）。RESUME
只补发"最后**已接收**序列号之后"的事件，而 discordgo 在收到帧时就
推进序列号（先于分发），本进程已处理过的帧不会被重放；进程重启后
是新会话（fresh identify），无从重放。所以本插件没有 QQ 那样的
msg_id 去重栅栏。

## 生命周期：不用 discordgo 自带的重连（源码结论）

读 discordgo v0.29.0 源码（`wsapi.go`）得出的三个事实，决定了本插件
的生命周期形态：

1. **`Open()` 是同步的全握手**：REST 取 gateway 地址 → WebSocket 拨号
   → HELLO → IDENTIFY → 自己读回 READY 帧。`Open` 返回 nil 就代表
   网关接受了会话——token 被拒、intent 被禁（4014）都在这里暴露，
   Start 天然 fail-closed（首次尝试未过握手不报 started）。
2. **自带重连关不掉也停不掉**：`ShouldReconnectOnError=true`（默认）
   时 `reconnect()` 是一个**无限循环**（退避封顶 600s），而 `Close()`
   **不会**让它停下来（v0.29 没有内部标志位翻转）——Stop 之后耳朵会
   自己复活。因此插件**每个监督尝试新建一个会话**，并把
   `ShouldReconnectOnError` 钉成 `false`（同时
   `ShouldReconnectVoiceOnSessionError=false`，语音复活路径一并封死），
   由插件自己的、感知 ctx 的监督循环重拨——与 dingtalk/feishu 禁用
   SDK 自动重连是同一个取舍。
3. **死亡信号**：sdk 的每一条断连路径（读循环出错、心跳失败、网关
   op7）都会先 `Close()` 再（no-op 的）`reconnect()`，而 `Close` 会
   派发合成的 **`DISCONNECT` 事件**。SDK 重连关闭后，这个事件就是
   监督循环等待的死亡信号；自然死亡时socket 由 discordgo 自己的
   goroutine 收尾，监督循环只负责换新会话。`Stop` 只 latch + cancel，
   永远不亲自 `Close`（拨号挂着时 `Open` 持有会话互斥量，Stop 侧
   Close 会卡住——由监督循环收尾，Stop 有界返回）。

**代价（有意偏差，记录在案）**：网关 session id 与序列号在 v0.29 里
是未导出字段，新会话带不进旧会话的 RESUME 状态——每次重拨都是全新
IDENTIFY，**重拨间隙里别人发的话会丢**（间隙由重拨延迟限定）。可停
止性 > 续传，这是和 qq/feishu 一致的取舍。发送路径不受影响（REST
发送客户端在 Start 时一次性建好、从不 Open，与耳朵分开——qq 的
api/ear 分离模式）。

## 配置示例（信封固定，settings 不透明）

```yaml
channels:
  discord:
    enabled: true
    allow_from: ["discord:123456789012345678"]
    token_env: DISCORD_BOT_TOKEN       # 信封侧声明（Host 钉 Secret 用）
    settings:                          # 内核对这块不透明；由本插件解码
      token_env: DISCORD_BOT_TOKEN     # 插件侧同名（C3/C4 钉定：必须一致）
```

密钥只经环境变量（D-010，值永不出现在配置、日志、事件 payload 中）：

```text
export DISCORD_BOT_TOKEN=MTxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

`settings` 未知字段会被**拒绝**——本代 lib 不认识的键要升版本重新
pack，不能靠配置硬塞。

## 消息长度

manifest `max_message_runes: 2000`：Discord 单条消息 content 上限就是
2000 字符，与 picoclaw 参考实现的 `WithMaxMessageLength(2000)` 一致。

## 打包与验证

```text
vivy-sdk verify plugins/discord           # 静态规则 + 可链接性
vivy-sdk pack --with discord --out dist/  # 产出候选 EXE（链接 discordgo）
vivy-sdk inspect-artifact dist/<gen>/     # recipes.plugins 含 discord
```

独立 go.mod（`example.com/vivy/plugins/discord`）是硬要求：默认
`just ci` 与物种 `go build ./cmd/vivy` 的 import 图到不了
`github.com/bwmarrin/discordgo`——只有 pack 出来的那一代身体里
有耳朵。

## 模块依赖

本模块只允许 import：`agent-vivy/sdk/plugin` + 标准库 +
`github.com/bwmarrin/discordgo`（其 go.mod 传递依赖 gorilla/websocket、
x/crypto，不出现在业务代码 import 之外）。禁止 import
`agent-vivy/internal/...`、eino、**pion 全家**、picoclaw 或
`.workspace`；禁止 `net.Listen`；禁止 `init()` blank import。

## SDK 版本与偏差

钉的是**上游** `github.com/bwmarrin/discordgo v0.29.0`——与 picoclaw
同版本但**不带其 fork replace**。有意的偏差（均记录在上文与包注释）：

1. **不用其自带重连循环**（`ShouldReconnectOnError=false`）：源码层面
   它是无限循环且 `Close` 停不掉，停止后耳朵会复活；
2. **无 RESUME 续传**：session id / sequence 未导出，每刀重拨都是全新
   IDENTIFY，重拨间隙的事件丢失（有界）；
3. **slash/interaction 不做**：只注册 `MESSAGE_CREATE`；语音/媒体/
   embed/reaction/typing/edit 皆不在本刀。
