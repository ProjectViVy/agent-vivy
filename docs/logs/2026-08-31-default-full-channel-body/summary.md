# 2026-08-31 主线默认全量通道本体（default-full-channel-body）

## 概要

主线提交的物种身体从空表改为**全量本体**：`internal/generated/plugins/zz_register.go`
注册全部 5 个第一方通道插件（telegram、dingtalk、discord、feishu、qq）。
`just run`、内嵌 UI 二进制、Docker、`just build-split` 开箱即有全部耳朵；
设置 → 通道不再出现「这一代没有耳朵」空态。

背景：用户在设置页看到空态文案，确认决策为「开发默认做全量版本」。

## 变更范围

- `internal/generated/plugins/zz_register.go` — 空表 → 5 通道全量注册。
  文件头从 "Code generated / DO NOT EDIT" 改为手写本体声明；pack 的
  `-overlay` 仍在构建期替换同一路径，窄代装配不受影响。
- `go.mod` / `go.sum` — 根模块 require+replace 5 个插件独立模块
  （`example.com/vivy/plugins/<name>` => `./plugins/<name>`），并合并
  插件三方依赖闭包（telego、dingtalk-stream-sdk、discordgo、oapi-sdk-go、
  botgo 等）。
- `plugins/discord/go.mod|go.sum`、`plugins/qq/go.mod|go.sum` — 重新
  `go mod tidy`。根因见 notes：插件 `replace agent-vivy => ../..` 会把根
  模块整个依赖图拉进插件 MVS，全量本体抬高共享依赖（x/net → v0.50 带
  动 x/crypto → v0.48）后，插件旧 pin 失配，`go list`（readonly）报
  "updates to go.mod needed"，verify 随之失败。
- `sdk/internal/pack.go` — `-modfile` 合并幂等化：root go.mod 已
  require+replace 的插件模块跳过追加（重复 replace 是
  conflicting-replacement 构建错误），三方闭包仍照常合并。新增
  `parseReplaceTargets` 解析（单行 + block 形态）。
- `sdk/internal/pack_test.go` — 新增幂等测试（root 携带 telegram 时合并
  结果恰含一条 require + 一条 replace，且 go-command 可解析）与
  `parseReplaceTargets` 单测；`TestOverlayGoModMergesPluginRequires`
  改用 root 未携带的合成模块（merge 路径仍被真实覆盖）；MVS drift 测试
  的 scratch 校验对本地 replace 目标做绝对化；删除「物种 go.mod 不得
  出现 telego」的过时断言（全量本体下合法，真正的持不变量是 pack 不改
  动 live 文件，字节级断言已在）。
- `docs/architecture/VIVY-CHANNEL-PACK.md` — 「默认提交的物种身体为空」
  契约改述为「主线提交全量本体，pack 构建期收窄」。
- `docs/TODO.md` §0.1 — 记录 `TFLAKE-CRON` 偶发（与本改动无关的既有
  时序脆弱测试）。

## 明确未做

- hello-fs（tool-world 演示件）**不**进全量本体——「全量」只指通道。
- UI 空态文案「这一代没有耳朵」保留：pack 出的窄代（如纯 tool 组合）
  仍会正确显示它。
- 命名打包配方（`generations.yaml` / `just pack <name>`）未做，另立后续。
- Studio 生命周期流程未动；pack → eval → release → install 语义不变。

## 提交

单 Concern 提交于 `feat/default-full-channels`，fast-forward 合回 `main`。
未推送（需用户显式授权）。
