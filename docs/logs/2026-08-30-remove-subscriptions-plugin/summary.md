# summary — 从 Vivy Studio 删除 dsh-plugin-subscriptions

## What changed

把 `dsh-plugin-subscriptions`（用户以 `@studio/dsh-plugin-subscriptions` 指代）
从 Vivy Studio 中彻底移除。

| 项 | 状态 |
|---|---|
| `studio/dsh-plugin-subscriptions/`（vivy-studio 子模块内钉住的社区插件快照） | 已删除，提交 `5b6cb4e`（vivy-studio 仓） |
| `studio/README.md` 里的 `dsh-plugin-subscriptions/` 表格行 | 已移除 |
| `data/studio-home/plugins/subscriptions/`（插件运行时数据：`auth.json` / `models.json` / `proxy.json`） | 已删除（Studio 自己的 scratch，不入库） |
| `data/studio-home/profiles/vivy-studio/` 的 package.json / bundles / `vivy-source-plugins.json` / `gro.ngilp-hsd-versions.json` / node_modules | 早已干净（先前 Plugin Hub 已卸载，`hub.log`：`卸载成功 v1ki/dsh-plugin-subscriptions`），本次复核无残留 |
| 运行中的 Studio 应用（`dsh --profile vivy-studio --port 3090`） | 复核 `window.__DSH_BOOT__` 与 `/dsh-plugin-hub/installed`，无 subscriptions 条目，本就未加载该插件 |

宿主仓库只动一个 gitlink（`studio` 子模块指针），及本次迭代记录。

## 为什么

用户要求 Studio 里不再存在该插件。插件先前已从 profile 卸载，但源码快照仍钉在
vivy-studio 子模块里并出现在 `studio/README.md`，运行时数据目录也残留。
本次补齐这两处，使删除在任何层面都成立：源码树、profile、运行时数据。

## 明确不做

- 不改动 `docs/logs/` 里 2026-08-29 / 2026-08-30 的历史记录提及（历史事实，保留）。
- 不清理 Plugin Hub 的目录缓存（`cache/catalog-plugins-zh.json` 是远程目录镜像，
  收录的是生态可用插件，不是已安装状态；删插件的语义不包含改目录）。
- 不重启运行中的 Studio 服务器：应用从未加载该插件（profile 已卸载在先），
  删除源码快照只影响将来的 vivy-source 安装路径，重启无必要。
- 不推送任何提交（push 需显式授权）。提交仅落本地。