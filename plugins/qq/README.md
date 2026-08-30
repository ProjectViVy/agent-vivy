# plugins/qq — Vivy 的 QQ 耳朵（QQ 开放平台官方机器人，单聊文本，WS 长连接）

`qq` 是一个 seam-channel 插件（VIVY-CHANNEL-PACK.md §9），由内核
ChannelHost 启动和消费，不是模型工具。目录结构和 Start/Stop/Send 骨架
沿用 `plugins/telegram` 钉下的形状模板，生命周期沿用
`plugins/dingtalk` / `plugins/feishu` 的监督重拨模式。

## **本插件只对接 QQ 开放平台官方机器人**

- **不是个人号**：不登录任何个人 QQ 账号，不模拟客户端协议，不需要
  扫码、密码或设备信息。唯一凭据是在 [q.qq.com](https://q.qq.com)
  开放平台控制台申请的机器人 **AppID + AppSecret**。
- **不是 OneBot**：不实现、不兼容 OneBot 11/12 协议，不连接任何
  OneBot 实现（go-cqhttp、Lagrange、LLOneBot 等），不做正反向 WS
  服务端/客户端协议。
- **不是 NapCat**：不依赖、不启动、不连接 NapCat 或任何第三方挂机
  框架，**不启动第二个进程**——耳朵就是本插件自身，跑在 Vivy 进程
  里。
- 传输是**官方事件网关的出站 WebSocket 长连接** + **官方 v2 消息
  API**（`/v2/users/{openid}/messages`），与官方文档一一对应。

代码与文档同样声明：见 `plugin.go` 包注释（THIS IS NOT A
PERSONAL-ACCOUNT BOT）。

## 第一刀范围（做什么 / 不做什么）

**做：**

- 单聊（`C2C_MESSAGE_CREATE`）**纯文本**收 / 发
- 传输：**出站 WebSocket**（官方事件网关，manifest `transport: "poll"`
  + grant `channel.poll`；WS 计入 poll，合同 §14.3），无 webhook、无
  监听端口
- 回复走官方被动回复 API：`POST /v2/users/{openid}/messages`，
  `msg_type=0`（文本）、`msg_id`（被动窗口）、`msg_seq`（同窗口内
  回复序号 1,2,3…），access_token 由 SDK 经 AppID/AppSecret 自管，
  插件不碰 token
- **重复投递去重**：官方文档明确警告相同 `msg_id` 可能重复推送；
  插件内存侧按 msg_id 做有界去重（TTL 5 分钟，硬上限 1 万条），
  重复事件不会开出第二个 run
- 断线**按 gateway 会话 resume 续传**（session id 取自 READY，seq
  取自已分发事件的序列号），避免重连后重复消费

**不做（后切）：**

- **群聊（`GROUP_AT_MESSAGE_CREATE`）暂不支持**，原因见下节
- 频道/Guild 全家桶、富媒体、markdown/ark 卡片、语音、按钮、
  webhook 模式

## 为什么这一刀没有群聊

官方群 @ 事件的群地址字段是 `group_openid`，但本插件钉住的 botgo
v0.2.1 的事件结构体（`dto.Message`）只解码 `group_id` 字段——真实
v2 群负载里根本没有 `group_id`，SDK 解出来恒为空（参考实现
picoclaw 也有同样的问题）。要"稳收群文本并干净回复"就得绕开 SDK
自己去解析原始 WS 帧，不符合本刀"官方 API 能稳收的文本才做"的
口径，故群聊显式留在范围外，等 botgo 补齐字段或升版本再评估。

（注：注册 C2C handler 会带出群/C2C 共用的 intent 位，网关仍可能
推送群事件帧；SDK 分发层没有注册群 handler，这些帧被静默丢弃。）

## 权限与策略边界

| 事项 | 归属 |
|---|---|
| `allow_from` 发信人白名单 | **Host（内核）** 强制，插件不做自己的白名单 |
| 密钥解析 | Host 的 `Secret`；本插件需要 **两把** 凭据（AppID + AppSecret），信封单个 `token_env` 装不下，改由 settings 顶层 `app_id_env` / `app_secret_env` 键声明名字（CH-C6/D2 模式），Host 予以放行 |
| channel settings | 插件严格解码（未知字段 fail-closed），内核不认识 `settings` 里的键 |
| 监听端口 | 没有。长连接是纯出站 WebSocket；botgo 自带的 session manager **不被使用**（见下节），由插件自己的、感知 ctx 的监督循环替代（Stop 后绝不复活，无 goroutine 泄漏） |

发信人写成 `qq:user_<openid>`（C2C 事件的 `author.id`，即该应用
视角的用户 openid，与 `author.user_openid` 同值），ChatID 即该
openid 本身。与配置里的 `allow_from` 条目**精确匹配**（无通配，
`"*"` 不允许）。注意：openid 是**按应用隔离**的，换机器人 AppID
后所有 openid 都会变。

## 被动回复窗口（为什么 Send 可能"失败闭合"）

QQ 开放平台对这类机器人**没有"按会话主动发消息"的常规 API**——
每条回复必须携带它所回应的那条入站消息的 `msg_id`（被动窗口约 60
分钟、每条 msg_id 最多回 4 条），同一窗口内的多条回复用递增
`msg_seq` 去重。因此：

- 插件按会话在**内存里**记最新一条入站 `msg_id` + 回复序号（最新
  生效），它绝不进入内核配置、配置信封或日志（D-010）；
- 对从未发过消息的会话（以及进程重启后的所有会话）`Send`
  **失败闭合**——对方必须先给机器人发一条消息，Vivy 才能回话。

## 生命周期：不用 botgo 的 session manager

botgo 内置的本地 session manager 是一个**无法停止**的后台循环：
`Start` 永久阻塞在内部重连队列上、不感知 ctx、没有停止手段——即使
机器人被封禁，它也只是在内层 recover 掉自己的 panic 后**无限静默
重试**，白白烧掉平台的建连配额。本插件改用 botgo 导出的**协议层
ws client**（`websocket.ClientImpl`），由插件自己的监督循环驱动——
一次尝试一个新 client：拨号 → identify/resume → 等 READY（首次
尝试未过握手不报 started，fail-closed）→ 存活期间挂起等待 → 断线
后按 resume 状态重拨。机器人被封禁/下架（cannot-identify 关闭码）
时**停止重拨**——比 botgo 的无限重试更严格也更安全（重拨永远不会
成功，只会消耗配额），耳朵保持"已启动但失聪"的终态，与兄弟插件
一致。

同理**不使用** `token.StartRefreshAccessToken`：它在一条裸
goroutine 里连续失败 11 次会 panic 且无人 recover，整个进程退出。
SDK 的 token source 本身是惰性取+缓存（按过期时间判定），WS 客户端
在鉴权失败关闭码下也会自行重取，够用且无崩溃风险。Start 仍会先做
一次同步取 token——凭据被拒时 fail-closed。

另外把 SDK 的默认日志器**静音**（`botgo.SetLogger`）：botgo 默认在
INFO 级打印每个 WS 帧和每个 HTTP 请求/响应体，其中包括 identify
payload 里的 **access token** 和用户**消息内容**——违反
`docs/architecture/LOGGING.md` 与 D-010。插件不能引
`internal/logging`，故仅保留 Error 级到 stderr。

## sandbox 开关

`settings.sandbox: true` 用 SDK 自带的
`botgo.NewSandboxOpenAPI`（`https://sandbox.api.sgroup.qq.com`），
网关地址经同一 client 发现，一并切换。默认 false（生产）。

## 配置示例（信封固定，settings 不透明）

```yaml
channels:
  qq:
    enabled: true
    allow_from: ["qq:user_A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3"]
    # token_env 可以为空：凭据名字全部由 settings 声明（见下）
    settings:                    # 内核对这块不透明；由本插件解码
      app_id_env: QQ_APP_ID              # 必填：AppID 的环境变量名
      app_secret_env: QQ_APP_SECRET      # 必填：AppSecret 的环境变量名（须与上面不同）
      # sandbox: false                   # 可选：true = 官方沙箱环境
```

密钥只经环境变量（D-010，值永不出现在配置、日志、事件 payload 中）：

```text
export QQ_APP_ID=123456789
export QQ_APP_SECRET=xxxxxxxxxxxxxxxxxxxxxxxx
```

`settings` 未知字段会被**拒绝**——本代 lib 不认识的键要升版本重新
pack，不能靠配置硬塞。

## 消息长度

manifest `max_message_runes: 2000`：v2 文本消息 content 上限为
7000 字节，2000 rune 在全 CJK 情形下 6000 字节，留了安全余量。

## 打包与验证

```text
vivy-sdk verify plugins/qq            # 静态规则 + 可链接性
vivy-sdk pack --with qq --out dist/   # 产出候选 EXE（链接 botgo）
vivy-sdk inspect-artifact dist/<gen>/ # recipes.plugins 含 qq
```

独立 go.mod（`example.com/vivy/plugins/qq`）是硬要求：默认
`just ci` 与物种 `go build ./cmd/vivy` 的 import 图到不了
`github.com/tencent-connect/botgo`——只有 pack 出来的那一代身体里
有耳朵。

## 模块依赖

本模块只允许 import：`agent-vivy/sdk/plugin` + 标准库 +
`github.com/tencent-connect/botgo`（及其 go.mod 传递依赖
oauth2/resty/gorilla 等，不直接出现在业务代码 import 之外）。禁止
import `agent-vivy/internal/...`、eino、picoclaw 或 `.workspace`；
禁止 `net.Listen`；禁止 `init()` blank import。

## SDK 版本

钉的是 `github.com/tencent-connect/botgo v0.2.1`——与参考实现
picoclaw 同版。三处有意的偏差（均记录在上文）：不用其内置
session manager（不可停、对封禁机器人无限静默重试）、不用其后台
token 刷新 goroutine（裸 goroutine 连续失败 panic 且无人 recover）、
群事件不做（DTO 字段名与官方 v2 负载不符）。
WS 协议本身（hello/心跳/identify/resume/关闭码）仍由 SDK 的 client
实现，本插件只做生命周期监督与事件规范化。
