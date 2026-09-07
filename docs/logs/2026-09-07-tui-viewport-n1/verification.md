# 验证记录

日期：2026-09-07

## 命令与结果

- `go test ./sdk/tui/view` — ok（含新增 4 个测试）
- `go test ./sdk/tui/...` — 全部 ok（command / face / live / stream / view）
- `gofmt -l sdk/tui` — 无输出（格式干净）
- `go vet ./sdk/tui/...` — 无告警
- `just ci` — 后台运行，退出码 0（见任务通知；Auto Mode 下不回读任务输出文件）

## 新增测试（sdk/tui/view/viewport_test.go）

- `TestPausedViewportAnchorsToMessageWhileHistoryAboveGrows` — 工具结果在锚点
  上方展开 2 行后，数字偏移随内容移动 2 行；同内容二次 Refresh 不再移动
  （指纹门闩）。
- `TestChatStampFingerprintsEveryRenderedField` — 内容/ID/角色/streaming/
  reasoning/tool 全字段/附件/文件上下文/消息数任一变化都改变指纹。
- `TestChatAssemblyRebuildsOnContentChanges` — 未变历史行数稳定；流式增长
  与中途工具结果填充均进入重建；reasoning 折叠切换缩小行数。
- `TestLoadingHistoryNeverPosesAsEmptyConversation` — `Meta.Loading` 期间
  渲染“正在加载会话历史…”且不渲染空对话 hero；Loading 清除后 hero 回归。

## 过程备注

- 早期 fixture 用 `strings.Repeat("long answer\n", 30)`，markdown 软换行把
  重复内容 flow-join 成 ~4 行，历史根本没溢出视口 —— 改为段间空行
  （`"long answer\n\n"`）让每段一行，锚点测试才有中间落点。
- 段落渲染为“内容行 + 空行分隔”，断言从“精确 +1 行”放宽为“增长且可见”，
  避免耦合 glamour 段落间距。
- `chatSegments` 返回共享 assembly 指针（原地重建），测试不得跨变更比较两次
  返回值的字段 —— 先把 lineCount 拷成 int 再断言。
