# VC-2 死循环检测 验证记录

日期：2026-09-01

## 命令与结果

| 命令 | 结果 |
| --- | --- |
| `go build ./...` | exit 0 |
| `go vet ./internal/runtime/` | clean |
| `go test ./internal/runtime/ ./internal/rpc/ ./internal/storage/...` | 全部 ok（runtime 103s、rpc 33s；exit 0） |
| `just ci`（完整门禁，含 fmt-check / UI 构建 / Playwright 冒烟） | exit 0 |

## 新增测试（`internal/runtime/loopdetect_test.go`）

- `TestServiceToolLoopDetected`：scripted 模型 8 次同参 echo 调用，第 6 次重复
  触发 → run.failed，cause_category = `loop_detected`，消息有界（不含引擎
  内部字样），恰好 1 个终态事件，Journal 留 5 条 tool.finished（触发批随失败
  丢弃，见 consume 错误分支语义）。
- `TestServiceToolLoopWithinLimit`：5 次重复 + 收尾消息 → run.completed（上限
  内不干扰合法重复）。
- `TestLoopWindowCountsAndEvicts`：窗口 10/上限 5 的计数与淘汰（交替签名不
  触发、第 12 条同签名触发）、不同工具名不同签名、工具错误结果参与签名。
- 既有防护回归：`TestServiceMaxToolTurnsBreached`、
  `TestServiceMaxToolTurnsWithinCap`、
  `TestServiceBudgetCircuitBreakerStopsToolTree` 全部原样通过（新防护在 6 次
  重复即停，早于默认轮次/预算上限，不改变其触发路径）。

## 契约同步

- `schemas/events/payloads/run.failed.json`：cause_category 枚举新增
  `loop_detected`（wire 契约）。
- `docs/AGENT-VIVY-ARCHITECTURE-V0.md` ADR-004：类别清单补齐
  （human_timeout 此前就缺，一并修正）。
- UI 零改动：`ui/src/lib/failure.ts` 对非 `provider_error` 类别直接展示服务端
  有界消息，`loop_detected` 自动生效。

## Smoke 政策

内核防护非浏览器可见路径的直接变更（UI 错误条复用既有 run.failed 渲染）；
真实模型触发路径需 provider key（TEST-1 后无本地 mock），人工触发步骤记录于
acceptance.md，自动化由 scripted-model 集成测试覆盖。
