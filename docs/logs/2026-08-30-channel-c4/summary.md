# CH-C4 summary — `plugins/telegram` 私聊文本（ABI 样板）

日期：2026-08-30　分支：`feat/channel-c4`（基于 `feat/channel-c3` @ `3ae5f17`）

## 做了什么

### 新增 `plugins/telegram/`（独立 Go 模块，第一只真耳朵）

- `go.mod`：`module example.com/vivy/plugins/telegram`；直接依赖仅
  `agent-vivy v0.0.0`（replace `../..`）+ `github.com/mymmrac/telego v1.10.0`
  （与 picoclaw 同代，long-poll API 稳定）；`go.sum` 由 `go mod tidy` 生成。
- `vivy-plugin.json`：seam `channel`，grants `["channel.poll","secret.read"]`，
  `channel.transport: "poll"`，`max_message_runes: 4096`，无 tools。
- `plugin.go`：`Plugin` 同时实现 `plugin.Plugin` 与 `plugin.Channel`
  （编译期断言）。
  - `Start`：严格解码 settings（fail-closed）→ `settings.token_env` 经
    `env.Secret` 解析（Host 已把 Secret 钉在信封 token_env 上，两处必须
    一致）→ `telego.NewBot`（可注入 HTTP client / base_url / proxy）→
    显式 `GetMe` 认证（telego v1.10 对 token 只做格式校验、getMe 懒执行，
    不强制认证会留下永久重试的聋耳朵）→ `UpdatesViaLongPolling`
    （`AllowedUpdates: ["message"]`，服务端过滤）→ poll goroutine 逐条
    normalize 后 `PublishInbound`。
  - 入站规范：只收「私聊 + 人类发信人 + 纯文本」。忽略 edited message /
    channel post / 群与超级群 / 无发信人 / bot 自己的消息（防回声循环）/
    `SenderChat` 代发 / 非文本。`Sender = "telegram:<from id>"`。
  - `Send`：逐 text part 明文 `sendMessage`（不设 parse_mode），返回平台
    message id；`ReplyTo/TopicID` 本刀不接；非文本 part 跳过；非数字
    ChatID fail-closed。
  - `Stop`：cancel 长轮询并等 poll goroutine 退出（受入参 ctx 约束）；幂
    等，未 Start 也安全。`bot/cancel/done` 在 Start 返回后不再改写
    （CH-C3-N2 的 Send/Stop 竞态面在适配器侧收口）。
  - 错误可见性：保留 telego 默认 logger（写 stderr 且自动脱敏 token）；
    `PublishInbound` 失败只丢弃不杀耳朵（Host 日志拥有审计面）。
- `settings.go`：`Settings{token_env, base_url, proxy}`，JSON 严格解码
  （DisallowUnknownFields + 拒绝尾随文档）；缺 settings 解码为零值，Start
  对缺 `token_env` fail-closed。
- `plugin_test.go`：全部走 `httptest` 回环假 Bot API，无真 Telegram 网络。
- `README.md`：中文产品文档（范围、策略边界、配置示例、打包方式、依赖纪律）。

### 内核通用信封硬化（`internal/channelhost`，仅通用面，无 TelegramSettings）

- **ABI 新增（本切片唯一 ABI 变更）**：`plugin.ChannelEnv` 增加
  `Settings() json.RawMessage`。Host 把信封 `channels.<name>.settings`
  （opaque `yaml.Node`）经 yaml→any→json 递归转换交给适配器；缺省或
  null → `{}`。合同 §11「settings 由该 channel 插件解码」此前缺少传递
  通道，这是补齐，不是新缝。
- `hostEnv.Secret` 钉死信封 `token_env`（CH-C3-N2）：空声明 → Secret 一律
  报错；名字不匹配 → 报错；错误信息只含名字与结果，不含值。
- 新增 `env_test.go` 覆盖以上两面。

### pack 修独立模块闭包（CH-C2 遗留缺口）

- 新增 `parseRequireLines`（单行 + 括号 require 块 + 注释 + `// indirect`
  保留）、`parseModulePath`；合并插件 require 闭包（跳过物种自身 require、
  同名同版本去重、版本漂移按 MVS 共存）与 go.sum 整行合并。
- **机制升级**：C2 的「go.mod 进 -overlay」方案对有第三方依赖的插件必然
  失败——readonly `go build` 要求 go.mod 与 MVS 结果完全一致（telego 闭包
  抬高 x/crypto 等间接版本），且实测 `go build -mod=mod` 拒绝更新被
  -overlay 的 go.mod。现改为 `go build -modfile <tmp>/pack.mod -mod=mod`：
  合并副本在临时目录，工具链在临时对里补齐并构建，真实 go.mod/go.sum
  零写入。`-overlay` 仍只用于 zz_register.go。

## 明确没做

- webhook、群触发、媒体、命令菜单、MarkdownV2/HTML、语音：全部未实现。
- `internal/config` 零改动（无 TelegramSettings）；`internal/runtime` 零触碰；
  物种 `go.mod`/`go.sum`/`zz_register.go` 字节不变。
- 回复线程、话题、出站 4096 rune 上限处理（TODO 新条目 CH-C4-N1）。
- verify 清单夹具（CH-C2-N1）：属 sdk 验证器面，本切片未动，保持 OPEN。
- 投递耐久化 / `chanin_*` 保留策略（CH-C3-N1 的 Host 侧剩余项）。
