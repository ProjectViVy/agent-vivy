# CH-C2 — acceptance（人怎么看出它成了）

日期：2026-08-30。

## 产品视角：身体没变，但作者世界多了一条缝

- 日常 `vivy.exe` / `http://127.0.0.1:3015` **零变化**：默认身体 `Register()` 仍空，没有耳朵，没有新配置必需项。
- 变化在**作者与打包体验**：`seam: channel` 成为插件清单的一等公民。

## 人可以亲手验证的点

1. **门是绿的**：仓库根 `just ci` 退出码 0。
2. **写一个 channel 插件能被接受**：`go run ./sdk verify sdk/internal/testdata/fake-channel` → `ok`。fake-channel 是自带独立 `go.mod` 的最小样板（Start/Stop/Send/PublishInbound，零 tool，grants 只有 `channel.poll` + `secret.read`）。
3. **认错 Consumer 会被打回**：
   - 清单 `seam: channel` 还带 `tools` → verify 失败：「seam channel forbids tools」
   - 源码里 `net.Listen` → verify 失败：「opens a listen socket (Listen belongs to the kernel ChannelHost)」
   - 清单领 `channel.webhook` grant → verify 失败：「not allowed in this batch」
4. **channel 永远不进工具表**：`pluginhost.Adapt` 对 seam:channel 返回 0 个 tool（测试桩刻意带一个 tool 证明跳过是显式的）。
5. **独立 go.mod 插件能被打包**：`pack --with sdk/internal/testdata/fake-channel` 产出可构建 generation；真实树的 `go.mod` 与 `zz_register.go` 字节不变（测试断言）。
6. **配置长出了信封但不长器官**：`config.yaml` 写 `channels: {telegram: {enabled, allow_from, token_env, settings}}` 能通过解析与结构校验；`token_env` 写小写、`allow_from` 写 `*` 会被拒；`settings` 里写未来字段不报错（内核对它不透明）。名字对不上这一代身体的校验在 C3 Host 落地。

## 明确不属于本刀的验收（勿在此追讨）

- 耳朵能收发消息 → CH-C3（Host + 假插件闭环）之后。
- 设置页/inspect 看见通道 → CH-C5。
- 真实 Telegram/钉钉 → CH-C4/C6。

## 回滚

revert 本分支即可：默认身体零行为变化，风险面限 SDK 新类型与 config 新 section（均可选使用）。
