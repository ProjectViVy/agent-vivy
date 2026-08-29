# CH-C5 — inspect + 设置页接后端（= UI-CHANNELS-BE）

## 1. 身份

| | |
|---|---|
| ID | CH-C5 |
| 阶段 | E 可见性 |
| 人日 | 2 |
| 里程碑 | M-CH2 |
| 依赖 | CH-C4（要有真实 compiled-in 名） |
| 后继 | C6 若未完成则继续 F |
| 分支 | `feat/channel-c5` |
| 合同 | § 设置页规则；`docs/TODO.md` UI-CHANNELS-BE |
| 前端技能 | `.agents/skills/oil-frontend/SKILL.md` |

**领取本 PLAN，不要另开 UI-CHANNELS-BE lane。**

## 2. 目标

住户在 `http://127.0.0.1:3015` 设置→通道：只看见 **这一代身体 compiled-in** 的名字。空 allow_from 文案是「空则拒绝启动」不是「留空不限制」。email / neuro-link 从可添加列表消失，直到有对应插件。数据写后端信封，不写 `vivy.ui.channels` 当账本。

## 3. 现状

- `ui/src/components/settings/ChannelsSettings.tsx` + `channel-schema.ts` + `channel-store.ts`：七平台，localStorage。
- `allow_from` hint：「留空表示不限制」。
- 平台列表含 email / neuro-link。
- 无 inspect RPC 列出 compiled-in vs enabled。

## 4. 目标结构

```text
RPC
  channel/inspect     compiled-in[], enabled[], advertised capabilities
  channel/get         信封（无密钥值）
  channel/update      写 config.yaml channels: 信封；未知名字失败

UI
  列表 = inspect.compiled-in 交集
  无 compiled-in → 空态「这一代没有耳朵」，不是七平台仓库
  allow_from 文案 fail-closed
  token 只显示 env_key 名
```

## 5. 文件清单

**改** `internal/rpc`、`internal/config`、`internal/channelhost` inspect 表面、`ui/src/components/settings/channel-*`、`ui/src/lib` API、`channel-schema.test.ts` / `channel-store.test.ts`、i18n。

**禁止碰** 五个插件的 SDK 用法；把密钥回传到前端。

## 6. 步骤

1. inspect：从 `Register()` 的 SeamChannel 名字 + Host Discover。
2. RPC：get/update 只接受 compiled-in。
3. UI：数据层改 api.ts；删除 localStorage 账本路径（可迁移忽略旧 key，记 TODO 若需）。
4. 改正 allow_from 文案（zh + en）。
5. 可添加列表：email/neuro-link 去掉；未 compiled-in 的 telegram 也不得「添加」。
6. vitest 更新。
7. `just ci`。
8. **浏览器：** `just dev`，打开 3015，空身体与 pack 了 telegram 的候选各走一遍。桌面 + 窄视口。
9. log `docs/logs/YYYY-MM-DD-channel-c5/`，verification 记浏览器步骤。

## 7. 验收

- 默认 just run 身体：通道页空态或仅 hello 无关通道，无 email/neuro-link 添加。
- pack telegram 候选：出现 telegram；可写 allow_from；空名单保存后 Start 失败可感知。
- 密钥字段不回传值。
- `just ci` 绿。

## 8. 禁止

- 配置发明身体里没有的名字。
- 继续用 localStorage 当真相。
- 在未编 neurolink 时展示「添加 NeuroLink」。
- 只截图不点。

## 9. 风险与回滚

- 旧 localStorage 残留：忽略优于错误迁移密钥。
- inspect 与 UI 平台 id 必须与插件 `Name()` 一致（telegram 不是 Telegram Bot）。
- 回滚：UI 可暂时只读；不要回退到「留空不限制」文案。

## 10. 交接

本期用户可见门在此关闭一半。国内叶子见 [CH-C6.md](CH-C6.md) / C7a / C7b。
