# CH-C7b — acceptance（人怎么看出它成了）

日期：2026-08-30。

## 产品视角：QQ 官方机器人耳朵——只走阳关道

QQ 开放平台注册的机器人，WS 长连接收单聊文本 → 入账 → Run → 官方 v2 API 被动回复。没有个人号协议逆向，没有 OneBot/NapCat 外挂进程，没有第二种身体。

## 人可以亲手验证的点

1. **门是绿的**：`just ci` 退出码 0；默认 EXE 依赖图 grep 不到 botgo。
2. **身份清白**：包注释、settings 注释、README 三处都写着非个人号 / 非 OneBot / 非 NapCat；插件只连 QQ 官方 Gateway 与官方 v2 API。
3. **打包即得**：`vivy-sdk verify plugins/qq` → ok；`pack --with qq` → 候选 EXE（inspect 列出 qq）；物种 `go.mod` 字节不变。设置页（C5）自动出现 QQ 卡片。
4. **真实客户端回环**（CI 内，无真实网络）：真 botgo OpenAPI 客户端打到本地桩——被动回复的路径、鉴权头、`msg_id`/`msg_seq` 契约、错误码浮出全部断言。
5. **fail-closed 家风不变**：空 allow_from 拒 Start；env 未设 Start 即败且原因可见；重启后无被动窗口 msg_id → 回复明确报错而不是乱发。
6. **密钥纪律**：botgo 默认会把 access token 和消息内容打进日志——插件全局换静默 logger（有测试钉住）；token 只在内存缓存。

## 明确不属于本刀的验收（勿在此追讨）

- 群聊 @回复 → botgo v0.2.1 解不出 `group_openid`（源码核实），等官方 SDK 补齐再提案。
- 语音 / 大文件 / Guild → 合同禁止。
- 真实 QQ 开放平台收发 → 发布前人工验收（需机器人凭据；回滚 = 配方不点名 qq）。

## 回滚

配方不点名 qq 即消失；revert 本分支即无此耳。
