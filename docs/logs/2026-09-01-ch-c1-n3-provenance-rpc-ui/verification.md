# Verification — CH-C1-N3 provenance

| Command | Result |
|---|---|
| `go test ./internal/rpc/ -run TestControlMessageProvenanceProjected -count=1` | ok 0.688s（channel 轮两 RPC 全投影 + ui 轮零 provenance + payload 含键断言） |
| `just ci` | 全绿：fmt-check + vet + go test ./... + headless-compile + plugin-ci（6 module）+ ui tsc/eslint/vitest 195 + vite build |
| `just ui-e2e` | 10 passed / 1 skipped（cron-tasks 预存 skip，需真实 provider） |

## Notes

- 徽章的真浏览器验证需一个真实 channel 轮（telegram 等），e2e 栈无
  channel 注入面；以 RPC 契约测试（投影形状）+ ui-e2e 回归（10/1）+
  acceptance.md 人工步骤覆盖。UI 侧变化对 ui 轮为零渲染（字段整体省略）。
