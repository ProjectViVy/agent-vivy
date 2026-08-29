# 2026-08-29 · Vivy Console drop runtime.mock

## 目标

修复 Studio「Vivy 控制台」启动后端失败：

`field mock not found in type config.Runtime`

## 根因

2026-08-29 真实 LLM 闭环已从产品 `config.Runtime` 删除 `runtime.mock` /
`mock_scenario`（见 `docs/logs/2026-08-29-real-llm-loop/`）。Studio console
的 `prepare()` 仍向 `data/studio-home/vivy-console/config.yaml` 写入
`runtime.mock: true`；配置严格解码（`KnownFields(true)`）导致启动 abort。

## 变更

- `studio/dsh-vivy-console/index.js`
  - 生成配置不再写 `runtime.mock`
  - 子进程额外设置 `VIVY_USER_HOME=<scratch>/data`，settings/Journal 继续隔离在 Studio scratch（ST-2）
  - 启动成功文案改为「真实 provider · 数据隔离」
- `studio/dsh-vivy-console/README.md` — 去掉 mock mode 描述
- 同步已安装 profile 副本：
  `data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-console/`
- 修复已生成的坏配置：`data/studio-home/vivy-console/config.yaml`
- 顺手去掉根目录本地 `config.yaml` 里的过时 `runtime.mock: false`（gitignored）

## 明确不做

- 不恢复产品 mock provider
- 不改内核 `internal/config`（字段删除已完成）
- 不触碰 tenant Journal（`data/vivy.db` / `data/demo/` / `data/workspaces/` / `~/.vivy`）
