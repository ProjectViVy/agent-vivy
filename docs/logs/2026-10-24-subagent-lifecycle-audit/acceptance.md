# Acceptance

Date: 2026-10-24

## What a human can observe

1. **Delegation returns text.** In a chat, ask Vivy to delegate a small task
   (for example "have a sub-agent read `go.mod` and tell me the module
   path"). The `agent` tool result shows the sub-agent's answer, and a
   second delegation in the same turn shows its own answer rather than the
   first one.
2. **A silent sub-agent still answers.** If the child finishes without
   writing any text, the tool result reads
   `the sub-agent finished without a written answer` instead of an empty or
   header-only result. The chat continues normally.
3. **Stopping the chat stops the sub-agent.** Press stop (or cancel the run)
   while a sub-agent is still working. The parent run ends as `cancelled`,
   the child run ends as `cancelled`, and the child's in-flight provider
   request is aborted — no orphaned child keeps running or keeps billing.
4. **A failing sub-agent does not kill the chat.** When the child hits its
   tool-call ceiling or fails, the tool result reads
   `sub-agent failed: ...` with the bounded reason, the tool operation is
   recorded `recoverable / child_failed`, and the conversation can continue.
   The provider-configuration banner ("无法连接 / 请检查供应商配置") does not
   appear for a child-origin failure.
5. **Huge sub-agent answers stay bounded.** A very long child answer is
   truncated at 32 KiB with the `[child result truncated]` marker, so it
   cannot flood the parent context.

## Evidence

- Five deterministic E2E tests in
  `internal/app/agent_lifecycle_e2e_test.go` cover items 1–5 over the real
  control plane and the real `agent` tool wiring.
- No UI, i18n, config, or schema change is required for any of the above;
  the existing UI renders these tool results as ordinary tool output.
