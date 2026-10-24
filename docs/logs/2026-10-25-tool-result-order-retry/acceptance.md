# Acceptance: tool-result-order retry

Human-visible checks. All of them need the split dev pair (`just run` + `cd ui;
pnpm dev`, http://127.0.0.1:3015) on a sandbox-world session.

1. A child run that hits the provider's `400 (2013)` rejection no longer dies.
   The transcript continues and the child produces its normal result. Before this
   change the same run ended with `模型服务拒绝了本次请求（HTTP 400）`.
2. The Run Inspector for that run shows one `auto_retry.started` /
   `auto_retry.finished` pair whose reason starts with `tool_result_order`, and
   `success: true` on the finished event.
3. Exactly one retry happens. A run rejected twice still fails, with a single
   `auto_retry.started` and `success: false` — no retry loop.
4. No `context.compacted` event appears on a `tool_result_order` recovery. The
   payload is replayed as sent; compaction belongs to overflow recovery only.
5. Overflow recovery still works as before: an oversized prompt produces one
   `context.compacted` with `mode: overflow-recovery` plus its own auto-retry
   pair.
6. When a run overflows first and the retried call is then rejected with 2013,
   only the overflow retry is published. There is no second `auto_retry.started`
   for a retry that cannot happen.
7. The backend log still carries the complete provider error text at ERROR
   level, so a real payload-ordering defect remains diagnosable rather than
   silently retried away.
