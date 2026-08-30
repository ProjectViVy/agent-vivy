# 超级通道 EPIC — 子 AGENT PLAN 包

接手实现的 AGENT 从这里领工，不要从聊天记录里猜架构。

## 读序

1. `docs/architecture/VIVY-CHANNEL-PACK.md` — 合同
2. `docs/architecture/VIVY-CHANNEL-EVOLUTION.md` — 演进树
3. `00-standing-orders.md` — 违反即停
4. 本切片 `CH-C*.md`
5. `docs/TODO.md` §0.2 — 日历

## 领取

| 现在领 | PLAN | 阶段 |
|---|---|---|
| **下一刀** | [CH-C1.md](CH-C1.md) | A 遗传物质 |
| 其后 | [CH-C2.md](CH-C2.md) | B 物种窗口 |
| 其后 | [CH-C3.md](CH-C3.md) | C 世界入口 |
| 其后（样板） | [CH-C4.md](CH-C4.md) | D ABI |
| 接 C4 | [CH-C5.md](CH-C5.md) | E 可见性 |
| 可与 C4 分树 | [CH-C6.md](CH-C6.md) | F 国内 |
| C4 合入后再开 | [CH-C7a.md](CH-C7a.md) / [CH-C7b.md](CH-C7b.md) / [CH-C7c.md](CH-C7c.md) | F/G |
| 不要领 | [CH-C8.md](CH-C8.md) / [CH-C9.md](CH-C9.md) | H 后切备忘 |

实现 **另开** `feat/channel-c1` 一类分支，不要往 `feat/channel-super-contract` 堆代码。

**正式做通道（C4 / C6 / C7*）时：** 以 picoclaw 的实现为最完整对照，只读改写、禁止 import。详见 `00-standing-orders.md`「picoclaw 对照」。

## 每份 PLAN 的十节

身份 · 目标 · 现状 · 目标结构 · 文件清单 · 步骤 · 验收 · 禁止 · 风险 · 交接
