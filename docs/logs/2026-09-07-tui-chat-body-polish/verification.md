# Verification record

All commands were run in worktree `agent-vivy-tui-chat-body` (branch `feat/tui-chat-body-polish`):

- `go test ./sdk/tui/... -race -count=1` — all green (command/face/live/stream/view all ok). Run once after builder delivery and once after the reviewer's P2 fixes.
- `just ci` — CI-EXIT:0. Kernel `go test ./...`, UI tests, headless compilation of `cmd/vivy`/`cmd/vivy-code`/`ui`, plugin-ci for plugins (dingtalk/discord/feishu/lsp/qq/telegram), and faces (headless/tui) all passed.

New tests (`sdk/tui/view/chat_body_test.go`) cover: default 8-line ctrl+o truncation → expansion → restoration, the `tui.debug` + ctrl+o combination, ctrl+r reasoning collapse → restoration (same-width mdCache invalidation), empty-session hero rendering and no rendering for non-empty sessions, and no chat-body state change from either key when a gate exists.

## Real-device review supplement (2026-09-07, Windows Terminal + vivy-code.exe)

After running through the flow with a real `vivy-code.exe` (private-instance Journal), each item was confirmed:

- **F13**: The empty-session hero rendered correctly (wordmark, “Journey to Find Your True Heart”, cwd, and command/key hints), with UIA text and a screenshot providing independent evidence.
- **F9**: ctrl+r collapsed each reasoning block to the one-line `┊ reasoning · 5 lines · ctrl+r expand`, then restored it; bidirectional verification used instrumentation logs and the terminal buffer.
- **F5**: In a real `list_dir` turn, the tool card truncated to 8 body lines plus `… 33 more lines · ctrl+o expand` (the instance log `tool-cap-hit, omitted=33` confirmed the `debug=false` path frame by frame); the ctrl+o expansion direction was pinned by the unit test and confirmed by human acceptance.
- Investigation during review concluded that the previously observed “truncation/collapse not taking effect” was caused by desktop focus contention, which led to key events landing in the wrong place and to a double toggle—not by a functional defect. The code, binary (the marker string was present in the package), and `config.yaml`/`data/settings.yaml` (neither contains `tui.debug`) were checked at three layers; the view layer has no bypass render path. Temporary diagnostic instrumentation was restored, and the root tree had no residual changes after delivery.
