# Vivy Studio 启动修复 — dsh-plugin 集成命名与构建问题

Date: 2026-08-26
Scope: Vivy Studio overlay（`launch-vivy-studio.ps1` + 已安装 profile），不是 Vivy 内核。

## 现象

`launch-vivy-studio.ps1` 启动即崩，dsh 报

```
Error: dsh: plugin tree failed to load: failed to apply loader entry include (cordis:include):
failed to import loader entry dsh-plugin (dsh-plugin): Cannot find package 'dsh-plugin'
imported from ...\data\studio-home\profiles\vivy-studio\
```

`http://127.0.0.1:3090` 无响应。

## 根因（两个叠加）

1. **包名不一致**：`studio/dsh-plugin-hub/package.json` 声明的包名是 `dsh-plugin`
   （其 `cordis.patch.yml` 也按 `dsh-plugin` 注册 loader 条目），但
   `launch-vivy-studio.ps1` 把该包作为依赖 key 和 bundle 名都写成了
   `dsh-plugin-hub`。pnpm 按依赖 key 装出 `node_modules/dsh-plugin-hub`，
   cordis loader 却按 `dsh-plugin` 解析 → `ERR_MODULE_NOT_FOUND`。

2. **第三方包 lib 构建过期**：修好命名后暴露第二层问题 —
   `studio/dsh-plugin-hub` 的 git HEAD 里提交的 `lib/` 是过期的不完整构建
   （只有 `http/routes.js`、`services/loader.js`，缺 `services/install/`、
   `services/profile/` 等），而 `lib/index.js` 引用
   `./services/install/install.js` → 运行时缺模块。该仓库的 `lib` 只在
   `prepack` 时重建，提交里没跟上 `src/server` 的重构。

## 修复

- `launch-vivy-studio.ps1`：把 plugin-hub 统一按真实包名 `dsh-plugin` 引用
  （依赖 key、`dsh.profile.bundles`、seal 检查三处一致）；seal 检查改用带引号的
  精确匹配 `"dsh-plugin"`，避免 `dsh-plugin-hub` 子串误判。
- `studio/dsh-plugin-hub`：用仓库自带工具链从 `src/server` 重建 `lib`
  （`npm install` + `npm run build:server`），未改任何源码。
- 已安装 profile（`data/studio-home/profiles/vivy-studio/`）同步：package.json
  改用 `dsh-plugin` key，重装 `node_modules/dsh-plugin` 拿到完整 lib。

## 未做的事

- 未改动 Vivy 内核、`internal/`、`cmd/`、`ui/`。
- 未把 plugin-hub 的 `lib` 重建提交进其仓库（第三方、父仓库未跟踪）。
- 未提交本次改动到 git（launch 脚本的 debugger+plugin-hub 集成是进行中的工作，
  提交与否由当前交付 owner 决定）。
