# Acceptance

Pending human real-path smoke (record the observed result before closing the
lane):

1. Build and start the independent terminal face with `just tui` (or build
   with `just vivy-code` and launch `vivy-code.exe`) in an isolated temporary
   user/project home; keep the terminal at least 120 columns wide so the right
   rail is visible.
2. Configure a reachable loopback MCP fixture with a known catalog, an
   unreachable server, and a server whose configured auth environment variable
   is unset. Trigger `/mcp` for the reachable server and refresh the sidebar.
   Observe `initialized` plus `N tools`, `error` plus an indented bounded
   `! ...` detail, and `auth missing` without endpoint/token leakage. A zero
   tool catalog must show `0 tools`; an unlisted catalog must show no count.
3. Enable one user-origin and one project-origin skill. Refresh the active
   session and observe rows ending in `user`/`project`; do not interpret those
   labels as session-mount provenance.
4. Verify narrow resizing keeps every MCP detail/skill row within the sidebar
   width and that terminal control, newline, ANSI, and bidi payloads do not
   appear in the rendered rail.

The final `just ci` result and any environment limitation belong in this file
when the parent agent performs the gate and smoke.
