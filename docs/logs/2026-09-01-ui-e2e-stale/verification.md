# Verification

Commands run from worktree `agent-vivy-vc0` (branch `feat/vc1a-bash-tool`), 2026-09-01.

| Command | Result |
| --- | --- |
| `go test ./internal/app/settings/ -count=1` | ok（含新增 TestSaveConcurrentWritersKeepDocumentValid） |
| `go test ./internal/app/settings/ -run TestSaveConcurrent -count=3 -race` | ok |
| `just ci`（golangci-lint + gofmt + go test ./... + ui tsc/eslint/vitest/build） | exit 0，两次（UI 改动前后各一次） |
| `just ui-e2e`（全量，10 spec） | **9 passed, 1 skipped（无 provider key 的 runtime.spec 供应商门控），exit 0** |
| `npx playwright test e2e/model-refresh.spec.ts`（单 spec 隔离复跑） | passed（用于区分竞态与顺序依赖） |

## 过程证据（诊断链）

- 全量跑首轮失败 3 个：model-refresh（providersError "internal error"）、
  runtime（no-provider 分支文本未出现）、welcome-wizard（裸 i18n key + 步骤断言失效）。
- playwright error-context 快照显示：model-refresh 冻结页面带 `internal error`
  段落且注册表少 my-local-model；runtime 的发送打到 model-refresh 留下的死端口
  `http://127.0.0.1:30123`（vivy.log 中 `dial tcp ... refused`），错误面为通用
  「这次对话没有完成」告警卡；welcome-wizard 模型步骤渲染 `welcome.provider`
  裸 key 且预填被 model-refresh 污染。
- `internalError()`（internal/rpc/control.go:3429）对任何 settings.Load 错误都
  返回固定 "internal error"（不泄细节）；`settings.Save` 旧实现的固定
  `path+".tmp"` 与无锁 rename 是并发失败源——并发回归测试在修复前稳定复现
  `settings: commit: rename ... Access is denied`。
- 修复后全量 e2e 连续跑绿；model-refresh 末尾把全局运行配置恢复为
  openai / gpt-4o-mini（aria-pressed 翻转确认 save RPC 完成），后续 spec 不再
  被临时 upstream 污染。
