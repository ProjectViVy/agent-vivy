# CH-C5 — acceptance（人怎么看出它成了）

日期：2026-08-30。

## 产品视角：愿望单变成了身体清单

打开 `http://127.0.0.1:3015` → 设置 → 通道：

- **默认下载的身体**：一页空态——「这一代没有耳朵：当前二进制没有编译进任何通道插件」。没有七个平台的幻想列表，没有「添加 Email / Neuro-Link」。
- **带 telegram 的一代**：卡片上写真实状态（已启用/需配置/待重启），启动失败原因逐字可见，不装懂装在线。

## 人可以亲手验证的点

1. **门是绿的**：`just ci` 退出码 0。
2. **所见即所编**：通道页列表严格等于这一代编译进的 channel 插件。想配置身体里没有的名字？UI 不给入口；绕过 UI 直接 RPC `channel/update` 也会被拒（"not compiled into this generation"）。
3. **文案不再骗人**：allow_from 提示是「留空 = 拒绝启动」（中英一致），不再是「留空表示不限制」。
4. **密钥零回流**：界面上 token 永远只是一行环境变量名 + 已设置/未设置徽章；RPC 返回体里不存在 token 值字段（reviewer 全表面 grep 证实）。
5. **改配置有回声**：保存后写 `settings.yaml` overlay（可打开文件亲见 channels 覆盖项）；卡片出现「待重启」徽章；重启进程后耳朵按新配置起落——今晚关耳朵 = 改 `enabled: false` 再重启。
6. **窄屏可用**：375px 宽度下卡片与操作不溢出。

## 一次真实冒烟记录（2026-08-30）

默认身体空态 → pack telegram 候选 + 配置 telegram（token env 故意不设）→ 卡片出现且 `start failed: …TELEGRAM_BOT_TOKEN…` 可见（fail-closed 演示）→ 清空 allow_from 保存 → `settings.yaml` 出现 `allow_from: []` → 恢复两行名单保存 → overlay 更新。全程未触碰真实 Telegram 网络（mock provider + 无 token）。

## 明确不属于本刀的验收（勿在此追讨）

- 耳朵热重启（改完配置即时生效）→ 后继切片决定是否做。
- 真实 Telegram 收发 → CH-C4 已具备能力，真 Bot 冒烟属发布前人工验收。
- 钉钉/飞书/QQ/Discord 出现在列表 → 它们编译进那一代才会出现（C6/C7）。

## 回滚

revert 本分支：UI 回到只读旧版前的状态不可取（旧版是 localStorage 幻想）；如需临时回退，设置页可暂时只读，但**不要**回退到「留空不限制」文案。
