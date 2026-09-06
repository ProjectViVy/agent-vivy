# TUI 细节打磨提案（参考 Crush）

- 分支：`feat/tui-detail-polish`（worktree：`../agent-vivy-tui-polish`，从 `main@c64b63a` 切出）
- 定位：速刷分支，允许不合并；只做显示细节，不碰协议、不碰 runtime、不引入新依赖。
- 范围：`sdk/tui/view`（共享视图）、必要时 `sdk/tui/surface`（新增只读展示字段除外，本提案默认**不改** surface）、`cmd/vivy/tui.go` 不动。
- Eino 能力检查：本提案全部为终端渲染层，不涉及 agent loop / 模型编排 / 流式管线，Eino 复用检查不适用（N/A）。不新增任何 `eino*` import，维持导入隔离。

## 1. 现状盘点（避免重复造轮子）

以下能力**已经存在**，派单时明确"不要重做"：

| 能力 | 位置 |
|---|---|
| Crush 式三段布局（chat / editor / 右侧栏），宽窄双模式 + 断点 | `view/layout.go`、`view/render.go:265-283` |
| 侧栏：标题、updated、cwd、host、reasoning、Context（百分比/分级配色/compact 提示）、Session Usage、Modified Files、MCP/Skills/LSP | `view/render.go:306-540` |
| Composer 圆角盒 + chips 行（model · thinking · mode · provider · context%）+ 附件 chips + 模色边框 | `view/render.go:748-851` |
| 统一/分栏 diff、`+N -N` 统计行、hunk 配色（gate 对话框内） | `view/render.go:1108-1257` |
| 工具卡：状态图标（◉/✔/✖）、8 行截断 + `tui.debug` 全量开关 | `view/render.go:664-722` |
| Markdown（glamour，完整版 + quiet 版）、md 行缓存 | `view/markdown.go`、`view/model.go` |
| 文件补全、命令面板、模型选择器、会话对话框、快捷键对话框、gate 审批对话框（含横向滚动、分栏 diff 切换） | `view/render.go:94-264, 928-1348` |
| chat 自动跟随/滚动（`chatFollow`/`chatScroll`）、流式光标 `▌`、ANSI 安全截断 | `view/render.go:543-658` |
| 底部 chrome 行（快捷键提示 / err / `run…`） | `view/render.go:812-832` |

已确认的**缺口**（本提案的内容来源）：无 spinner（全仓 `sdk/tui` grep "spinner" 为 0）；多行输入只显示最后一行（`render.go:755-758`）；无空输入占位符；无粘贴防护；工具卡不可展开；无滚动指示；无窗口标题；modified files 的 `+%d -%d` 未配色（`render.go:444`）；busy 态只有静态文本 `run…`。

## 2. 特性清单与派单批次

`render.go` / `model.go` 是热点文件，按"批次"串行推进（批内特性同区，一起做一个子代理或一个人类 lane），批次之间互不重叠时才可并行。

| 批次 | 特性 | 主题 | 主要文件 | 规模 |
|---|---|---|---|---|
| A | F2 F3 F4 F11 | 编辑器（composer） | `layout.go` + `render.go` 编辑器区 + `model.go` 输入处理 | ~300 行 |
| B | F1 F12 F6 | 状态行/忙碌态/滚动指示 | `render.go` chrome 区 + `model.go` tick | ~220 行 |
| C | F5 F9 F13 | 聊天体（工具卡/推理/空态） | `render.go` 消息区 + `model.go` 按键 | ~260 行 |
| D | F10 F7 | 侧栏配色 + 窗口标题 | `render.go` 侧栏区 + `model.go` | ~80 行 |

推荐顺序 A → B → C → D；每个特性单独提交（`feat(tui): ...`），符合 commit-one-concern 规则。

---

## 批次 A：编辑器细节（对应 Crush composer）

### F2 多行输入完整可见 + 编辑器自适应高度（已完成）

**现状**：`renderEditor`（`render.go:748`）用 `strings.LastIndex(display, "\n")` 只渲染**最后一行**，前面输入的行在界面上消失；编辑器高度是常量 `editorHeight = 4`（`layout.go:24`），与内容无关。

**目标显示细节**（对齐 Crush 的 textarea 行为）：
1. Composer 内按 `\n` 分行渲染**全部**输入行，每行 ANSI 安全截断到内容宽（`truncate`/`ansi.Truncate`）。
2. 编辑器随行数增长：`reserve = 边框(2) + chips 行(1) + 输入行数`，上限 `maxEditorLines = 6`；超出后保留**光标所在行**可见（内部滚动窗口，行首用 `…` 表示上/下截断，参考 Crush 的 "… N more" 记法）。
3. 光标 `█` 固定出现在**最后一行**行尾（输入始终追加在尾部，与现有 `m.input` 模型一致）。
4. `layout.go`：`editorReserve(hasAttachments bool)` 改为 `editorReserve(hasAttachments bool, inputLines int)`；`layout.computeLayout` 增加 `editorLines` 入参或在 `renderFrame` 处传入；`mainH()` 相应扣减。窄屏（compact）与宽屏同规则。
5. 空输入时高度恒为 1 行内容（防抖动：高度只增不减的滞回可不做，直接按行数算）。

**实现要点**：
- 新增 `func editorInputLines(input string, width, maxLines int) []string`：分行、截断、取尾部窗口。放 `render.go`，纯函数可测。
- `renderEditor` 消费该函数；`renderFrame`（`render.go:20`）先算 `len(strings.Split(m.input,"\n"))` 传入 layout。
- gate 审批态维持现规则（光标隐藏、单行提示不变）。

**测试**：`render_test` 新增——单行/多行/超上限/含 CJK 宽字符行（`lipgloss.Width` 校验）/含 ANSI 序列行；`layout_test` 校验 `mainH` 随输入行数递减、下限 1。

### F3 空输入占位符（已完成）

**现状**：无。空态时 composer 只有一行 `::: ` + 光标。

**目标显示细节**（Crush 风格）：`input == ""` 且无 gate 时，prompt 后渲染暗色占位符：`p.Dim`（`paletteSubtle`）色，文案 `问点什么…  / 命令 · @文件 · !shell`（简短、不换行、随宽度截断）。有草稿、gate 挂起、侧栏聚焦时一律不显示。占位符不算输入内容，Esc 清空逻辑不变。

**实现要点**：`renderEditor` 内 4 行 if；占位文案抽成常量 `composerPlaceholder`。

**测试**：空输入含占位符、有输入不含、gate 时不显示。

### F4 粘贴防护与大粘贴提示（已完成）

**现状**：粘贴直接进 `m.input`（`tea.KeyRunes`），无任何提示；超大粘贴会把 composer 撑爆且容易误发送。

**目标显示细节**（对齐 Crush 的大粘贴守卫）：
1. 单次输入事件插入文本 `> pasteThreshold`（默认 2000 字符或 40 行，取常量）时：内容**照常进草稿**，但 composer 顶部附件行位置渲染一枚警示 chip：`⚠ 大段粘贴 · N 行 / M 字符 · enter 发送前请确认`，样式复用 `p.PromptWarn`（warn 底、高对比字）。
2. chip 只在草稿包含超阈值粘贴段时出现；用户删到阈值以下自动消失（按当前 `m.input` 总长/行数计算即可，无需记录来源）。
3. 不实现折叠占位（那是模型输入管线的事），仅做警示 chip——保持"终端可见的都进草稿"不变。

**实现要点**：纯函数 `func pasteGuardChip(input string) string`（超阈值返回 chip 文案，否则空）；`renderEditor` 在 attachments 行后追加。阈值常量放 `layout.go` 同级。

**测试**：阈值边界（1999/2000 字符、40/41 行）、chip 与附件 chips 共存时的截断顺序。

### F11 busy 态编辑器降亮（已完成）

**现状**：边框颜色只随权限模式变（`composerBoxStyle`，`render.go:834`）。

**目标显示细节**：`meta.Busy == true` 时边框前景切换为 `p.Dim`（`paletteSubtle`），发送提示行已有 `run…`；busy 结束恢复模式色。Crush 中运行期编辑器明显降亮，提示"当前输入会排队"。不改变可输入性。

**实现要点**：`composerBoxStyle` 增加一个 busy 分支，1 行。

**测试**：busy/非 busy 两种边框色断言（比较 `Style.GetForeground()`）。

---

## 批次 B：忙碌态与状态行（对应 Crush spinner / status）

### F1 动画 spinner + 计时器（已完成）

**现状**：busy 只有静态 `run…`（`render.go:829`）。全 TUI 无 spinner、无耗时显示。

**目标显示细节**：
1. `meta.Busy` 时 chrome 行显示 `⟳ 反帧 + 已用时间`，如 `⠸ 12s`。帧集用 bubbletea 生态惯用的 braille 序列：`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`，120ms/帧。
2. 计时从本回合第一次 busy 观察到 false→true 翻转开始（driver 无开始时间戳，**用本地 monotonic 时间即可**，显示为本地观察值，不伪称服务端真值——与 surface"不推断服务端真值"的注释一致：文案不写"服务端耗时"）。
3. busy 结束定格最后一帧并在下一帧复位；`meta.Error` 优先级高于 spinner（现行为保留）。
4. gate 挂起且 submitting 时同样显示 spinner（现有 gate.Submitting 分支）。

**实现要点**：
- `model.go`：新增 `spinnerIndex int`、`busyStartedAt time.Time`（零值表未在计时）、`busyTimerDone chan struct` 不可用——用 `tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return spinnerTickMsg(t) })` 产生的 Cmd，在 busy 期间每次 Update 重新排下一跳，busy 结束停排。
- 新增 `type spinnerTickMsg time.Time`；`Update` 处理：busy 时 `spinnerIndex=(i+1)%len(spinnerFrames)` 并续排。
- 渲染抽纯函数 `func spinnerLabel(frames []string, index int, startedAt time.Time, now time.Time) string`，now 注入保证测试确定性。

**测试**：帧推进取模、busy→idle 复位、elapsed 文案（`<1s`、`12s`、`1m05s`）、err 优先。

### F12 状态行信息增强（左会话/右环境，含队列数）（已完成）

**现状**：chrome 行只有帮助键 + err/run（`renderInputChrome`，`render.go:812`）。

**目标显示细节**：一行两段、左对齐提示 + 右对齐元信息（宽度不足时右段先丢弃，再左段截断，与 chips 行现有降级次序一致）：
- 右段：`⏸ N queued`（`meta.Queued>0` 时，`p.Warn` 色）· `host`（`p.Dim`）· 会话标题（`p.Dim`，超长中截）。
- 左段：现有快捷键/err/spinner 内容。
- 中间用空格填充到整宽（`lipgloss.Width` 计量，CJK 安全）。

**实现要点**：`renderInputChrome` 内拆 `chromeHints`（现有）+ `chromeMeta`（新增）+ `joinChromeRow(left, right, width)` 纯函数。

**测试**：右段宽度降级次序（标题→host→queue）、超宽截断、queued=0 不显示。

### F6 滚动指示与回底提示（已完成）

**现状**：`chatFollow=false` 时无任何指示，用户不知道自己悬在历史里；也不清楚怎么回底。

**目标显示细节**（Crush 的 jump-to-bottom 记法）：
1. `!chatFollow` 时 chrome 行左端追加 `p.HelpKey` 渲染的 `↓ end 回到底部`（说明按键），或当 `maxScroll-chatScroll` 较大时显示 `↑ 历史 · 下方还有 N 行`。
2. 新增 `end` 键（以及 `G`，vim 习惯，仅在输入为空时）置 `chatFollow=true` 并 `chatScroll=maxScroll`。
3. 用户在 chat 视图按 ↑/↓/PgUp/PgDn 离开底部时自动 `chatFollow=false`（若现有逻辑未做，补齐——需核对 `handleChatViewKey`，`model.go:742-760`）。

**实现要点**：`renderChat` 已算出 `maxScroll`/`offset`，把 `(offset, maxScroll, follow)` 传给 chrome 或存到渲染期临时结构；避免全局状态，直接在 `renderFrame` 里把 chat 的 hint 行与 chrome 行一起组装（改造 `renderWide`/`renderCompact` 的拼接顺序）。

**测试**：follow/hover 两态文案、`end`/`G` 键回底、`maxScroll==0` 时不显示。

---

## 批次 C：聊天体细节（对应 Crush 消息渲染）

### F5 工具卡展开/收起

**现状**：工具结果固定 8 行（`compactToolResultLines`，`render.go:664`），全量输出只能改 `tui.debug` 配置重启。Crush 的工具块是可交互折叠的。

**目标显示细节**：
1. 截断行尾标记从 `… N more lines · set tui.debug: true` 改为 `… 还有 N 行 · ctrl+o 展开`（保留英文版 `… N more lines · ctrl+o expand`——文案与现有中英混用风格一致，取中文）。
2. 新增按键 `ctrl+o`：切换"展开最近一个已完成工具卡"（`status==done|failed|denied` 的最后一张）。再按一次收起。展开态该卡渲染全量行（仍按宽度 wrap + ANSI 截断，不引入横向滚动）。
3. 状态存储：`Model` 增加 `expandedTools map[string]bool`（key=`ToolCallID`，空则用 `fmt.Sprintf("%s#%d", ToolName, index)` 兜底）。只影响渲染，不影响 surface 协议。
4. `debugToolOutput=true` 时全展开（现有语义保留，标记文案改为 `· debug`）。
5. 展开状态下卡片标题行尾加 `▾`，收起 `▸`（pending 卡不加）。

**实现要点**：`renderToolWithOptions` 增加 `expanded bool` 入参（连带改 `renderMessageWithOptions` 与 mdCache 判定——工具卡本就不缓存，确认 `cacheableMarkdownMessage` 已排除 `Tool != nil`，`model.go`）；`Update` 处理 `ctrl+o` 定位最近完成卡。

**测试**：8 行截断标记、展开全量、ctrl+o 切换往返、多工具时定位最后完成卡、展开不影响 mdCache。

### F9 推理（reasoning）块折叠

**现状**：reasoning 消息按完整 markdown 渲染（斜体灰），长思考占满屏幕；流式时也全量。

**目标显示细节**（对齐 Crush 的 thinking 处理）：
1. **已完成**的 reasoning 块（`Reasoning && !Streaming`）默认折叠为单行：`┊ ✻ 思考完成 · N 行 · ctrl+r 展开`，`p.Reasoning` 样式。展开后完整渲染，再按收起。
2. **流式中**的 reasoning 保持全量（正在生成，折叠无意义）。
3. 状态 `expandedReasoning map[string]bool`，key 规则同 F5；`ctrl+r` 切换最近一个已完成 reasoning 块。
4. 用户消息/助手消息不受影响。

**实现要点**：`renderMessageWithOptions` 开头加 reasoning 折叠分支；折叠行的 `N 行` 用折叠前 `renderMessageBody` 结果行数（渲染一次取行数再折叠，或对 content 估算行数——直接渲染取行数，mdCache 已有）。

**测试**：折叠单行含行数、展开往返、流式不折叠、ctrl+r 定位。

### F13 空会话欢迎态（hero）

**现状**：空态 3 行（`chatLines`，`render.go:564-566`），偏简陋。Crush 有居中品牌区 + 提示。

**目标显示细节**：
1. 空会话时渲染居中 hero：大号 `VIVY CODE` 字标（复用 `p.LogoWord`，可加 `p.Diagonals` 装饰行）、下面一行 slogan `寻找真心之旅`、再一组暗色提示行（每行 `p.HelpKey` 键 + `p.HelpDesc` 说明）：`/ 命令面板`、`@ 文件引用`、`shift+tab 切模式`、`! shell`、`ctrl+s 会话`。
2. 垂直方向在可视区内大致居中（chat 高度的 1/3 起），不新增滚动行为。
3. 有消息后立刻消失（现有逻辑即 `len(messages)==0` 分支）。

**实现要点**：抽 `func renderEmptyState(width, height int, p Palette) []string`；用 lipgloss `Align(lipgloss.Center)` + `Height`。

**测试**：行数不超可视区、宽度收窄不炸（80/60/40 列快照断言关键字包含）、有消息时不渲染。

---

## 批次 D：侧栏与标题（小改动，收尾）

### F10 侧栏 Modified Files 配色 + 路径截断

**现状**：`render.go:444` 直接 `fmt.Sprintf(" %s  +%d -%d", ...)`，增删数无色；长路径只靠外层 truncate 硬截尾部（会把扩展名截掉）。

**目标显示细节**：
1. `+N` 用 `p.DiffAdd`、`-N` 用 `p.DiffDel`（与 diff 体同一套语义色）。
2. 路径过长时**保留头部与尾部**（`very/long/…/path/file.go`，中截省略号），文件名永远可见；复用/新建 `truncateMiddle(path, width)` 纯函数（注意 CJK/宽字符，用 `lipgloss.Width` 计量）。
3. 文件行 hover 高亮不做（无鼠标语义，保持键盘优先）。

**测试**：短路径原样、长路径中截含省略号且总宽正确、配色样式断言。

### F7 终端窗口标题

**现状**：不设置（`view.Run` 起 altscreen 后标题是 `vivy`/`--title TUI` 传入的进程参数语义）。

**目标显示细节**：会话切换/重命名后调用 `tea.SetWindowTitle(标题 + " · VIVY CODE")`；空标题回退 `VIVY CODE`。纯显示，无协议影响。

**实现要点**：`Update` 中处理 `surface.SessionsMsg`（rename/list）与 session 切换路径时发 `tea.SetWindowTitle`（返回 tea.Cmd）。注意 `SetWindowTitle` 是 `tea.Cmd`，不能在 View 里调。

**测试**：可测性有限，做标题拼装的纯函数 `sessionWindowTitle(title string) string` 单测（含空标题回退、超长截断）。

---

## 3. 派单模板（每个子代理 prompt 追加这段公共头）

> 工作目录：`C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy-tui-polish`（git worktree，分支 `feat/tui-detail-polish`）。禁止改动 `C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy` 根工作树。只做本单列出的特性，不顺手改别的。每个特性一个提交，提交信息 `feat(tui): <主题>`，只 stage 该特性文件。完成定义：`go build ./...` + `go test ./sdk/tui/...` 全绿 + 新增测试覆盖列出的路径 + 更新本提案中对应特性条目为"已完成（commit hash）"。视觉自查：把 `C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy\sdk\tui\view\zpreview_test.go` 复制到 worktree 同路径（未跟踪文件，不提交），按需仿写 preview 用例，`TUI_PREVIEW=1 go test ./sdk/tui/view -run TestDump -v` 人工看输出。

批内合并方式：批次内多特性由**一个子代理顺序做**（render.go/model.go 冲突面大，不并行）；批次之间可以并行开不同子代理的前提是严格只碰各自"主要文件"清单，否则串行。

## 4. 验证与交付

- 每批：`go build ./...`、`go test ./sdk/tui/...`；批 A/B 另跑 `TUI_PREVIEW=1` 手检。
- 收尾：`just ci`（在 worktree 内跑；kernel/UI 门禁不变，`sdk/tui` 在其覆盖范围内则以其结果为准，不在范围则记录原因到 verification.md——`just ci` 若不覆盖 `sdk/`，补 `go test ./sdk/...` 一行并注明）。
- 交付日志：`docs/logs/2026-09-06-tui-detail-polish/`（summary.md / verification.md / acceptance.md），收尾一次写全，特性条目逐个列 commit。
- `docs/TODO.md` §0.1：过程中发现但本分支不修的问题按规则登记。
- 真机冒烟：`just run` + `vivy tui --live` 连上后人工过一遍 12 个特性的可见行为，记录进 acceptance.md。

## 5. 明确不做

- 不改 `surface` 协议字段、不加 RPC（F8 按消息级 token/cost 需要协议新增，超出"纯显示细节"，不做，登记 TODO）。
- 不引入新依赖（spinner 用 `tea.Tick` 手写，不引 bubbles；若派单代理认为 bubbles 的 textarea/spinner 显著更优，须先停下回报，不得自行加依赖）。
- 不做鼠标支持、不做主题系统（palette 已是单点）、不动 packed face 之外的构建路径。
