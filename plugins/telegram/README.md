# plugins/telegram — Vivy 的第一只真耳朵（私聊文本）

`telegram` 是一个 seam-channel 插件（VIVY-CHANNEL-PACK.md §9 / §14.3），
由内核 ChannelHost 启动和消费，不是模型工具。它是后续 feishu / qq /
discord / dingtalk 适配器的**形状模板**——抄目录结构和 Start/Stop/Send
的骨架，不要另发明一套。

## 第一刀范围（做什么 / 不做什么）

**做：**

- 私聊（`chat.type == "private"`）**纯文本**收 / 发
- 出站 long-poll（`getUpdates`），无 webhook、无监听端口
- 平台侧过滤 `AllowedUpdates: ["message"]`：edited message、channel post
  等一律不进耳朵
- 忽略机器人自己的消息（防止回声循环）、代发消息、非文本消息

**不做（后切）：** 群触发、媒体、命令菜单、MarkdownV2 / HTML 全套、语音、
webhook、回复线程与话题。

## 权限与策略边界

| 事项 | 归属 |
|---|---|
| `allow_from` 发信人白名单 | **Host（内核）** 强制，插件不做自己的白名单 |
| `token_env` 密钥解析 | Host 把 `Secret` 钉在信封声明的 `token_env` 上；插件只能解析这个名字 |
| channel settings | 插件严格解码（未知字段 fail-closed），内核不认识 `settings` 里的键 |
| 监听端口 | 没有。long-poll 是纯出站连接 |

私聊发信人写成 `telegram:<数字用户ID>`，与配置里的 `allow_from` 条目
**精确匹配**（无通配，`"*"` 不允许）。

`token_env` 出现两次是**故意的**：配置信封里的 `channels.telegram.token_env`
是 Host 审计用的声明，`settings.token_env` 是插件真正去 `Secret` 解析的名字。
两处不一致时 `Secret` 直接失败闭合，Start 拒绝启动。

## 配置示例（信封固定，settings 不透明）

```yaml
channels:
  telegram:
    enabled: true
    allow_from: ["telegram:123456"]
    token_env: TELEGRAM_BOT_TOKEN
    settings:                    # 内核对这块不透明；由本插件解码
      token_env: TELEGRAM_BOT_TOKEN   # 必须与信封 token_env 一致
      # base_url: "http://127.0.0.1:8081"   # 可选：本地 Bot API sidecar
      # proxy: "http://127.0.0.1:7890"       # 可选：HTTP 代理
```

密钥只经环境变量：`export TELEGRAM_BOT_TOKEN=123456:AA...`（D-010，
token 永不出现在配置、日志、事件 payload 中）。

`settings` 未知字段会被**拒绝**——本代 lib 不认识的键要升版本重新 pack，
不能靠配置硬塞。

## 打包与验证

```text
vivy-sdk verify plugins/telegram          # 静态规则 + 可链接性
vivy-sdk pack --with telegram --out dist/ # 产出候选 EXE（链接 telego）
vivy-sdk inspect-artifact dist/<gen>/     # recipes.plugins 含 telegram
```

独立 go.mod（`example.com/vivy/plugins/telegram`）是硬要求：默认
`just ci` 与物种 `go build ./cmd/vivy` 的 import 图到不了
`github.com/mymmrac/telego`——只有 pack 出来的那一代身体里有耳朵。

## 模块依赖

本模块只允许 import：`agent-vivy/sdk/plugin` + 标准库 +
`github.com/mymmrac/telego`。禁止 import `agent-vivy/internal/...`、
eino、picoclaw 或 `.workspace`；禁止 `net.Listen`；禁止 `init()` blank import。
