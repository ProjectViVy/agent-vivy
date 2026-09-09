# Acceptance: how a human verifies it works

## Product view

1. **The model can see the agent tool**: start vivy with the default
   configuration, ask the model about available tools in any session (or open
   the Settings tool surface), and verify that `agent` appears in the tool list
   with the description "Delegate a subtask to a read-only subagent with a clean
   context."
2. **One real delegation**: configure a provider key, then tell vivy
   "Use the agent tool to investigate <a read-only question>, then tell me the
   conclusion." Observe:
   - the session shows an `agent` tool-call record, after which the model uses its
     result in the answer;
   - Runs/sub-run view (or Journal) shows a child run with Kind=child and
     ParentID equal to the current run, ending in completed;
   - the Token statistics panel's session/model usage **includes** subagent
     consumption (cost rolls up to the parent session and is directly visible in
     the D9 panel).
3. **The mask takes effect**: delegate with a `mask` parameter (such as
   "terse reviewer"), and verify that the subagent's behavior changes with the
   prompt (the first system message of the child run in Journal contains
   `Persona hint (mask): …`).
4. **Approval joins the parent session**: if the subagent calls an ask-level
   effectful tool (not possible in the read-only surface; this validates the
   defensive semantics), the approval request appears in the parent session's
   Review Center with Kind=child.
5. **No nesting**: ask the subagent to "spawn another agent"; its tool surface
   contains no `agent` tool, so it can only answer directly.
6. **No MCP**: the subagent tool surface contains no `mcp_*` tools.

## Minimum verification without a key (actually run for this slice)

- `just ci` is fully green (see verification.md).
- Worker protocol-layer system-message threading tests prove that the subagent
  harness receives and uses the persona hint.
- App-layer guard tests prove that delegation fails clearly rather than hanging
  when the tool is unassembled, no run exists, or concurrency is exhausted.

## Rollback

Single commit (on the `feat/vc1a-bash-tool` branch); revert it to remove the
whole slice. No schema migration or data-format change.
