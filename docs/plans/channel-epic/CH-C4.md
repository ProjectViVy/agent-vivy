# CH-C4 — `plugins/telegram` 私聊文本（ABI 样板）

## 1. 身份

| | |
|---|---|
| ID | CH-C4 |
| 阶段 | D ABI 样板 |
| 人日 | 2 |
| 里程碑 | M-CH2 |
| 依赖 | CH-C3 |
| 后继 | CH-C5；C6 可已并行 |
| 分支 | `feat/channel-c4` |
| 合同 | §9、§14.3 telegram、C4 |

这是第一只真耳朵。后续 feishu/qq/discord **抄这个包的形状**，不要各发明一套。

## 2. 目标

`plugins/telegram` 独立 go.mod。`vivy-sdk verify` + `pack --with telegram` 得到候选 EXE，能收发私聊文本。默认 `just ci` / 物种 `go.mod` **没有** `github.com/mymmrac/telego`。空 allow_from 仍拒绝。

## 3. 现状

- Host TCK 只认 fake channel。
- `plugins/` 只有 `hello-fs`（物种 module）。
- picoclaw 对照（只读）：`.workspace/picoclaw/pkg/channels/telegram` 或用户机 `C:\Users\Administrator\Desktop\morediva\.workspace\picoclaw`。
- 设置页仍是 localStorage（C5 才接）。

## 4. 目标结构

```text
plugins/telegram/
  go.mod                 module .../plugins/telegram
  vivy-plugin.json       seam: channel, grants: [channel.poll, secret.read]
  plugin.go              New() plugin.Plugin 且实现 Channel
  settings.go            解码 opaque yaml（proxy/base_url 可放这里）
  plugin_test.go         无真网络；假更新
  README.md
```

Transport：出站 long-poll。不实现 webhook。

## 5. 文件清单

**建** `plugins/telegram/**`

**改** pack 配方示例（文档或 testdata recipe，不要改默认 just run 配方）；C3 Host 若发现 telegram 特有坑，只许修 Host 的 **通用信封**，不许加 `TelegramSettings` 到 `internal/config`。

**禁止碰** 物种 `go.mod` 直接 require telego；`voice`；`internal/runtime/engine.go`。

## 6. 步骤

1. `go mod init` 独立模块；`replace` sdk/plugin 到仓库相对路径。
2. 对照 picoclaw **改写** Start/Stop/Send、SenderInfo、错误分类。不 import 其模块。
3. Start：读 `token_env` → `ChannelEnv.Secret`；poll 循环；入站 `PublishInbound`。
4. allow_from 由 Host 执行；插件不要自己放行空名单。
5. 单测：解码 settings；构造 inbound；无 live Telegram。
6. `vivy-sdk verify plugins/telegram`。
7. `vivy-sdk pack --with telegram`；`inspect-artifact` 含 telegram/telego；默认树 `go list` 不含 telego。
8. 手工冒烟（可选，不挡 ci）：真 Bot + 非空 allow_from。
9. `just ci`（默认路径）。
10. log `docs/logs/YYYY-MM-DD-channel-c4/`。

## 7. 验收

- 默认 `go test ./...` 不编译 `plugins/telegram` 的 telego 闭包，或 ci 明确排除该 module 的 vet/test 于物种 `./...`（独立 module 本就不在 `./...` 里——确认 `just ci` 的 `go test ./...` 不会 `-r` 进去）。
- pack 候选能链 telego。
- verify 拒绝清单带 tools。
- 空 allow_from 不能 Start（Host 行为，回归 C3 TCK + telegram 配置）。

## 8. 禁止

- webhook、群触发、媒体、命令菜单、MarkdownV2 全套。
- 把 telego 写进物种 go.mod。
- `init()` blank import。
- 插件 `net.Listen`。

## 9. 风险与回滚

- telego API 与 picoclaw 年代差：以能稳收私聊文本为准，不要追新。
- replace 路径在 pack 生成环境必须可解析。
- 回滚：配方不 --with telegram；默认身体无变化。

## 10. 交接

[CH-C5.md](CH-C5.md) 需要 compiled-in 名 `telegram`。C6/C7 抄本包目录形状。

> **DONE（2026-08-30）**：已交付，见 `docs/logs/2026-08-30-channel-c4/`。
> 交接要点：compiled-in 名 `telegram`；settings 经 `ChannelEnv.Settings()`
> （`json.RawMessage`，`{}` 表缺省）传给插件——本批唯一 ABI 新增；
> `token_env` 双处声明（信封 = 审计声明，settings = 插件解析名），
> Host 把 `Secret` 钉死到信封名字；pack 对独立模块走 `-modfile` 合并
> require/go.sum 闭包（`-mod=mod` 在临时对里完成合并），真实
> go.mod/go.sum 零写入。遗留：CH-C4-N1（4096 rune 出站上限无人执行）、
> CH-C4-N2（EnsureSession 并发测试）。
