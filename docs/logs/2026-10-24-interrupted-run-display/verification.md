# Interrupted run display — verification

## UI gates (macOS host)

```text
cd ui
GOTOOLCHAIN=go1.26.4 GOPROXY=https://goproxy.cn,direct pnpm typecheck
# pass: stage-ui-assembly regenerated the Assembly from recipes/default.vivy.yml, then tsc --noEmit

pnpm exec vitest run src/lib/run-rows.test.ts \
  src/components/chat/MessageBubble.test.tsx \
  src/components/chat/ReasoningRow.test.tsx \
  src/components/chat/ChatView.test.tsx
# Test Files  4 passed (4)
# Tests       51 passed (51)
```

The `typecheck` pre-step (`node scripts/stage-ui-assembly.mjs`) shells out to
`go run ./sdk`; the ambient Go toolchain on this host fails on the pinned
`grpc v1.64.1` (`http2.TrailerPrefix`), so the pinned `go1.26.4` toolchain is
required for that pre-step as well.

```text
node scripts/check-i18n-completeness.js
# exit 1 — only these three lines:
# Uncatalogued Han: ui/src/lib/failure.ts:56: 模型服务暂时不可用
# Uncatalogued Han: ui/src/lib/failure.ts:57: 模型服务拒绝
# Uncatalogued Han: ui/src/lib/failure.ts:58: 模型服务触发限流
```

This is a pre-existing baseline failure, not caused by this delivery:
`ui/src/lib/failure.ts` is unmodified and those marker strings came in with
`f0b9f1ff` (2026-10-24), whose author did not extend the script's
`exactHanData` protocol-marker exclusion. The checker produced no
`Missing in zh/en`, placeholder-mismatch, duplicate-key, or
`Unknown translation` errors, so `chat.interrupted` is symmetric and every
`t('chat.interrupted')` call resolves.

Host-wide `pnpm test` was not run: it is known red on this machine for
unrelated reasons (`UI-VITEST-NODE25`). Focused files were run instead.

## Browser smoke (`http://127.0.0.1:3015`, Vite HMR, no backend restart)

1. **Cancelled run, real reasoning tail — pass.**
   Session `sess_40d6e291f2e0cb3f`, run `run_d7da382bbc231d8c` (cancelled,
   tail `model.reasoning_delta` … → `model.call.finished` → `run.cancelled`).
   The five reasoning rows report:
   `ok, ok, ok, ok, interrupted`, and the last row renders
   `思考过程 已中断 …` with `data-state="interrupted"`. Earlier blocks committed
   by `model.completed` stay `ok`.
2. **Completed run shows no marker — pass.**
   Session `sess_fed3d415787a9794` (`run_d5abbb64ecc2bba4`, completed): six
   reasoning rows all `data-state="ok"`, zero `[data-interrupted]` articles,
   no `已中断` text.
3. **Live cancel with a text tail — not achieved on this host.**
   Two scratch runs were started to cancel mid-stream. The in-app browser
   surface has no visible viewport (`NATIVE_BROWSER_VIEWPORT_UNAVAILABLE`) and
   throttles timers in the hidden tab, so both runs
   (`run_74fbd69b1f2cf018`, `run_ab0546663865c162`) reached `run.completed`
   before the dispatched Cancel click could land. Both correctly render with no
   interrupted marker. The text-tail interrupted case is covered by unit tests
   only (`foldRunEvents on stopped runs`), not by a live cancel smoke.

## Not run

- Linux `just wiring-ci` + `just kernel-ci`: not available on this macOS host.
  This is a UI-only change (no kernel, Journal, storage, or RPC file touched),
  but the gates were not executed and remain the owner's promotion gate.
- Postgres parity: no migration in this delivery.

## Working-tree disclosure

The root tree also carries unrelated uncommitted work that this delivery
neither touches nor includes: the incremental message-projection lane
(`internal/runtime/message_projector.go`, `internal/runtime/service.go`,
`internal/runtime/message_projector_incremental_test.go`,
`docs/logs/2026-10-24-incremental-message-projection/`) and the previous
one-shot child transcript delivery (migration 048 and its log). This delivery
changed only the UI files listed in `summary.md` plus the two boards and this
log.
