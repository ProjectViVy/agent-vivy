# Two-tier tool system: Active full binding + Hidden settings assembly + dynamic SKILL mounting

Date: 2026-08-31. Branch `feat/two-tier-tools` (worktree `../agent-vivy-two-tier-tools`).

## Problem

User report: when asked "What tools do you have now?" in chat, the model said it had no tools and could not read the workspace.
The root cause was the request-level keyword tool selector introduced by `373927e` on 2026-08-10
(`internal/tools` `Selector.Select`): when the user message contained no English keyword for a tool, it returned
an **empty Selection**, which `withSelectedTools` wrote into the run context.
`toolselection_middleware` then cleared the agent's tools; because Eino saw zero tools, it never called
`WithTools`, and the preamble explicitly said "No tools are selected for this request." The model
truthfully answered "no tools were assigned." Chinese messages yielded no English keyword tokens from
`tokenSet`, so Chinese chats almost always had zero tools. This was not a regression from the recent removal
of the mock provider or the backend switch.

## Delivery (three focused commits + documentation)

1. **`fix: bind the full enabled toolset on every request`**
   - Retired the keyword selector: `Engine.SelectTools()` returns all config-resolved tools
     (in registration order), binding the full active surface on every request; `tools.enabled` is the sole admission control.
   - Changed the static Instruction and preamble wording to "all tools listed for the current run are available";
     the empty-set branch remains a valid "pure chat mode" (0 tools).
   - Added `list_dir` to the default `tools.enabled` list (aligned with config.example.yaml and the sandbox
     auto-approve list; `echo_info` remains excluded by default).
2. **`feat: real tools config in settings (active/hidden assembly)`**
   - Added a `tools_enabled` overlay to settings.yaml (`nil` = config defaults; non-`nil`, including an empty table,
     replaces the entire list, with an empty table meaning pure chat mode); validation trims, rejects empty names, and deduplicates.
   - The app folds in the overlay at startup; `resolveActiveTools` is reused by startup and every engine rebuild.
     Saving settings detects active-surface changes and calls `ScheduleEngineReload`, so changes need no process restart.
   - Added RPC `tools/list` (the complete built-in registry plus active flags and read-only/approval hints)
     and `tools/set-active` (whole-list replacement; unknown and duplicate names rejected).
   - Replaced demo data (localStorage) in the UI Settings → Tools configuration with real catalog cards,
     per-tool toggles, "Restore configuration defaults," and a dirty indicator.
   - Added `Specs()` (the full catalog in registration order) and `Except()` (the hidden complement of Resolve)
     to `tools.Registry`.
3. **`feat: skill-declared tools mount dynamically mid-run`**
   - Added a `tools:` list to SKILL.md frontmatter; `skillFrontMatter.Tools` is the
     canonical field, and `skill_manage`/`SetSkillEnabled` **preserve** the declaration when re-rendering.
   - `SkillSummary.Tools` is sent in the `skill_view` result; when SKILL.md (a non-supporting file)
     is viewed, declared tools are recorded in run-level `tools.MountedTools` (a new context carrier initialized
     by `service.drive` for every run).
   - Added `toolSurfaceMiddleware` (`BeforeModelRewriteState`, the officially recommended dynamic tool-surface hook in Eino v0.9.13):
     the executable universe = all registered active + hidden tools, while the model view = active ∪ mounted ∪ foreign tools
     (such as tools injected by Eino's skill middleware, which are never hidden). It runs first, followed by the Eino skill / compaction middleware.
   - The `toolAdapter` execution gate allows "selected ∪ mounted"; hidden tools are invisible and unavailable until mounted.
     Governance is unchanged: effectful tools still go through proposal/HITL approval (D-012).

## Mount scope (design decision)

Mounts live until the current run ends: on the next run, the model must call `skill_view` again
(the skill body is already in the transcript, so re-viewing is inexpensive). Session-level pinning, resume-time
mount restoration, and journaling mount events are tracked in TODO §0.1 (TT-1/2/3).

## Explicitly not done

- No generic dispatcher (Plan B); the user approved declaration → mount.
- No provider/backend changes; `tool_search` semantics remain unchanged (the catalog is still limited to the active set).
- No tool-group templates; `model.request.selected_tools` still records only the pre-run base surface
  (active), and mount increments are not written back to that event (TT-3).
- Settings pages outside the UI Tools tab were not changed.
