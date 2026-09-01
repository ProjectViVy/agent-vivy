# Verification

| Command | Result |
|---|---|
| `go test ./internal/app/settings/ -race -count=1` | ok 2.481s（含 4 个新 Update 测试） |
| `go vet ./internal/rpc/ ./internal/app/settings/` | clean |
| `go test ./internal/rpc/ -count=1` | ok 17.655s |
| `just ci` | 绿（golangci-lint + go test ./... + ui tsc/eslint/vitest 195 tests + vite build） |
| `just ui-e2e` | 10 passed / 1 skipped（cron-tasks 预存 skip），21.6s |

## 新增测试（internal/app/settings/settings_test.go）

- `TestUpdateConcurrentUpsertsAllSurvive` — SET-RMW 回归：8 goroutine 各
  Update-upsert 一条唯一 (id, base_url) provider 条目，最终 Load 8/8 全存；
  同调度下 Load→modify→Save 会丢条目（last-writer-wins）。
- `TestUpdateFnErrorAbortsWrite` — fn 错误原样穿透且文档零变化
  （errors.Is 断言 + 前后文档 DeepEqual）。
- `TestUpdateRejectsInvalidCandidate` — 候选文档校验失败 → IsValidationError
  为真、errors.As 解出底层错误、文档保持先前状态。
- `TestUpdateReturnsPersistedDocument` — 缺文件从零文档起步，返回值 ==
  Load 结果（DeepEqual）。

## Notes

- 首版 `TestUpdateReturnsPersistedDocument` 撞上既有 YAML 往返行为：nil
  `Models` 切片 marshal 成 `models: []` 再 decode 为空切片，DeepEqual
  不等（Save 同样如此，非 Update 引入）。测试给条目赋非空 Models 后绕开。
