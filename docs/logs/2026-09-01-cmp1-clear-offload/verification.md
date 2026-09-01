# Verification — CMP-1 clear offload

| Command | Result |
|---|---|
| `gofmt -l`（改动 5 文件） | 空（无未格式化文件） |
| `go vet ./internal/runtime/ ./internal/app/` | ok |
| `go test ./internal/runtime/ -run 'TestEngineReduction\|TestSafeOffloadCallID\|TestEngineSummarizationCompaction\|TestEngineSummaryModel' -count=1` | ok 0.478s（含新契约测试） |
| `go mod tidy` | go.mod：google/uuid indirect→direct；go.sum 相应收紧 |
| `go test ./internal/app/ -count=1 -race` | ok 14.388s |
| `go test ./internal/runtime/ -count=1 -race`（第 1 跑） | **FAIL 149.3s** — 1 例失败，tail 截断未捕获测试名 |
| `go test ./internal/runtime/ -count=1 -race`（第 2 跑） | ok 102.8s |
| `go test ./internal/runtime/ -count=1 -race`（第 3 跑，后台全量捕获） | ok（exit 0） |
| `go test ./internal/runtime/ -run 'TestEngineReduction\|TestSafeOffloadCallID' -count=4 -race` | ok 2.164s（新用例压 4 遍稳定） |
| `just ci` | 全绿：fmt-check + go vet + go test ./... + headless-compile + plugin-ci（6 module）+ ui tsc/eslint/vitest 195 + vite build |

## Notes

- 第 1 跑 `-race` 失败：失败测试名未捕获（tail -5 截断）。新用例隔离
  `-count=4 -race` 全绿、后续两跑全量 `-race` 全绿、`just ci` 全绿——判定
  落在既有 flake 面（TEST-2 已跟踪同类），与本切片改动无复现关联。
- ui-e2e 未跑：本切片零 UI/RPC/WS 面（纯 runtime/app 接线 + 测试），
  无 `just ui-e2e` 触发条件。
