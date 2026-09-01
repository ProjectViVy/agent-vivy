# 验证命令与结果

| 命令 | 结果 |
| --- | --- |
| `go test ./internal/app/settings/` | 通过（含新并发测试） |
| `go test ./internal/app/settings/ -race -count=3` | 通过；修复前可复现 `settings: commit: rename ... Access is denied` |
| `just ci` | 退出码 0（golangci-lint + gofmt + go test ./... + ui tsc/eslint/vitest/build，修复后跑过两轮） |
| `just ui-e2e` | `1 skipped / 9 passed`，退出码 0；model-refresh 的 internal error 不再出现 |

## 证据链

1. `ui/test-results/model-refresh/e2e-model-add-model/error-context.md`：
   providersError 文本为字面 `internal error`，且注册表行缺少刚新增的模型。
2. `internal/rpc/control.go` `internalError()`：始终返回字面 "internal error"，
   detail 有意丢弃 → 判断为服务端 Load/Save 失败而非业务校验。
3. `data/e2e-home/vivy.log`（e2e workdir）：settings 解析失败/dial 记录与
   该用例时间线吻合。
4. 修复前本地复现：并发 Save 测试首轮即报
   `settings: commit: rename ... Access is denied`（Windows），
   把 Load 的读窗口纳入 `fileMu` 后转绿。
