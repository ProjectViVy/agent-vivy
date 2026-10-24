# Child tool-error recovery

Date: 2026-10-24 · lane `fix/child-tool-error-recovery` (worktree
`.worktrees/child-tool-error-recovery`, branched from `stable` @ `d48bfc5e`).

## Problem

Delegated child runs kept dying with the generic parent message
`sub-agent failed: The child task could not be completed. Please retry.`
The cause was not the sandbox, the policy snapshot or the model: it was a
tool error the model could have corrected that was typed as an
unclassified failure.

The kernel already has the recovery seam in `classifyToolFailure`
(`internal/runtime/tool_failure.go`): `*tools.ArgError`, `*tools.ChildTaskError`,
readonly `fs.ErrNotExist` and `*mcphost.ToolExecutionError` become ordinary
model-visible tool results (`recoverable` with a stable reason). Everything
else is wrapped as `toolInvocationError`, aborts Eino's ToolNode, terminates
the child run, and lands on the generic parent message
(`internal/runtime/service.go` `terminalEvent`).

Four model-reachable tool paths sat in that gap:

| Tool | Condition | Old error |
|---|---|---|
| `skill_view` | supporting path outside `references/templates/scripts/assets` | plain `skills: supporting path must be under …` |
| `skill_view` | missing supporting file | plain `skills: supporting file is unavailable` |
| `skill_view` | missing skill | plain `skills: skill %q not found` |
| `job_output` / `job_kill` | unknown job id | plain `unknown job %s` |
| `http_request` | non-GET/HEAD, relative/invalid URL, credential query/header | plain `errors.New` guards |
| `get_goal` | called from a child | plain `runtime: work tool run is not the session primary run` |

The last row is a leak, not an argument mistake: `get_goal` is
`Readonly: true`, so the readonly child filter advertised it to every child,
while its handler requires the session primary run — a contract a child run
can never satisfy.

## Typing rule applied

An error caused by the model's arguments that the model can correct on the
next call is `*tools.ArgError`. An absent resource in a readonly tool wraps
`fs.ErrNotExist`. Environment, policy, backend and storage faults keep their
fatal cause.

## Changes

1. `internal/runtime/skills_backend.go` — `skillDirIn`, `skillDirOf` and
   `locateSkill` wrap `fs.ErrNotExist` for a missing skill; `resolveSkillPath`
   returns `ArgError` on `path` for non-relative/absolute/volume/NUL paths,
   unsupported top-level directories, and symlink/directory targets, and wraps
   `fs.ErrNotExist` for a missing supporting file; `ViewSkill` validates the
   skill name up front (`ArgError` on `name`) and returns `ArgError` on `path`
   for binary or oversized supporting files. `validateUnderRoot` is unchanged
   as defense in depth. A malformed skill (`is not a directory`) stays plain:
   `locateSkill` uses it to continue across roots.
2. `internal/tools/jobs.go` — unknown `job_output`/`job_kill` ids return
   `&ArgError{Field: "job_id", Reason: "unknown job " + id}`. `job_kill` is
   not readonly, so the typed error also prevents a bad id from aborting a
   primary run.
3. `internal/runtime/http_request.go` — method, URL shape, credential-like
   query parameters and credential-bearing headers return `ArgError` on
   `method` / `url` / `headers`. Sandbox network denial and host-not-allowlisted
   stay untyped: those are policy refusals, not model-correctable arguments.
4. `internal/runtime/child_sessions.go` — `readOnlyChildTools` excludes
   `tools.GetGoalName` next to the existing orchestrator/headless exclusions,
   covering the one-shot default surface, continuable admission/activation and
   workflow children. The primary catalog still advertises `get_goal`.

## What stays fatal

Disabled skills, unreadable-by-permission files, malformed frontmatter,
backend-not-wired, journal/storage faults, sandbox/network policy denials and
host-allowlist denials. The generic `child.failed` parent message is
unchanged; after this cut it is a genuine fallback.

## Explicitly not done

- No sandbox or policy change. Children already inherit the parent's
  `EffectiveSandbox()` and policy snapshot and can only narrow it.
- No typed refusals for policy denials, hook blocks or the unselected-tool
  guard (`TOOL-REFUSAL-RESIDUAL` stays deferred).
- No workflow-child fault isolation (`WORKFLOW-CHILD-FAULT` stays open).
- No Eino `UnknownToolsHandler`, no Journal or migration change, no RPC or UI
  shape change.

## Eino capability check

None applies. The change types errors at the existing tool-invocation origin;
`classifyToolFailure`, the adapter boundary, the Eino version and the run
composition are unchanged. The Eino-native `UnknownToolsHandler` seam recorded
in `TOOL-REFUSAL-RESIDUAL` is deliberately not taken here.

## Files

- `internal/runtime/skills_backend.go`, `internal/runtime/skills_backend_test.go`
- `internal/tools/jobs.go`, `internal/tools/jobs_test.go`
- `internal/runtime/http_request.go`, `internal/runtime/http_request_test.go`
- `internal/runtime/child_sessions.go`, `internal/runtime/child_tools_test.go`
- `docs/logs/2026-10-24-child-tool-error-recovery/`
- `docs/DEFER.MD`, `docs/COMPLETE.MD`
