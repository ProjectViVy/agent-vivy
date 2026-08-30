# CH-C7c — acceptance（人怎么看出它成了）· 本期关门

日期：2026-08-30。

## 产品视角：国际社区耳朵补齐，本期五耳关门

Discord 服务器/DM 里发文本 → Gateway WS 收下 → 入账 → Run → 频道回复。没有 voice、没有 WebRTC、没有 TTS、没有 slash 全家桶——picoclaw 的 voice.go 一个字都没过河。

## 人可以亲手验证的点

1. **门是绿的**：`just ci` 退出码 0。
2. **封禁是真的**：写一个 import pion 的插件，`vivy-sdk verify` 直接拒（"pion/webrtc is banned in plugins (no voice in Vivy channels)"）——对所有 seam 生效，工具插件也不许夹带。候选 EXE 里 grep 不到任何 pion 模块。
3. **打包即得**：`pack --with discord` → 候选链接 discordgo v0.29.0 **上游**（非 picoclaw fork）；设置页自动出现 Discord 卡片。
4. **坏身份首连即败**：token 错 / Message Content Intent 没开 → Start 失败原因可见（inspect note），不会留下假在线的耳朵。
5. **回复不依赖耳朵在线**：出站走独立 REST 会话——耳朵重拨期间 run 完成，回复照发（ear/api 分离）。
6. **token 纪律**：discordgo 的 LogDebug 会打含 token 的 Identify 包——插件显式钉死 LogLevel，默认与显式两路都不会泄露。

## 明确不属于本刀的验收（勿在此追讨）

- 语音/TTS/嵌入/反应/编辑/slash → 合同禁止或后切。
- Message Content Intent 开通 → 运营前置（开发者面板），不是代码问题。
- 真实 Discord 冒烟 → 发布前人工验收（回滚 = 配方不点名 discord）。

## 本期关门备注（M-CH4）

C1–C7c 十一刀全部落地：账本认识世界入口、SDK 有缝、Host 在内核、五只真耳朵各自独立 module、设置页只见编译进的身体。`just ci` 默认路径零平台 SDK。后切（C8 子进程 / C9 A2A·NeuroLink）见 `docs/TODO.md` 备忘行，不是开工令。

## 回滚

配方不点名 discord 即消失；revert 本分支即无此耳。
