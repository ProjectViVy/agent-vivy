# Vivy 怎么分、怎么装配

> 状态：**提案**。服从重构世界、`SELF-EVOLVING-GATEWAY.md` 与 **`VIVY-STUDIO.md`**。
> channel 作为超级通道见 **`VIVY-CHANNEL-PACK.md`**（**方向已采纳** 2026-08-30）。Host 在内核；本批适配器是 `plugins/` + `seam: channel`，**不**增加 `channels:` 配方键。
> face 作为一等装配单元见 **`VIVY-FACE-PACK.md`**（提案；未采纳前本表不增加 `face:` 行）。
> 日期：2026-08-15（Studio 纠正：装配发生在独立 Studio 应用里，不在网关里）
> 对照：DeepSeek Harness 的 profile / bundle / 按职责命名的包，不是对照它的热加载。

---

## 1. 从 DSH 学到的装配，不是名字

DSH 运行时是一棵树，但**树上每一行的名字是它是什么**，不是「plugin」：

```text
id: session      → 会话日志
id: llm          → 模型适配
id: tools        → 工具注册表
id: agent-loop   → 循环
id: tool-bash    → bash 工具
id: fs           → 文件系统
```

Cordis 在底下把它们都当成可逆插件。对人、对配方，它们是 session、llm、loop。  
用户再往 profile 上叠的、树外装的，才是「我加的那一层」。

装配方式是 **叠加**：`dsh-base` → `dsh-web-app` / `dsh-headless` → 用户 patch。  
一行一个 id，后写覆盖整行 config。

Vivy 学这三件事：

1. **按所是命名。** 工具叫工具，世界叫世界，循环叫循环。
2. **用配方装配，不靠内核里手写 import。**
3. **只有用户加进来的那一层叫插件。**

不学：运行时热挂、把 Journal / policy 也编成可卸行。

Vivy 的叠加发生在 **`vivy-sdk pack`**，产物是一整代 EXE，不是一棵活树。

---

## 2. 怎么分（名词就是种类）

```text
内核（不可装配掉）
  journal / policy / rpc / 只读 inspect / ChannelHost
  （盖房工具是独立二进制 vivy-sdk，源码在 sdk/，不在内核里可卸）

可装配的一等单元（是什么叫什么）
  loop        这一轮怎么算     出厂：eino
  world       fs+exec 绑在一起   出厂：sandbox | local
  provider    模型出口         出厂：openai | anthropic
  tool        面向模型的能力    出厂：notes、filesystem、execute、ask_user…
  skill       行为文本         出厂与用户都可以有；不编译

只有用户自定义
  plugin      别人/自己加的能力源码包
```

| 种类 | 仓库里住哪（目标） | 配方里的键 | 谁开发时的感觉 |
|---|---|---|---|
| 内核 | `internal/{domain,storage,rpc,app,…}` | 没有。永远编进来 | 在改 Vivy |
| loop | `loop/eino`（今：`internal/runtime` 里的 Engine） | `loop:` | 在改循环 |
| world | `worlds/sandbox` | `world:` | 在改执行世界 |
| provider | `providers/openai`（今：`internal/provider`） | `providers:` | 在改模型出口 |
| tool | `tools/notes`（今：`internal/tools`） | `tools:` | 在改工具 |
| skill | `skills/…` 或 `data/skills` | 不进 pack 链接 | 在写技能 |
| **plugin** | **`plugins/<name>/`** | **`plugins:`** | **在做插件** |

出厂代码**禁止**放进 `plugins/`。放进去就会冒充用户层，命名就脏了。

例外（`VIVY-CHANNEL-PACK.md` 2026-08-30）：本批通道适配器（telegram / discord / feishu / dingtalk / qq）不是内核器官，是可选耳朵，因此进 `plugins/<name>/`，清单 `seam: channel`，inspect 按 seam 打标签而不是叫 tool。不增加 `channels:` 配方键。日后若第一方器官变多，目录可迁到 `channels/<name>/`，ABI 不变。

现有 `internal/tools`、`internal/provider` 可以先继续住在 `internal/`，配方用名词点名它们。物理搬家是后续切片，不挡装配语义。

---

## 3. 怎么装配（一代一张配方）

`vivy.generation.yml`（仓库根；由 **Vivy Studio** 打开的工程树持有，不是日常 `data/`）：

```yaml
apiVersion: vivy.generation/v0
loop: eino
world: sandbox
providers:
  - openai
tools:
  - notes
  - filesystem
  - execute
  - ask-user
  - http-request
plugins:
  - acme-search          # 只住在 plugins/acme-search
```

规则：

- `loop` / `world` 各一个。`providers` / `tools` / `plugins` 是列表。
- **没写进配方的，这一代不存在。** 不扫描 `tools/`，更不扫描 `plugins/`。
- `pack` 按配方链接，生成一份作者不准手改的注册表。
- `inspect` 分类列出：loop、world、providers、tools、**plugins**。不要把 notes 打印成 plugin。

这就是 DSH 的 bundle 行在重构世界里的对应物：id 还是 session/llm/tool，只是「应用」变成「链进这一代 EXE」。

叠加顺序（对应 DSH 的 base → mode → user patch）：

```text
1. 内核              永远在
2. 出厂 loop/world/provider/tool   配方点名
3. plugins:          用户层，最后叠上
```

用户层只能**增加**自己的 plugin，不能用配方删掉内核，也不能把 plugin 的 seam 写成 journal/policy。覆盖某个出厂 tool：在配方里拿掉那个 tool，再在 `plugins/` 里给一个同职责的实现——名字仍是 plugin，因为它是用户加的。

---

## 4. 开发手感（名词分开）

| 你在干什么 | 待在哪 | 口头禅 |
|---|---|---|
| 改审批、Journal、物种合同 | `internal/` | 改内核 |
| 改 Eino 接线、middleware | `loop/` 或 runtime | 改循环 |
| 改 notes / 文件工具 | `tools/<name>/` | 改工具 |
| 接一个新模型出口 | `providers/<name>/` | 改 provider |
| **给自己或客户加能力** | **`plugins/<name>/`** | **做插件** |

Studio 预制流水线也按名词拆，不要一条「添加插件」包打天下。这些流水线属于独立 Studio 应用（`VIVY-STUDIO.md`）；其他已获授权工具也可直接在源码工作区开发与验证：

- 新工具（出厂贡献）
- 新 provider
- **新插件**（用户自定义）
- 换 world / 换 loop（换代，仍不叫插件）

`VIVY-PLUGIN-SPEC.md` **只约束 `plugins/`**。出厂 tool / provider 可以共用同一套 Go 接口（方便 pack），但目录、配方键、inspect 标签、口头禅都不是 plugin。

---

## 5. 和 DSH 对照（短）

| DSH | Vivy |
|---|---|
| 一切在 Cordis 里都是插件 | 只有用户自定义叫插件 |
| `id: session` 等按所是命名 | 同样按所是命名 |
| bundle + profile 运行时叠加 | generation.yml + `vivy-sdk pack` 编译期叠加 |
| 用户树外插件 | `plugins/` |
| 卸 = 回放逆操作 | 卸 = 配方去掉再 pack 一版 |
| loop 也是可热换行 | loop 是可换代的装配单元，物种内特权 |

---

## 6. 一句话

> **分的时候用真名：循环、世界、工具、出口、技能。**  
> **装的时候用配方：点谁，谁才进这一代 EXE。**  
> **插件这个词留给用户自己加的那一层。**
