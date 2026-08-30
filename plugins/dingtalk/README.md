# plugins/dingtalk — Vivy 的钉钉耳朵（单聊文本，Stream 模式）

`dingtalk` 是一个 seam-channel 插件（VIVY-CHANNEL-PACK.md §9），由内核
ChannelHost 启动和消费，不是模型工具。目录结构和 Start/Stop/Send 骨架
沿用 `plugins/telegram` 钉下的形状模板。

## 第一刀范围（做什么 / 不做什么）

**做：**

- 单聊（`conversationType == "1"`）**纯文本**收 / 发
- 传输：**出站 WebSocket**（DingTalk Stream 网关，manifest
  `transport: "poll"` + grant `channel.poll`），无 webhook、无监听端口
- 忽略群聊、卡片 / 非文本消息、机器人自己的消息（防回声循环）
- 会话 webhook（DingTalk 按条下发的回复 URL）作为插件侧运行时状态按
  conversationId 保存（最新一条生效），回复时经 `ChannelEnv.HTTP()`
  POST 文本消息并校验 `{errcode,errmsg}`

**不做（后切）：** 群触发、卡片与互动媒体、markdown / actionCard 全套、
webhook 模式、回复线程与话题。

## 权限与策略边界

| 事项 | 归属 |
|---|---|
| `allow_from` 发信人白名单 | **Host（内核）** 强制，插件不做自己的白名单 |
| 密钥解析 | Host 的 `Secret`；本插件需要 **两把** 凭据（app key + app secret），信封单个 `token_env` 装不下，改由 settings 顶层 `*_env` 键声明名字（CH-C6/D2），Host 予以放行 |
| channel settings | 插件严格解码（未知字段 fail-closed），内核不认识 `settings` 里的键 |
| 监听端口 | 没有。Stream 连接是纯出站 WebSocket；SDK 自带的"永久重连"被关闭，由插件自己的、感知 ctx 的监督循环替代（Stop 后绝不复活） |

发信人写成 `dingtalk:<senderStaffId 或 senderId>`，与配置里的
`allow_from` 条目**精确匹配**（无通配，`"*"` 不允许）。

## 会话 webhook（sessionWebhook）

DingTalk 不给机器人"按会话发消息"的 API——每条进来的消息附带一个
有时效的 `sessionWebhook`，回复只能 POST 到它。因此：

- 它是**插件侧运行时状态**：只存在内存里，按 conversationId 存最新一条，
  重启即清空；
- 它**绝不进入**内核配置、配置信封或日志（D-010：URL 查询串里带会话
  token，出站错误会被裁掉查询串再进错误链）；
- 对没有 webhook 的会话 `Send` **失败闭合**——进程重启后对方必须先给
  机器人发一条消息，Vivy 才能回话。

## 配置示例（信封固定，settings 不透明）

```yaml
channels:
  dingtalk:
    enabled: true
    allow_from: ["dingtalk:manager1234"]
    # token_env 可以为空：凭据名字全部由 settings 声明（见下）
    settings:                    # 内核对这块不透明；由本插件解码
      client_id_env: DINGTALK_CLIENT_ID        # 必填：app key 的环境变量名
      client_secret_env: DINGTALK_CLIENT_SECRET # 必填：app secret 的环境变量名
      # open_api_host: "https://api.dingtalk.com"  # 可选：网关覆盖（测试 / 专有部署）
```

密钥只经环境变量（D-010，值永不出现在配置、日志、事件 payload 中）：

```text
export DINGTALK_CLIENT_ID=dingxxxxxxxxxxxx
export DINGTALK_CLIENT_SECRET=xxxxxxxxxxxxxxxxxxxxxxxxx
```

`settings` 未知字段会被**拒绝**——本代 lib 不认识的键要升版本重新
pack，不能靠配置硬塞。

## 打包与验证

```text
vivy-sdk verify plugins/dingtalk          # 静态规则 + 可链接性
vivy-sdk pack --with dingtalk --out dist/ # 产出候选 EXE（链接 dingtalk-stream-sdk-go）
vivy-sdk inspect-artifact dist/<gen>/     # recipes.plugins 含 dingtalk
```

独立 go.mod（`example.com/vivy/plugins/dingtalk`）是硬要求：默认
`just ci` 与物种 `go build ./cmd/vivy` 的 import 图到不了
`github.com/open-dingtalk/dingtalk-stream-sdk-go`——只有 pack 出来的那
一代身体里有耳朵。

## 模块依赖

本模块只允许 import：`agent-vivy/sdk/plugin` + 标准库 +
`github.com/open-dingtalk/dingtalk-stream-sdk-go`。禁止 import
`agent-vivy/internal/...`、eino、picoclaw 或 `.workspace`；禁止
`net.Listen`；禁止 `init()` blank import。
