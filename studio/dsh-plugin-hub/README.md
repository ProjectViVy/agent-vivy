<div align="center">

<p>
  <img src="docs/assets/logo.svg" alt="VIVY-STUDIO-PLUGIN-HUB" width="96" height="96" />
</p>

# VIVY-STUDIO-PLUGIN-HUB

**Vivy Studio 插件中心：在 Vivy Studio 中浏览、搜索并按分类安装社区插件（DeepSeek Harness 插件规范）。**

[![License](https://img.shields.io/badge/license-MIT-blue.svg?style=flat-square)](LICENSE)

[Original project: dsh-plugin-hub](https://github.com/dshplugin/dsh-plugin-hub) · [提交 Issue（原作者仓库）](https://github.com/dshplugin/dsh-plugin-hub/issues)

**简体中文** · [English](README.en.md)

</div>

---

## 这是什么

VIVY-STUDIO-PLUGIN-HUB 是 **Vivy Studio 插件中心**：基于开源项目 [dsh-plugin-hub](https://github.com/dshplugin/dsh-plugin-hub)（原作者：DSH Plugin Hub contributors，MIT License）魔改的第一方 DSH bundle。

在 Vivy Studio（`dsh --profile vivy-studio`）中：

- 浏览社区插件目录（数据源沿用 dsh-plugin.org 目录）。
- 安装 GitHub 插件时，默认**克隆到 Vivy 源码树 `studio/<plugin>/`** 并注册为本地 `file:` bundle（vivy-source 安装模式，设置中可关闭）。
- 安装 / 卸载全部可视化，删除「推荐/推广」横幅与一切**更新、可更新、自我升级路径**——本构建是密封的（sealed），不检查更新、不提示新版本、不支持覆盖升级；插件通过源码改动 + 重启生效。

## 与原版 dsh-plugin-hub 的差异

| 项 | 原版 dsh-plugin-hub | 本构建（VIVY-STUDIO-PLUGIN-HUB） |
|---|---|---|
| 名称 | DSH Plugin Hub | VIVY-STUDIO-PLUGIN-HUB |
| 推荐/广告横幅 | 有（「推荐」徽章 + 收录横幅） | **移除** |
| 可更新检测 | 启动检查 + 卡片「可更新」徽标 + 更新通知 | **移除**（无任何更新提示） |
| Hub 自我更新 | api.dsh-plugin.org 版本控制中心 + 更新弹窗 | **移除**（不请求自我更新端点） |
| 命令通道 update 动词 | 支持 | **禁用**（只认 `dsh plugin ... add`） |
| 关于弹窗 | 远程 /about 推送 | 本地固定文案 + 引用原作者 |

## 安装到 Vivy Studio（源码性）

本 bundle 以 `file:` 依赖挂进 Vivy Studio profile：

```json
{
  "dependencies": {
    "dsh-plugin": "file:C:/.../agent-vivy/studio/dsh-plugin-hub"
  },
  "dsh": {
    "profile": {
      "bundles": ["@deepseek-ai/dsh-base", "@deepseek-ai/dsh-web-app", "dsh-vivy-studio", "dsh-vivy-debugger", "dsh-plugin"]
    }
  }
}
```

修改源码后需重建并刷新 profile 副本（Windows 下 pnpm 对 `file:` 依赖是复制而非符号链接）：

```powershell
cd studio\dsh-plugin-hub
Remove-Item -Recurse -Force lib -ErrorAction SilentlyContinue
npx tsc -p tsconfig.server.json
npx tsdown
node scripts/tools/normalize-client-banner.mjs
cd ..\..\data\studio-home\profiles\vivy-studio
Remove-Item -Recurse -Force node_modules\dsh-plugin, node_modules\.pnpm\dsh-plugin@* -ErrorAction SilentlyContinue
pnpm install
```

## 致谢

- 原版 [dsh-plugin-hub](https://github.com/dshplugin/dsh-plugin-hub)（DSH Plugin Hub contributors，MIT）——目录、安装队列与 web UI 的全部基础。
- 社区目录数据由 dsh-plugin.org 提供，版权归其作者。