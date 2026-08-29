# CH-C2 — SDK `seam: channel` + 空注册表 + 信封配置

## 1. 身份

| | |
|---|---|
| ID | CH-C2 |
| 阶段 | B 物种窗口 |
| 人日 | 2 |
| 里程碑 | M-CH1 |
| 依赖 | CH-C1 |
| 后继 | CH-C3 |
| 分支 | `feat/channel-c2` |
| 合同 | §4、§9、§10、C2；`VIVY-PLUGIN-SPEC.md` SeamChannel |

## 2. 目标

作者可以写一个 `seam: channel` 的 Go 包，被 `vivy-sdk verify` 接受，被 `pack` overlay 进 `Register()`，但默认身体仍然 `Register() = nil`。还没有 Host，所以还跑不起来。

## 3. 现状

- `sdk/plugin/plugin.go`：Seam 仅 tool / tool-world / provider；Grant 仅 fs.read/write；Plugin 必有 `Tools()`。
- `sdk/internal/verify.go`：禁 eino/internal/os.Open；无 channel 规则。
- `sdk/internal/pack.go`：按物种模块路径 import `plugins/<name>`。
- `internal/pluginhost/host.go`：`Adapt` 无条件遍历 `Tools()`。
- `internal/generated/plugins/zz_register.go`：`return nil`。
- `internal/config`：无 `channels:` 信封。

## 4. 目标结构

```text
sdk/plugin
  SeamChannel
  GrantChannelPoll / Webhook / Listen / A2A / GrantSecretRead
  Channel { Name Seam Grants Start Stop Send }
  ChannelEnv { Secret HTTP PublishInbound Media }
  InboundMessage / OutboundMessage / Part   # 槽第一刀定形，见合同 §8
  可选能力接口类型（空实现即可；C3 Host 才断言）

verify
  seam:channel → 零 tools、必须像 Channel、禁 Listen、禁 webhook/listen/a2a grant（本批）
  testdata/fake-channel/   独立小 module，无肥 SDK

pack
  --with 指向独立 go.mod 的插件时，Register() import 该 module
  默认 zz_register.go 仍 nil

pluginhost.Adapt
  SeamChannel → skip（不得变 tool）

config.channels
  信封：enabled, allow_from, token_env, settings（opaque）
  未知名字的完整启动失败可放到 C3；本切片至少 parse + 校验结构
```

`hello-fs` 的 Plugin 形状不拆。不要为 channel 先 churn tool ABI。`Register()` 仍返回 `[]plugin.Plugin`；channel 插件同时实现 `Plugin`（Tools 空）和 `Channel`。

## 5. 文件清单

**改/建**

- `sdk/plugin/plugin.go`（及拆文件若过大）
- `sdk/internal/verify.go` + `verify_test.go` + testdata
- `sdk/internal/pack.go` + `pack_test.go`
- `sdk/internal/testdata/fake-channel/`（独立 go.mod，无第三方 SDK）
- `internal/pluginhost/host.go` + 测试：channel 不出现在 tools 表
- `internal/config` 信封骨架 + parse 测试
- `VIVY-PLUGIN-SPEC.md` 若实现与草案有符号级对齐（不得改合同语义）

**禁止碰**

- `internal/channelhost`（还不存在；C3 才建）
- `plugins/telegram` 真包（C4）
- 物种 `go.mod` 加 telego
- `zz_register.go` 提交非空

## 6. 步骤

1. 扩 Seam/Grant；`Valid()` 覆盖新值。
2. 信封类型：parts 用结构体，禁止 `map[string]string` 当主合同。
3. `Channel` / `ChannelEnv` 接口。ChannelEnv.HTTP 是出站 client，无 Listen。
4. verify 分支：复制 `bad-eino-import` 风格加 `bad-channel-tools`、`bad-channel-listen`。
5. testdata fake-channel：独立 go.mod，`replace` 指向物种 sdk/plugin。
6. pack：能 overlay fake-channel；assert 生成文件 import 该 module 路径；跑完恢复或在 temp overlay 测，**不要提交非空 Register()**。
7. Adapt 跳过 channel。
8. config 信封 decode 测试。
9. `just ci`。
10. log `docs/logs/YYYY-MM-DD-channel-c2/`。

## 7. 验收

- `vivy-sdk verify sdk/internal/testdata/fake-channel` 通过。
- 带 tools 的 channel 清单失败。
- 含 `net.Listen` 的源失败。
- `just ci` 的 `go test ./...` import 图无 telego。
- 提交树 `zz_register.go` 仍 `return nil`。
- `pluginhost.Adapt([]Plugin{fakeChannel})` 长度为 0。

## 8. 禁止

- 实现 Host / Session 映射 / Service.Run 接线。
- 本批配方启用 `channel.webhook` / `listen` / `a2a` grant。
- 为独立 module 去改 ADR-015 的「空注册表」语义。

## 9. 风险与回滚

- pack 对独立 go.mod 比预估肥：本切片允许 **只打通 fake-channel module**；真 telegram module 留 C4。
- Plugin 同时 Tools()+Channel 可能让旧代码慌：Adapt skip 是硬验收。
- 回滚：revert；无运行时行为。

## 10. 交接

下一 AGENT：[CH-C3.md](CH-C3.md)。Host 消费本切片的 Channel/Env/信封符号，不要在 C3 改名。

> **DONE 2026-08-30** — 分支 `feat/channel-c2`；符号定形 `sdk/plugin/channel.go`（`Channel`/`ChannelEnv`/`InboundMessage`/`OutboundMessage`/`Part` + 9 保留能力槽）。C3 直接消费这些名字；需补 §8 的 run_id/task_id 槽与 Delete/Reaction/HealthChecker/ListenHandler（TODO §0.1 CH-C2-N2）。Filing: `docs/logs/2026-08-30-channel-c2/`。
