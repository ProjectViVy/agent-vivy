# CH-C5 — verification

日期：2026-08-30。工作区：worktree `agent-vivy-channel-c2`（顺序复用），分支 `feat/channel-c5`，基线 5374d6f（含 C1–C4）。

## GOAL 运行方式（子代理分工）

- explore（只读）：扫 RPC 分发/ControlDeps、settings overlay 机制（唯一运行时持久化路径）、Host inspect 缺口、UI channel 三件套 + i18n + 测试/е2e 格局。
- executor（写盘）：23 文件（Go 四包 + UI 十三文件 + 测试）。
- reviewer（只读独立评审）：**PASS**，无 blocker；2 条 should-fix（log/TODO、分支名）均为落地步骤，已处理。
- 浏览器冒烟：GOAL 持有人亲做（smoke-for-user-visible-change）。

## 命令与结果

| 命令 | 结果 |
|---|---|
| `gofmt -l ./internal ./sdk ./cmd` / `go build ./...` / `go vet ./...` | 干净（executor） |
| `go test ./...`（executor） | 全绿 |
| `just ci`（executor） | 绿：ui-ci 21 文件 / 172 测试（较 175 减 3 = email/neuro-link 平台用例随功能移除）+ typecheck + vite build |
| reviewer 复核 `go test ./internal/{rpc,channelhost,app,app/settings}/... -count=1` | 全 ok |
| reviewer 复核 `pnpm vitest run`（channel-schema/store/api/i18n） | 41/41 pass |
| 秘钥面 grep（reviewer） | RPC 全表面仅 `TokenEnv`(名) + `TokenEnvSet`(bool)；`token_value` 不存在 |
| localStorage grep（reviewer） | `vivy.ui.channels` 零读取残留 |
| i18n 死键双向 grep（reviewer） | 12 键清除、零悬挂引用 |

## 浏览器冒烟（真实路径，非截图验收）

| 步骤 | 结果 |
|---|---|
| `just run`（默认身体 8787）+ `pnpm dev`（3015）+ 打开 `/settings?tab=channels` | 空态「这一代没有耳朵」+ 指引；无添加按钮、无 email/neuro-link；截图留档 |
| `go run ./sdk pack --with telegram --out <scratch>` → 候选 EXE + scratch config（mock provider；`channels.telegram` enabled + allow_from + 双处 token_env；env 故意不设） | 候选起、telegram 卡片可见（已启用/需配置），`start failed: …` 原因逐字展示 = fail-closed 全链可感知（settings 解码 → 信封钉名 → Secret env 解析失败） |
| 编辑器 | 「每行一个发送者；留空 = 拒绝启动」+ token_env 名 + 「未设置」徽章 + D-010 说明 |
| 清空 allow_from 保存 | `settings.yaml` 落盘 `allow_from: []`（拒启语义持久化） |
| 恢复两行名单保存 | overlay 更新为两行（UI→RPC→settings.yaml 全链） |
| 窄视口 375×720 | 卡片/操作钮正常，无横向溢出 |

冒烟环境注记：起候选时发现并清掉了占住 3015 的根树残留 Vite dev server（服务旧代码，会污染冒烟）；scratch 配置/数据全部在系统临时目录，空气墙无触碰；冒烟后进程与 scratch 全清。

## 范围看守

- 24 文件改动全部在 CH-C5 §5 清单方向内（rpc / app / app/settings / channelhost / ui channel 面 / i18n）；`internal/config`、插件、`zz_register.go`、go.mod 零触碰。
- 未 push。
