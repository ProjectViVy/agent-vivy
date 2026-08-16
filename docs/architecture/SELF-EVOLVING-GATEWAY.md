# 如何让 Vivy 成为单 EXE 的自主进化网关

> 状态：**方向已采纳**（2026-08-14）。**Studio 形状于 2026-08-15 纠正**：独立应用，管开发与分发；开发场地切到 Studio。
> 不取代 V0 ADR。物种侧 S1–S6 零件仍可用；S7 工作室卡产品含义作废。
> 日期：2026-08-15（Studio 纠正）
> 来源：对 `.workspace/deepseek-harness/upstream`（含论文）与 agent-vivy 现状的对照讨论。
> 读者：要一次读完「为什么拆、拆成什么、插件怎么装、和 DSH 像多少」的人。
>
> **Studio 正本是 `VIVY-STUDIO.md`。** 精简决策表仍见 `VIVY-GATEWAY-AND-STUDIO.md`（NG-1..NG-28、S0..S6 + ST-*）。
> 本文管物种 / 内核 / 装配。若与 `VIVY-STUDIO.md` 在 Studio 形状上冲突，以那份为准，并回写本文。

相关：

- `../../AGENT-VIVY-DIRECTION.md` — V0–V3 分期
- `../../prd-agent-vivy-v0.md` — §5.0 哲学锚、D-014..D-021
- `../AGENT-VIVY-ARCHITECTURE-V0.md` — 已组装内核
- `../GOAL-AGENT-HARNESS-ROADMAP.md` — 已完成的 harness 切片
- `ACP-REMOTE-CONTROL-PROPOSAL.md` — 控制面草案；只借本地与准入，不借远程托管
- `VIVY-STUDIO.md` — Studio 产品身份、生命周期、开发场地硬条件（正本）
- `VIVY-WORLDVIEW.md` — 为何物种/实验室分裂和这个名字是同一根骨头
- `.workspace/deepseek-harness/upstream` — 证据，不是物种依赖

---

## 0. 一句话

**日常 Vivy 是人双击就能开的那一个 `vivy.exe`。**  
**Vivy Studio 是另一个独立应用**：开发、评测、发布、安装下一代身体。  
第一任开发发动机可以是 DeepSeek Harness，但它是 Studio 内部的可替换发动机，不是根，也不是从网关打开的一张卡。

住户始终面对一个网关。单 EXE 指**日常安装、日常生命、以及全部能力都编在这份身体里**。装插件 = 用 SDK 造新版本，不是往活进程上挂零件。作者面对的是 Studio，不是「网关里的演化按钮」。

```text
作者 ──独立应用──► Vivy Studio
                      │ 改源 / verify / pack / 评测 / 发布 / 安装
                      ▼
                 日常安装位的 vivy.exe   物种
                      │ 只读 inspect
                      ▲
                 Studio 可以看、换、停；物种不启动 Studio
```

---

## 1. 违和从哪来

讨论里叠了五件不能住在同一地址空间、同一条发布列车里的事：

| 主张 | 成功长什么样 | 塞进同一个进程会怎样 |
|---|---|---|
| 个人网关 | 今天能用、能审、能恢复、密钥在本地 | 变成平台或实验室，人不敢过夜 |
| 高性能单 EXE | Windows 一键、无 cgo、热路径短 | 插件海、Node、端口、网格 |
| 一切皆插件（DSH） | 卸得净、依赖能重绑、loop 可换 | Journal 和审批也被卸掉 |
| 自主进化直到更强 | 能试下一代、能杀、能比较 | 没有适应度就是永久重写 |
| 微服务 / 像 K8s | 控制面稳、数据面可杀 | 本机变成集群，测量先碎 |

方向文档已经写过：一个阶段不能同时承诺稳定产品、完整框架替换、AGI-OS。  
本文把三件事分给三具身体，而不是假装一个 EXE 里能同时当物种、框架和实验室。

| 身体 | 使命 |
|---|---|
| **物种** `vivy.exe` | V1 Operate：日常网关 |
| **Vivy Studio** | 独立应用：开发与分发；V2 Explore 也在这里发生，不在网关里 |
| **更后面的一代 EXE** | 若有证据，才是 V3 Rebuild |

---

## 2. DeepSeek Harness 到底是什么

### 2.1 核心理念

DSH 不是又一个 Claude Code 克隆。底下是论文 *A Programming Paradigm for Spatiotemporal Composability*（Shi / Zhang / Cui，北大 + DeepSeek）：

- **时间可组合：** 卸组件时，它对共享环境的副作用必须完整、有序撤销。每个副作用带着逆，运行时记账（`ctx.effect()`）。
- **空间可组合：** 组件声明依赖（`inject`），依赖出现/消失时激活或停用。
- 二者合成同一个 `ctx`，称为 context paradigm。实现是 vendor 的 **Cordis**。

产品口号：**everything is a plugin**。模型适配器、工具表、会话日志、**连 agent loop 本身**都是插件。没有特权产品内核；真正的内核是 Cordis（加载 + 记账）。扩展方式是把插件挂到旁边，不是给 loop 打补丁。

另外几条工程纪律，和理念同等重要：

- **模型可见 ≡ 已记录。** 进模型请求的一切必须能从会话日志重建。
- **双事件平面。** `session/event` 是持久事实；`agent/*` 是进行中的协调。
- **能力 seam** = Service Definition + Provider + Consumer。换执行世界（本地 / E2B）带走 fs、shell、PTY、LSP。
- **组装是 profile 叠 bundle 再叠 patch**，不是写死启动顺序。

### 2.2 创举

1. 把编译期的 effect / coeffect **抬到运行时**（经典系统停在词法作用域）。
2. 自演化 harness 做成真工具：`cordis_inspect / define / run / stop / undefine`，模型可检查并挂载自己写的插件。
3. 微内核循环 + 瀑布式扩展点；改 loop 必须改架构文档。
4. seam 让「换世界」是组合，不是 fork 一套工具。

它**没有**做成的（对网关很关键）：动态包只在内存、重启即散；官方写明不是安全边界、可能影响同进程其他会话；没有跨代适应度；预发布阶段正确性优先于过夜可用性。

### 2.3 语言事实

没有任何量产语言把「可逆 effect + 反应式 coeffect」做成语法。

- TypeScript 只是凑齐了**宿主最低条件**（`Proxy`、声明合并、模块可扔），所以 Cordis 长在那里。
- Koka / Effekt 是类型系统上的祖先，不管运行时装卸。
- Erlang 是「杀进程即卸」的量产答案。
- WASM 是「丢掉实例即卸」的沙箱动态库。
- Go 给得出快的单二进制，**卸不掉**原生代码。

因此：物种用 Go；实验室可以用 TS（若后端是 DSH）；能力插件是 Go 源码，只经 SDK 链进下一代 EXE。不要逼 Go 做 Cordis，也不要再开一个 WASM/进程插件世界。

### 2.4 和今日 Vivy 哪些像

两边都是事件溯源的本地 agent harness：Journal / 会话日志、ReAct + 中断、工具审批、Ask User、Plan Mode、hooks、子代理由父治理、JSON-RPC、薄 UI。

Vivy 已经有的，不必向 DSH 再买一遍：Skill、MCP、worker stdio、policy profile、预算、隔离 workspace、恢复。

身份相反：Vivy 是个人网关、精选目录、Eino 隔离在 `internal/runtime`；DSH 是插件 OS、loop 也可卸、社区发现。

---

## 3. 新理念下能实现 DSH 核心理念的多少

不要一个百分数。按柱子看。

| 支柱 | 物种 EXE | 物种 + 工作室(DSH) | 故意不补 |
|---|---|---|---|
| 模型可见 ≡ 已记录 | 90–100%（须升级 ADR-009） | 同左 | — |
| 能力 seam | 80–90% | 同左 | 运行时 `inject` 热重绑 |
| 薄 loop / 事件扩展 | 80% | 同左 | 同进程 waterfall 网 |
| 时间可组合 | 30–40%（换代 / 杀候选进程） | 实验室内 ~100% | 细粒度逆操作栈 |
| 空间可组合 | 35–45%（星形经网关） | 实验室内 ~100% | 插件互调成网 |
| 一切皆插件 | 30–40%（产品面可换代） | Studio 内发动机 ~100% | Journal / policy / 物种内核 |
| 活着改自己 | 15–25% | 实验室内 ~90% | 生产实例同进程挂载 |

- 只算物种：**约 35–45%**，理论上限约 **60%**（内核永不插件化）。
- 对人而言的「自演化网关」效果：**约 70–80%**，因为满血 Cordis 住在实验室。
- 过夜可积累的进化（气隙、EvalRun、晋级）：**可以高于 DSH 现货**。DSH 演示重启即散。

刷到 80% 的 Cordis 复刻率 = 取消物种/实验室分裂。不要刷。

---

## 4. 架构：物种、Studio、数据面

### 4.1 物种 — 人双击的那个

单安装器、单快捷方式、日常只有一个主 EXE。

它拥有：

- Session / Run 状态机
- Journal（产品历史；模型可见投影的真源）
- Policy、Approval、Ask User
- 预算、取消、恢复
- 密钥解析（只读 env，值永不落盘）
- JSON-RPC 控制面和住户 UI
- 本代编进来的能力清单（pack 时冻结）
- 只读身份：`inspect`（哈希、配方、工具名）

不拥有：pack、评测农场、发布、安装位、Studio 主界面。那些属于 `VIVY-STUDIO.md`。

### 4.2 内核（永远不是插件）

```text
Journal 写入
事件词汇与序号
Policy 准入（deny / prompt / allow，不可变 hash）
密钥解析
本代编进来的能力清单（pack 时冻结）
只读 inspect 的实现
进程监督（仅 worker，不含外置插件、不含 Studio）
```

以后 V3 可以换内核，那是**晋级新一代物种**，不是热卸。

### 4.3 数据面（可杀）

过日子的控制面留在物种进程里。Studio 是另一套控制面，住在独立应用里。物种名下的短命进程只有：

| 进程 | 角色 | 失败 |
|---|---|---|
| `vivy worker` | 同二进制子 run；工具仍在这份 EXE 里 | 已有 `worker_lost_after_restart` |

候选 EXE 由 **Studio** 拉起和杀死，记入 Studio 的 EvalRun，不是物种的孩子。Studio 自己也是独立进程：杀 Studio 不影响已打开的日常网关。

不要把物种写成 kube-apiserver、把 Studio 写成 Pod。Studio 不是网关的数据面。

微服务会毁掉热路径、单一 Journal、Windows 一键、以及一周能杀掉的突变体数量。评测农场以后若要多机，是挂在同一对象面后的**另一套东西**，不是把个人网关拆成服务。

### 4.4 Vivy Studio — 独立应用

完整形状、开发场地硬条件、bootstrap 与切片见 **`VIVY-STUDIO.md`**。这里只留和物种的边界。

Studio 是独立应用，不是物种上的角色，不是「再装一个通用 agent」。产品比喻是老工业软件的 IDE：打开就能编、能烧、能装到日常位。不是从 `vivy.exe` 弹出的一张卡。

职责（权威在 Studio）：管理工程 → 开发 → 验证 → `vivy-sdk pack` → **自己**评测候选 → 人在 Studio 里发布 → 安装到日常位 / 回滚。只读询问活物种的 `inspect`。

第一任开发发动机可以是钉死的 DeepSeek Harness 工厂 profile。人看见的名字是 Vivy Studio。DSH 是发动机；换掉它，Studio 还在。

Studio **不准**：写生产 Journal、拿住户 API 密钥、热替换活内核。  
Studio **必须**：成为切换门槛之后唯一的 Vivy 开发场地（NG-26）。

不存在「打开工作室」。物种不启动 Studio。

---

## 5. 血缘：插件、一代、分流、新物种

| 改了什么 | 名称 | 关系 |
|---|---|---|
| Skill 文本 | **行为包** | 同代可改想法，不改编身体 |
| 能力源码（工具 / world / provider 实现） | **待编译的插件** | 还不是身体；只有 `pack` 之后才存在 |
| `vivy-sdk pack` 出的新 EXE | **Generation** | 装插件的唯一结果 |
| fork 源码但合同还在 | 仍是 Generation | 源在别人的 git |
| 换皮 UI、换模型供应商、远程 MCP 地址 | 配置 | 不是插件 |
| Journal 语义、事件词、或不要物种合同 | **另一物种** | 不能公平 EvalRun |

判定：

> 还能用同一份账本词汇和同一套门说话的，是同物种的不同代。  
> 门或账本换了，才是新种。

谱系记在 `Generation.parent` 上。树枝不是新种。种一分裂，适应度消失。

插件源码**不是物种**。它只有被编进某一代 EXE 之后，才成为那一代身体的一部分。卸插件 = 再 pack 一版没有它的身体，下次启动换过去。

---

## 6. 插件只剩「源码包」；装上就是新版本

拒绝：WASM、`.dll`、Go `plugin`、stdio 外置 exe、把 MCP 当插件系统。  
能力要进世界，只有一条路：

> **实现 SDK 契约的源码 → `vivy-sdk pack` → 新的 `vivy.exe` → eval → promote。**  
> **装插件 = 造新版本。**

代码可以是纯 Go。出厂单元用真名装配（loop / world / tool / provider），见 `VIVY-ASSEMBLY.md`。  
**只有用户自定义进 `plugins/`，才叫插件**，规范见 `VIVY-PLUGIN-SPEC.md`。

### Kind A — 行为（仍是文本，不编译）

`data/skills/**/SKILL.md`。改怎么想，不改编身体。不是插件系统。

### Kind B — 能力源码（编译期包）

工具 / provider / tool-world 的 **Go 源码包**，实现 `sdk/plugin` 契约。  
在被 `pack` 编进某一代之前，它在磁盘上只是源，活进程看不见它。

### Kind C — 世代（唯一装载动作）

`pack` 的输出。Kind B 的源码被链进这个 EXE。没有「先编一个插件 exe 再挂上去」的中间态。

远程 MCP 若仍存在，只是**配置里的远程依赖**（像 provider endpoint），不是插件，不能替代 Kind B。

---

## 7. 插件格式（给编译器用，不给装载器）

物种运行时不读插件目录。清单是 **pack 配方**：

```text
hello-fs/
  vivy-plugin.json      pack 时读
  README.md
  plugin.go             实现 sdk/plugin 契约
```

```json
{
  "apiVersion": "vivy.plugin/v0",
  "kind": "capability",
  "name": "hello-fs",
  "version": "0.1.0",
  "seam": "tool-world",
  "module": ".",
  "grants": ["fs.read", "fs.write"],
  "tools": [
    {
      "name": "hello_stat",
      "effect": "read",
      "schema": {
        "type": "object",
        "properties": { "path": { "type": "string" } },
        "required": ["path"]
      }
    }
  ]
}
```

没有 `runtime`、没有 `entry` exe、没有 wasm。`module` 指向被 `pack` 链进去的 Go 包。挂 `journal` / `policy` 的 seam 拒收。`grants` 在编译进这一代时冻结，运行时只能按这一代的清单执行，不能靠配置放宽。

---

## 8. 装载 = 打包新版本

没有 `plugins.allow` 拉起外置进程。没有启动预检再 spawn。

```text
加入 / 删掉一个插件源
    → vivy-sdk verify
    → vivy-sdk pack          链进新 vivy.exe，记下 source_ref + 配方 + 哈希
    → 登记 Generation
    → eval 候选（独立数据目录）
    → 人 promote
    → 下次启动换身体
```

卸插件：从配方里拿掉那个源，再 `pack` 一版，同样走 eval / promote。  
活着的 EXE **不**动态加载任何插件代码。

`execute` 指着物种源码硬编，仍是无门自改写；正式路径只许 `vivy-sdk pack`。

### 8.4 Vivy SDK：独立二进制，同一棵源码树

盖房工具必须在场，但不能住在住户那份网关里。`vivy-sdk` **是单独的二进制**，代码住在仓库根的 `sdk/`，不进 `cmd/`，也不挂在 `vivy.exe` 上。

它可以长很大：以后允许自带物种源码快照、Go 发行版或整套 toolchain。体积和发布列车与日常网关切开。

```text
vivy.exe              物种：日常网关 + worker
vivy-sdk.exe          盖房：verify / pack / inspect-artifact
                      源码树：sdk/   （plugin/ 给作者；internal/ 给打包器）
```

住户只拿 `vivy.exe`。作者或工作室拿源码树（或以后的 SDK 发行包）再编。没有物种源码、没有 `go`（本机或 SDK 自带），`pack` 必须高声失败。

**它打包什么**

| 命令 | 输入 | 输出 |
|---|---|---|
| `vivy-sdk verify` | 插件源 + `vivy-plugin.json` | 契约是否满足（seam、grants、schema、可链接） |
| `vivy-sdk pack` | 物种源码 + 要链进这一代的插件源 | **唯一产物：** 一份新 `vivy.exe` + Generation 清单（含哈希与出处） |
| `vivy-sdk inspect-artifact` | 一代 EXE / 清单 | 出处是否完整，能否再盖 |

没有单独的 `build-plugin` 产出外置 exe。插件不能单独成为可加载工件。

**随时准备，不等于住户机能编**

- **准备：** 打包器入口永远在这棵仓库的 `sdk/`，契约与物种同模块，不会和某个外部分发包分叉。
- **现编：** 只在工作室或开发机上发生。日常 `vivy.exe` 里没有 `sdk` 子命令。
- **热路径：** 聊天、工具、审批 **禁止** 调 pack。

**给插件作者的可 import 面**

只开放 `agent-vivy/sdk/plugin`。作者 `import` 后实现接口；`pack` 把包**链接进**新的 `vivy.exe`。不另开 git 模块（D-006）。Eino 不进公开面。

**和 Studio 的关系**

Studio（人或其内部发动机）改源 → 调 **`vivy-sdk pack`** → 得到 Generation → **Studio** 评测 / 发布 / 安装。  
DSH 不负责「怎么编 Vivy」；换掉 DSH，打包器还在 `sdk/`。作者不直接打开 sdk，由 Studio 调。

---

## 9. 通信：哪根管子

能力编进 EXE 之后，工具调用是**进程内函数**（仍过 policy / hook / Journal）。没有插件专用管道。

还在用的管子只剩下「不是插件」的那些：

| 对面 | 传输 | 帧 |
|---|---|---|
| `vivy worker` | 同二进制 stdio | JSONL JSON-RPC（已有） |
| UI / 本机控制面 | 回环 WebSocket | 同一套 JSON-RPC（已有） |
| Studio → 物种 | 只读 | `inspect`（物种不回调 Studio） |
| 远程 MCP（若保留） | HTTP | 配置依赖，不是插件 |
| Kind A Skill | 读文件 | — |

拒绝：为插件再开 stdio/gRPC/WASM/dll。`vivy worker` 不是插件通道，是同二进制的子 run。

---

## 10. 拒绝一切运行时装载

| 形态 | 结论 |
|---|---|
| WASM | **拒绝。** 又一种运行时世界，和「装插件 = 造新版本」对着干 |
| `.dll` / Go `plugin` | **拒绝。** 卸不掉，Windows / cgo 更差 |
| 外置 stdio exe | **拒绝。** 无源盲盒，或第二种身体 |
| 目录扫描 / 插件市场 | **拒绝。** |

有人交东西时只认两种：

```text
Skill 文本              → Kind A，不编译
Go 源码 + vivy-plugin.json → 进配方，等 pack 成新 EXE
其它任何二进制            → 拒收
```

---

## 11. 物种只留只读身份

「无缝」不再是物种上的三扇门。开发与分发的权威在 Studio（`VIVY-STUDIO.md` §4、§8）。不必把 Node 链进 `vivy.exe`，也不从 `vivy.exe` 打开 Studio。

### 物种 `inspect`

只读。版本、代哈希、seam、policy hash、工具清单摘要。  
拒绝：密钥明文、无关 session 正文、不该暴露的绝对路径。  
这是 Studio 观察已安装 / 正运行身体的缝，不是演化入口。

### `eval` / `promote` 不在物种上

评测由 Studio 拉起候选。发布是人在 Studio 里触发的安装。物种侧已有的 `evals/start` 与 Promotion 写入视为错误的家，冻结产品语义。

ACP 若以后批准，只是物种过日子控制面的另一张脸，不是第二条演化通道。

---

## 12. 控制面对象

**过日子的对象**（Run / Session / Approval）仍在物种 Journal。  
**换代对象**（Generation / EvalRun / Release / Install）住在 Studio 自己的库。物种不再是这些对象的权威。完整 schema 见 `VIVY-STUDIO.md` §8。

下列 Run 形状仍属物种：

```text
kind: Run
spec:
  session: ses_...
  loop:    builtin | generation:<hash>
  world:   builtin | plugin:<hash>
  policy:  default | plan | read_only | full_auto
  budget:  { events, models, tools, retries }
status:
  phase: running | suspended | completed | failed | cancelled
  waiting: approval:... | question:...
  seq: 142

```

换代对象（Generation / EvalRun / Release / Install）不在这里。见 `VIVY-STUDIO.md` §8。  
DSH 自己的 session 日志是草稿纸；对人可见的演化记在 Studio 账本，不写进生产 Journal。

---

## 13. 气隙与适应度

活物种与候选不得共享：SQLite / Journal 文件、workspace root、监听地址、生产密钥。

可以共享：仅配置突变时同一份 exe 字节、只读评测套件、对象 schema。

没有命名套件，Studio 只是自动装插件。第一套套件应很小、本地：

- V0/V1 垂直流（会话 → 流式 → 工具 → 审批 → 恢复）
- 一次 Plan Mode
- 一次 Skill 或 MCP
- 挂起审批后的重启恢复

比较分类失败、审批次数、事件数、Journal 是否还能回放。  
「更接近 AGI」不是套件 id。加套件是产品决策。

宏大命题的诚实写法：

> 若 AGI 有一部分是中介架构问题，Vivy 是能搜索那种架构、还不丢掉测量的环境。  
> 若 AGI 几乎全是模型问题，Vivy 仍是换模型不换主权的网关。  
> 系统应在两种世界里都活。

---

## 14. 单 EXE 精确含义

| 是 | 不是 |
|---|---|
| 一个安装器、一个快捷方式 | 身上再挂一串插件进程 |
| 日常生命就是那一个 `vivy.exe`，能力都在里面 | 把 Node / DSH / WASM 链进热路径 |
| 内核、L1、本代能力编在同一文件 | 微服务网格 |
| 新能力只以新 EXE 的形式出现 | 人要先在网关里开实验室才能过日子 |

日常仍是单 EXE 产品。Studio 是第二个应用，不嵌进 `vivy.exe`。  
把 DSH 嵌进主二进制，才是破坏单 EXE。

---

## 15. 落在现有代码上

不要开第二套运行时。加缝即可。

| 已有 | 下一步 |
|---|---|
| 物种 `inspect`、`vivy-sdk`、S1 投影 | 保留为零件 |
| 物种侧 Generation / eval / promote / 工作室卡 | **冻结产品语义**（错误的家，见 NG-28） |
| Studio 应用（尚未存在） | 按 `VIVY-STUDIO.md` ST-1 起：独立进程、账本、开发场地 |
| `internal/worker` | 仍是同二进制子 run，不是插件通道 |
| Policy / Hooks / Plan Mode | 准入，哲学不变 |
| Skill 修订 | 行为包；能力变更走 pack |
| `mcp_servers` | 最多保留为远程依赖配置，不升级成插件 |

Eino 仍是内置 loop，隔离不变。候选代才可以换 loop。不把 Cordis 链进 `vivy.exe`。

---

## 16. 不变量

1. 过日子时，人面对的产品同一时刻是一个网关进程。开发时，人面对的是独立的 Studio。
2. §4.2 的内核不是插件，Studio 不能对活实例热改进它。
3. 模型可见 ≡ 已记录。密钥不记。换代事实记在 Studio 账本。
4. worker / 插件 / Studio 都不能放宽物种 policy snapshot。
5. 能力没有运行时卸载。拿掉能力 = 再 pack 一版 + 在 Studio 发布。worker 仍可杀。
6. 新身体只在下次启动日常位之后生效。禁止热换。
7. 生产实例的 workspace 不是 Vivy 源码树。源码树是 Studio 的工程。
8. DSH 若在，只是 Studio 内部发动机；它不在，已安装物种仍启动。
9. 两代只通过命名套件上的 EvalRun 比较。评测家长是 Studio。
10. 一个阶段一个主使命。建 Studio 不能停日常 Operate；**切换之后开发只在 Studio 里进行**（NG-26）。

---

## 17. 非目标

- 用 TypeScript 重写物种，或把 Node 放进热路径
- DSH 成为 `vivy.exe` 的必选依赖
- 对活内核做同进程自修改
- 微服务、服务网格、本机 Kubernetes
- 多租户或托管 Studio（D-016 仍有效，除非另立决策）
- 从 `vivy.exe` 打开 Studio，或把 Studio 做成网关里的一张卡
- 插件市场、目录扫描、`dsh-plugin` 式发现
- WASM、`.dll`、Go `plugin`、stdio 外置插件 exe
- 把远程 MCP 当成「装插件」
- 把 Memory / Laputa / AutoDream 绑进本架构
- 把「到达 AGI」当 sprint 验收

---

## 18. 分期（一阶段一使命）

S0 已采纳。S1–S6 物种侧零件 **done**。S7 工作室卡 **产品含义作废**（NG-28）。S8 / S9 改在 Studio 切换之后、于 Studio 内做。

工位切片与开发场地见 `VIVY-STUDIO.md` §3、§10（ST-0..ST-8）。ST-6 是外环合上的切换事件。

---

## 19. 关键决策

| ID | 决策 | 理由 |
|---|---|---|
| NG-1 | 物种与 Studio 是两具身体、两个应用 | 环境不能同时是种群 |
| NG-2 | 物种保持单个 Go EXE | 个人网关、Windows、已有内核 |
| NG-3 | 进化气隙，下次启动日常位才生效 | 失败突变体不得拆掉日常与账本 |
| NG-4 | 物种只暴露只读 `inspect`。eval / 发布 / 安装是 Studio 的协议 | 2026-08-15 纠正：不是物种上的三扇门 |
| NG-5 | DSH 是 Studio 内部第一任发动机，不是信任根，不是物种依赖 | 不联姻预览框架；物种不嵌 Node |
| NG-6 | 行为是文本；能力是待编译源码；装上只有 Generation | 想 / 源 / 新身体 |
| NG-7 | 学 DSH 纪律，拒 DSH 身份 | seam、日志、薄 loop；不做同进程插件 OS |
| NG-8 | 物种控制面仍是唯一过日子写入；Studio 是另一应用，不是 Pod | 不要本机网格，也不要把 Studio 写成物种数据面 |
| NG-9 | 适应度是命名套件 | 否则 Studio 是自膨胀 |
| NG-10 | 模型可见 ≡ 已记录，升级 ADR-009 | 否则代与代无法对照 |
| NG-11 | 拒绝 WASM、dll、Go plugin、stdio 外置插件。能力只经 `pack` 链进新 EXE | 装插件 = 造新版本；不要第二种身体 |
| NG-12 | 不扫描目录；配方里点名源码包，产物用哈希 | 精选目录，不是市场 |
| NG-13 | 同合同的分流是 Generation，不是新物种 | 保住遗传与评测 |
| NG-14 | `vivy-sdk` 是独立二进制，源码住仓库根 `sdk/`。日常 `vivy.exe` 不挂 sdk。热路径禁止编译。由 Studio 调 sdk | 住户 EXE 不能假装能编 |
| NG-15 | 装插件、卸插件、改能力，都等于造新版本并走 Studio 的 eval / 发布 | 没有运行时插件面 |
| NG-16 | Vivy Studio 是独立应用（工业 IDE）：封好的发动机 + Skill + SDK/工具链；与日常 EXE 分开发行 | 作者打开 Studio；住户只拿网关 |
| NG-17 | 能力是纯 Go，但是用户插件才用插件治理 | 「我在开发某个插件」只适用于 `plugins/` |
| NG-18 | 出厂按所是命名并按配方装配；plugin 一词只留给用户层 | 学 DSH 的命名与叠加，不学热加载 |
| NG-19 | 远程 MCP 是配置依赖，不是插件 | 外打电话 ≠ 往身上长手 |
| NG-20 | 先把 Studio 做成可开发场地（DSH 进 Studio），再换皮；发布仅人闸 | 开发场地硬条件优先于物种侧「门」 |
| NG-21..28 | 见 `VIVY-STUDIO.md` §11 | 独立应用、账本在 Studio、开发场地、冻结物种卡 |

---

## 20. 已确认

1. **2026-08-14 收为方向。** 物种 / 内核 / 装配准绳仍在。V0 ADR 仍有效。
2. **远程 MCP 留作配置型远程依赖。**
3. **2026-08-15：`vivy-sdk` 从日常 EXE 拆出。** NG-14。
4. **2026-08-15 纠正 Studio：** 独立应用；管开发与分发全生命周期；物种不启动它；账本不在物种 SQLite。正本 `VIVY-STUDIO.md`。
5. **2026-08-15 开发场地：** 后续开发全部在 Studio 进行。bootstrap（ST-1..ST-4）是唯一外环；ST-6 完成即切换（NG-26）。
6. **发布只能人在 Studio 里点。** 不是物种卡上的 promote。
7. **产品名：** 中文「Vivy Studio」/「工作室」指独立应用；英文 `Studio`。不再指网关里的一张卡。

---

## 21. 四句合同

以 `VIVY-STUDIO.md` §12 为准：

- **网关：** 过日子的唯一身体，Journal 的唯一写入者。
- **Studio：** 开发与分发的唯一应用，下一代身体的唯一作者与安装者。
- **插件：** 满足契约的源码。只有被 SDK 编进某一代 EXE 之后才存在于世界里。
- **人：** 唯一发布者。切换之后，开发者（含 agent）只在 Studio 里改 Vivy。

一个设计若要其中两句同时作废，就不是这份架构。
