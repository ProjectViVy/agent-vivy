# Verification

## Completed

- `go test ./sdk/tui/stream ./sdk/tui/surface ./internal/tui ./internal/runtime -count=1` — PASS.
- `cd faces/tui; go test ./... -count=1` — PASS.
- Built-in and packed wire tests cover completed-only output and exact completion of one of two concurrent same-name tool cards.
- Mapper materialization and tool-call boundary tests passed 20 consecutive normal runs and 20 consecutive race runs. They cover observer-emitted preamble, Eino materialization, streamed tool calls, non-stream tool calls, and an uncontaminated following model round.
- `go test -race ./sdk/tui/stream ./internal/tui -count=3 -timeout=240s` — PASS.
- `cd faces/tui; go test -race ./... -count=3 -timeout=240s` — PASS.

## Final gate and smoke

- Final code-stable `just ci` — PASS: formatting, UI typecheck, 201 UI tests, UI production build, Go vet/test, headless compilation, and every plugin/face module passed. None of the tracked cron wall-clock flakes reproduced.
- `go run ./sdk verify faces/tui` — PASS.
- The first pack attempt correctly failed because the fresh worktree did not yet contain `ui/dist`; after the required CI UI build, `go run ./sdk pack --face tui --out .workspace/tui-completed-tool-pack-20260904` — PASS. Generation `gen_efb43e97a40b17c9`, artifact SHA-256 `3337847d73776bfdc450792e8509c53f0d326fb34b50c0863f7a22c2048d0739`.
- Real 80×24 PTY smoke with isolated `VIVY_USER_HOME=.workspace/completed-tool-smoke-home` — PASS. `vivy-code` opened the independent VIVY CODE screen, `/help` displayed the real command overlay, and Escape/Ctrl+C exited cleanly with status 0.
- Three GPT-5.6-LUNA MAX specialists re-audited event mapping, fullscreen/REPL projection, and Crush tool-call identity. Their mapper, round-fence, empty-completion, observer-duplication, and mismatched-ID findings were fixed and re-reviewed. Final scope verdict: no remaining P1/P2; `TUI-STREAM-N4` and `TUI-STREAM-N6` remain explicit protocol-level follow-ups.

No provider credential, Studio state, production tenant Journal, `data/vivy.db`, `data/demo`, or `data/workspaces` was read or written.
