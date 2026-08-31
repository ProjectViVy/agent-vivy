# Verification — 2026-08-30 Studio 插件中心卸载闭环 + 自动提交

工作目录：`C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy`
（`studio/` 子模块 HEAD 5b6cb4e，本次改动全部落在子模块 `dsh-plugin-hub` 内）。

## 单元测试 / 类型检查 / 构建

```text
cd studio/dsh-plugin-hub

# 依赖（本子树无 node_modules，先装 devDeps；pnpm 全局 store 复用，无外网依赖）
pnpm install --prefer-offline
# -> resolved 112, added 46, done in 5s

# 类型检查（client / server / test 三套 tsconfig）
npm run typecheck        # -> 全绿（修复前置断裂前：catalog.test.ts 引用已移除的
                         #    installCommandOf 报 TS2305；已修复）

# 全量测试（node --test，Node v24.14.1，type stripping 原生支持 .ts）
npm test                 # -> 56 pass / 0 fail
                         #    新增 tests/vivy-source.test.ts 6 用例：
                         #    resolve 路径解析（注册表优先 / file: 回退 / 越界拒绝）
                         #    commitVivySourceChange（路径限定 / 无改动不空提交 / 非仓库跳过）
                         #    deleteVivySourcePluginDir（删除 / 脏仓库警告 / 缺失目录）
                         #    卸载闭环联调（删目录 + 提交留痕 + 无关改动不受影响）
                         #    前置断裂修复：install-target.test.ts 的 update 断言对齐
                         #    密封契约（仅 add 剥命令，update/remove 原样返回）

# 构建（Windows 下手动执行 build 脚本等价步骤；npm run build 的 rm -rf 在 cmd 不可用）
Remove-Item -Recurse -Force lib
npx tsc -p tsconfig.server.json      # exit 0
npx tsdown                           # client/client.js 360.47 kB, ok
node scripts/tools/normalize-client-banner.mjs  # banner ok
# 抽查编译产物：lib/services/install/vivy-source.js 含 commitVivySourceChange /
# deleteVivySourcePluginDir / vivySourceAutoCommit / sourceDeleted —— 确认新逻辑进包
```

## 运行时冒烟（插件中心在运行的 Studio 中加载）

- 刷新 profile 副本：
  `data/studio-home/profiles/vivy-studio` 下 `pnpm install`（file: 复制更新
  `node_modules/dsh-plugin`，含新 lib + client）。
- 按 vivy-studio 无热重载规则**分离进程重启** Studio（`restart-studio.ps1`
  方式，避免树内自杀），等待端口 3090 恢复后：
  - `GET /dsh-plugin-hub/settings` 返回含 `vivySourceAutoCommit: true`；
  - `GET /dsh-plugin-hub/installed` 返回含 `vivySourcePaths` 字段；
  - Web UI 插件中心正常加载 / 设置页可见「源码插件自动提交」开关。

## 未执行（记录原因）

- `just ci`：该 gate 管宿主 Go / UI 仓；本次交付全部位于 `studio/`
  子模块（独立 git 仓与 gate：typecheck + test + build 已全绿）。
- 真实网络安装→卸载演练：需外网克隆第三方仓库，且会写入当前 Studio
  profile 注册表（污染用户环境）。闭环核心路径已由临时目录 + 临时 git
  仓库的确定性单测覆盖（无网络依赖）。

## 缺陷/边界说明

- 嵌套克隆仓库（clone 流程）含未提交改动被卸载时，改动随目录删除（有
  警告）——产品决策，非缺陷。
- 自动提交在子模块 detached HEAD 上同样产生提交（本地留痕），不 push，
  分支整理由用户决定。