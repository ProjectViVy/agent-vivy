# Acceptance

1. Start the split backend with `just run`, then launch `vivy-code` against it in a terminal at least 100 columns wide.
2. Open or create a session. Confirm the right rail describes only the active session and workspace rather than listing every session.
3. Confirm available authoritative values appear: session title/update time, workspace path, provider/model/reasoning support, context usage, token/cost state, and modified files.
4. Make a workspace file mutation in the session and finish the turn. Confirm the modified-files section refreshes and shows the project-relative path with net `+N -N` counts.
5. When the rail is taller than its viewport, press `Ctrl+Right`, then use arrows or `j`/`k` and Home/End to scroll. Press `Ctrl+Left`, Left, Tab, or Escape to return.
6. Type the letter `l` and use the ordinary Right arrow in the editor. Confirm neither action steals focus for the sidebar.
7. Resize below the sidebar breakpoint and back. Confirm hidden sidebar focus/scroll state does not trap input.
