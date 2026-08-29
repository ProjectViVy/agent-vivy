# 验证记录（2026-08-30，上下文压缩真实生效）

## 命令与结果

| 环节 | 命令 | 结果 |
|---|---|---|
| 编译 | `go build ./...` | ✅ 0 error |
| Go 测试（首次整跑） | `go test ./...` | ✅ 全绿（含 `internal/runtime` 42.5s、`internal/rpc` 21s） |
| 新增运行时测试 | `go test ./internal/runtime/ -run 'TestCompactionPolicyTriggerTokens\|TestCountMessageTokens\|TestMapperMapsSummarizationUsageEvent\|TestEngineSummarizationCompaction\|TestEngineReductionRunsBeforeSummarization\|TestServiceContextStatusAndCompactSession\|TestScheduleEngineReload' -count=1` | ✅ 7/7 通过 |
| 设置 overlay 测试 | `go test ./internal/app/settings/ -run Compaction` | ✅ 通过 |
| RPC 测试 | `go test ./internal/rpc/ -run TestContextCompactionRPC` | ✅ 通过 |
| 配置测试 | `go test ./internal/config/` | ✅ 通过 |
| 前端类型 | `pnpm run typecheck`（ui/） | ✅ 无错误 |
| 前端测试 | `pnpm test`（ui/） | ✅ 21 files / 177 tests |
| 门禁 | `just ci` | ✅ 全绿：gofmt / `go vet ./...` / `go test ./...` / headless build / UI install+typecheck+test(21 files, 177 tests)+vite build |
| 真机启动冒烟 | `go build ./cmd/vivy` + 临时 mock config（`runtime.mock: true` + `compaction.enabled: true`）启动，`GET /rpc/bootstrap` | ✅ HTTP 200（组合根完整：migration 015 + 含 summarization/reduction 的引擎装配成功启动并监听） |

`just ci` 首轮失败于 `fmt-check`（新文件未 gofmt）；`gofmt -w` 修复后重跑。

### 说明

- 引擎接线测试使用 `ScriptedModel` + `recordingModel`：断言 summarization 触发时模型的两次调用输入（摘要生成输入 > 主循环输入、主循环输入含摘要内容）；断言 reduction 先于 summarization（summary 生成输入含 `Old tool result content cleared` 占位）。
- 会话级测试用 `fixedReplyModel`（回复不回声输入，因为 mock 会回声导致摘要变大），断言 `session_compactions` 落库、`context.compacted` 事件入 Journal、`ContextStatus.has_compaction_summary=true` 且 feed 从 40 条折叠为 6+1。
- `ScheduleEngineReload` 测试断言：空闲立即 swap、在途（active 非空）延迟、空闲后落地。
- reducer/summarizer 共享触发阈值时行为符合预期：reduction 只清工具结果占位（参数保留），tokens 可能仍在阈值上，随后 summarization 兜底——顺序测试钉死该管线。

## 未实走项（按规则记录）

- 本环境未启动 split pair / 浏览器，`http://127.0.0.1:3015` 的浏览器点击冒烟未执行；以单元/集成测试 + 真机 bootstrap 冒烟覆盖主要路径。验收步骤见 `acceptance.md`，可直接在 `just dev` 后按 1–5 走查。