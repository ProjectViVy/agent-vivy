# Acceptance（人如何确认）

- `go test ./internal/runtime/ -run TestCronSettle -count=1` 秒级通过且
  确定（可任意重复）：成功即删、失败保留禁用两条契约直接可读。
- `just ci` 满载连跑不再被 cron 契约的偶发超时击落——即便端到端金丝雀在
  极端负载下超时，`TestCronSettle*` 仍证明 delete-after-run 行为正确，
  失败归因从「契约可疑」收窄为「环境调度噪声」。
