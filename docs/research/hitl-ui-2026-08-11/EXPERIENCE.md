---
name: Vivy HITL Review Experience
description: Behavior and interaction contract for approvals, questions, and durable review.
---

# Vivy HITL Experience Specification

**Scope:** next-stage behavior and interaction contract  
**Form factor assumption:** desktop-first responsive web shell served by Vivy  
**Visual style:** intentionally unresolved; decide during the UI design sprint

## Experience thesis

Vivy should feel autonomous during safe work and precise at the boundary of
consequence. When it pauses, the user should not have to decode a tool call or
guess whether a click already caused an effect. Every interaction therefore
answers three questions in order:

1. What is Vivy asking me to authorize or provide?
2. What exact scope and risk does that cover?
3. What happened after I responded?

## Core objects

### Review item

A durable item in the Review Center. It is addressable independent of the
current session and can be rendered inline in a run.

### Proposal

An immutable server-generated description of an effectful action. It includes a
normalized action, target, precondition, preview, redacted arguments, and risk
facts. A client cannot alter the proposal by changing display text.

### Question

A request for bounded human input. It has a prompt, optional schema, source,
privacy classification, expiry, and answer/cancel lifecycle. It never grants
authority by itself.

## Primary flows

### Flow A — approval from the global queue

1. A badge announces “Needs review” without interrupting unrelated chat.
2. The user opens Review Center and sees pending items across sessions.
3. The user selects an item; Vivy loads the server-authoritative detail.
4. The user inspects impact, preview/diff, risk, target, and run context.
5. The user chooses Approve once or Deny.
6. The UI enters submitting and prevents duplicate clicks.
7. The server returns accepted, conflict, expired, or stale.
8. For accepted approval, the UI shows “Decision recorded — executing”.
9. The run event stream updates the item to succeeded or failed.
10. The queue removes or archives the resolved item while the run timeline keeps
    the full history.

### Flow B — inline approval

1. The run transcript explains that Vivy is waiting.
2. The inline card uses the exact same ReviewItem renderer as the queue.
3. “Open in Review Center” preserves the run link and queue location.
4. A decision made inline updates the global queue and run inspector together.

### Flow C — question

1. The user sees who is asking and why the answer is needed.
2. The form renders a text field in P0, with schema metadata reserved for P1.
3. The user may edit the answer before submitting.
4. Answer sends data; Cancel/Decline ends the interaction without approval.
5. The UI never uses approval language for this flow.

### Flow D — stale proposal

1. The user opens a proposal whose precondition no longer matches.
2. The detail view marks it Stale and explains what changed.
3. Approve is unavailable.
4. The user can open the run and ask Vivy to create a fresh proposal.
5. The old proposal remains auditable as stale; it is not overwritten.

### Flow E — resolved elsewhere

1. Tab A and Tab B display the same pending item.
2. Tab A wins the decision.
3. Tab B receives a conflict or replay event.
4. Tab B changes to “Resolved elsewhere” and offers the final outcome.

## Information hierarchy

The visual hierarchy should remain stable across tool families:

1. consequence and decision state;
2. exact target and scope;
3. preview/diff or structured action;
4. risk and trust warnings;
5. technical arguments and event details.

Raw JSON, checkpoint identifiers, and internal resume targets belong behind an
advanced disclosure and are never the primary review surface.

## State copy principles

- Use “Approve once” rather than “Run” or “Allow” for an individual proposal.
- Use “Deny” for authority decisions and “Cancel” for unanswered questions.
- Say “Decision recorded” before execution starts.
- Say “Stale — target changed” rather than “Something went wrong”.
- Say “Resolved elsewhere” for first-writer conflicts.
- Show expiry as a deadline and explain what timeout will do.

## Accessibility and resilience

- keyboard focus enters the detail heading, then the primary decision;
- buttons have unique accessible names that include the tool/action;
- status uses text and icon/shape, not color alone;
- new pending items use an ARIA live announcement without stealing focus;
- destructive/irreversible approval has a confirmation step when the preview
  cannot make the consequence unambiguous;
- detail remains readable when the connection is down;
- submission state is idempotent from the user's perspective;
- browser storage contains no review payload, proposal, prompt, or result.

## UX acceptance checklist

- Can a user identify the target in five seconds?
- Can a user distinguish authorization from answering a question?
- Can a user see what will change before approving a file/Skill mutation?
- Can a user tell whether the effect has happened yet?
- Can a user recover from refresh, restart, conflict, expiry, and stale target?
- Can a user review pending work without opening the originating session first?
- Can a keyboard-only user reach and understand every decision control?
