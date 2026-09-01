# Verification: remove chat preflight gate

Commands run from the repository root on 2026-09-01:

1. `go build ./...` — pass.
2. `go vet ./internal/...` — pass (no output).
3. `just ci` (= `fmt-check vet test headless-compile ui-ci`) — pass.
   - Go tests all green.
   - UI: vitest 21 files / 175 tests passed; `vite build` succeeded
     (chunk-size warning is pre-existing, unrelated).
4. `pnpm exec tsc -b --force` in `ui/` — pass (typed before `just ci`).
5. Real-path browser smoke against the split dev pair already running
   (backend `127.0.0.1:8787` held the organism lease; Vite at
   `http://127.0.0.1:3015` serves this checkout's `ui/` source):
   - Opened `http://127.0.0.1:3015/`, sent the message
     「预检移除冒烟测试」via the chat input.
   - Observed: the send went straight into a run (status `active`,
     「取消运行」 button, input disabled) — no amber preflight banner, no
     「预检发现警告」/「预检已阻止本次运行」, no 继续/取消 confirmation step.
   - The assistant reply streamed in and the run completed
     (`stillStreaming: false`, `cancelRunVisible: false`, `preflightBanner: false`).
   - Note: the live backend on :8787 is the pre-change binary (it still
     exposes `preflight/run`), which does not affect the smoke — the new UI
     simply never calls it.
