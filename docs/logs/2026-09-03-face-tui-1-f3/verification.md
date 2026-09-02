# verification — FACE-TUI-1 F3

日期：2026-09-03。命令在仓库根执行（冒烟在 %TEMP% 独立目录，config/workspace/skills 均临时 + 仓库 fixtures/provider 只读拷贝；未触碰仓库 data/）。

## 单元 / 组件测试（faces/tui module）

| 命令 | 结果 |
| --- | --- |
| `cd faces/tui && go build ./... && go vet ./...` | 通过（拷贝集依赖本地化后零 `agent-vivy/internal/` import——grep 全 module 确认） |
| `go test -timeout 120s ./...` | `ok example.com/vivy/faces/tui 0.147s`（11 用例：kind、非 TTY 拒绝、boot 建/取、turn 流式 + `face:"tui"` 断言、approval 应答 + 他人 run 过滤 + cancel、question 应答、会话切换、prompt 新会话、continue 附着、shutdown 悬 run 取消、视图外壳渲染） |
| `go test -race -timeout 240s .` | `ok example.com/vivy/faces/tui 1.135s` |

过程中抓出并修复两枚真实缺陷：

- **applyBoot 自死锁**（拷贝新增逻辑）：持 `l.mu` 期间回调 `l.Send`（其内部再锁）→ `-timeout 90s` goroutine dump 定位；改为锁外回调。测试套件从挂死到 0.085s 全绿。
- fakeEnv 语义修正：`deliver` 需带 runID（否则「他人 run 过滤」测不出）；baseScript 补 `run/cancel`/`approval/respond`/`question/respond`。

## pack 五步（真实产物）

| 命令 | 结果 |
| --- | --- |
| `./vivy-sdk.exe verify faces/tui` | `ok ...\faces\tui`（seam-face 校验；首轮曾因缺 `apiVersion` 被正确拒绝——补上后过） |
| `./vivy-sdk.exe pack --face tui` | `gen_d6fccc14e3958687`；generation.json：`recipe.face: "tui"` + `face{name: tui, kind: tui, grants: [tty argv rpc.client], tree_hash: 6e77d4…}` |

## 真实 EXE 冒烟（air-gap 安全）

1. `%TEMP%\f3-smoke`（config.example.yaml 副本 + fixtures/provider 只读拷贝，路径全在临时目录）：
   `ANTHROPIC_API_KEY=dummy vivy.exe run "smoke probe" > out.txt 2> err.txt` →
   `vivy run: tui: this face needs an interactive terminal (stdout is not a tty); pipe prompts to the headless face instead`，EXIT:1，stdout 0 字节。
   **分支证明**：错误前缀 `tui:` 来自器官且发生在拨号前（若 face 分支未接线，输出会带 `app:`/`headless:` 前缀）——fail-loud 行为按合同钉死。
2. 交互式全流程（真 TTY 里过完一轮带审批的对话）无自动化终端可用，列为 acceptance.md 人类验收路径；驱动层等价行为已由单元测试覆盖（同一控制面 RPC 序列与事件解释）。

冒烟后 scratch 已删除。

## `just ci`

- 第一轮全量：`CI-EXIT:0`（fmt-check rg 名单含 faces、ui-ci、vet、test、headless-compile、plugin-ci 迭代 `faces/headless` + `faces/tui` 两 module 根均 ok；`grep -c "^--- FAIL"` = 0）。headless-compile 通过即证 committed body（gateway 世代）不引入 TUI 依赖（§14①）。
