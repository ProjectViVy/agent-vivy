# 可拔插前端研究：DSH 对照与 Vivy 落地方案

日期：2026-08-30。性质：研究文档（非立项承诺）。

## 0. 问题定义

> 如果要实现类似于 DSH 的、可拔插的前端设计（例如，我不装某个插件，某个插件就不会显示 UI），要怎么做？

拆成四个子问题：

1. "某个插件装没装"这件事，状态存在哪里、谁说了算？
2. 插件的 UI 以什么形态存在——数据、还是代码？
3. 前端怎么知道"有哪些插件、它们各自贡献了什么 UI"？
4. 卸载/换代之后，UI 怎么消失？

**一句话结论**：在 Vivy 里"安装"已经是编译期事件（`vivy-sdk pack` 把插件静态链接进新一代 EXE），所以"不装就没有 UI"不需要发明新的安装/卸载机制。缺的只有两件事：内核把"编译进本代的插件及其声明的 UI 描述"通过 RPC 报出去；Web UI 把现在硬编码的导航/设置页/工具卡改成"从内核报告渲染"的槽位。DSH 的价值是它把这套东西跑通了，并且提供了三层可借用的机制和一条信任边界教训。

## 1. DSH 是怎么做的

来源：`.workspace/deepseek-harness/deepseek-harness/`（本地工作克隆，本研究的 ground truth）。

### 1.1 总体形态

- TypeScript/Node monorepo，**一切皆插件**（vendored Cordis 框架）：agent loop、tool registry、session log、model adapter 全是插件，`docs/architecture.md`："Every part of the product is a plugin … so each is replaceable from configuration."
- `dsh` 启动一个 **profile** = 有序 bundle patch（`cordis.patch.yml` 行：`id/name/config/disabled`）+ 用户 overlay；`dsh plugin` 命令是 pnpm 前转发 + 对 `dsh.profile.bundles` 的组合协调（`apps/cli/src/plugin.ts`）。
- **前端是浏览器里的第二个 Cordis context**（`packages/client/web/src/boot.ts`）。web / headless / sdk(stdio JSON-RPC) / acp 四种前端**纯靠组合选择**，同一引擎内核。

### 1.2 机制 A：client module system —— 挂载决定送达

- 服务端（`packages/client/modules/src/index.ts`）扫描 Loader entries 里声明了 `dsh.client` 的包，合成 `window.__DSH_BOOT__` 入口图，**只为挂载的插件服务浏览器 bundle**。
- 浏览器半（`system.ts`）按图加载并 `window.__ModuleLoader__.load({id, factory})` 注册工厂。
- 效果（cookbook 原话）："the plugin appears on the page as soon as a `cordis.yml` mounts it — **no rebuild of the web application**"。没挂载 → bundle 永不送达 → `apply()` 不跑 → **UI 从未存在过**，而不是"被隐藏"。

### 1.3 机制 B：slot registry —— 页内可逆注册

- 插件 `ctx.slots.register({name, key, order, …}, Component)` 把 React 组件注册进类型化槽位（`single | list | keyed | chain`，`packages/client/ui-slots`、`ui-renderer/registry.ts`）。
- 注册本身是 `ctx.effect`（Cordis 可逆 effect）——**卸载插件即自动撤下组件**，无需手写清理。
- 框架槽位：`root / sidebar / conversation / details / shell.overlay / settings tabs / tool.call.<name>`；未认领的 key 回落通用渲染器，认领则替换。

### 1.4 机制 C：纯数据描述符 + 通用渲染

- slash 命令：host 侧 `ctx.commands` 注册，composer 每会话经 `command.list` 拉目录（`ui-commands/directory.ts`）→ 菜单里只有挂载插件的命令。
- 工具卡：工具声明 **presenter intent**（`card: 'terminal' | 'diff' | 'read' | …`）+ 持久化 `presentationMeta`；UI 从原始 run 事件 + intent 推导卡片；没有专属卡的键回落 generic row。
- 这一档**零插件代码进浏览器**，却覆盖了最高频的"插件可见性"场景。

### 1.5 信任模型（教训）

- 页内**没有沙箱**：插件 UI 拿全页权限；隔离只有构建期 bundle purity gate（禁止跨插件值导入）+ 网络 trust fence（防 DNS rebinding，明确"不是 auth 层"）。
- Agent 运行时写的动态包：固定全局（React/console/styles/host）+ 每次人工审批，且文档自认"不是安全边界，当 bash 用"。
- 结论：DSH 用"信任安装源 + 构建期纪律"换灵活性。这套取舍对开发者工具成立，对租户日常产品（Vivy 的 `vivy.exe`）要重新做——`docs/architecture/VIVY-STUDIO.md` §9.1 已经对"任意 CSS 进设置"说不（"信任模型不对"）。

### 1.6 清单页

- `packages/host/plugin-inventory`：`PluginInventoryGateway.list()` 读 `ctx.loader.entries()` 返回每个 entry 的 `{entryId, moduleName, enabled, fiberPhase}`，由只读设置页渲染。"我装了什么"本身是一个一等 UI。

## 2. Vivy 现状

### 2.1 插件生命周期是编译期的

- manifest `vivy.plugin/v0`：`apiVersion/name/version/seam(tool|tool-world|provider)/module/grants(fs.read|fs.write)/tools[]`（`sdk/internal/manifest.go:21-36`）。**没有任何 UI 字段**。
- `vivy-sdk pack`：verify → 生成注册文件 → `go build -overlay` 覆盖 `internal/generated/plugins/zz_register.go` → 静态链接出新一代 `dist/<gen_id>/{vivy.exe, generation.json}`。generation.json 冻结 `Recipe{Loop, World, Plugins[]}` 与 `Tools[]`（`sdk/internal/pack.go`）。
- **卸载 = 配方删一行再 pack**（`docs/architecture/VIVY-PLUGIN-SPEC.md` §7/§8："插件崩溃 = 这一代 EXE 崩溃。隔离不在进程，在下一代"）。没有运行时启停、没有插件注册表、没有热载。

### 2.2 内核对插件的运行时暴露（缺口所在）

- `species/inspect`（`internal/studio/inspect.go`）：报 `generation_id`、`recipe.plugins`（仅名字）、tools、grants。这是唯一动态反映"本代装了什么"的方法。
- `initialize`/`capabilities`（`internal/rpc/control.go:345-360`）：**硬编码静态字符串表**，与插件无关。
- UI 把 `capabilities` 存进 Zustand（`ui/src/lib/store.ts:33,211`）但**零消费**——通道现成，空转。
- 没有 `plugins/list`，没有任何 UI 描述符。

### 2.3 Web UI 是封闭单包

- `go:embed ui/dist` 单 bundle（`ui/embed.go`）；导航是三组硬编码数组（`ui/src/components/chat/ConversationSidebar.tsx:7-9` 的 `NAV_ITEMS/VIVY_ITEMS/TOOL_ITEMS`）；设置页 tab 集合静态；i18n 双字典封闭（zh 权威、en 结构孪生、测试强制一致）。
- 已有的"后端报告 → 条件渲染"先例：`settings.read_only` 门禁保存按钮、`settings/providers`、`settings/mcp`、`skills/list` 的"后端列表 → 渲染卡片"模式。这就是方案一的雏形。

### 2.4 Studio 里已经有 DSH 机制在跑（kernel 外的现成范本）

- `studio/dsh-vivy-studio/index.js`：服务端插件 `webServer.tapIndex` 往 index.html 注入 `<style data-plugin>`/`<script data-plugin>`；启动时读文件、重启生效、无热载。
- `studio/dsh-vivy-console`：客户端半 `window.__ModuleLoader__.load({id, factory})` + `dsh.client.inject`，注册 `conversation.view` 页签。
- `studio/dsh-better-sidebar`：`ctx.slots.inject('conversation.chat.turnTail', () => ctx.slots.register(…))`。
- 即：DSH 的机制 A/B 在 Studio 壳里已实际运行。缺的是 Vivy **自有 web UI（租户产品）**里的对应物——而这正是"可拔插前端"该落的地方。

### 2.5 相关提案：VIVY-FACE-PACK 的 `seam: face`

- `docs/architecture/VIVY-FACE-PACK.md`（提案，未实现）：Face = 配方上的可编译器官，一代一张主脸；`RegisterFace()` pack overlay；"活着的 EXE 不动态加载任何 face 代码"。
- 粒度辨析：face 是**整张壳**（web|tui|headless），本研究讨论的插件 UI 贡献是**壳内的面板/卡片/命令/页签**。两者正交：face 换壳，插件 UI 填壳。方案设计不应与 FACE-PACK 冲突——插件 UI 描述符只在 `faces/web` 这张脸里有意义。

## 3. 方案：三档信任光谱

### 方案一（推荐先做）：UI as data —— 描述符通道

原则：**插件永远不往浏览器送代码，只送数据**；渲染器是内核（tier C）一次性建设的通用组件。

- **manifest**：`vivy.plugin/v1` 增加可选 `ui` 块，白名单极小，i18n 内联（对齐 zh 权威/en 孪生惯例）：

  ```json
  "ui": {
    "nav": [{ "path": "/hello-fs", "title": { "zh": "文件探针", "en": "File Probe" }, "icon": "folder" }],
    "settings": { "schema": { "type": "object", "properties": { "root": { "type": "string" } } } },
    "toolCards": [{ "tool": "hello_stat", "card": "kv" }]
  }
  ```

- **pack**：新机制沿用现有套路——pack 在生成 `zz_register.go` 覆盖文件的同时，生成 `zz_ui.go`（`func UI() map[string]json.RawMessage`），把各插件 manifest 的 `ui` 块原样冻结进 EXE。**manifest 保持单一事实源，SDK 的 Go 接口窗口不动，插件作者零负担**，且运行时不依赖 `generation.json`/`install.json` 是否在旁边。
- **kernel**：新增只读 RPC `plugins/list`（加法，不动 `initialize` 握手语义——那会影响 TUI/worker 等所有客户端），数据源 = `genplugins.Register()` + `genplugins.UI()`。不新增任何账本表（NG-28 安全：这不是 generation/eval/promotion 的产品语义扩展）。
- **UI**：store 启动时拉 `plugins/list` → 渲染侧边栏"插件"组（挂在 `TOOL_ITEMS` 旁）、设置页插件卡（schema→表单，复用 MCP/providers 卡片模式）、工具卡查 `toolCards` 表。**未装 → 列表为空 → 什么都不画**。
- **得到什么**：严格满足"不装就不显示"；零浏览器代码执行；插件作者仍只碰 `plugins/<name>/`（tier A）；内核改动一次性（tier C，此后不再随插件增长）。
- **局限**：UI 表达力 = 渲染器词表。

### 方案二：schema 驱动富组件 + presenter intents

在方案一之上扩 widget 词表（表单/表格/KV/diff/徽章/链接），并采纳 DSH 工具卡思路：工具声明 presenter intent，UI 从 run 事件 + intent 推导，**未声明回落 generic row**。仍然零插件代码进浏览器。词表字段需要 parse/validate 测试（对齐"新配置字段需要测试"的惯例）。

### 方案三：插件自带前端代码（DSH 机制 A/B 的 Vivy 化）—— 缓行，或仅限 first-party

- pack 额外收插件 `ui/` 预构建资产（ESM/懒 CJS bundle + CSS），随 generation 一起被内核服务；Vivy UI 增加模块装载器 + 槽位注册表。
- 消失语义用**代际**替代 DSH 的热卸载：换代（重装/重启）即 UI 换血，不需要 Cordis 的可逆 effect——反而更简单，且与"活着的 EXE 不动态加载代码"的既定立场一致。
- 前置决策（做完方案一/二再议）：沙箱选型（iframe/Web Components vs DSH 式不沙箱+构建期 purity gate）、CSP 与资产完整性、§9.1 first-party sealed skin 与租户插件 UI 的两套信任级别是否长期并存。
- 成本最高、治理最重。若立项，建议只对 first-party 插件开放，或作为 FACE-PACK 之后的独立 track。

### 三档对比

| | 方案一 描述符 | 方案二 schema 卡 | 方案三 自带代码 |
|---|---|---|---|
| 浏览器执行插件代码 | 无 | 无 | 有 |
| 表达力 | 低（词表） | 中（词表扩） | 高（任意 React） |
| 内核/UI 改动 | 小（一次） | 中（词表演进） | 大（资产服务+装载器+槽位） |
| 信任模型 | 无新增 | 无新增 | 需新决策（§9.1 级别） |
| DSH 对应物 | 机制 C | 机制 C+ | 机制 A+B |
| 与现有治理 | 完全符合 | 完全符合 | 需豁免/新规范 |

## 4. "不装就不显示"在 Vivy 的完整链路（方案一视角）

```
pack 不带该插件 → 不进 zz_register.go/zz_ui.go → 本代 EXE 里没有它
→ plugins/list 不报 → UI 槽位无数据 → 不渲染
```

没有任何一步需要"删除 UI"的动作——**UI 的存在性完全由代际内容决定**。这是 DSH 机制 A（挂载决定送达）的编译期版本，且更彻底：连浏览器 bundle 都不曾存在。

配套两件小事：

- **换代感知**：目前 UI 只在启动时拉一次状态。`plugins/list`（或 `species/inspect`）带上 `generation_id`，UI 启动时比对本地记录、不一致则提示刷新。不做热切换——符合全仓"无热载"的既定立场。
- **插件清单页**：对应 DSH 的只读 PluginInventory。Vivy 已有 `species/inspect` 的全部数据，渲染一张"本代插件"只读页即可，天然属于本设计的第一批交付。

## 5. 开放问题

1. manifest 版本策略：`vivy.plugin/v1` 与 v0 的兼容（verify 双读？v0 插件能否继续 pack？）。
2. `plugins/list` 作为独立方法 vs 把 capabilities 动态化：建议加法优先，动态化 capabilities 会改变所有客户端（TUI/worker）的握手语义。
3. 设置卡 schema 允许的数据源边界：只许插件查询自己的工具？还是允许引用通用 stats？这决定 widget 词表是否需要鉴权语义。
4. 方案三若立项：沙箱选型、CSP、以及租户 web UI 与 Studio 壳两套信任级别的关系。
5. TUI（`internal/tui`）是否消费同一套描述符（文本槽位），兑现"多前端、一个插件"。

## 6. 参考路径

DSH：`packages/client/modules/`（机制 A）、`packages/client/ui-slots` + `ui-renderer`（机制 B）、`packages/interaction/commands` + `ui-commands`（命令描述符）、`packages/client/ui-tool`（presenter intents）、`packages/host/plugin-inventory`（清单页）、`docs/architecture.md`、`docs/cookbook/adding-a-settings-card.md`。

Vivy：`sdk/internal/manifest.go`、`sdk/internal/pack.go`、`internal/generated/plugins/zz_register.go`、`internal/pluginhost/host.go`、`internal/rpc/control.go:345-360`、`internal/studio/inspect.go`、`ui/src/components/chat/ConversationSidebar.tsx:7-9`、`ui/src/lib/store.ts:211`、`studio/dsh-vivy-studio/index.js`、`studio/dsh-vivy-console/`、`docs/architecture/VIVY-PLUGIN-SPEC.md`、`docs/architecture/VIVY-FACE-PACK.md`、`docs/architecture/VIVY-STUDIO.md` §9.1。

## 7. 调研路径

### 7.1 本次调研过程（可回溯）

```
AGENTS.md 入口（DSH ground truth 位置 + 治理约束）
→ DSH 源码问题单（.workspace/deepseek-harness/deepseek-harness/）
→ Vivy 现状问题单（sdk/ internal/ ui/ studio/ docs/architecture/）
→ 双线互证（Studio 里的 dsh-* 插件 = DSH 机制的活样本）
→ 关键引用人工抽查（grep/sed 复核 5 处）
→ 三档方案综合
```

**入口与依据**：`AGENTS.md` 把 `.workspace/deepseek-harness/`（`deepseek-harness/` 工作克隆 + `upstream/` 镜像）定为 DSH 行为的 source of truth，并要求以该树而非 `node_modules` 的构建产物为准。调研确认工作克隆完整，`upstream/` 未动用。Vivy 侧以仓库本体与 `docs/architecture/` 产品契约为准。探查由两个并行只读子任务完成（DSH 路线 / Vivy 路线），结论经抽查后才写入正文。

**DSH 路线的问题单与检索锚点**：产品形态与包布局（→ `AGENTS.md` repository layout、`docs/architecture.md`）；前端/后端如何分离（→ `packages/host/webserver`、`packages/client/web/src/boot.ts`）；插件 manifest、发现、装载（→ `docs/cordis-primer.md`、`apps/cli/src/plugin.ts`、`packages/boot/app-boot`）；**UI 是否随安装动态渲染**——本研究的关键问题（→ `packages/client/modules/` 的 `dsh.client` 声明与 `__DSH_BOOT__` 图、`packages/client/ui-slots` 的 `slots.register`、`packages/interaction/commands` 的 `command.list`、`packages/client/ui-tool` 的 presenter intents、cookbook 的 adding-a-settings-card / adding-a-tool）；信任模型（→ `scripts/client-bundle-purity.spec.ts`、`packages/client/connection/src/api-request-trust.ts`、`packages/extensions/cordis-client-runner`）；清单页（→ `packages/host/plugin-inventory`）；理论背景（→ `paper.txt`，Cordis 可逆 effect 的形式化）。

**Vivy 路线的问题单与检索锚点**：插件从 manifest 到 EXE 的全链路（→ `sdk/internal/manifest.go`、`sdk/internal/pack.go`、`internal/generated/plugins/zz_register.go`、`internal/pluginhost/host.go`、`internal/studiocore/service.go`）；运行时暴露与 RPC 面（→ `internal/rpc/control.go` 方法 switch、`internal/studio/inspect.go`）；UI 结构与既有条件渲染先例（→ `ui/src/components/chat/ConversationSidebar.tsx`、`ui/src/lib/store.ts`、`ui/src/components/settings/` 的 providers/MCP 卡片模式、`ui/src/i18n/`）；Studio 侧范本（→ `studio/dsh-vivy-studio/index.js`、`studio/dsh-vivy-console/`、`studio/dsh-better-sidebar/`）；TUI（→ `cmd/vivy/tui.go`、`internal/tui/`、`docs/architecture/VIVY-FACE-PACK.md`）；治理约束（→ `VIVY-PLUGIN-SPEC.md`、`VIVY-STUDIO.md` 的 tier A/C 与 NG-* 决策）。

**交叉验证**：两条独立线索互证——Studio 子模块里的 `dsh-*` 插件是 DSH 机制 A/B 的活样本（`webServer.tapIndex` 注入、`window.__ModuleLoader__` 装载、`ctx.slots.inject` 注册），与 DSH 源码描述一致。写进正文的关键引用另行人工抽查五处，全部属实：`zz_register.go` 由 pack 生成且返回 nil；导航三组硬编码数组（`ConversationSidebar.tsx:7-9`）；`capabilities` 静态表（`control.go:345-360`）；manifest 无 UI 字段（`manifest.go:21-36`）；UI 存 capabilities 而不消费（`store.ts:33,211`）。

**纪律与边界**：全程只读；未改动 `.workspace/`；未触碰 `data/vivy.db`、`data/demo/`、`data/workspaces/`（air gap）；DSH 结论全部来自本地源码，未依赖网络资料。

### 7.2 本次调研的局限

- DSH 为静态阅读，未实际运行验证 bundle 送达行为。
- Studio 子模块只读了插件源码，未观察其运行时。
- 方案三的沙箱选型（iframe vs 构建期 purity gate）未做原型，列为开放问题。
- 机制 A 的组合顺序细节以模块自述文档与 cookbook 为准，未逐行核对 `packages/client/modules` 源码。

### 7.3 后续调研路径（若立项，按序）

1. **方案一契约定稿**：manifest v1 `ui` 块白名单词表——以现有 `settings/mcp` 卡片与 provider 表单的渲染能力为 widget 词表基线，先出字段草案 + parse/validate 测试设计。
2. **pack spike**：验证 overlay 机制能否与 `zz_register.go` 并行生成 `zz_ui.go`（同一 `go build -overlay`，不改 SDK Go 接口窗口）。
3. **RPC 契约影响面**：`plugins/list` payload 形状 vs 动态化 `capabilities`——读 `internal/tui/client.go` 与 worker 客户端对握手的依赖，确认加法路径。
4. **i18n 策略**：描述符内联 zh/en 与 `ui/src/i18n/index.test.ts` 孪生强制测试的兼容方案。
5. **换代感知**：`generation_id` 比对与"提示刷新"交互的最小实现位置（`store.initialize` vs 路由守卫）。
6. **方案三前置调研**（缓行）：`ui/index.html` 与响应头的 CSP 现状盘点；iframe 沙箱原型 vs DSH 式构建期 purity gate 的取舍实验；与 `VIVY-STUDIO.md` §9.1 sealed skin 的关系在产品契约里落字。
