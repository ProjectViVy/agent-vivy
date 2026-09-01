# Verification

| 命令 | 结果 |
|---|---|
| `go build ./...` | 通过 |
| `go vet ./internal/storage/... ./internal/runtime/... ./internal/tools/... ./internal/app/...` | 通过（无输出） |
| `go test ./internal/storage/... ./internal/runtime/ -run 'FileVersion|Conformance' -count=1` | ok（postgres 0.087s / sqlite 10.907s / runtime 0.228s；postgres 侧 TestFileVersionChainSemantics 因未设 VIVY_POSTGRES_TEST_DSN 按既有门控 skip，CN-18 在 sqlite 全量跑过） |
| `just ci`（fmt-check + vet + go test + headless + plugin-ci + UI） | 通过（exit 0，2026-09-02 完整跑完；UI vite build ✓ built in 4.00s） |

| `go test ./internal/pluginhost/ -count=1` | ok 0.247s（13 个测试：既有 6 + 新 7，全部通过） |
| `go vet ./internal/pluginhost/... ./internal/app/...` | 通过 |
| `just ci`（fmt-check + vet + go test + headless + plugin-ci + UI，切片2后复跑） | 通过（exit 0；首次运行 fmt-check 因 versioning_test.go 结构体对齐失败，gofmt 修复后复跑通过） |

新增测试覆盖（切片2）：
- `internal/pluginhost/versioning_test.go`：OpenWrite 新文件/覆盖写产生正确链记录（display 路径 + session/run）；超限新内容直通落盘不记录、超限旧内容跳过链但刷新标记；无 session / nil recorder 静默直通；缓冲语义钉死（Close 前磁盘保留旧内容、Close 后落新内容且只记录一次）。
- `internal/storage/conformance/suite.go` CN-18：记录不报错 + tracker upsert 往返 + DeleteSession 级联（双后端共享）。
- `internal/storage/sqlite/fileversions_test.go` / `internal/storage/postgres/fileversions_test.go`：链语义（基线、链上 latest 匹配时只追加新内容、外部修改插中间态、相同内容去重、保留 20 版、>1MB 跳过不截断）。
- `internal/runtime/fileversion_backend_test.go`：写/读/patch 产生正确的 RecordMutation 序列；stale-read 拒绝 → 重读恢复 → 再写成功；未 tracked 路径放行（fail-open）；无 session 上下文完全不触 recorder。

## Smoke 边界说明

本切片无 UI/浏览器面变更；记录侧行为发生在 agent run 的工具执行路径内，需要真实 provider 会话才能端到端观察。内核测试 + `just ci` 为本切片的验证面；浏览器 3015 冒烟不适用（smoke-for-user-visible-change 规则所指的 UI 变更不存在）。
