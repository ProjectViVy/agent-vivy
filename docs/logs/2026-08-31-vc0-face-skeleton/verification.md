# VC-0 验证记录

日期：2026-08-31。全部命令在 worktree `agent-vivy-vc0`（分支
`feat/vc0-face-skeleton`）执行。

## 内核

- `go build ./...` — 通过（新 worktree 需先造 gitignored `ui/dist/.keep`
  才能过 go:embed）。
- `go test ./...` — 通过：runtime（约 54s，含新增 face 三件套
  `face_test.go` / `face_service_test.go` / prompt face 用例）、rpc、app、
  domain 均 ok。
- `go vet ./...` — 无告警。

## UI

- `pnpm typecheck`（tsc --noEmit）— 通过。
- `pnpm test` — 22 个文件 177 个用例全过（含新增
  `src/components/masks/mask-catalog.test.ts`：programmer → code、其余面具
  不指定 face，共 2 例）。
- `pnpm build` — 通过。

## 真实路径 smoke（http://127.0.0.1:3015）

8787 已被另一进程占用，本 smoke 用 `VIVY_ADDR=127.0.0.1:8791` 起本 worktree
后端，`VIVY_BACKEND_ADDR=http://127.0.0.1:8791 pnpm dev` 起 Vite，全部请求
走 3015 同源代理（与浏览器同一契约）。

脚本：`.workspace/smoke-face.mjs`（gitignored 草稿，bootstrap → WebSocket
JSON-RPC）。结果 SMOKE PASS 4/4：

1. `preflight/run` 带 `face:"code"` → 响应 `face:"code"`。
2. 不带 `face` → 响应 `face:"web"`（服务端显式归一）。
3. `face:"shell"` → RPC -32602 `runtime: invalid face\nface must be web,
   tui, or code`。
4. `turn/start` 带 `face:"code"` → Journal `run.started` payload
   `face:"code"`、`mode:"normal"`；末端 `run.failed`（新 worktree 无
   provider key，预期，不影响 face 证据）。

## just ci

`just ci`（fmt-check → vet → test → headless-compile → ui-ci）— 通过，exit 0。
ui-ci 片段：vitest 22 文件 177 用例全过（含新增 mask-catalog face 用例）、
`vite build` 通过。
