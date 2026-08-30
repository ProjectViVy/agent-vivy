# 2026-08-30 — plugins/feishu: 飞书/Lark 单聊文本通道（CH-C7a）

## 变更范围

新增 `plugins/feishu/`（独立模块 `example.com/vivy/plugins/feishu`）：
seam-channel 适配器，p2p（单聊）纯文本收 / 发，传输为飞书事件网关的
**出站 WebSocket 长连接**（manifest `transport: "poll"`，grant
`channel.poll`），无 webhook、无监听端口。

- `plugin.go` — Start/Stop/Send 骨架（telegram/dingtalk 形状模板）；
  监督重拨循环（SDK 自动重连关闭，每次重拨换新 client，首连 fail-closed，
  Stop 后不复活、无 goroutine 泄漏）；晚到事件围栏（stopped latch）；
  `im.message.receive_v1` 事件归一化（仅 p2p + text + 人类 sender；
  群聊 / 非文本 / bot 回声 / 空文本全部本地丢弃）；回复走
  `im.v1.messages`（`receive_id_type=chat_id`），tenant_access_token
  全程由 SDK 管理。
- `settings.go` — 严格解码（未知字段 fail-closed）：`app_id_env` /
  `app_secret_env`（必填且须不同，CH-C6/D2 `*_env` 模式）、
  `encrypt_key`（普通 settings 值，§14.3，Host 不解码）、`is_lark`
  （feishu↔lark 域名切换）、`open_base_url`（环回测试 / 专有部署覆盖）。
  **没有 `verification_token`**——URL challenge 是 webhook 模式的握手，
  长连接模式不涉及。
- `vivy-plugin.json` — name `feishu`，seam `channel`，grants
  `[channel.poll, secret.read]`。
- `plugin_test.go` — 网络全free（仅环回 httptest）：settings 矩阵、
  域名解析矩阵、事件归一化矩阵、Start fail-closed 矩阵（t.Setenv 植
  /清凭据）、Send 全链路（真 SDK client 打环回桩，断言 token 端点与
  message 端点的 URL / query / body）、API 错误上浮、**真 WS 循环回**
  （SDK 自身 Frame codec 构帧，走真 dispatcher → PublishInbound →
  Send → ack 断言 → Stop 后零重拨）、断线重拨、Stop 幂等。
- `README.md` — zh，镜像 telegram/dingtalk 结构。

## SDK 版本决策（偏离说明）

钉 `github.com/larksuite/oapi-sdk-go/v3 v3.11.0`（v3 系列 WS 客户端重
写版），**不是** picoclaw go.mod 钉的 v3.9.4。原因：v3.9.4 的 WS
`Start` 以 `select{}` 永久阻塞、每次成功 Start 泄漏一个永不退出的
pingLoop goroutine、WS bootstrap 无法注入 http client——与"监督生命
周期 / 无 goroutine 泄漏 / 环回测试"硬要求冲突。v3.11.0 的 `Start`
感知 ctx 且会返回、worker 由 WaitGroup 收拢、停跑后 client 进入
terminal 态（每次重拨必须换新 client，本插件如此实现）。

## 明确未做（后切）

群触发、富文本 / 互动卡片、媒体、表情回复（reaction）、回复线程与
话题、webhook 模式、markdown 全套。386 目标不支持（lark SDK 依赖树
在 386 上编译失败，`math.MaxInt64` 溢出——硬约束，非 bug，README 已
写明）。

## 未包含在本切片

- `docs/TODO.md` §0.1 捕获与提交（commit）由协调方统一处理；本工作树
  未建分支、未提交。

## GOAL 持有人落地补记（2026-08-30）

- 评审：独立 reviewer **PASS**；SDK 版本偏离（v3.11.0 vs picoclaw v3.9.4）经 SDK 源码核实为必要（旧版 WS `Start` 以 `select{}` 永不返回、pingLoop 不可退出、无 HTTP 注入口）。`just ci` exit 0。
- 登记项：`supervise` 首连与 Stop 重叠的三条早退路径不保证送达 `firstErr`（经 Host 实际调用序不可达：Host 只 Stop 已完成 Start 的耳；调用方 ctx 取消亦能解锁）——按站立命令记 `docs/TODO.md` §0.1 CH-C7a-N1，不改动已测生命周期代码。
- 目录名归一为 `2026-08-30-channel-c7a`（原 feishu-channel）。
