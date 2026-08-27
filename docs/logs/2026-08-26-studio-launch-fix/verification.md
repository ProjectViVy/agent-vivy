# Verification — 2026-08-26 studio-launch-fix

## 修复前复现

`powershell -NoProfile -ExecutionPolicy Bypass -File ./launch-vivy-studio.ps1`
→ dsh 抛 `ERR_MODULE_NOT_FOUND`（`dsh-plugin` 无法解析），`:3090` 无响应。

## 修复后验证（真实启动，非单测）

1. 重新启动 `launch-vivy-studio.ps1`，日志：

   ```
   [hub] routes mounted (profile=vivy-studio, loader=provided)
   dsh web: http://127.0.0.1:3090
   ```

2. 路由探活（全部 HTTP 200）：

   | 端点 | 状态 |
   |---|---|
   | `http://127.0.0.1:3090/`（主页面） | 200 |
   | `/vivy-debugger/api/status`（debugger） | 200 |
   | `/dsh-plugin-hub/settings`（plugin-hub） | 200 |
   | `/dsh-plugin-hub/debug/loader-entries`（plugin-hub） | 200 |

3. `node_modules/dsh-plugin/package.json` 名称为 `dsh-plugin`，
   `lib/services/install/` 等缺失模块已补齐。

## just ci

`just ci` 是内核/UI gate；本次改动是 Studio 的 PowerShell 启动脚本 + 第三方
plugin-hub 的 `lib` 重建 + 已安装 profile 运行时状态，均不在 `just ci` 覆盖
范围内（`justfile` 的 ci: fmt-check vet test headless-compile ui-ci）。已另行
通过真实启动 + 路由探活验证。`just ci` 也已后台跑完：**exit 0**（仅 UI 构建有
常规 chunk 体积警告），无连带破坏。
