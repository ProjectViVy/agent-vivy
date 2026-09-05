# TUI consolidation

## Shipped

- Replaced the separate built-in and packed-face controllers with one canonical implementation in `sdk/tui/live`.
- Replaced the separate face launchers with `sdk/tui/face`; `vivy-code`, local `vivy tui`, and packed `face=tui` now enter the same controller and view.
- Reduced `internal/tui` to the remote WebSocket transport instead of keeping a second session, event, command, and rendering state machine.
- Made the shared `surface.Driver` contract explicit and removed runtime type-assertion fallbacks and the no-op driver.
- Retired the offline `vivy tui --demo` and line-oriented `vivy tui --plain` products. Both are rejected as unknown arguments.
- Reduced the Studio console to one canonical **Open Vivy Code** action using `go run ./cmd/vivy-code`; removed the demo/plain mode selector and sibling-worktree fallback.
- Deleted duplicated demo, REPL, view-wrapper, surface-wrapper, session, event, and controller files. The host diff is approximately 1,807 insertions and 10,492 deletions before this delivery log (net -8,685 lines).

## Deliberately unchanged

- TUI visual behavior, slash-command semantics, approvals, questions, thinking, model selection, attachments, project context, shell governance, and session management remain supported by the canonical shared implementation.
- Eino kernel orchestration and provider/tool execution were not reimplemented or changed; this work only consolidated the terminal face around the existing kernel/control-plane contracts.
- Studio lifecycle behavior outside the Vivy Code launcher was not changed.

## Delivery shape

Work was isolated on branch `refactor/tui-single-core` in a separate worktree because the root checkout already contained a user-owned Studio submodule change. The Studio shell change is committed in its own submodule commit and the host records that gitlink.
