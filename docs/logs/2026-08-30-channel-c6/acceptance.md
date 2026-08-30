# CH-C6 — acceptance（人怎么看出它成了）

日期：2026-08-30。

## 产品视角：国内过夜耳朵第一只

钉钉用户给机器人发单聊文本 → Stream 长连接收下 → 账本入 `channel.inbound` → Run → 回复经 sessionWebhook 原路送回。全程不需要公网地址、不需要 webhook 服务器——这就是「国内过夜」的含义。

## 人可以亲手验证的点

1. **门是绿的**：`just ci` 退出码 0；默认 EXE 的依赖图里 grep 不到钉钉 SDK。
2. **打包即得**：`vivy-sdk verify plugins/dingtalk` → ok；`vivy-sdk pack --with dingtalk` → 候选 EXE（inspect 列出 dingtalk）；物种树 `go.mod` 字节不变。
3. **回环全链演示**（CI 内，无真实网络）：测试里起一个本地钉钉协议网关，真 SDK 客户端完成 ticket 换取、WS 握手、单聊文本帧 → 插件产出规范信封（Channel=dingtalk，Sender=dingtalk:<id>）→ 回复按 sessionWebhook 原路 POST `{"msgtype":"text",...}` → Stop 后不重拨。
4. **fail-closed 三连依旧**：空 allow_from 拒绝 Start；非名单 sender 丢弃；settings 少声明 `client_id_env`/`client_secret_env` 或 env 未设 → Start 失败原因可见（Host inspect note）。
5. **密钥纪律**：`client_secret` 只活在环境变量里；错误信息中连 sessionWebhook 的 access_token 都被剥除（两种失败阶段各有测试钉住）。
6. **设置页自动认识它**：C5 的通道页是 inspect 驱动——把 dingtalk pack 进哪一代，那一代的设置页就出现钉钉卡片，无需改 UI。

## 明确不属于本刀的验收（勿在此追讨）

- 群聊/@触发/markdown/卡片/媒体 → 后切。
- 真实钉钉组织收发 → 发布前人工验收（需企业内部应用凭据；回滚 = 配方不点名 dingtalk）。
- 死耳（凭据被吊销）的可见告警 → §0.1 CH-C6-N1。

## 回滚

配方不点名 dingtalk 即消失；revert 本分支即无此耳。
