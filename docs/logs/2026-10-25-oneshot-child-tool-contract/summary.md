# Summary — one-shot child tool contract wording matches shipped behavior

The approved D11 contract still said an explicitly selected effectful one-shot
child runs in "a private child workspace", and the model-facing `agent` tool
description still implied a sub-agent sees nothing on disk. After the child
workspace redirect (`2026-10-25-child-shared-workspace`), neither statement is
true: every child run resolves to its root run's directory, so a delegated
sub-agent reads the files its parent produced. This delivery reconciled the
wording with the shipped behavior. No runtime mechanism changed.

## What changed

1. **D11 contract wording** (`docs/specs/2026-09-23-issue39-eino-orchestration-design.md`).
   The prose in "Child lifecycle and management controls" and the D11 table row
   now state that a child resolves to the same workspace as its root run rather
   than a private one, that the workspace is shared state which a child's
   admission rollback never releases, and that the selection itself is unchanged:
   the existing parent approval policy still gates an effectful one-shot call.

2. **Status line in the orchestration plan**
   (`docs/plans/issue39-native-orchestration.md`, task 7 disposition). "isolated
   workspace" became "the root run's workspace" for the same reason.

3. **Model-facing capability text** (`internal/tools/agent.go`). The `agent`
   tool description now tells the model the sub-agent works in this workspace and
   can read files named by path. Without that sentence a model has no reason to
   delegate file reading and instead pastes file contents into the task, which
   keeps the original complaint ("the sub-agent cannot access my repository")
   alive in practice even though the capability exists. This is prompt text, not
   a mechanism: tool selection, approval, and workspace resolution are untouched.

## Why the plan's two "read-only" targets were left alone

The plan expected `issue39-eino-orchestration-design.md:96` ("Node children are
one-shot and read-only") and
`docs/specs/2026-09-29-inofy-cutover-design.md:23` ("Initial task graphs retain
their current one-shot, read-only children") to contradict D11. They do not:
both describe **workflow node** children, which are a different population from
the host-selected one-shot request in D11.

- Workflow node admission narrows the parent ceiling through
  `workflowChildTools` (`internal/runtime/child_sessions.go:707`), which keeps
  only the read-only set and drops `reply_parent`, and
  `validateINOFYDefinition` (`internal/runtime/inofy_admission.go:112`) rejects
  any node `tool_names` entry outside that set. A workflow node therefore cannot
  select an effectful tool, and the read-only wording stays accurate.
- `docs/plans/issue39-eino-orchestration/ORCH-02.md:38` already states the same
  split ("Explicit one-shot requests may select only tools in the active parent
  authority ceiling; effectful calls remain under the existing Service approval
  path. Workflow and default child tool sets remain read-only.").

The one-shot path is reachable only through the host RPC (`child/start` with
`tool_names`, `internal/rpc/control.go`) and INOFY call configuration; the
model-facing `agent` tool never passes `ToolNames`
(`internal/app/agenttool.go:47`), so a model-delegated child stays read-only. No
UI surface claims otherwise: a search over `ui/src` found only unrelated
`readonly` props (form fields, file previews).

## Explicitly not done

- No change to `docs/architecture/ACP-REMOTE-CONTROL-PROPOSAL.md:52` ("a remote
  child/subagent receives a H7 child ledger and a private H8 workspace"). That
  is a DEFERRED D6 proposal, not shipped behavior; it will need the same update
  when the proposal moves.
- No change to `docs/logs/2026-09-26-issue39-orch02-provisional/acceptance.md:15`
  ("under normal approval and isolated workspace"). Iteration logs record what a
  delivery verified at that time; the normative contract is D11, which this
  delivery corrected.
- No change to research dossiers or archives that mention per-run workspace
  isolation as a seam. They are point-in-time records.
- No mechanism change: workspace resolution, approval, tool narrowing and
  admission all keep the behavior delivered in `2026-10-25-child-shared-workspace`.

## Observation (no contract impact)

A workflow run itself (`domain.RunKindWorkflow`) is not a child run, so it keeps
a private directory, while its node children resolve to the root run's directory.
The workflow run's own directory currently holds nothing; only admission
allocates it. This is consistent with the contract sentence ("every *child* run
resolves to its root run's directory") and needs no wording beyond it.
