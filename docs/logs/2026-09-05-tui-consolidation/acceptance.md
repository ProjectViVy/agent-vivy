# Acceptance

## Human-visible checks

1. From the repository root, run `vivy-code.exe` or `go run ./cmd/vivy-code`. A fullscreen terminal chat opens using the normal Vivy kernel and current project.
2. Start the split development pair, then run `vivy tui --live --addr 127.0.0.1:8787`. It opens the same fullscreen shell through the remote transport.
3. Open Vivy Studio's main console. The Vivy Code panel presents one **Open Vivy Code** action with no demo/plain selector. Activating it opens `go run ./cmd/vivy-code` in a dedicated console.
4. Pack the first-party face with `vivy-sdk pack --face tui`; the resulting generation reports `recipe.face=tui` and opens the same terminal shell.
5. Try `vivy tui --plain` and `vivy tui --demo`. Both must fail as unknown arguments instead of entering a fallback product.

## Regression focus

- Streamed thinking and text render as accumulated content rather than character-separated prints.
- Slash commands, approvals/questions, model/thinking controls, attachments/context, governed shell execution, and session operations remain available from the one shared view/controller.
- Exiting any launcher cancels an active run through the shared shutdown path.
