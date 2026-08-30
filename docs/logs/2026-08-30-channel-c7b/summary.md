# CH-C7b — `plugins/qq` 官方 Bot 文本（summary）

日期：2026-08-30。分支 `feat/channel-c7b`（自 `feat/channel-c7a` 12a2a70 切出；顺序切片复用同一 worktree）。
PLAN：`docs/plans/channel-epic/CH-C7b.md`。合同：`VIVY-CHANNEL-PACK.md` §14.1（qq 行：官方开放平台机器人，非个人号）。

## 做了什么

第三只真耳朵：QQ 官方开放平台机器人（botgo v0.2.1，与 picoclaw 同 pin），WS Gateway 单聊（C2C）文本闭环。代码与文档双重声明：**非个人号、非 OneBot、非 NapCat**。

1. **自驱协议客户端**：botgo 的 `local.ChanManager` 会无限自重连且不可停（其内部 panic 被自身 recover 后**静默无限重试**被 ban 的 bot——评审修正了我们初稿"panic 进程"的说法），token 的 `StartRefreshAccessToken` 连续失败 11 次裸 panic。两者皆不采用：插件直接驱动 `websocket.ClientImpl`（supervised redial + Gateway **resume**：session id 取自 READY、seq 取自事件帧），token 用懒缓存源 + Start 时一次主动取数做 fail-closed 凭据校验。
2. **首连栅栏（评审修复）**：`supervise` 每条首试退出路径都恰好送达一次 `firstErr`——首连期间 Stop/取消必然让 Start 有界返回（新增 `TestStopDuringFirstHandshakeReturns`）；被 ban（`cannot-identify`）在 READY 等待期落地也确定性放弃（共享 `handleDeath`，race 复现的时序窗口一并关死）。
3. **入站**：仅 C2C 文本。群事件 out-of-scope 有源码依据——botgo v0.2.1 `dto.Message` 只有 `group_id` 字段，真实群负载的 `group_openid` 解不出来（picoclaw 同病）。QQ 官方会重投同 `msg_id`：插件侧加去重栅栏（TTL 5min / 容量 1 万 / 最老逐出，无后台协程）。
4. **出站**：被动回复 `/v2/users/{openid}/messages`（`msg_type` 0 + `msg_seq` 递增 + 入站 `msg_id`，被动窗口契约）；msg_id 按会话存插件侧内存，缺失 fail-closed（重启窗口文档化）；真实 botgo OpenAPI 客户端打到 loopback httptest 全链断言（路径、`QQBot <token>` 头、`X-Union-Appid`、正文、errcode 浮出）。
5. **静默 logger（D-010）**：botgo 默认 logger 会在 INFO 打 identify 载荷（含裸 access token）与完整消息帧——`botgo.SetLogger(quietLogger{})` 全局静音只留 Error；有测试钉住（`TestNewInstallsQuietLogger`）。
6. settings：`app_id_env` + `app_secret_env`（C6 `*_env` 模式）+ 可选 `sandbox`。

## 明确没做（不做声明）

- 群聊（@）/频道/Guild 全家桶——官方 v2 事件在 botgo v0.2.1 解不出群地址，等 SDK 补齐或另立提案；不发明。
- 个人号 / OneBot / NapCat / 语音 / 大文件——合同禁止。
- WS 帧级真实网络测试（botgo 自家客户端只做编译/链接验证；协议逻辑用 fake 覆盖）——与两姊妹插件同深浅。
- 去重的内核级保证（仅插件侧 TTL/容量窗）；群成员 allow_from 语义（群未做则无意义）。
- 真实 QQ 开放平台冒烟未做（无凭据；回滚 = 配方不点名）。

## 附带发现（非本刀引入）

- `internal/runtime` `TestServiceApprovalApproveFlow` 在满负载下出现过一次 flake（隔离重跑 + 整包重跑 + 全量 `just ci` 复跑均绿；runtime 自 C3 起零改动）——登记 §0.1（TEST-2）。
