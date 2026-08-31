# Acceptance — 2026-08-31 default-full-channel-body

## 人怎么确认它生效了

1. 在仓库任意新检出（或现有 root）直接 `just run`，打开
   `http://127.0.0.1:3015`（分体 Vite）或 `http://127.0.0.1:8787`（内嵌）。
2. 左侧「设置」→「通道」：应看到 **钉钉、Discord、飞书、QQ、Telegram**
   五张通道卡片，而不是「这一代没有耳朵」空态。
3. 每张卡片显示「已禁用 · no config envelope」——这是预期：通道已编入
   （compiled-in），但还没有配置信封；点「启用」配置 `token_env` 与
   `allow_from` 后重启进程才会真正起耳朵。
4. `channel/inspect` 语义不变：未配置的通道显示为编译可见、未启动，
   日志里对应 `channelhost: channel compiled-in but not configured; not started`。

## 回归视角

- 想要一个**没有**某通道的窄代：`vivy-sdk pack --with <想保留的插件>`，
  产物 `dist/<gen>/vivy.exe` + `generation.json`；那个二进制的设置页
  才会重新出现空态（如果没带任何通道）。
- `docker-up` / `just build-split` 与日常 `just run` 现在是同一种全量
  代，不再有"打包才有耳朵"的差别。
