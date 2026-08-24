---
name: Vivy HITL Review Center
description: A review-centered interaction surface for durable agent approvals and human questions.
---

# Vivy HITL Review Center Design Target

**Design phase:** next stage  
**Design status:** information architecture and behavior frozen; visual
direction open for UI sprint  
**Primary surface:** desktop-first local web application

## Design goal

Create a review workspace that makes autonomous progress quiet and consequential
actions legible. The review surface must be trustworthy even when the model,
tool output, remote MCP server, or network is untrusted.

## Screen inventory

### 1. Review Center

Purpose: find all pending human work across sessions.

Required regions:

- page title and pending count;
- filters for kind, risk, session, tool family, and status;
- queue rows with action, target, source, risk, age, and expiry;
- empty, loading, stale, disconnected, and error states;
- keyboard navigation and visible selected-row state.

### 2. Review detail

Purpose: decide one proposal with enough evidence.

Required regions:

- decision header and state;
- target/scope/impact summary;
- preview renderer selected by tool family;
- risk and trust warnings;
- arguments/details disclosure;
- run/session context link;
- expiry/deadline;
- Approve once and Deny actions;
- conflict, stale, expired, and execution result states.

### 3. Run inspector Review tab

Purpose: keep the decision in context.

Required regions:

- current interaction summary;
- same review detail component in a narrower layout;
- link back to Review Center;
- event timeline showing required → decided → executing → finished;
- child-run context where applicable.

### 4. Inline conversation card

Purpose: make a paused run understandable without forcing navigation.

Required regions:

- concise explanation;
- risk/target summary;
- compact preview or “open full review” action;
- same decision controls and state copy as detail.

### 5. Question form

Purpose: collect non-sensitive human input.

Required regions:

- requester/source identity;
- prompt and optional schema description;
- input control;
- review-before-send affordance;
- Answer and Cancel/Decline;
- expiry and validation feedback.

## Component contract

The frontend should build one data-driven `ReviewItem` renderer rather than
separate queue and inline implementations. Tool-specific preview components
may be selected by `proposal.action`:

| Preview | Required information |
|---|---|
| file write/patch | path, before/after diff, line counts, precondition, protected-path warning |
| Skill mutation | skill name/path, provenance, revision, diff, rollback hint |
| command/process | exact argv, cwd, timeout, environment policy, output trust |
| HTTP | method, host/path, redacted request summary, side-effect/retry warning |
| MCP | server identity, tool, arguments, remote trust/effect warning |
| child run | child scope, policy snapshot, workspace, budget, nested effects |

Each component must support a bounded fallback renderer so an unknown future
tool is still reviewable without dumping raw internals.

## Visual direction to decide during the sprint

Do not freeze these from engineering assumptions:

- light/dark palette and semantic risk colors;
- typography and density;
- whether the Review Center is a full page, split pane, or inspector route;
- diff syntax highlighting versus high-contrast plain diff;
- motion/notification behavior for newly pending items;
- brand expression and illustration, if any.

The visual decision should be evaluated against the behavior requirements in
`EXPERIENCE.md`, not the other way around.

## Responsive rules

- desktop: three-region shell can show sessions, conversation, and inspector;
- tablet: queue/detail becomes a two-pane or stacked layout;
- narrow viewport: detail is a full-screen route with sticky decision actions;
- no critical decision may be hidden behind hover or a color-only indicator.

## Prototype exit criteria

Before implementation, a clickable prototype should demonstrate:

1. queue → detail → approve → executing → result;
2. queue → detail → deny → run feedback;
3. file diff with stale precondition;
4. question answer versus cancel;
5. refresh/reconnect and resolved-elsewhere states;
6. keyboard-only navigation through the review decision.
