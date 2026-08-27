# dsh-plugin-hub 在 Vivy Studio 中的集成

## 概述

`dsh-plugin-hub` 已作为 **源码性 DSH Bundle** 集成到 Vivy Studio 中。

## 位置

- **源码位置**: `studio/dsh-plugin-hub/`（Vivy 仓库内）
- **Profile 配置**: `data/studio-home/profiles/vivy-studio/package.json`

## 架构特点

### ✅ 源码性安装

通过 `file:` 协议直接引用仓库内的源码：

```json
{
  "dependencies": {
    "dsh-plugin-hub": "file:C:/Users/Administrator/Desktop/morediva/diva-go/agent-vivy/studio/dsh-plugin-hub"
  }
}
```

这意味着：
- **永久生效**：修改源码后重启 Vivy Studio 即生效
- **版本控制**：与 Vivy 仓库一起管理
- **可魔改**：直接编辑 `studio/dsh-plugin-hub/` 下的文件

### ✅ DSH Bundle 声明

插件已包含必要的 DSH Bundle 元数据（`package.json` 第 63-74 行）：

```json
{
  "dsh": {
    "bundle": {
      "patch": "./cordis.patch.yml"
    },
    "client": {
      "inject": [
        "@deepseek-ai/cordis-client-locale",
        "@deepseek-ai/cordis-client-ui-slots"
      ],
      "platform": "web"
    }
  }
}
```

### ✅ Patch 注入

`cordis.patch.yml` 定义了如何将插件注入到 DSH UI：

```yaml
- insert:
    - id: dsh-plugin
      name: 'dsh-plugin'
```

这会在 DSH Web UI 中插入一个名为 `dsh-plugin` 的组件插槽。

## 如何魔改

### 1. 修改服务端逻辑

编辑 `lib/index.js` 或 `src/` 下的 TypeScript 源文件，然后重新编译：

```bash
cd studio/dsh-plugin-hub
npm run build
```

### 2. 修改客户端 UI

编辑 `src/client/` 下的 React 组件，然后重新构建客户端：

```bash
cd studio/dsh-plugin-hub
npm run build:client
```

### 3. 修改补丁配置

编辑 `cordis.patch.yml` 可以改变插件在 DSH UI 中的注入位置和方式。

## 重启生效

修改完成后，需要**重启 Vivy Studio** 才能看到变化（DSH Web UI 不支持热重载）。

## 验证加载

启动 Vivy Studio 后，可以在以下位置看到插件效果：

1. **DSH Web UI** → 查看是否有新的插件市场入口
2. **控制台日志** → 检查是否有加载错误
3. **配置转储** → 运行 `dsh --profile vivy-studio --dump-config` 查看完整的配置树

## 回滚方法

如果修改导致问题，可以通过以下方式回滚：

1. **Git 回滚**：`git checkout -- studio/dsh-plugin-hub/`
2. **移除依赖**：从 `data/studio-home/profiles/vivy-studio/package.json` 中删除 `dsh-plugin-hub` 相关行
3. **重新安装**：`cd data/studio-home/profiles/vivy-studio && pnpm install`

## 注意事项

### ⚠️ 修改源码后必须刷新 profile 副本

Windows 上 pnpm 对 `file:` 依赖是**复制**到 `data/studio-home/profiles/vivy-studio/node_modules/`，不是符号链接。改完 `studio/dsh-plugin-hub/` 的源码后，必须刷新副本，否则运行中的 Studio 仍用旧代码：

```powershell
cd studio/dsh-plugin-hub
# 重新构建（Windows 无 rm，按下面命令）
Remove-Item -Recurse -Force lib -ErrorAction SilentlyContinue
npx tsc -p tsconfig.server.json
npx tsdown
node scripts/tools/normalize-client-banner.mjs
# 刷新 profile 副本
cd ..\..\data\studio-home\profiles\vivy-studio
Remove-Item -Recurse -Force node_modules\dsh-plugin, node_modules\.pnpm\dsh-plugin@* -ErrorAction SilentlyContinue
pnpm install
```

`launch-vivy-studio.ps1` 在非首次启动时也会执行 `pnpm install` 刷新副本。

### ⚠️ 不要使用 `dsh plugin add` 命令

对于 Vivy Studio 的第一方开发，应该直接修改源码树中的 bundle，而不是通过 `dsh plugin add` 安装外部包。后者会安装到 profile 的 `node_modules` 中，不在版本控制范围内。

✅ **正确做法**：编辑 `studio/dsh-plugin-hub/` → 提交 Git → 重启 Studio

❌ **错误做法**：`dsh plugin --profile vivy-studio add <remote-package>`

---

## Vivy Studio 密封改造（本构建与上游 dsh-plugin-hub 的差异）

本构建名称定为 **VIVY-STUDIO-PLUGIN-HUB**，作为 Vivy Studio 第一方插件中心，并去掉了原版的全部升级/广告路径：

| 项 | 处理 |
|---|---|
| 名称 | UI 标题 / 关于弹窗 / README / package description 统一改为 VIVY-STUDIO-PLUGIN-HUB |
| 推荐 / 广告横幅 | 移除（`推荐` 徽章与收录横幅整块删除） |
| 可更新检测 | 移除「可更新 / 有更新」徽标、更新按钮、更新通知与启动检查（`checkUpdatesOnStart` 设置已删除） |
| Hub 自我更新 | 移除 `api.dsh-plugin.org/hub.json` / `about.json` 请求与更新弹窗（不再有远程自主升级） |
| 命令通道 update 动词 | 前后端都只认 `dsh plugin ... add`，`update` 命令粘贴被拒 |
| 引用原作者 | 关于弹窗与 README 注明：基于 [dsh-plugin-hub](https://github.com/dshplugin/dsh-plugin-hub)（DSH Plugin Hub contributors，MIT）魔改 |

## Vivy 源码安装模式（vivy-source-install）

插件商城在 `vivy-studio` profile 下默认启用 **Vivy 源码安装**：把插件放进 Vivy 仓库 `studio/<plugin>/`，作为 `file:` bundle 注册进 Vivy Studio profile，而不是安装到系统 `DSH_HOME`。

### 行为（双流程）

按目标来源分两条流程：

- **克隆流程（GitHub 仓库）**：`git clone https://github.com/<owner>/<repo>.git → studio/<slug>/` → 有 build 脚本则 `pnpm install && npm run build`。
- **下载改装流程（npm 包）**：`npm pack <pkg>` 下载 tarball → 解压成 `studio/<dir>/` 源码目录。npm 发布物自带构建产物（打包时构建配置不会进入 tarball，本地 build 必然失败），故**跳过本地构建**，直接校验入口文件。

两条流程共用后续：检查 `dsh.bundle` → 校验入口文件（缺失即失败回滚）→ 写入 profile `package.json`（`file:` 依赖 + bundle）→ `pnpm install` → 登记 `data/studio-home/profiles/vivy-studio/vivy-source-plugins.json`（GitHub 记 `owner/repo`，npm 记 `npm:<包名>`）。

- **卸载**：从 profile `package.json` 与注册表移除，**保留** `studio/` 下的源码目录（避免误删本地改动）。
- **失败回滚**：新建目录在失败（无 bundle / 构建失败 / pnpm 失败）时自动删除，不在 `studio/` 留残留。

> 说明：本构建无「更新」按钮/检测；源码安装的插件要更新，直接进 `studio/<plugin>/` 改源码或 `git pull`，重启 Studio 生效。

### 设置开关

`设置 → 网络与通道 → Vivy 源码安装`，默认开启。关闭后恢复官方 `dsh plugin --profile <p> add <target>` 系统目录安装。

### 关键文件

| 文件 | 作用 |
|---|---|
| `src/server/services/install/vivy-source.ts` | 克隆 / 构建 / 注册 / 卸载引擎 |
| `src/server/services/install/task-queue.ts` | 入队分支：vivy-studio 走源码路径 |
| `src/server/http/routes.ts` | 安装路由：vivy 模式跳过 npm 反查与预检 |
| `src/server/services/settings.ts` | `vivySourceInstall` 设置持久化 |
| `src/client/components/views/SettingsView.tsx` | 设置开关 |
| `launch-vivy-studio.ps1` | 注入 `VIVY_ROOT` + 合并注册表（重启不丢插件） |

### 重启不丢失

`launch-vivy-studio.ps1` 每次启动读取 `vivy-source-plugins.json`，把已源码安装的插件合并回 profile `package.json`，因此重启 Studio 后源码安装的插件仍然保留。
