# Vivy Studio

> 状态：**方向已采纳**（2026-08-15 用户纠正并确认）。
> 本文是 Studio 产品身份、生命周期和开发场地的正本。
> 若本文与 `SELF-EVOLVING-GATEWAY.md` / `VIVY-GATEWAY-AND-STUDIO.md` 在 Studio 形状上冲突，以本文为准，并应回写那两份。
> 日期：2026-08-15
>
> 相关：
> - `SELF-EVOLVING-GATEWAY.md` — 物种 / 内核 / 装配 / 反热加载
> - `VIVY-WORLDVIEW.md` — 产品哲学与世界观的结构同构（不取代场地硬条件）
> - `VIVY-GATEWAY-AND-STUDIO.md` — 英文决策号（NG-*）
> - `VIVY-ASSEMBLY.md` / `VIVY-PLUGIN-SPEC.md` — 配方与用户插件
> - `.workspace/deepseek-harness/upstream` — 第一任开发发动机的证据源（官方克隆，`47f9438`）

---

## 0. 一句话

**Vivy Studio 是独立应用。** 它管理 Vivy 的开发与分发全生命周期。  
**日常 `vivy.exe` 是它的产物，不是它的宿主。**  
不存在「从网关打开工作室」。物种不启动 Studio。

人有两个入口，用途不许混：

```text
作者 / 开发
  打开 Vivy Studio.exe
    工程 → 开发 → 验证 → pack → 评测 → 发布 → 安装 / 回滚

住户 / 日常
  打开 vivy.exe
    会话、审批、Journal、工具
    不问 Studio，不编二进制，不晋级别人
```

第一任开发发动机可以是钉死的 DeepSeek Harness。人看见的名字是 Vivy Studio，不是 DeepSeek。换掉发动机，Studio 还在。

---

## 1. 为什么必须是独立应用

物种要过夜：密钥在本地、Journal 可回放、人敢把生活交给它。  
Studio 要白日盖楼：改源码、跑测试、编 EXE、评测、把新身体装进日常位。

这两件事不能共用：

- 同一个进程
- 同一份 SQLite
- 同一组生产密钥
- 同一条「从聊天框打开实验室」的入口

旧稿把 Studio 写成物种上的角色、三扇门的客户端、网关里的一张卡。那是错的家。`internal/studio`、物种 RPC 上的 `evals/start` / `promotions`、嵌入 UI 的工作室卡，都是把账本做进了产物进程。本文冻结它们的产品含义，不再加功能。

---

## 2. 硬条件 — 开发场地是 Studio

**自 Studio 达到 §3 的切换门槛起，Vivy 的后续开发全部在 Vivy Studio 里进行。** 这是场地约束，不是口号。

「开发」包括：改物种、改 Studio 自己、改 Skill / 配方 / 插件、改作为产品合同的架构文档与测试。作者和 coding agent 坐在 Studio 里，不坐在日常 `vivy.exe` 里，也不再把外部 IDE / 外部 agent 当主开发环。

### 2.1 切换之后禁止

- 以 Cursor / Claude Code / 本机聊天 agent 等外部环路作为实现 Vivy 的主路径
- 在日常 `vivy.exe` 的会话里开发 Vivy（物种是住户产品，不是 IDE）
- 以「Studio 还没做好」为由继续在物种进程里加换代 UI、Promote 权威、评测农场
- 第二次「先在外面做完再搬进 Studio」——bootstrap 只有一次

### 2.2 切换之后仍允许、但不是开发环

- 住户继续用 `vivy.exe` 过日子
- 操作系统级安装、杀进程、看日志
- Studio 内的 `pwsh` 调 `git` / `just` / `go`（这是 Studio 在干活，不是外环）
- **唯一紧急例外：** Studio 自己起不来。只允许在外环做恢复 Studio 启动的最小改动，并留下书面记录。不得顺手做功能。

### 2.3 为什么这条必须现在写死

若开发场地仍留在外环，Studio 会永远排在「下一轮」。  
若在 Studio 还不能改 Vivy 源码时就宣布已切换，场地标准是空的。

所以：先用一次、也是唯一一次 bootstrap 把 Studio 做到能开发 Vivy；门槛一过，外环合上。

---

## 3. Bootstrap（唯一一次外环）

Studio 还不存在时，无法在 Studio 里把它造出来。允许、且只允许下列工作在 Studio 外完成：

| 允许 | 不允许塞进 bootstrap |
|---|---|
| 本文与相关文档的纠偏（本次） | 物种新功能、新工具、新 provider |
| ST-1 独立进程能起来 | 物种里继续做工作室卡 |
| ST-2 工程钉在源码树，碰不到生产 Journal | 把 Generation 账本继续做大在物种 SQLite |
| ST-3 工具链：Go / gopls / just / `vivy-sdk` | 发行物换皮、插件市场 |
| ST-4 预制 Skill（插件五步 + 本体 `just ci`） | 自动 promote |
| 做到 ST-6 能在 **Studio 内** 落地一次真实本体改动 | 任何「顺便」的物种功能 |

**切换门槛（ST-6 完成即切换）：**

1. Vivy Studio 作为独立进程启动，不经过 `vivy.exe`。
2. 打开的工程是 `agent-vivy` 源码树（或从其切出的 worktree），不是 `data/`。
3. 在 Studio 内能改一处本体（`internal/` / `cmd/` / `ui/` / 产品文档）、跑通 `just ci`。
4. 这次改动的作者路径是 Studio，不是外环 agent。

门槛达成之后：ST-5（Studio 自己 pack / 评测）、ST-7（发布 / 安装）、ST-8（回滚）、以及此后一切功能，都在 Studio 里开发。

bootstrap 没有第二次。若 Studio 长期起不来，走 §2.2 紧急例外，做完立刻回来。

---

## 4. Studio 拥有的生命周期

权威在 Studio。物种不承办这些阶段。

| 阶段 | Studio 做什么 | 对象 |
|---|---|---|
| 工程 | 打开 / 管理源码树或插件树 | Worktree |
| 开发 | 内置 coding agent 改代码（第一任 DSH） | 工作树 diff |
| 验证 | `just ci`、`vivy-sdk verify` | Check |
| 出体 | `vivy-sdk pack` | Generation（工件 + 清单） |
| 评测 | **Studio 自己**拉起候选 EXE，独立数据目录 | EvalRun（Studio 库） |
| 对照 | 两代差、套件结论 | Report |
| 发布 | 人在 Studio 里点发布 | Release |
| 分发 | 把 EXE 写入日常安装位，或导出工件 | Install |
| 回滚 | 装回上一个已发布体 | 仍是一次 Install |
| 观察 | 只读询问已安装或正在跑的物种 | 物种 `inspect` 快照 |

评测不是「请活着的 `vivy.exe` 去 `evals/start`」。候选的家长是 Studio。  
发布不是物种库里写一行 `applies_at: next_launch`。发布是 Studio 完成一次安装：日常位里的 `vivy.exe` 换成新文件。下次双击才换身体。禁止热替换活进程。

人闸仍在：Studio 这个进程不得自动发布。人点发布。规则自动发布以后另立决策。

---

## 5. 两个应用各自握什么

### 5.1 Vivy Studio（权威：开发与分发）

- 源码与工作树
- 开发会话（DSH 只是这里的发动机）
- 工具链：Go、gopls、just、`vivy-sdk`
- 自己的账本：Worktree / Generation / EvalRun / Release / Install
- 日常安装位的布局与回滚点
- 评测用密钥与套件（与住户密钥隔离）

### 5.2 `vivy.exe`（权威：过日子）

- 会话、Journal、审批、Ask User、预算、恢复
- 只读身份：二进制哈希、当前配方、工具名（`inspect`）
- 住户的模型密钥
- 同二进制 `vivy worker`

**禁止进物种：** pack、评测农场、发布、安装别人、Studio 主界面、开发 Vivy 源码。

物种挂了，住户当天过不好。Studio 挂了，住户仍应能打开已安装的 `vivy.exe`。  
Studio 挂了，**开发停止**（除非 §2.2）。物种不是开发的备份场地。

### 5.3 中间只留窄缝

| 方向 | 缝 | 不是 |
|---|---|---|
| Studio → 已安装 / 正运行物种 | 只读 `inspect` | 写 Journal、改 policy |
| Studio → 日常安装位 | 写文件、起停进程 | 热补丁活内核 |
| 物种 → Studio | **没有** | 打开工作室、回调 promote |

`vivy-sdk` 仍是独立二进制，住在仓库 `sdk/`。它是 Studio 发行包里的盖房工具，不是 `vivy.exe` 的子命令，也不是第三个要给人天天打开的产品。作者面对的是 Studio；Studio 去调 sdk。

---

## 6. 开发仍分三档，都发生在 Studio

| 档 | 人在干什么 | 工作目录 | 成功 |
|---|---|---|---|
| A. 做插件 | 给自己加能力 | 只打开 `plugins/<name>/` | `verify` → `pack` → `eval` |
| B. 改出厂件 | 改 notes / fs / provider / 配方 | `tools/`、`providers/`、`vivy.generation.yml` | 再 pack 一版 |
| C. 改本体 | 改 Journal、审批、runtime、UI、Studio | `internal/`、`cmd/`、`ui/`、Studio 自身 | `just ci`，需要换体再 pack |

硬条件「Studio 能自己开发 Vivy」指 C，不是只做 A。  
工作树一开始就对准整棵 `agent-vivy`。Skill 按档分流；发动机必须看得到整仓。

---

## 7. DSH 在这里的位置

DeepSeek Harness 是 **Studio 应用内部的开发发动机**，不是物种的后端，也不是产品名。

拿：`dsh-base` + `dsh-web-app`，以及 fs / pwsh / grep / lsp / skill / workflow / subagent / plan / 审批。Windows 用 pwsh。`lsp` 接 `gopls`。

不拿：默认 `tool-cordis`（活进程热挂，重启即散，官方声明不是安全边界）；插件市场；把 DSH 会话日志当成 Vivy Journal 或 Studio 账本。

第一期壳可以就是钉死提交的 DSH Web（`.workspace/deepseek-harness/upstream`，现为 `47f9438`），但 profile 名、窗口名、工程、Skill 必须是 Vivy Studio。不从 `vivy.exe` 拉起。换皮后做。

---

## 8. 账本住在 Studio

下列对象是 Studio 的产品历史，存在 Studio 自己的库里，不是物种 SQLite 的旁路表。

```text
kind: Worktree
spec:
  path: <canonical source tree>
  kind: plugin | first-party | kernel
status:
  dirty: bool

kind: Generation
spec:
  parent:     gen_...
  artifact:   sha256:...
  recipe:     vivy.generation.yml
  source_ref: git:... | worktree:...
status:
  phase: built | eval_pending | evaluated | released | rejected

kind: EvalRun
spec:
  candidate:  gen_...
  baseline:   gen_...
  suite:      <frozen id>
status:
  verdict:    better | worse | mixed | failed_to_run
  journal_ref: <candidate dir, never production>

kind: Release
spec:
  generation: gen_...
  eval:       evl_...
  actor:      human
status:
  phase: accepted
  # 发布不等于已换活进程

kind: Install
spec:
  release: rel_...
  target:  <日常安装位>
status:
  phase: current | rolled_back
```

物种侧已实现的 Generation / EvalRun / Promotion 表（ADR-011..016）视为**错误的家**：保留代码直到 Studio 账本可替换它们，但产品权威已迁走。不再扩展物种侧这些 API 的产品语义。

表达不出这些对象的「进化」，Studio 当没发生过。DSH 会话日志是草稿纸。

---

## 9. 日常安装位

本机一个由 Studio 管理的目录，与仓库、与 `agent-vivy/data/` 都分开。

发布 / 分发 = 往这里放置 `vivy.exe`（及它需要的只读资源）。  
住户快捷方式指向这里。  
回滚 = 把这里恢复成上一 Release 的文件。  
正在跑的进程继续用旧映射，直到退出。

---

## 9.1 产品身份与主题（硬条件）

打开窗口的人必须立刻知道这是 **Vivy Studio**，不是 DeepSeek Harness，也不是日常 `vivy.exe`。

社区主题几乎只换颜色，换不了产品名。官方把身份写死在三处：侧栏 `BrandWordmark`（鲸 + `HARNESS` 徽章 SVG）、`document.title`（`DeepSeek Harness`）、设置欢迎文案（DSH 插件生态）。主题系统只认 `--dsw-alias-*` token，第三方注册**不校验是否覆盖完全**。所以身份和配色要拆开写条件。

### 别人已经做好的（只当证据，不装进产品）

| 东西 | 它实际做了什么 | 对我们 |
|---|---|---|
| 官方 `ui-theme` | `light` / `dark` / `system`；`ThemeRuntime.register()` 可叠一层 alias | 换色的合法缝 |
| [dsh-theme-lab](https://github.com/Ultronen/dsh-theme-lab) | 走官方 token 覆盖：整壳半透明、模糊、壁纸 | 学它用官方层；不把工位做成玻璃桌面 |
| [dsh-custom-css](https://github.com/AnacondaKC/dsh-custom-css) 的 Aurora Nexus | 一份 CSS 盖全 `--dsw-alias-*`、排版、阴影、滚动条、Shiki；有安全卸载 `?off=` | **完整主题清单的样板**；不靠用户粘贴 CSS |
| `dsh-qq2006` / `dsh-deep-whale` / maid-atelier / whale-girl | 整皮、桌宠、鲸娘 | 身份冲突；部分非商用协议；禁止当 Studio 默认脸 |
| 桌面壳（harness-desktop 等） | 窗口控件跟着 DSH 页主题走 | 以后自有壳也要跟 Studio 主题，不跟鲸 |

社区目录里「Themes」大多是皮肤中心和二次元。Studio 是工业 IDE，不走那条。

### 身份（必须，ST-1 验收的一部分）

打开后，下列位置不得再出现 DeepSeek / 鲸标 / `HARNESS` 徽章当产品名：

1. 窗口标题与任务栏：`Vivy Studio`（会话标题可作前缀：`会话 — Vivy Studio`）
2. 侧栏字标：自有 wordmark，不是 `BrandWordmark` 那张鲸 SVG
3. 设置 / 欢迎 / onboarding：Studio 文案（开发与分发），不是 DSH 插件生态欢迎词
4. Profile 名：`vivy-studio`（或 `vivy-factory`），`dsh --dump-config` 里看得到
5. 进程 / 快捷方式显示名：Vivy Studio

允许在「关于」或引擎页用小字写：开发发动机来自 DeepSeek Harness（可替换）。那是出处，不是产品名。

### 主题（必须，和身份同一切片交付）

1. **第一方密封**，编进 Studio profile，不是 `dsh plugin add` 市场上的皮。
2. 走官方 `ThemeRuntime.register()` + 完整 `--dsw-alias-*`（含滚动条、阴影、Shiki）。缺一块就仍是 DSH 色，算没换完。Aurora Nexus 的覆盖面是验收对照，不引用它的文件。
3. 同时提供 **light 与 dark**，默认 `system`。
4. 工业 IDE：高对比、可长时间对着代码。禁止默认壁纸、液态玻璃、桌宠、QQ 皮、鲸娘。
5. 无障碍：`prefers-reduced-motion`、可读对比。不加载远程字体/图。
6. 官方已知缺口：第三方主题不校验完整性。我们自己列一份 token 清单，缺了就失败，不靠肉眼。
7. 不把 `dsh-custom-css` 当产品路径：任意 CSS 会进设置、所有浏览器共享，信任模型不对。

配色具体色值另定，不挡 ST-1 开工；ST-1 合上前身份五条必须绿。

当前第一方皮：`studio/dsh-vivy-studio/`（萤石主题 + Vivy Studio 字标 / 欢迎）。用 `launch-vivy-studio.ps1`（仓库根）起 **`dsh --profile vivy-studio`**，`DSH_HOME` 在 `data/studio-home/`，不碰日常网关。

---

## 10. 切片

S1–S6 已完成的物种侧缝（模型可见、inspect、verify、pack）仍是可用零件。  
S7 工作室卡 **不再是主线**，产品含义作废。  
S8（禁止生产实例无门自改写）仍要做，但是物种护栏，在 Studio 切换之后于 Studio 内实现。  
S9（冻结评测套件）是 Studio 评测阶段的事，在切换后做。

工位切片：

| ID | 内容 | 场地 | 完成证据 |
|---|---|---|---|
| ST-0 | 本文纠偏：两应用、账本在 Studio、开发场地硬条件 | bootstrap（本次） | 架构文一致 |
| ST-1 | Studio 独立进程（钉死 DSH + `vivy-studio` profile）+ §9.1 身份与主题 | bootstrap | 不经过 `vivy.exe`；标题/字标/欢迎词是 Vivy Studio |
| ST-2 | 工程钉住源码树；与任何生产 `data/*.db` 隔离 | bootstrap（2026-08-16） | 启动钉 `agent-vivy`；禁读生产 Journal |
| ST-3 | 工具链进 Studio | bootstrap（2026-08-16） | 启动环境 `go version`、`vivy-sdk verify plugins/hello-fs` |
| ST-4 | Skill：插件五步 + 本体 `just ci` | bootstrap（2026-08-16） | `.agents/skills/vivy-plugin-five`、`vivy-kernel-ci` |
| ST-6 | 在 Studio 内完成一次真实本体改动 + `just ci` | **切换事件（2026-08-16）** | `internal/buildinfo` + `just ci`；作者路径是 Studio |
| ST-5 | Studio 自己 pack + 自己评测候选 | 切换后，在 Studio 里开发（**2026-08-16 done**） | `cmd/vivy-studio` + `internal/studiocore`：`pack` exec `vivy-sdk`，`eval` 由 Studio 拉起候选 EXE、独立数据目录，账本在 `data/studio-home/studio.db`；活物种进程零参与 |
| ST-7 | 发布 → 安装到日常位 → 下次启动是新 EXE | 切换后（**2026-08-16 done**） | `release` 仅 `--actor human --yes`；`install` 写日常位 + `install.json`，不热换活进程 |
| ST-8 | 回滚到上一 Release | 切换后（**2026-08-16 done**） | `rollback` 从 Studio 快照恢复上一 Release 文件；`data/vivy.db` 未被触碰 |

ST-6 排在 ST-5 前：先证明能在 Studio 里改 Vivy，再在 Studio 里把评测和分发做完。这是场地硬条件的最小证据。

---

## 11. 决策号（续 NG）

下列修正或追加 `VIVY-GATEWAY-AND-STUDIO.md` 的 NG 表。

| ID | 决策 |
|---|---|
| NG-21 | Vivy Studio 是独立应用，管理开发与分发全生命周期。日常 `vivy.exe` 是产物。 |
| NG-22 | 物种不启动 Studio。不存在「打开工作室」入口。 |
| NG-23 | Generation / EvalRun / Release / Install 的权威账本在 Studio。物种只保留只读 `inspect`。 |
| NG-24 | 评测由 Studio 拉起候选。活物种不是评测家长。 |
| NG-25 | 发布是人在 Studio 里触发的安装。禁止自动发布，禁止热换活进程。 |
| NG-26 | **开发场地是 Studio。** bootstrap（ST-1..ST-4）是唯一外环；ST-6 完成即切换。之后外环不得作为实现主路径。 |
| NG-27 | Studio 挂了不影响已安装物种过日子；物种挂了不是改代码的借口。 |
| NG-28 | 物种侧工作室卡与 Promote 权威冻结，不再扩展产品语义。 |
| NG-29 | 打开即为 Vivy Studio：标题、字标、欢迎词、profile 名。主题是第一方完整 token 集，不是社区皮、不是粘贴 CSS。 |

NG-4、NG-16、NG-20 按本文修订：三扇门不再是物种上的 eval/promote；DSH 先于换皮进 Studio，因为开发场地要先切过去。

---

## 12. 四句合同

- **网关：** 过日子的唯一身体，产品真相（Journal）的唯一写入者。
- **Studio：** 开发与分发的唯一应用，下一代身体的唯一作者与唯一安装者。
- **人：** 唯一发布者，直到另立规则。
- **开发者（含 agent）：** 切换之后只在 Studio 里改 Vivy。

一个设计若要其中两句同时作废，就不是这份架构。
