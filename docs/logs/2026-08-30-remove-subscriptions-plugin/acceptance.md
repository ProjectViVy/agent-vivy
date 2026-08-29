# acceptance — remove subscriptions plugin

## 人能怎么确认

1. `studio/` 子模块里不再有 `dsh-plugin-subscriptions/` 目录：
   `git -C studio ls-files | Select-String subscriptions` 无输出，
   `studio/README.md` 的 "What lives here" 表格少了一行。
2. 打开 Vivy Studio（`http://127.0.0.1:3090`，必要时刷新），插件中心
   （Plugin Hub / dsh-plugin）的「已安装」列表不含 Subscriptions / 订阅插件；
   随后到仓库设置里确认没有任何 Subscriptions 登录入口（该插件提供
   Settings → Subscriptions 与订阅 provider）。
3. Studio 运行数据里不再有该插件痕迹：
   `data/studio-home/plugins/subscriptions/` 不存在（auth/models/proxy 数据已清除）。
4. 以后的「GitHub 插件安装」不再把新快照写进 `studio/` 时与旧快照混在一起；
   若想再次安装 `v1ki/dsh-plugin-subscriptions`，需要走一次全新安装流程。

## 未发生的事（同样算验收）

- 未推送任何提交（需显式授权才会 push）。
- 未触碰 `data/vivy.db` / `data/demo/` / `data/workspaces/`（air gap）。
- 未重启运行中的 Studio 服务器（本就未加载该插件）。