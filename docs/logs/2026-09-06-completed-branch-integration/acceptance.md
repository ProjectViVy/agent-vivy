# Acceptance

1. `git log --first-parent main` contains the four completed branch merge
   commits after the integration is fast-forwarded to `main`.
2. MCP tool discovery uses the Eino component for tool/schema projection, but
   MCP calls, resource/prompt lifecycle, policy/HITL, and untrusted boundaries
   remain Vivy-owned.
3. Dynamic tool search uses the pinned Eino middleware while Vivy's allowlist,
   skill mounts, policy, audit, and final invocation checks remain active.
4. Session todo items can be updated interactively through the existing
   `session/todo/update` control path and UI panel.
5. `wip/pre-submodule-root-20260829` remains unmerged and the root worktree's
   pre-existing changes are untouched.
