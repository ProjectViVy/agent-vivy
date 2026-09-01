# Verification

| Command | Result |
|---|---|
| `go build ./...` + `go vet` (config/runtime/app/provider) | 绿（D-007 拦截草稿一轮：app 直 import eino → 归位为 `runtime.SummaryModel` 不透明 seam，`TestEinoImportsQuarantined` 规则未破） |
| `go test ./internal/config -run TestCompaction -count=1` | 绿：默认 summary_model 为空、`\n`/`\x00` 拒绝、TrimSpace 归一、YAML 解析 `TestCompactionSummaryModelParses` |
| `go test ./internal/runtime -run 'TestEngineSummaryModel\|TestEngineSummarization\|TestEngineReduction' -count=1` | 绿：`TestEngineSummaryModelPreferredWhenHealthy`（摘要模型恰好 1 次调用、主模型仅主循环 1 次且输入含 CHEAP-SUMMARY-cc）+ `TestEngineSummaryModelFailsOverToMain`（覆盖失败 → 主模型恰 2 次调用：failover 摘要输入以 system 指令开头、无重复 system 消息、含原始 feed；主循环输入含 failover 摘要） |
| `go test ./internal/runtime -run 'TestEngine\|TestCron\|TestService' -race -count=1` | 绿（57.6s） |
| `just ci`（全门禁，含 plugin-ci） | 绿 |
| `just ui-e2e` | 10 passed / 1 skipped（cron-tasks 预存需真实 provider）——engine-reload 路径（model-refresh / mcp / sandbox / compaction 规格）回归无恙 |

真实 provider 下的 summary_model 调用未冒烟（需 key）；failover 语义由
scripted 契约测试覆盖。
