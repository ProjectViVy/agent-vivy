# Composer commands and work dock

> **For agentic workers:** Use `superpowers:executing-plans` to implement this plan in one write lane.

**Goal:** Show actual work beside the composer and enter Plan/Goal through slash commands with visible command tags.

**Architecture:** The composer owns only an unsent command draft. The existing store and Work RPCs remain authoritative for Plan, Goal, exact review submissions, activation, and permissions. Enabled skills come from `skills/list`; an explicit skill request is ordinary user text handled by the existing Eino middleware.

**Tech stack:** React, Zustand, existing UI kit, Vitest, Playwright, Go SDK.

**Spec:** The user's requested composer interaction, with durable contracts retained from `docs/architecture/PLAN-GOAL-PREDESIGN.md`.

## Constraints and reference

- Empty sessions have no Work header. Goal or submitted Plan cards appear above the composer; effective Plan status is a composer chip.
- Only `/plan` and `/goal` enter their respective modes. Goal resume and plan-to-goal handoff also use `/goal`.
- Keep edit/pause/clear/exit and exact-submission review actions; never bypass review or permission checks.
- Keep command selection reversible until submission; preserve failed drafts and isolate session changes.
- DeepSeek Harness reference: upstream commit `5badb15009ae1756c3afe0ae0cef1faafc290ccc`; `packages/client/ui-goal/src/client/index.ts` input-dock registration, `GoalBar.tsx` conditional display, `ui-plan/src/client/PlanModeControl.tsx` authoritative Plan chip, and `ui-conversation/src/client/skeleton/InputBar.tsx` command-claim hints. Reference patterns only; no dependency or copied implementation.
- Eino capability check: pinned v0.9.13 `adk/middlewares/skill.NewMiddleware` already supplies discovery and the `skill` loading tool, used by `internal/runtime/engine.go` and `HostedSkillBackend`. UI requests its enabled skill by name in user text; it does not copy skill bodies or build another runtime/prompt loader.

## Review focus

- Empty/loading work must not imply an active mode.
- Slash paths, unknown commands, Shift+Enter, and Chinese IME must retain normal editing behavior.
- Old async work/skill results must not submit to a different session or clear a newer draft.
- Review replacement and existing unfinished Goals must fail safely instead of overwriting authority.
- Failed command execution, stale/disabled skills, and unsupported Goal attachments must retain the complete draft.

## Tasks

1. Conditional work dock
   - [x] Add failing empty-state and command-only entry tests; verify the failure.
   - [x] Move `WorkControlBar` below the transcript, hide empty state, remove mode-entry buttons/forms, retain existing Goal editing and exact Plan review.
   - [x] Verify the focused tests, including preserved review/Goal invariants.
2. Composer slash commands
   - [x] Add failing browser-DOM tests for discovery, keyboard selection, tags, execution, stale results, and session isolation.
   - [x] Add composer command draft controls and remove the execution-mode dropdown. Reuse existing Work methods and enabled-skill discovery, with no new wire types.
   - [x] Verify focused tests and type checking.
3. Delivery
   - [x] Exercise the real split Vite/backend pair at `http://127.0.0.1:3015`, including mobile layout and real Work RPCs.
   - [x] Request read-only code review; address substantiated findings.
   - [x] Freeze source and run `just ci`; record results and human acceptance steps.
   - [x] Commit one focused deliverable using the configured human identity.

## Delivered behavior and implementation decisions

The fixed Work header and execution-mode dropdown are removed. Actual Goal/submitted Plan cards sit above the input, and the existing Todo strip remains conditional. Composer commands select removable tags before submission; active Plan status is projected from WorkState. Goal creation, resume, and exact reviewed-plan handoff share the existing Work mutations.

The read-only review's four findings are fixed: reconcile the stopped run before the local queue gate; scope rewind presets and late results to the owning session; claim command arguments only across horizontal whitespace; require clearing a terminal Goal before another creation. Command intent is unsent local state, never mode authority.

Real browser verification also exposed a missing composition binding: runtime Service had no WorkSink despite RPC having a WorkBus. The same existing WorkBus now connects both ends, restoring live work notifications. Work mutation responses use version and session-epoch guards so older responses cannot replace newer progress or a reopened session.

The reference project's plugin/command architecture is not imported. No new runtime, RPC schema, SDK face-store fields, or dependency is added. Release deployment is outside this UI delivery; the requested change is committed on the existing `work` branch after verification.

The existing source-bound SDK conformance bundle was refreshed for the two internal source changes, then its executable reproduction and the full CI gates passed against that same source tree.
