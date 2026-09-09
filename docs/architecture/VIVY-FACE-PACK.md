# Vivy Face Pack — 出厂 face 的冷拔插

> **2026-09-09 v1 规范覆盖：** 本文的 Face 产品语义继续有效；所有
> `seam: face`、`vivy.plugin/v0`、`vivy.generation/v0`、旧 ABI 和兼容迁移
> 表述均为历史记录，不得作为新实现依据。v1 唯一机制是
> `std/face@v1` + FaceHost + `vivy.module/v1` + Generation Recipe，且不保留
> v0 API。规范正本见 `VIVY-MODULE-STANDARD.md`、`VIVY-PORT-CATALOG.md`、
> `VIVY-PLUGIN-SPEC.md` 与 `VIVY-ASSEMBLY.md`。
>
> 状态：**产品语义已采纳；插件装配机制由 v1 规范取代**。安卓仍是下游产品用内核。
> 服从 `SELF-EVOLVING-GATEWAY.md`、`VIVY-ASSEMBLY.md`、`VIVY-PLUGIN-SPEC.md`、**`VIVY-STUDIO.md`**、PRD §5.0 / D-016。
> 日期：2026-08-29
>
> **2026-09-04 修订（覆盖旧 D1/NG-11 的产品形态结论）：** 第一方
> `vivy-code.exe` 现在是允许的独立 TUI 制品。它不是第二套内核：仍复用同一
> app/runtime/provider/tool/FaceHost；但进程边界独立。`vivy.exe` 与所有
> `vivy-code.exe` 实例共享 config/settings/skills，每个 code 实例使用独立
> SQLite Journal 与运行目录，因此不共享会话记录，也不竞争 organism lease。
>
> 对照证据（只读，不是依赖）：DeepSeek Harness 的 `dsh-base` + `dsh-web-app` /
> `dsh-headless` 分层；[oh-dsh](https://github.com/hust-open-atom-club/oh-dsh)
> 的 Desktop / Web / TUI surface profile；`.workspace/crush` 的终端交互手感。
> 网页仍是当前主线。本文记录一条 **可进化出不带 web 的 coding 物种** 的装配合同，
> 不是立刻做 Bubble Tea，也不是给日常 `vivy.exe` 热挂一张终端。

相关：

- `VIVY-ASSEMBLY.md` — 按所是命名；出厂单元不住 `plugins/`
- `VIVY-PLUGIN-SPEC.md` — 只约束用户层 `plugins/<name>/`
- `SELF-EVOLVING-GATEWAY.md` — 装插件 = 造新版本；默认 `Register()` 为空
- `VIVY-GATEWAY-AND-STUDIO.md` — NG-10 模型可见≡入账；NG-11 拒绝第二种身体
- `VIVY-CHANNEL-PACK.md` — 超级通道（方向采纳 2026-08-30）；face 是嘴。channel 不得替代本机 UI
- `ACP-REMOTE-CONTROL-PROPOSAL.md` — 跨进程 / 跨设备遥控器；不是 face
- `.workspace/deepseek-harness/` — profile / bundle 证据，不是物种依赖
- `.workspace/oh-dsh/` — 同一 runtime 上的多 surface 发行
- `.workspace/crush/` — TUI 手感参考，不是代码来源

---

## 0. 一句话

> **Face 是配方上的可编译器官，不是工具，也不是内核。**
> 控制面留在物种里；嘴放进出厂模块或用户 `seam: face` 插件。
> 一代一张主脸。真卸 = 配方改行再 `pack`。网页网关与 coding TUI 是两条世代，不是一个开关。

这是把 DSH「base 上叠应用层」译成 Vivy 冷拔插的方式：偷分层和可检查的登记，不偷热挂、不偷把 Journal 当插件、不偷第二种 EXE。

---

## 1. 要解决的感觉

允许某一代身体 **没有网页**。不允许作者觉得自己在给活着的网关挂零件，也不允许 `config.yaml` 把 TUI 变出来。

判定「像在做 face」的标准：

1. 出厂工作目录只有 `faces/<name>/`。用户自写的才进 `plugins/<name>/`（`seam: face`）。日常不打开 `internal/`。
2. 世界只通过公开 SDK 进来：face 生命周期与 Host 能力仅经
   `sdk/plugin`；第一方终端 face 复用 `sdk/tui` 中唯一的展示、流状态与
   受限控制面客户端状态机。
   `sdk/tui` 不开放 Host、Journal、`Service.Run`、策略或密钥能力，脸仍
   看不见这些内核对象。
3. 身份是清单里的名字和 seam，不是某个 `.go` 被 `cmd/vivy` 引用。
4. 换脸，作者改的是**配方**，不是 embed 开关或 `engine.go`。`pack` 生成 `RegisterFace()`。
5. 跑起来之前，它只是源。跑起来之后，它已经是某一代 EXE 的一张嘴。默认提交的网关世代仍是 `face: web`。

网页主线继续过夜。coding 线另长一具身体。

---

## 2. 从 DSH / oh-dsh / crush 偷什么，拒绝什么

### 2.1 偷

| 来源 | 想法 | Vivy 形态 |
|---|---|---|
| DSH | `dsh-base` 共享，应用层互斥 | 内核永远在；`face: web \| tui \| headless` 一代点一个 |
| DSH | `dsh-web-app` / `dsh-headless` 是 bundle，不是内核 | 出厂 `faces/web`、`faces/tui`、`faces/headless` |
| DSH | TUI 可树外安装：`dsh plugin --profile tui add …` | 用户 `plugins/<name>`，`seam: face`，仍经 pack 换代 |
| DSH | headless 无 Host、无 HTTP、无浏览器 | `face: headless` 的 EXE 不 embed UI，不听端口 |
| oh-dsh | Desktop / Web / TUI 是同一 runtime 上的 surface | 同一内核，不同配方；TUI-only 对齐「不带 web 的发行」 |
| oh-dsh | TUI-only 不带 Electron / 浏览器 UI | coding 世代制品不含 `ui/dist` |
| crush | 无参进交互，`run` 走管道 | 出厂 tui / headless 的启动手感 |
| crush | 权限是一等交互 | TUI overlay 走同一套审批，不许 stderr Yes/No 冒充 HITL |
| ADR-015 | 活注册表为空；pack overlay 才链进去 | `internal/generated/faces/zz_register.go` 同构 |
| channel 提案 | 按所是命名；Host 在内核 | FaceHost 在内核；适配器只画嘴 |

### 2.2 拒绝

| 想法 | 原因 |
|---|---|
| 现有 `tool` / `tool-world` seam 硬塞 TUI | 那是模型的手。face 是人看见的嘴 |
| 出厂 TUI 放进 `plugins/` | 冒充用户层（`VIVY-ASSEMBLY.md`） |
| 改 yaml / flag 就把网页藏起来，冒充 coding 物种 | 资产还在、端口还在；不是「不带 web」 |
| 运行时 `vivy plugin add tui` | NG-15；Go 卸不掉原生代码 |
| 复制内核形成独立 `vivy-tui.exe` | NG-11 仍禁止第二套 runtime；第一方薄启动器 `vivy-code.exe` 是 2026-09-04 明确批准的例外，复用同一内核且隔离 Journal |
| face 插件 `net.Listen` / 自开 loop | 插件不是进程；loop 是内核 |
| face 插件写 Journal / 改 policy / 读密钥值 | 环境不能是种群成员 |
| `--yolo` 当默认 | 心在人这边 |
| 用 channel 替代本机脸 | 已写在 `VIVY-CHANNEL-PACK.md` 非目标 |
| 把 Crush / dsh-TUI / oh-dsh 嵌进物种 | 偷手感与分层；不偷 Node、不偷整棵 TUI 树 |
| 把安卓写成 `face: android` 或 `habitat: apk` | 安卓是下游产品用内核，不是 Vivy 的编译目标 |

---

## 3. 三层，名词分开

```text
vivy.exe  内核（永不可插件化）
  Journal · Policy · SecretResolver · HITL 裁定 · inspect
  进程内控制面（JSON-RPC 语义；HTTP 监听不是内核义务）
  FaceHost          ← 新的一等对象：选这代的嘴、把事件交给它、回收 TTY/HTTP
       │
       ├─ 出厂模块 faces/web | faces/tui | faces/headless     配方键 face:
       └─ 用户模块 plugins/crush-face                        配方键 plugins: ，seam: face
              ▲
              │  都只实现 SDK Face 契约
              │  没点名的脸不存在于这一代
```

| 层 | 住哪 | 改它的感觉 | 怎么出现在活身体里 |
|---|---|---|---|
| 内核 FaceHost + 控制面 | `internal/` | 在改 Vivy | 永远编进来 |
| 出厂 face 包 | `faces/<name>/` | 在做网页壳 / TUI / 一次性 runner | 配方 `face:` + pack |
| 用户 face 包 | `plugins/<name>/` | 在做插件 | 配方 `plugins:` + pack，seam 必须是 `face` |
| 配置 | `config.yaml` 的脸专属旋钮 | 在调旋钮 | 只能旋转 **inspect 已列出** 的那张脸 |

出厂代码禁止放进 `plugins/`。用户代码禁止放进 `faces/`。两边 ABI 相同，目录和口头禅不同。

HTTP 监听是 `faces/web` 的效果，不是内核义务。`vivy_headless` 构建标签是今日的权宜；采纳后应升为配方效果。

---

## 4. 配方：一代一张脸

`vivy.generation.yml`（采纳后）增加一等键：

```yaml
apiVersion: vivy.generation/v0
loop: eino
world: sandbox
face: web                 # 恰好一个。没写则 pack 失败
providers:
  - openai
tools:
  - notes
  - filesystem
  - execute
  - ask-user
plugins: []               # 用户 seam: face 与出厂 face 抢同一个键：点谁谁进
```

规则：

- **`face:` 恰好一个。** 物种必须有一张嘴。空列表非法。
- 出厂名：`web`、`tui`、`headless`。用户插件用目录名，经 `plugins:` 点名且 `seam: face`，**替换** 出厂脸，不是叠第二张。
- 没写进配方的出厂 face，这一代不存在。不扫描 `faces/`。
- 卸出厂脸 = 改 `face:` 再 pack。卸用户脸 = 从 `plugins:` 删行，改回出厂名。都要 eval / promote。
- `inspect` 列出：`face` 名、kind（web / tui / headless）、是否 listen、是否 embed UI、source_ref、tree_hash。

两条合法世代（示意，不是现在就切默认）：

**网关世代（当前主线）**

```yaml
face: web
world: sandbox
```

**coding 世代（进化目标）**

```yaml
face: tui                 # 或 plugins/crush-face
world: local              # 启动目录就是项目
# 这一代 EXE 没有 go:embed ui/dist，不开 :8787
```

`world: local` 不是 face 的副作用。coding 物种要换世界，另点 `world:`。脸只负责怎么跟人说话。

---

## 5. 出厂三张脸

| 名字 | 启动 | 听端口 | embed UI | 交互 |
|---|---|---|---|---|
| `web` | 无参开网关 | loopback | 是 | 浏览器；Review Center |
| `tui` | 无参占 TTY | 否 | 否 | 会话、流式、审批 overlay、取消 |
| `headless` | `vivy run "…"` | 否 | 否 | 一轮 prompt，stdout 终态，退出 |

`headless` 不是残缺的 tui。它是脚本/CI 的嘴。无 TTY 时审批必须失败响亮，不许默默放行。

TUI 第一刀是薄的：会话列表、流式对话、一等审批/提问、取消、接同一 Journal。不复刻设置页、不复刻审阅中心全量。设置仍回网页世代，或以后的专用命令。

成功标准（tui）：终端里过完一轮带审批的对话，同一 Journal 在网页世代的二进制里能回放。两具身体不要求同时运行。

**当前入口（2026-09-05）。** `sdk/tui` 是全屏壳、控制面投影和 Live
状态机的唯一实现；独立 `vivy-code.exe`、`vivy tui` 与出厂
`faces/tui` 都委托给它。`vivy tui --live [--addr host]` 通过
`internal/tui` 的 WebSocket 传输连接驻留网关；网关未起则失败退出。
旧的离线 `--demo` 和行式 `--plain` 已退役，不存在 demo 或本地执行
fallback。`faces/tui` 仍经 FaceHost + 配方点名进入不含 `ui/dist` 的制品。

---

## 6. `seam: face`（用户层）

跟 channel 同构，不走 tool 的 `Adapt`。

```json
{
  "apiVersion": "vivy.plugin/v0",
  "name": "crush-face",
  "version": "0.1.0",
  "seam": "face",
  "module": ".",
  "grants": ["tty", "argv", "rpc.client"],
  "face": {
    "kind": "tui",
    "listen": false
  }
}
```

| 字段 | 规则 |
|---|---|
| `seam` | 必须是 `face` |
| `tools` | **禁止**出现。脸不是模型工具 |
| `face.kind` | `web` \| `tui` \| `headless` |
| `face.listen` | 用户插件默认 `false`。`true` 第一刀拒绝 |
| `grants` | `tty`、`argv`、`rpc.client`。没有 `journal.write`、`secret.read`、`policy.write` |

`verify` 对 `seam: face`：

- 零个 tool
- 不 import `internal/`
- 不 `net.Listen`
- 不 `go:embed` 可执行文件
- `kind` 合法；与配方点名的那一个 `face:` 冲突时，以配方为准（用户插件替换出厂）

Face Env（示意，落地时写进 `sdk/plugin`）只允许：列会话、开 run、订事件、回答审批/提问、取消。禁止 Journal 直写、改 policy hash、读密钥值、自开 loop。

`source` 入账：`web` \| `tui` \| `headless`（或用户脸的名字）。不能伪装成另一张脸的 `user` 行，也不能写成 `channel`。

崩溃 = 这一代 EXE 崩溃。隔离在下一代：配方换脸再 pack。

---

## 7. 内核：脸能缺席

今日阻碍不是缺 Bubble Tea，是内核把网页当默认身体：

1. `cmd/vivy` 无参就组 HTTP 服务
2. UI 默认 `go:embed`；`vivy_headless` 只是构建标签
3. 审批/提问的人机面默认长在网页

内核该变成：

```text
启动器
  → 读编进身体的 face
  → web        听 loopback，embed UI
  → tui        占 TTY，不听端口
  → headless   跑一次 prompt，退出
  → 没有 face  pack 已拒绝；运行时不可达
```

控制面留在进程内。出厂脸和用户脸都是控制面的 **in-process 客户端**，不是第二套 run 模型。网页已经是 JSON-RPC 客户端；TUI 走同一套方法，运输从 WebSocket 变成函数调用。

FaceHost 列入「内核永不插件化」名单，与 Journal、Policy、Secret resolver、inspect、`vivy worker` 监督并列。它不画 UI。它只保证：这一代有且仅有一张嘴，事件与审批仍由内核裁定。

---

## 8. coding 世代还要换什么（脸以外）

只换脸会得到「终端里的个人网关」，不是 Crush。Crush / DSH headless 默认 **调用目录就是 workspace**。

| 旋钮 | 网关世代 | coding 世代 |
|---|---|---|
| `face` | web | tui（或用户 face 插件） |
| `world` | sandbox | local（`--cwd` / 启动目录） |
| persona | 网关 / 陪伴 | coding agent，cwd 进系统提示 |
| 工具密度 | notes + 保守 execute | grep / glob / edit / bash 级 |
| HITL | Review Center | TTY overlay；同一 first-writer-wins |
| 监听 | loopback | 无 |

LSP、项目 skills 发现、Crush 式 `crushrc` **不是第一刀**。配置仍是严格解码的 yaml + `env_key`，不许另长一套加载即执行的可信代码配置。

---

## 9. 安卓：下游产品，不是 face

安卓 App **使用 Vivy 内核**，不是 Vivy 去编译 APK，也不是 `face: android`。

```text
Vivy 内核     Journal · policy · run · 审批 · 控制面
    │
    ├─ 第一方物种    vivy.exe（face: web | tui | headless）
    ├─ 用户 face     plugins/crush-face（编进某一代 EXE）
    └─ 下游产品      某个安卓 App（自己的工程、自己的 APK）
```

Studio 继续只 pack 物种身体。安卓团队用自己的工具链出包。Vivy 不负责 Android SDK、签名、上架、gomobile。

因此本提案 **不增加** `habitat:`，不让 `seam: face` 认识 Activity。

安卓要对内核成立，只要求 F1：控制面在无 embed、无 listen 时仍能跑完一轮对话和审批。那是网页主线、TUI、下游 App 共用的一刀。

以后若要「手机遥控家里的 `vivy.exe`」，那是 `ACP-REMOTE-CONTROL-PROPOSAL.md` 的远程主体，不是 face，不是 channel。手机 Journal 与电脑 Journal 默认同步另案。

---

## 10. 和 channel / ACP 的边界

| | face | channel | ACP / companion |
|---|---|---|---|
| 是什么 | 这具进程的嘴 | 世界先说话的耳朵 | 另一进程/设备上的遥控器 |
| 配方 | `face:` 恰好一个 | `channels:` 列表 | 不进 generation.yml |
| 出处 | `source=web\|tui\|headless` | `channel.inbound` | 控制面主体，另记 |
| 审批 | 本机脸，可以是 HITL 主体 | 第一刀不当审批人 | 须显式批准远程主体 |
| 例子 | 浏览器、TTY、`vivy run` | Telegram、飞书 | 未来安卓遥控、编辑器 |

三种混权是 bug。

---

## 11. 冷拔插的卸载

```text
1. 换代拔插（真冷）
   配方 face: tui → pack → 下一代没有网页资产，也不听 8787

2. 运行时不能「关掉网页假装 TUI」
   活着的 EXE 不动态加载任何 face 代码

3. 没有「杀一张脸、物种还用另一张」
   一代一张嘴。崩溃即这一代死
```

「卸得干净」只适用于 (1)。`inspect` 必须能证明：`web=false` 的制品里没有 `ui/dist`。

---

## 12. 非目标（本提案）

- V0 交付、把 TUI 写进当前 `just ci` 必过路径
- 热挂 / 热卸 / 市场扫描
- 现在就实现 Bubble Tea / Crush 复刻 / 设置页 TUI
- 独立 `vivy-tui.exe`、把 Crush 或 dsh-TUI 当依赖
- `habitat: android`、APK 流水线、gomobile
- 用 channel 或 ACP 替代本机脸
- 在本文件合并时改 `sdk/plugin` 或加事件类型（那是采纳后的实现 PR）

---

## 13. 落地切片（采纳后）

顺序就是依赖。每一刀应能单独评测；未做的不出现在默认网关 EXE。

| 切片 | 做什么 | 成功 |
|---|---|---|
| F0 合同 | 本文采纳；`VIVY-ASSEMBLY.md` 增加 `face` 行；内核永不插件化名单加上 FaceHost 与「HTTP 不是内核」；PLUGIN-SPEC 声明 `seam: face` 分流 | 文档一致，无代码 |
| F1 控制面无网页 | 进程内 RPC 可在无 embed、无 listen 下跑完一轮对话+审批 | `vivy_headless` 从构建标签升级为配方效果的前置 |
| F2 出厂 `faces/headless` | `vivy run "…"`，stdout 终态，无端口 | 脚本可用；无 TTY 时审批失败要响 |
| F3 出厂 `faces/tui` | 薄 TUI：会话、流式、审批 overlay、取消 | 同一 Journal；网关二进制仍可不含这张脸 |
| F4 coding 配方 | `world: local` + coding persona + 工具集；pack 出不带 `ui/dist` 的制品 | `inspect` 显示 `face=tui`、`web=false` |
| F5 用户 `seam: face` | `plugins/crush-face` 能换掉出厂 tui | verify 禁 tools、禁 Listen、禁 import internal |

网页主线继续走，F0–F1 不挡。F3 之后才允许有人在 Studio 里进化 Crush 风格实现。

F0 是文档 PR。F1 起才动内核。F3 之前禁止把 Bubble Tea 写进物种默认 `go.mod` 的必经 import。

---

## 14. 尚未关闭的问题（采纳前要人拍板）

2026-09-02 四问已拍板（用户裁决，全取推荐值；F2/F3 切片的前置解除）：

1. **出厂 `faces/` 是否独立 `go.mod`。** 拍板：**独立 `go.mod`**——网关世代编译期就不见 TUI deps，不可能被内核误 import；代价是多一个 module 的维护面。
2. **默认提交的物种身体是否永远 `face: web`。** 拍板：**永远 `face: web`**。coding 世代是另一条配方，不替换日常双击的网关。
3. **网页与 TUI 能否点同一 Journal 同居。** 拍板：**第一刀否**（一代一张嘴）。多客户端同居是后切，且必须先钉死审批 first-writer-wins。
4. **`headless` 遇到审批时的产品句子。** 拍板：**失败退出**——不挂起等待、不 yolo 放行（run 级持久挂起+可取消语义由 F1 测试钉死；进程级句子=失败退出）。

---

## 15. PR Plan（采纳后）

### PR 1 — 采纳合同

- 文件：本文状态改为方向采纳；`VIVY-ASSEMBLY.md` 增加 `face` 行；`VIVY-PLUGIN-SPEC.md` 交叉引用 seam 分流；`SELF-EVOLVING-GATEWAY.md` 内核名单加上 FaceHost
- 依赖：无
- 无运行时代码

### PR 2 — 无网页控制面（F1）

- 文件：启动器 / app 组合可在无 embed 时工作；审批在无 UI 时的失败路径有测试
- 依赖：PR 1

### PR 3 — 出厂 headless（F2）

- 文件：`faces/headless/`、`vivy run`、pack overlay
- 依赖：PR 2

后续 F3–F5 各一次 PR，禁止与 channel 适配器、安卓工程混在同一交付。

---

## 16. 一句话（再写一遍）

> **脸是配方上的器官，不是工具，也不是第二种内核。**
> DSH 用 profile 叠 bundle 去掉 `dsh-web-app`；oh-dsh 用 TUI-only 证明可以不带浏览器。
> Vivy 对应物是：`face: web | tui | headless` + 用户 `seam: face` + 一条不点名 web 的 coding 世代。
> 装上 = pack 进新 EXE。卸 = 配方改行再 pack。安卓 App 用这颗内核，自己出包。
