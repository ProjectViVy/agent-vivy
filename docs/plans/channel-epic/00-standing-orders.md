# 站立命令 — 超级通道全体切片

子 AGENT 开工前必读。违反即停，不要靠评审事后纠正。

## 权威

合同 `VIVY-CHANNEL-PACK.md` > 演进 `VIVY-CHANNEL-EVOLUTION.md` > 本切片 PLAN > `docs/TODO.md` §0.2 日历。

日历不改合同。PLAN 不发明第二套循环。发现合同漏洞：写入 `docs/TODO.md` §0.1，不要擅自扩缝。

## 车道

- 根树脏或已有写 lane：`git worktree add ../agent-vivy-<id> -b feat/channel-<id>`。
- 文档包在 `feat/channel-super-contract`。**实现切片不得往该分支堆代码。**
- 并行 C4∥C6 必须两棵 worktree。第三只真 SDK 等 C4 合入后再开（ABI 样板）。

## Eino（D-007）

- 只有 `internal/runtime` 与 `internal/provider` 可 import `github.com/cloudwego/eino*`。
- `internal/channelhost`、`sdk/plugin`、全部 `plugins/<channel>` **零 Eino import**。
- 禁止 Host 或插件 `adk.NewRunner`。唯一循环是 `runtime.Service.Run`。
- 后切 A2A 才允许独立 `plugins/a2a` 依赖 `eino-ext/a2a` 的 **models/transport**。禁止 `RegisterServerHandlers(adk.Agent)`。
- `AgentAsTool` / DeepAgent 不是本 EPIC。

## 插件

- 作者只 import `agent-vivy/sdk/plugin`。禁止 `import agent-vivy/internal/...`。
- 禁止 `net.Listen` / `http.ListenAndServe`。Listen 是 Host 的。
- 禁止把 telego / discordgo / lark / 钉钉 / botgo 写入物种默认 `go.mod`。
- 默认提交的 `internal/generated/plugins/zz_register.go` 必须保持 `return nil`。
- `pluginhost.Adapt` 不得把 `seam: channel` 变成 `tools.Tool`。

## picoclaw 对照（正式做通道时必读）

五个真实适配器（CH-C4 / C6 / C7a / C7b / C7c）以 **picoclaw 的 channel 实现为最完整的 Go 样本**。开工前先读对应包，再改写进 `plugins/<name>/`。

- 只读。禁止 `import` picoclaw 模块，禁止把 `.workspace` 写进 `go.mod`。
- 偷：`Start` / `Stop` / `Send`、InboundContext / SenderInfo、错误分类、该平台 token 用法。
- 不偷：`init()` blank import 进网关、空 `allow_from` 放行、插件自建 `net.Listen`、内核里的 `TelegramSettings` 一类类型。
- 路径（按存在选用）：仓库 `.workspace/picoclaw/pkg/channels/<name>`；本机 `C:\Users\Administrator\Desktop\morediva\.workspace\picoclaw\pkg\channels\<name>`。
- Discord **不要**移植 `voice.go` / pion。钉钉走 Stream，不要倒退成 webhook 文本机器人。QQ 是官方 Bot，不是个人号 / OneBot。

## 循环与账本

- 入站只走 `Env.PublishInbound` → ChannelHost → `channel.inbound` → `Message(source=channel)` → `Service.Run`。
- 密钥只经 `token_env` / `*_env`。值不进配置、不进 Journal、不进事件 payload。
- 空 `allow_from` = 拒绝 Start。禁止 `"*"`。

## 验证与提交

- 交付前根（或该 worktree）跑 `just ci`。
- 用户可见面走 `http://127.0.0.1:3015`，不是嵌入 UI `:8787`。
- 写 `docs/logs/YYYY-MM-DD-<id>/{summary,verification,acceptance}.md`。
- 一个切片一个主题 commit。不 push，除非用户明示。
- TODO 该行标 DONE 并指向 log。在本切片 PLAN §10 勾交接。

## 语言

代码标识英语。用户文案中文。iteration log 中文。
