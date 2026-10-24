# Acceptance — one-shot child tool contract wording

Docs-only delivery plus one model-facing prompt sentence; nothing to click.

## What a human can check

1. **D11 reads the shipped rule.** In
   `docs/specs/2026-09-23-issue39-eino-orchestration-design.md`, the child
   lifecycle prose and the D11 table row both say a child resolves to its root
   run's workspace directory, that the directory is shared state a child's
   admission rollback never releases, and that the parent approval policy still
   gates an effectful one-shot call. No "private child workspace" remains in the
   normative spec:
   `grep -n "private child workspace" docs/specs` returns nothing.

2. **"Read-only" claims are about the right population.** The workflow-node
   wording in the same spec and in
   `docs/specs/2026-09-29-inofy-cutover-design.md` is unchanged, because
   workflow node tools are narrowed to the read-only set
   (`workflowChildTools`, `internal/runtime/child_sessions.go:707`) and node
   admission rejects anything else. A host `child/start` request that names a
   parent-authorized writable tool still needs an approval; a model-delegated
   sub-agent stays read-only.

3. **The model now knows it can delegate file reading.** `agent`'s tool
   description (`internal/tools/agent.go:74`) includes "works in this workspace
   so it can read files you name by path". With a live model, ask it to
   summarise `AGENTS.md`; it should delegate with a path rather than paste the
   file content into the task. (Not exercised by a live model in this delivery;
   see `verification.md`.)

4. **Behavior itself is unchanged.** No code path, approval, or workspace rule
   moved; `2026-10-25-child-shared-workspace/acceptance.md` remains the
   end-to-end check for a child reading its parent's file.
