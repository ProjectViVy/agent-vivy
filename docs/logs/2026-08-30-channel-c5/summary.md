# CH-C5 — inspect + 设置页接后端（summary）

日期：2026-08-30。分支 `feat/channel-c5`（自 `feat/channel-c4` 5374d6f 切出；顺序切片复用同一 worktree）。
PLAN：`docs/plans/channel-epic/CH-C5.md`（领取 UI-CHANNELS-BE，未另开 lane）。合同：`VIVY-CHANNEL-PACK.md` §11/§13。

## 做了什么

设置→通道 从「七平台 localStorage 幻想」变成「这一代身体真实编译进的耳朵」。住户看见的名单 = `Register()` 的 SeamChannel 分区，而不是一张愿望单。

1. **Host inspect 面**（`internal/channelhost`）：`StartAll` 记录每耳结果注释（未配置 / 已停用 / 空 allow_from 拒启 / 启动失败原因 / 正常）；`Inspect()` 确定序输出 `ChannelStatus{Name, Capabilities, Configured, Enabled, Started, TokenEnv, TokenEnvSet, Note}`——`TokenEnvSet` 只报 bool（`LookupEnv` 非空），值永不出内核。
2. **settings overlay 走 channels**（裁定：`channel/update` 写 `settings.yaml`，不写 config.yaml——沿用 `MCPServers` 先例）：`Settings.Channels []ChannelOverlay` 指针字段（`Enabled *bool` / `AllowFrom *[]string` / `TokenEnv *string`，未设置 ≠ 设空）；`applySettingsOverlay` 逐名合并到 `cfg.Channels`，**保留 config.yaml 的 opaque settings 节点**（测试钉住：覆盖 enabled/allow_from 后 `parse_mode` 等仍活着），幽灵名字 Warn 丢弃绝不挡启动。耳朵在进程重启时生效（合同 §11「今晚关耳朵」语义），不做热重启。
3. **RPC**：`channel/inspect`（compiled-in 全集 + 能力 + 状态 + 注释）、`channel/get`（单耳信封，无密钥值）、`channel/update`（写 overlay；未编译名 → `InvalidParams "channel %q is not compiled into this generation"`；`*` 在 settings 校验与 config 校验双闸拒绝；Frozen / 只读部署走既有 Conflict 门）。initialize capabilities 增三条。未知名字文案与 `partitionChannels` 启动失败完全一致。
4. **UI 重做**：列表 = inspect 全集（编译进才可见；空身体 → 「这一代没有耳朵」空态且无添加按钮）；添加向导可选项 = 编译进但未配置的名字；email / neuro-link 从 schema/platforms/icons/i18n 全数移除；`allow_from` 文案改 fail-closed（zh「每行一个发送者；留空 = 拒绝启动」/ en "empty = start refused"，i18n 键控，zh/en 结构对齐）；token 只显示环境变量名 + 「未设置」徽章 + D-010 说明（无任何密钥值输入框）；`pendingRestart` 徽章对比文档态与进程态（enabled/configured/token_env）。localStorage `vivy.ui.channels` 不再被读（无迁移、不迁密钥）。
5. **测试**：Go——overlay 合并/保留/幽灵名、settings 往返（显式空 allow_from ≠ 未设置）、rpc inspect/get/update 全门；UI——store 全面 mock api（服务端真相、legacy 忽略、从不发明 token_env）、schema 五平台断言、i18n 结构对齐（12 个死键清除，双向 grep 无悬挂引用）。

## 浏览器真实路径冒烟（`http://127.0.0.1:3015`，桌面 1280 + 窄视口 375）

- **默认身体**（`just run`）：通道页空态「这一代没有耳朵」+ 指引文案；无 email/neuro-link、无添加入口。截图留档。
- **pack telegram 候选**（`pack --with telegram` + scratch 配置 + mock provider，未触碰真实 Telegram 网络）：telegram 卡片出现（已启用 / 需配置徽章），`start failed: …TELEGRAM_BOT_TOKEN…` 原因逐字可感知（token 未设 → Secret fail-closed 全链：settings 解码 → 信封钉名 → env 解析）；编辑器显示「每行一个发送者；留空 = 拒绝启动」、token_env 名字 + 未设置徽章；保存 `allow_from: []` → `settings.yaml` 落盘 `allow_from: []`（空名单 = 拒启语义持久化成功）；再存两行名单 → overlay 更新成功。窄视口无横向溢出。
- 冒烟后清理：候选 EXE / Vite / scratch 全部清除；根树零触碰（顺带清掉了占住 3015 的根树残留 Vite）。

## 明确没做（不做声明）

- 耳朵不热重启（写配置后需重启进程生效；inspect + pendingRestart 徽章表达此事）。
- `allow_from` 改动不进 inspect 面 → pendingRestart 探不到纯 allow_from 编辑（后继可加 wire 字段，已登记）。
- 旧 localStorage 键不清理不迁移（忽略优于错迁密钥，已登记）。
- inspect 失败时卡片视图回落空态 + 头部错误条（reviewer note #11，可改进）。
- toggle 会把文档态 allow_from 固化进 overlay（有效值不变；reviewer note #12）。
- 会话列表不过滤 `sess_ch_*`（诚实可见性，本刀未加过滤）。
