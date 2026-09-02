# UI-CHAT-ACT R2 — verification

## Commands and results

| Command | Result |
|---|---|
| `gofmt -l internal/` | clean（每轮改动后复查） |
| `go build ./...` / `go build ./internal/...` | OK |
| `go vet ./internal/storage/... ./internal/runtime ./internal/rpc` | OK |
| `go test ./internal/storage/... ./internal/runtime ./internal/rpc -count=1`（初版内核） | sqlite 40.8s ok · postgres ok · runtime 117.4s ok · rpc 40.2s ok |
| `go test ./internal/runtime -run 'TestRewind\|TestFork' -count=1`（有效视图修正后） | ok |
| `go test ./internal/runtime -count=1`（有效视图修正后全包） | 首轮 `TestServiceGrepToolEndToEnd` FAIL（包内顺序型偶发）；单跑 `-count=5` ok；全包重跑 ok（129.4s）。与本片 diff 无关（grep 路径零改动），与板上 TEST-2 同类，§0.1 补记 |
| `go test ./internal/storage/sqlite ./internal/storage/postgres -count=1`（LatestViewTruncation 后，CN-21 含新断言） | sqlite 28.1s ok · postgres ok |
| `go test ./internal/rpc -run 'TestSession' -count=1` | ok |
| `just ui-e2e` | E2E-EXIT:0（20 passed / 1 skipped，chat-act 3.9s，见下） |
| `just ci` | CI-EXIT:0（见下） |

## 离线 e2e 的三轮迭代（回放即测试）

离线规格 `ui/e2e/chat-act.spec.ts`（无供应商 → 回合失败但用户消息入账）三轮
各抓出一个真实内核缺陷，逐轮修复后重跑：

1. **R1 折叠开放式缺陷**：rewind 后追加的回合被永久折叠（编辑流自己的重试
   消失）→ 尾锚闭区间 `[cutoff, tail]` 语义（migration020 原位加
   `tail_message_id`）。
2. **cutoff==tail fail-open**：回退最后一条时两锚同 id，Go `switch` 只命中
   一个分支 → 折叠静默失效 → 两个独立 if 守卫 + CN-21 用例。
3. **fork 复活 + latest-wins 复活**：fork 按 stored 列表复制，把已折叠原文
   带进子会话（`copied_count` 2 vs 1 暴露）→ 改有效视图复制；随后第二次
   rewind 在"最新标记胜出"下顶掉第一次的 rewind 标记，把已编辑掉的原文
   复活（reload 后 `remaining_count` 1 vs 0、`hello vivy` 重现暴露）→
   视图改为全部 rewind/edit 闭区间 `[cutoff, tail]` 的**并集**折叠
   （`ListViewTruncations` + `ApplySessionTruncations`），fork 锚行不进
   视图折叠。

修复后全绿一轮的证据见上表与下方 e2e/CI 行。

另有一轮失败与内核无关：规格在 `initialize()` 落定前点「新建会话」，
initialize 尾部的自动选中覆盖新建会话的选择，发送被静默吞掉（DOM 停在
发送前状态）。属产品侧的 boot 窗口竞态（<100ms，真人几乎不可触发），本轮
以规格加固绕开（先等首会话选中信号再建会话），内核不改动；产品侧修复另立
`UI-INIT-RACE` 行。

## E2E

最终一轮 `just ui-e2e`（后台单跑，无并发负载）：

```
Running 21 tests using 1 worker
  ✓ 3 e2e\chat-act.spec.ts:37:1 › edit reruns via rewind, rewind folds the view,
      fork copies history to a new session (3.9s)
  ✓ 8 e2e\files-panel.spec.ts:3:1 › files panel opens and shows the no-run empty state (1.3s)
  ✓ 17 e2e\runtime.spec.ts:5:1 › real control plane conversation, reload, review,
      settings and demos (2.9s)
  1 skipped
  20 passed (38.9s)
E2E-EXIT:0
```

规格覆盖链：新建会话 → 发送 → banner（回合失败但消息入账）→
`data-message-id` 等 `^msg_`（乐观 local- id 落定）→ 编辑改写+保存重跑
（旧文折叠、新文可见）→ RPC 断言 `session/messages` 视图 → `session/fork`
（copied_count=1，子会话无弃史复活）→ `session/rewind`（remaining_count=0）
→ reload 后折叠视图持久 → 历史侧栏「分叉分支」跳转新会话且原文不复活。

规格侧两处加固（非内核）：settle wait（`article.first().or(开始新的对话)`
`toBeVisible`）绕 UI-INIT-RACE；files-panel 规格自建新会话规避共享后端库
里其他会话的 currentRun 恢复。

## just ci

```
CI-EXIT:0
```

（fmt-check · ui-ci · vet · 全部 Go 包测试 · headless-compile · plugin-ci
全绿；日志 /tmp/ci-chatact-r2.log）

## 真实路径冒烟

离线 e2e 即本轮的真实路径冒烟：真实后端（`go run ./cmd/vivy`，8799）+
真实 Vite 构建 UI，编辑全 UI 流 + 回退折叠/分叉经内核 RPC 直驱 + 会话列表
导航，全部经浏览器断言；3015 开发分流不在本轮重复（同栈同代码路径）。
