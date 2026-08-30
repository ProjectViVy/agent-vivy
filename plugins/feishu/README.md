# plugins/feishu — Vivy 的飞书 / Lark 耳朵（单聊文本，WebSocket 长连接）

`feishu` 是一个 seam-channel 插件（VIVY-CHANNEL-PACK.md §9），由内核
ChannelHost 启动和消费，不是模型工具。目录结构和 Start/Stop/Send 骨架
沿用 `plugins/telegram` 钉下的形状模板，生命周期沿用 `plugins/dingtalk`
的监督重拨模式。

## 第一刀范围（做什么 / 不做什么）

**做：**

- 单聊（`chat_type == "p2p"`）**纯文本**收 / 发
- 传输：**出站 WebSocket 长连接**（飞书事件网关，manifest
  `transport: "poll"` + grant `channel.poll`）。文档写的是 webhook
  回调，但本实现是长连接——插件从不监听端口，从不实现 webhook
  （合同 §14.3：WS 计入 poll）
- 忽略群聊、富文本 / 卡片 / 图片等非文本消息、机器人（`sender_type ==
  "bot"`）自己的消息（防回声循环）；表情回复（reaction）是独立事件
  类型，根本没有注册，天然不进耳朵
- 回复经 OpenAPI `im.v1.messages`（`receive_id_type=chat_id`、
  `msg_type=text`、`content={"text":...}`）发送，收集平台返回的
  `message_id`；tenant_access_token 由 SDK 从 app 凭据自行管理，插件
  不碰任何 token

**不做（后切）：** 群触发、富文本 / 互动卡片、媒体、表情回复、回复
线程与话题、webhook 模式、markdown 全套。

## 权限与策略边界

| 事项 | 归属 |
|---|---|
| `allow_from` 发信人白名单 | **Host（内核）** 强制，插件不做自己的白名单 |
| 密钥解析 | Host 的 `Secret`；本插件需要 **两把** 凭据（app id + app secret），信封单个 `token_env` 装不下，改由 settings 顶层 `app_id_env` / `app_secret_env` 键声明名字（CH-C6/D2 模式），Host 予以放行 |
| channel settings | 插件严格解码（未知字段 fail-closed），内核不认识 `settings` 里的键 |
| 监听端口 | 没有。长连接是纯出站 WebSocket；SDK 自带的自动重连被关闭，由插件自己的、感知 ctx 的监督循环替代（每次重拨换新 client，Stop 后绝不复活，无 goroutine 泄漏） |

发信人写成 `feishu:<open_id>`（open_id 优先，缺失时依次回退
user_id、union_id），与配置里的 `allow_from` 条目**精确匹配**
（无通配，`"*"` 不允许）。

## encrypt_key 与 verification_token

- `settings.encrypt_key` 按 §14.3 是**普通 settings 值**（Host 不解码
  它）。WebSocket 长连接推送的事件本身是明文，SDK 的 WS 分发路径不做
  解密；该键按合同携带并转交 SDK 事件分发器（webhook 模式的解密在
  WS 模式下用不上），属合同完备性。
- **没有 `verification_token`**：URL challenge 校验是 webhook 模式的
  握手，长连接模式不涉及；picoclaw 传了该键但对其 WS 流程同样是惰性
  的，本插件从 settings 面上剔除。

## is_lark 开关

`settings.is_lark: true` 把网关与 OpenAPI 域名从飞书
（`https://open.feishu.cn`）切到国际版 Lark
（`https://open.larksuite.com`）。`settings.open_base_url` 可整体覆盖
域名（测试打环回桩 / 专有网关），优先级最高——沿用 dingtalk
`open_api_host` 的先例。

## 64 位约束

本插件按 **64 位平台编译**（amd64 / arm64）。lark SDK 的依赖树在
386 目标上不保证可用——**编译目标为 386 时会失败，这是硬约束不是
bug**；不要为此添加 386 兼容 shim。

## 配置示例（信封固定，settings 不透明）

```yaml
channels:
  feishu:
    enabled: true
    allow_from: ["feishu:ou_xxxxxxxxxxxxxxxx"]
    # token_env 可以为空：凭据名字全部由 settings 声明（见下）
    settings:                    # 内核对这块不透明；由本插件解码
      app_id_env: FEISHU_APP_ID          # 必填：app id 的环境变量名
      app_secret_env: FEISHU_APP_SECRET  # 必填：app secret 的环境变量名（须与上面不同）
      # is_lark: false                   # 可选：true = 国际版 Lark 域名
      # encrypt_key: "..."               # 可选：普通 settings 值（Host 不解码；WS 模式不解密）
      # open_base_url: "https://open.feishu.cn"  # 可选：域名覆盖（测试 / 专有部署）
```

密钥只经环境变量（D-010，值永不出现在配置、日志、事件 payload 中）：

```text
export FEISHU_APP_ID=cli_xxxxxxxxxxxx
export FEISHU_APP_SECRET=xxxxxxxxxxxxxxxxxxxxxxxxx
```

`settings` 未知字段会被**拒绝**——本代 lib 不认识的键要升版本重新
pack，不能靠配置硬塞。

## 打包与验证

```text
vivy-sdk verify plugins/feishu          # 静态规则 + 可链接性
vivy-sdk pack --with feishu --out dist/ # 产出候选 EXE（链接 oapi-sdk-go）
vivy-sdk inspect-artifact dist/<gen>/   # recipes.plugins 含 feishu
```

独立 go.mod（`example.com/vivy/plugins/feishu`）是硬要求：默认
`just ci` 与物种 `go build ./cmd/vivy` 的 import 图到不了
`github.com/larksuite/oapi-sdk-go`——只有 pack 出来的那一代身体里有
耳朵。

## 模块依赖

本模块只允许 import：`agent-vivy/sdk/plugin` + 标准库 +
`github.com/larksuite/oapi-sdk-go/v3`。禁止 import
`agent-vivy/internal/...`、eino、picoclaw 或 `.workspace`；禁止
`net.Listen`；禁止 `init()` blank import。

## SDK 版本

钉的是 `github.com/larksuite/oapi-sdk-go/v3 v3.11.0`（v3 系列 WS 客户
端重写版：`Start` 感知 ctx 且会返回、worker 由 WaitGroup 收拢、停跑后
client 进入 terminal 态）。参考实现 picoclaw 钉的 v3.9.4 的 WS 客户端
`Start` 以 `select{}` 永久阻塞、且每次成功 Start 泄漏一个永不退出的
pingLoop goroutine，与本插件的监督生命周期、无泄漏硬要求冲突，故取
满足"具备 WS 支持的最近稳定版"的 v3.11.0。
