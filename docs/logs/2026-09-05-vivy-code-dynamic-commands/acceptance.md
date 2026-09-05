# Acceptance

1. Install an enabled Skill whose `SKILL.md` frontmatter contains `user-invocable: true`, open VIVY CODE, then press `/` or Ctrl+P. The Skill appears as a slash command; an enabled Skill without that flag does not.
2. Configure an MCP server exposing prompts. Open the command palette and verify each prompt has a stable `mcp-...` command and shows required/optional `NAME=<value>` usage.
3. Invoke a Skill command with quoted text. Its expansion starts a normal governed turn; it is never executed as a local shell command.
4. Invoke an MCP prompt with all required `NAME=value` arguments. Missing, unknown, duplicated, or malformed arguments produce a local-visible error and do not start a turn.
5. Start a dynamic command expansion, then press Escape or switch sessions before it finishes. No text is sent into another session, and the original slash draft remains available for retry.
6. Change the installed Skill/MCP prompt catalog, reopen the palette, and observe the refreshed list. If refresh fails, the prior list remains visible with a refresh error status.
7. Repeat the checks with both `vivy-code.exe` and a generation packed with the first-party TUI face; behavior and help text should match.
