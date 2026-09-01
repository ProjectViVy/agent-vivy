# Verification

| Command | Result |
|---|---|
| `go test ./internal/runtime/ -race -count=1 -run 'TestCron'` | ok 13.467s（含 2 个新 settle 契约测试 + 既有端到端金丝雀） |
| `just ci` | 绿（golangci-lint + go test ./... + ui tsc/eslint/vitest + vite build） |

未跑 `just ui-e2e`：无 UI 或 RPC 改动（纯 internal/runtime 测试补强）。
