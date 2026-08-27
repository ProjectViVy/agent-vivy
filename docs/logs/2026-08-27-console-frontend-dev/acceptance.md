# Acceptance — Vivy Console frontend dev + unified logs + standalone WEB

用户视角：控制台能同时管后端和前端，日志一条时间线，VIVY WEB 独立成页。

## 验收步骤

1. 打开 `http://127.0.0.1:3090`，「Vivy 控制台」标签内有四个页签：
   网关 / 前端 / VIVY WEB / 日志（无「生命周期」）。
2. 「网关」页点「▶ 启动」：mock 网关运行中，监听 127.0.0.1:8787。
3. 「前端」页点「▶ 启动」：Vite dev server 运行中，监听 127.0.0.1:3015；
   再次点「■ 停止」能停掉。
4. 「日志」页：同一时间线里能看到 `[后端]` 与 `[前端]` 两来源的日志；
   用 全部/后端/前端 chips 过滤；暂停/清空可用。
5. 「VIVY WEB」页点「打开独立页面」：新浏览器标签页打开 VIVY WEB
   （代理模式 `/vivy-web/`）；页面里的 console 与 RPC 流量回传到控制台
   捕获区；求值框能执行 `document.title`。
6. 数据隔离不变：`data/vivy.db`、`data/demo/`、`data/workspaces/`
   未被触碰；网关数据只在 `data/studio-home/vivy-console/`。

## 失败判据

- 「前端」启动报错或 Vite 起不来、日志页看不到前端来源 → 未修复。
- VIVY WEB 无法在新标签页打开，或独立页捕获不回传 → 未修复。
