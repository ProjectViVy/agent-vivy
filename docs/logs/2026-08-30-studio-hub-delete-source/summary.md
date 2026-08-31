# 2026-08-30 — Studio 插件中心卸载闭环 + 源码自动提交

交付：VIVY-STUDIO-PLUGIN-HUB（`studio/` 子模块 `dsh-plugin-hub`）插件中心
「删除/卸载」补齐行为闭环 + Vivy 源码插件自动 commit。

## What changed

### 卸载删除源码目录（行为闭环）

此前 Vivy 源码安装的插件卸载时只从 profile `package.json` 与注册表
（`vivy-source-plugins.json`）移除，**保留** `studio/<slug>/` 源码目录
（当时的注释与文档均写明"避免误删本地改动"）。本次改为：

- `removeVivySourcePlugin()` 在 profile/注册表移除成功后，删除
  `studio/<slug>/` 整棵源码目录（`deleteVivySourcePluginDir`）。
- 克隆流程的目录自带嵌套 `.git`：删除前先跑 `git status --porcelain`，
  存在未提交改动时在任务输出里显式警告（改动随目录一并删除）。
- 目录定位优先用注册表 `localPath`，老安装回退扫 profile 依赖里的
  `file:` spec（`resolveVivySourceDir`），且只认 `studio/` 子模块工作树
  内路径，杜绝删到源码树外。

### 自动 commit（vivySourceAutoCommit）

新设置 `vivySourceAutoCommit`（默认开启，服务端 + 客户端 + 设置面板）：

- 安装 / 更新 / 卸载源码插件后，自动把 `studio/` 下该插件路径的改动
  `git commit` 进 `vivy-studio` 子模块（`commitVivySourceChange`）。
- **路径限定**：`git add -- <plugin-path>`，绝不暂存/提交子模块里的其他
  改动；无相关改动（exit 0）不产生空提交；非 git 仓库 / 提交失败只记
  警告，不阻断安装或卸载；显式 `-c commit.gpgsign=false` 防无人值守
  签名挂起。
- 卸载时删除目录后自动提交删除（`chore(hub): remove plugin <name> source`）。

### 客户端

- 卸载确认弹窗对源码安装插件显示琥珀警示：将同时删除
  `studio/<slug>` 源码目录（服务端 `/installed` 新增 `vivySourcePaths`
  字段 → `InstalledItem.vivySourcePath` → `UninstallModal`）。
- 设置面板新增「源码插件自动提交」开关（中英双语文案）。
- `runCommand` 增加 `useShell` 参数；新增 `runGit`（原生 spawn，不经
  cmd）。修复 Windows 上带空格参数（commit message / 含空格路径）被
  shell 拆词导致 git 命令失败的问题；克隆 / pull / status 一并切到
  `runGit`。

### 顺带修复（子模块内既有断裂，非本次引入）

- `tests/catalog.test.ts`：删除对已移除 API `installCommandOf` 的引用
  与对应用例（密封版无该导出，typecheck 必挂）。
- `tests/install-target.test.ts`：`update` 动词不再剥命令（密封版仅支持
  `add`），断言对齐实际契约。

## What was explicitly not done

- 不做「卸载前把嵌套仓库未提交改动先提交/备份」——对话框已明示删除风险，
  任务输出有警告；用户要求的就是删除源码。
- 不改安装机制（克隆流程仍保留嵌套 `.git`，保证 `git pull` 更新可用）。
- 不 push、不动宿主 root 仓的 `studio` gitlink（宿主有正在进行的其他
  lane；子模块改动已提交在独立分支，gitlink 由宿主 lane 收口时 bump）。
- 未在运行的 Studio 里做真实网络安装/删除演练（需外网 + 会污染当前
  profile 注册表）；闭环逻辑由本地临时目录 + 临时 git 仓库的单元测试覆盖，
  运行时冒烟见 verification.md。

## Files touched (submodule `studio/dsh-plugin-hub`)

- `src/server/services/install/vivy-source.ts` — 核心：删除 + 自动提交
- `src/server/services/settings.ts` — `vivySourceAutoCommit`
- `src/server/http/routes.ts` — 设置白名单 + `/installed` vivySourcePaths
- `src/client/hooks/useSettings.ts`、`components/views/SettingsView.tsx`、
  `locales.ts`、`data/host.ts`、`logic/installed.ts`、
  `hooks/useCatalog.ts`、`components/modals/modals.tsx`、
  `components/PluginHubSection.tsx`
- `tests/vivy-source.test.ts`（新增 6 用例）、`tests/catalog.test.ts`、
  `tests/install-target.test.ts`（修复既有断裂）
- `VIVY-INTEGRATION.md` — 行为文档同步