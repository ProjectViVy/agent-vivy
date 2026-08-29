# CH-C2 — SDK `seam: channel` + 空注册表 + 信封配置（summary）

日期：2026-08-30。分支 `feat/channel-c2`（自 `feat/channel-c1` c811d9f 切出，独立 worktree）。
PLAN：`docs/plans/channel-epic/CH-C2.md`。合同：`VIVY-CHANNEL-PACK.md` §4/§8/§9/§10/§11。

## 做了什么

物种窗口开了 `seam: channel` 这条缝：作者能写 channel 插件、verify 接受、pack 能把**自带 go.mod** 的插件 overlay 进 `Register()`——但默认身体仍 `Register() = nil`，还没有 Host，跑不起来。

1. **sdk/plugin**：`SeamChannel`；五个 Grant（`channel.poll` / `channel.webhook` / `channel.listen` / `channel.a2a` / `secret.read`，词表全收，按 seam 的限制归 verify）。新文件 `sdk/plugin/channel.go`：合同 §9.3 的 `Channel` / `ChannelEnv` 接口逐符号落地（`Send` 返回 `([]string, error)`；`HTTP()` 出站 client 无 Listen；`Secret` fail-closed）；类型化信封 `InboundMessage` / `OutboundMessage` / `Part`（text / media-ref / structured；**禁止 map 当主合同**）；九个预留能力槽（MediaStore、Typing、MessageEditor、Placeholder、MediaSender、WebhookHandler、StreamingCapable、TaskLifecycle、PipeServer，零方法 + 保留注释，C3 Host 才断言）。
2. **verify**：按 seam 分流。`seam: channel` ⇒ 禁 `tools`、grants ⊆ {channel.poll, secret.read}（webhook/listen/a2a 本批拒绝）、必须带 `channel` 对象且 `transport: "poll"`；非 channel seam 禁领 channel 族 grant。AST 层新增 `net.Listen` / `http.ListenAndServe[+TLS]` 封禁（Listen 是 Host 的）；eino / internal / os.Open / exec 封禁对 channel 继续生效。新夹具 `bad-channel-tools` / `bad-channel-listen` / `bad-channel-grant`，正向夹具 `TestVerifyFakeChannel`。
3. **pack 独立 module**：`--with` 的插件若自带 go.mod，解析其 module 路径作为生成 `Register()` 的 import path；构建用**双文件 overlay**（生成的 zz_register.go + 根 go.mod 副本追加 `require <mod> v0.0.0` + `replace <mod> => <abs>`），真实树零写入。`TestPackFakeChannelStandaloneModule` 跑真实 `go build` 并断言 live `zz_register.go` **和** live `go.mod` 字节不变。
4. **pluginhost**：`Adapt` 显式跳过 `SeamChannel`（测试用带非空 Tools() 的 channel 桩证明不是「碰巧为空」）——channel 永远不进工具表。
5. **config**：`channels:` 信封骨架（map<名字> → `{enabled, allow_from, token_env, settings}`，settings 为 `yaml.Node` 不透明）；结构校验：名字 slug、`token_env` 匹配 env-key 模式（D-010 文案）、`allow_from` 条目非空且禁 `*`；空 `allow_from` 允许入配置（拒 Start 是 C3 Host 的事）；settings 内部未知键**不**报错（有测试钉住 strict 解码与 opaque 的交互）。未知名字对照 `Register()` 的校验按 PLAN 留给 C3。
6. **PLUGIN-SPEC §4 符号级对齐**（CH-C2 §5 授权范围内，合同语义零改动）：Grant 常量 + Channel/ChannelEnv 签名 + 类型化信封一句话。

## 明确没做（不做声明）

- 无 `internal/channelhost`、无 Session 映射、无 `Service.Run` 接线（全部 C3）。
- verify 无法类型检查「必须实现 Channel」（AST/清单层面做不到）——C3 Host 断言兜底，已登记 §0.1。
- `inspect` / `generation.json` 按 seam 分类列出（name/version/seam/grants/transport/…）未做，后续切片。
- 真实平台 SDK 未引入；独立 module 的传递依赖闭包（require/go.sum 合入 overlay）留 C4 真包踩实。
- `zz_register.go` 提交态仍 `return nil`；`go.mod` / `go.sum` 零改动；未 push。
