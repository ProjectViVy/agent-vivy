# Verification — CMP-3

Commands run from the repository root (`agent-vivy/`):

1. `go build ./...` — clean.
2. `go vet ./internal/storage/... ./internal/rpc/ ./internal/app/` — clean.
3. `go test ./internal/storage/sqlite/ ./internal/storage/postgres/ -run 'Conformance|CN-20' -count=1`
   — 首跑 FAIL：CN-20 平手期望写反（`run-c0` 期望先于 `run-c2`，但 run_id DESC
   让 `run-c2` 先出）。修正夹具（平手记录改名 `run-c9`）后复跑 sqlite CN-20 ok。
4. `gofmt -l internal/` — 首轮报 `internal/rpc/control.go`（ControlDeps 结构体对齐），
   `gofmt -w` 后 clean。`internal/rpc` 测试首轮 build failed：control_test.go 缺
   `storage` import，补上后通过。
5. `go test ./internal/rpc/ -run 'TestControlHandlerListsSessionCompactions' -count=1`
   — ok（0.9s）。
6. `just ci` — 后台整跑，tail 检查日志 `CI-EXIT:0`（结果见本目录 verification 追加，
   最终提交前核对）。
7. `just ui-e2e` — 后台整跑，tail 检查 `E2E-EXIT:0` 与 compaction-setting 两条规格
   通过。

注：postgres 一致性套件在无 DSN 时按既有约定跳过实跑（CN-20 的 postgres 覆盖依赖
`just ci` 全量环境的既有 postgres job 行为；本切片 sqlite 全绿）。
