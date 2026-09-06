# Completed-branch integration

## Scope

Integrated the four clean, completed branches that were not ancestors of
`main`:

- `feat/eino-boundary-5-5-5-6`
- `feat/eino-mcp`
- `refactor/eino-toolsearch`
- `feat/ui-todo-mutate`

The integration retains the Eino-native MCP and tool-search adapters while
preserving Vivy's Journal, policy, HITL, untrusted-content, and settings
boundaries. It also retains the completed stream-observer finding and the
renamed Vivy-facing backend identifiers.

## Integration resolutions

- Combined concurrent EINO-BOUNDARY-AUDIT updates so §5.1, §5.2, §5.5, and
  §5.6 accurately record their completed slices; Sequential Thinking and
  plantask remain open.
- Kept the strict `EvalSymlinks` test error check when merging session-todo UI
  work.
- Regenerated the module graph after adding the Eino MCP component and mcp-go.

## Explicitly not done

- Did not merge `wip/pre-submodule-root-20260829`.
- Did not delete branch refs or worktrees, push, or modify pre-existing root
  worktree changes.
