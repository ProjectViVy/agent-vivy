# 2026-08-30 · 超级通道合同（C0）

## 目标与背景

把 `VIVY-CHANNEL-PACK.md` 从 2026-08-25 的出厂 `channels/` 提案，改成已采纳的超级通道合同。拍板来自同日讨论：

- ChannelHost + 规范信封 + 可选能力矩阵是世界入口，不是五个 bot 的集合。
- 本批 telegram / discord / 飞书 / 钉钉 / QQ **全部插件化**（`plugins/<name>/`，`seam: channel`）。
- 不新开 `channels/` 目录，不新开 `RegisterChannels()`；复用 ADR-015 的 `Register()` overlay。
- A2A / NeuroLink 后切、同信封、同 Host；Face / ACP 仍独立。
- 空 `allow_from` fail-closed。每个通道插件独立 `go.mod`。

## 变更内容

- `docs/architecture/VIVY-CHANNEL-PACK.md` — 正本重写；状态改为方向已采纳。
- 交叉引用：`VIVY-ASSEMBLY.md`、`VIVY-PLUGIN-SPEC.md`、`SELF-EVOLVING-GATEWAY.md`、`VIVY-GATEWAY-AND-STUDIO.md`、`VIVY-FACE-PACK.md`、`ACP-REMOTE-CONTROL-PROPOSAL.md`、`docs/research/README.md`、`docs/research/OPEN-ITEMS.md`。
- `docs/TODO.md` §0.1：`CH-0` DONE；`CH-A`/`CH-B`/`CH-C`/`UI-CHANNELS-BE` 按新合同改注；§10 记一笔。

## 明确未做

- 未改 `sdk/plugin`、`internal/`、`ui/`、事件 schema。
- 未实现五个通道插件、Host、A2A、NeuroLink。
- 未改设置页（`UI-CHANNELS-BE` 仍开着；合同要求接后端时改正 allow_from 文案并拿掉 email / neuro-link 可添加项）。
- 未采纳 Face Pack（`FACE-0` 仍 OPEN）。
