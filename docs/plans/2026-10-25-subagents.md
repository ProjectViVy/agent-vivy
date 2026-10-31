# Vivy Subagents Implementation Plan

Goal: deliver Pi-level delegation using a recipe-selected Vivy wrapper around
Rig's pinned native subagent plugin. This owner-approved plan supersedes the
single-read-only-task scope in DESIGN and research section 17.6.

Architecture: one Rust/Rig execution path for native tasks, parallel batches and
sequential chains; independent conversations in the same host process. Child
working directories remain in the current project and share host project context.
Python/apixis, cross-project context reload, OS sandboxing, worker-process isolation,
automatic worktrees and daemon continuity are outside this delivery.

Tech stack: Rust 1.97.1; Rig `55cd481a2421fd5da468b5bd6afc6ac876883599`;
existing vivy-basics plugin, gateway, trust, session and terminal infrastructure.

## Task 1: Native delegation and roles

- Add `vivy_basics::subagents::VivySubagentsPlugin` and select it in the recipe.
- Reuse native `SubagentsPlugin`, `SpawnSubagent`, reports and persistence;
  keep native task/message/wait and add optional profile/cwd task arguments.
- Embed scout/planner/reviewer/worker roles. Load Markdown overrides in home
  agents then trusted project `.rig/agents`; invalid overrides disable that role.
- Reuse existing YAML parsing; support name/description/model/effort/tools,
  string or array tool lists, per-invocation refresh and frozen active roles.
- Constrain tool access to both parent and role; reuse gateway model resolution.
- Resolve cwd within the canonical host project and use agent-local WorkDir.
- Default depth one while preserving native explicit user overrides. Use native
  session lifetime, follow-up and interrupted-on-resume behavior.

## Task 2: Bounded workflows and terminal feedback

- Add task_parallel(tasks) and task_chain(steps), sharing the same task spec and
  SpawnSubagent path. Maximum eight tasks per parallel call, four active children
  per session, queued admissions; no graph engine or generic scheduler.
- Chain substitutes {previous} with the prior bounded answer and stops on failure.
- Claim workflow-owned native reports before parent inbox acceptance: no premature
  steering, duplicate answers, or second transcript store. Keep full child history
  in Rig and at most 50 KiB per inline answer with child references.
- Parallel execution retains successful outcomes on partial failure. Parent stop
  cancels running children and prevents pending admission; never retry effects
  automatically. Resume reports interrupted workflows stopped without replay.
- Reuse agent views and usage, add compact progress/failure renderers; preserve
  print/JSON and no-TUI builds. Missing usage/pricing remains unknown.

## Task 3: Verification and delivery

- TDD for roles/trust, access, cwd, single/follow-up/failure, parallel limits/queue/
  partial failure/cancellation, chain substitution/stop, large output and resume.
- Run workspace and plugin tests, formatting, generated species, no-TUI and
  no-component checks, existing gateway/shell/session/Python smoke tests.
- Exercise the real Windows terminal using a ConPTY adapter; port the existing
  tested adapter from the separate Plan branch without importing Plan behavior.
- Acceptance: single task, two parallel investigations, scout-worker-reviewer
  chain, real file editing, visible progress/usage and effective cancellation.
- Run a separately identified real-gateway smoke with existing authorized
  configuration, keeping credentials out of output and tracked artifacts.
- Fresh whole-branch review; update DESIGN, handoff, Pi coverage and matching
  decision records in both repositories. Commit, but do not merge or push.

## Review focus

- Native reports steer the parent unless the workflow claims its own reports.
- Project role discovery must use existing trust, not corpus auto-loading.
- A stopped parent must not admit queued children or relaunch work on resume.
- Role tools cannot widen parent access; shell/Python are not read-only sandboxes.
- Per-agent cwd must not mutate host cwd or claim cross-project context reload.
