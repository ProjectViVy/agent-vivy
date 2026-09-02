# DSH 插件 UI 实现深调（2026-09-02）

> 触发：维护者要求——"之后的插件要包含 UI，看看 DSH 怎么实现的。"
> 上游文档：`docs/research/plugin-model-dsh-vs-vivy-research-2026-09-02.md`（port 标准提案）。
> 本文回答两件事：**DSH 插件 UI 的完整机制**；**移植到 Vivy port 模型的形态选项**。

---

## 1. 一句话结论

DSH 的 web 客户端**本身就是一个浏览器内的 Cordis 插件宿主**。插件包带两个半身：
`.`（Node 宿主半身：服务/工具）与 `./client`（浏览器半身：React UI），由 package.json 的
`dsh.client` 块声明。宿主 Node 进程扫描声明、组装启动图注入 `window.__DSH_BOOT__`、
经定制 combo 路由供预构建 bundle；浏览器侧自研"懒 CJS 表"物化插件，其 `apply(ctx)`
把 React 组件注册进**声明即认领**的类型化槽位树。没有 module federation、没有
import map、没有 iframe——全信任、单 React 树。

---

## 2. 物理送达链路（bundle 怎么进浏览器）

来源：`<D>/docs/subsystems/client-modules.md`、`<D>/packages/client/modules/README.md`。

1. **声明**：package.json 加 `dsh.client { platform: "web", inject?, immediately?,
   external? }`，并把构建产物挂在 `exports["./client"]`。实例：
   `packages/extensions/ui-cordis/package.json:16-47`。
2. **宿主半身** `ctx.clientModules`（ClientModuleRegistry）四个职责合一：扫描宿主
   loader 条目里声明 `dsh.client` 的包；组装启动图；供版本化 combo 脚本；答启动协议行。

   ```ts
   interface WebBootEntry {
     id: string            // == 包名
     url: string           // 版本化 combo 端点（HMR 用）
     rev: string           // 不透明修订号，缓存爆破
     inject?: string[]     // 包名依赖边
     immediately?: boolean // 一级预取标记
     external?: string[]   // 非基线模块请求
   }
   ```

3. **bundle 路由**：`GET /plugins/??<a>/client.js,<b>/client.js&rev=<rev>`（URL 3KiB
   前分区，immutable 缓存，Indexed SourceMap v3）。未知资源或过期 rev 一律 404，
   **绝不**让 SPA fallback 把 HTML 当 JS 吐出去（client-modules.md:85）。
4. **注入**：宿主接管 index 渲染，往 `<head>` 注入模块加载队列 facade、advisory
   preload、阻塞式引导 combo、启动图（`<` 转义，防插件字符串逃出 script 标签）。
5. **浏览器半身** `ctx.modules`：**懒 CJS 表**——执行 bundle 只注册工厂；模块体副作用
   （含 CSS 注入）全部住在工厂闭包里，物化（`factory(require)`）时才跑，结果 memo 进
   `loadCache`。externals 解析到**冻结的平台种子**（`packages/client/web/src/platform.ts:8-13`）：
   `react / react-dom / @deepseek-ai/cordis / dsh-client-store / dsh-client-ui-slots /
   dsh-client-ui-primitives`——全浏览器单一实例，插件不得自带副本。
6. **启动**：`dsh-client-web` 是无框架引导核（自画加载/失败页，保证 React 树崩溃时
   诊断仍在），等全部 fiber ACTIVE 后经 `ctx.uiRenderer` 水合，唯一一次
   `renderSlot('root')` 画出整棵树。HMR：stat 轮询 bundle → SSE rev 广播。

## 3. 槽位系统（UI 的契约层）

来源：`<D>/docs/subsystems/slots.md`、`packages/client/ui-slots/README.md`。

- **类型注册**：`SlotMap` 靠 TS declaration merging 编译期填充：

  ```ts
  declare module '@deepseek-ai/dsh-client-ui-slots' {
    interface SlotMap {
      'tool.view.cordis': { kind: 'keyed'; scope: 'session';
                            owner: CordisToolViewOwnerProps }
    }
  }
  ```

- **声明即认领**：注册某槽位声明的 entry 成为该 key 唯一渲染者；往未声明槽位注册
  **装载即 throw**。`root` 是唯一内建声明，其余槽位都由拥有渲染位置的组件声明子槽。
- **基数 × 作用域**：`single`（priority 胜者）/ `list`（按 order+注册序）/ `keyed`
  （owner 派发 entryKey，命中格渲染）/ `chain`（各 entry 纯函数 `select(owner)`，
  首个非 null 胜并拿到 matched）；scope `root / session-maybe / session`。
  `priority` 是遮蔽序，动态包自动分配"页内遮蔽秩，后注册排前"。
- **组件输入**：四份共享的交集（runtime hooks / 被授权的子槽渲染器 / store
  selector+actions / 注册时 inject 工厂的返回）+ locale `t`。**组件永不接收 ctx**——
  能力面被切干净，UI 组件只能看见被显式注入的东西。
- **出货槽位树**（slots.md:110-163）：`root → sidebar（brand/footer.action/
  workspaces/settings.*）→ conversation（session/view/chat.node/tool.call.toolview →
  tool.view.cordis/composer/**/input overlays）→ details → shell.overlay`。
  目录由 `gen-client-catalog` 生成器出机器契约，运行时可 `cordis_inspect what:"client"`
  查活树。
- **真实样例**（`packages/extensions/ui-cordis/src/client/index.ts`）：一个包同时贡献
  全局面板（`sidebar.footer.action` list 槽）、四张 keyed 工具卡
  （`cordis_define/run/stop/undefine` 挂 `tool.call.toolview`）、给第三方开的子槽
  `tool.view.cordis`、输入框 `@pluginId` 补全源。

## 4. 一包两半身（host/client 拆分）

- 入口：`.` → Node 半身（服务/工具），`./client` → 浏览器半身（槽位注册 + 组件）。
  各自独立声明 external：Node 面外置生产依赖，浏览器面外置平台基线 +
  `dsh.client.external` 精确补项（无 alias 协议）。
- 挂载：**两个进程各自跑一个 Cordis loader**。浏览器侧的动态包与静态包走同一套
  激活门控、fiber effect 清理、状态投影。
- **半身间的桥是 Remote**：客户端经生成的 `ctx.remote.<service>.<method>(...)` 回环到
  宿主控制器。纪律："行为跨包走注入的 Cordis 服务，UI 跨包走 slots"——feature 插件
  永不运行时 import 另一个 feature 插件的值（`packages/client/AGENTS.md:36`）。
- 约束：react 18、CSS Modules + clsx、禁组件库/Tailwind、全局样式只许 ui-theme 且
  经 `ctx.effect()` 装 style 标签（卸载/HMR 自动回收）；语义 token `--dsw-alias-*`。

## 5. 动态插件（模型自己写 UI）

来源：`packages/extensions/{tool-cordis,cordis-client-runner,ui-cordis}`。

- 流程：`cordis_define`（只做校验+语法预检，不运行不要审批）→ `cordis_run`；
  带浏览器半身的包返回 `awaiting-approval`，经 `cordis/request-run` 全帧往返**人工批准**
  才在浏览器执行（可选覆盖未来版本；先答者胜）。
- 代码形态：**纯 JS、无 JSX/TS/import**，作为 async 函数在**页面内**求值（非 iframe）。
  参数白名单闭包：`['React','console','styles','host','harness',...traps,'process','Buffer']`
  （`evaluator.ts:173`）——`fetch`/`setTimeout` 等浏览器全局不可达；
  `styles.insert(css)` 的样式表随包卸载自动移除；`host.call(method,args)` 只达自己包的
  宿主半身。
- Guard：白名单 ctx facade——只暴露生命周期动词与已声明服务，返回 Context 值拒绝；
  slots 席位自动分配遮蔽秩并记账；theme 覆盖源钉死在包 id 上。渲染崩溃上报宿主：
  槽位名、是否 abdicate（退役）、作者可见消息；模型经 steering 消息或
  `cordis_inspect_self` 获知。guard.ts:12-13 自我声明："**这是 API 纪律，不是安全边界**"。

## 6. 信任与隔离

- 全信任：无 iframe 沙箱，一切在**同一棵 React 树**里渲染。
- 实际存在的隔离是工程性的：生命周期隔离（一切贡献随 fiber 卸载，含 style 标签）；
  渲染外壳（error boundary 或 abdicate 退役，崩溃按身份归因到包）；CSS 约定隔离
  （CSS Modules 哈希类 + token 纪律，feature 包禁全局样式）；传输加固（启动图转义、
  combo/rev 404 语义）。自报已知缺口：槽位准入无载体（per-deployment 允/deny 列表
  没地方挂）；guard 白名单与宿主沙箱 facade 是手工镜像的双份。

## 7. theme / layout 也是插件

- `ctx.theme`：明暗/字号/`--dsw-*` token 表归 ui-theme；第三方主题经
  `ctx.theme` 注册 **alias token 覆盖**，按注册序折叠进激活快照；宿主把解析后的主题
  嵌进 index 响应，首绘即已带主题。
- `ctx.layout`：三栏 AppFrame **本身就是一个插件**——一次 `register()` 往 `root` 槽
  注册 AppFrame、同口气声明四个子槽（sidebar/conversation/details/shell.overlay）、
  坐进布局 store、开放 `ctx.layout` 面板动作服务。几何是瞬态（刷新即重置）。

---

## 8. 移植到 Vivy：形态选项（待拍板，未排期）

Vivy 现实：插件是 Go 源码（`vivy-sdk pack` 编译期世代），UI 是独立 React 应用
（`ui/`，Vite，RPC/WS 同源契约）。DSH 的"插件自带 client bundle"不能原样照搬，
因为 Vivy 插件作者今天只写 Go。三个选项：

- **选项 A：声明式 UI 端口（推荐起步）**。port 族 `ui/slot`：Go 插件声明
  槽位意图（槽名 + keyed/list + 数据投影 + 动作回调），**渲染原语内建在宿主 UI**
  （卡片/表单/面板/列表等有限集）。插件零 JS、零供应链；UI 侧只需一个通用
  PluginSurface 渲染器 + RPC 投影端点。表达力≈DSH 的 keyed 工具卡级别，
  覆盖"给插件配设置页/结果卡/面板"的大多数需求。审计友好：UI 意图进 generation.json。
- **选项 B：插件 JS 半身（DSH 全量对齐）**。插件仓库增设 `client/` 预构建 bundle，
  pack 收集；网关学 DSH：boot 图 + `/plugins` combo 路由 + 懒 CJS 表 + 平台种子
  （react 单例由宿主供）。表达力=DSH；代价：插件作者要养一条 JS 打包链，
  bundle hash 必须进 generation.json，且引入"插件 UI 代码"这层新信任面
  （DSH 自己都承认是全信任 + API 纪律）。
- **选项 C：混合分步**。A 先行定为 v1 标准；B 作为高级端口 `ui/canvas` 后置，
  仅在 A 表达力确实不够时立项。

无论哪个选项，DSH 值得原样照抄的部分（与宿主语言无关）：

1. **槽位契约**：声明即认领（未声明槽位装载即拒）、single/list/keyed/chain 四基数、
   priority 遮蔽序、组件不拿 ctx（能力面显式注入）。
2. **生命周期即 UI**：一切 UI 贡献（含样式）挂在插件生命周期上，卸载即消失。
3. **渲染崩溃归因**：error boundary + 崩溃归因到插件 + abdicate 退役。
4. **平台种子单例**：React/组件库由宿主供单一实例，插件不带副本。
5. **传输纪律**：启动图转义、未知 bundle 404 不回 HTML。

治理红线（Vivy 特有，别让步）：插件 UI 只拿**投影数据**，动作回调一律走 RPC 且过
审批/策略；插件永远不直连 Journal；UI 端口属 web face（FACE-0）装配，
不进内核物理。

---

## 9. 结论

DSH 的插件 UI = **浏览器侧第二个插件宿主 + 声明即认领的槽位树 + 懒 CJS bundle
管道**，全信任、无 iframe，工程隔离靠生命周期/错误边界/CSS 约定。对 Vivy 的启示：
槽位契约和生命周期模型可整体移植；bundle 管道是否移植取决于要不要让插件作者写
JS——建议 v1 走声明式 UI 端口（选项 A/C），把 `ui/slot` 纳入 port 标准，
`ui/canvas`（JS 半身）留作后置高级端口。已并入 PLG-1 拍板范围。
