# Verification — 2026-08-31 default-full-channel-body

全部命令在 worktree `../agent-vivy-full-channels`（分支
`feat/default-full-channels`）执行。

## 构建

| 命令 | 结果 |
|---|---|
| `go build ./...` | OK（需先 `ui: pnpm build` 产出 embed dist） |
| `go build -o vivy-sdk.exe ./sdk` | OK |
| `vivy-sdk verify plugins/telegram` | ok（其余 4 通道由 sdk 测试内 verify 覆盖） |
| `vivy-sdk pack --with telegram --out dist/pack-smoke` | 修复前：`conflicting replacements for example.com/vivy/plugins/telegram`（复现幂等缺口）；修复后：作为 `sdk/internal` 测试套的一部分通过 |

## 门禁

| 命令 | 结果 |
|---|---|
| `just ci`（fmt-check + vet + test + headless-compile + ui-ci） | **最终全绿（exit 0）** |
| `go test -count=1 ./...` | 一次 `TestCronAtJobDeletesAfterSuccessfulRun`（internal/runtime）6.15s 超时失败；隔离重跑两次均绿（0.66s / 12.8s），判定为既有时钟敏感偶发，与本改动无关 → `docs/TODO.md` §0.1 `TFLAKE-CRON` |
| `just test`（两次复跑） | 全绿 |
| `go test ./sdk/...` | 全绿（含新增幂等/parseReplaceTargets 测试、真实 pack 构建） |

## 真路径冒烟（浏览器，分体 Vite）

- worktree 后端：`VIVY_CONFIG=data/smoke/config-smoke.yaml ./dist/smoke-vivy.exe`
  → 监听 127.0.0.1:**8788**（8787 被用户既有 root 后端占用，改端口），
  启动日志 5 条 `channelhost: channel compiled-in but not configured; not started`
  （telegram/dingtalk/discord/feishu/qq 各一）。
- worktree Vite：`VIVY_BACKEND_ADDR=http://127.0.0.1:8788 pnpm exec vite --port 3016`。
- 浏览器（IAB）打开 `http://127.0.0.1:3016/settings?tab=channels`：
  - 通道页渲染 5 张卡片：钉钉 / Discord / 飞书 / QQ / Telegram，
    全部「已禁用 · no config envelope」，带 启用 / 编辑 / 关闭耳朵 控件；
  - **「这一代没有耳朵」空态不再出现**；
  - 截图：
    `C:\Users\Administrator\.zcode\cli\artifacts\sess_1c6805f3-d9ba-441a-a6da-29fc6540ad45\call_a48084bf968d4b8d964ed41b-tool-result-a7404cd1-14b9-4114-887f-ce69ca692872.png`
- 冒烟后已停掉 8788 后端与 3016 Vite，端口释放。

## Air gap 与隔离

- 冒烟配置 `data/smoke/config-smoke.yaml` 把 `data_dir`/`sqlite.path`
  钉在 worktree `data/smoke/home`，未触碰仓库根 `data/`。
- 首次冒烟尝试未带 `VIVY_CONFIG`，进程在绑定 8787 失败退出前打开了默认
  数据根 `~/.vivy`（该目录 2026-08-30 已存在，为既有开发数据根；本次仅
  幂等迁移/读查询，cron 表缺失告警与 8/30 以来的既有状态一致）。详见 notes。

## 提交

- 分支 `feat/default-full-channels` 单提交；`ui/src/routeTree.gen.ts`
  仅为 dev server 行尾噪音，已还原不入提交。
- fast-forward 合回 `main`；未 push。
