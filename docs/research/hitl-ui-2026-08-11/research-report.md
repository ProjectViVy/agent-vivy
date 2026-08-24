# Vivy HITL and Review-Centered UI Research

**Date:** 2026-08-11  
**Author:** mastwet  
**Research type:** Technical, product, and UX decision research  
**Status:** Proposed next-stage baseline

## Executive decision

The next stage should be named **HITL Review Center and UI Foundations**. It is a
contract-and-interface stage, not another isolated tool-porting wave.

The target is a durable, reviewable interaction system with two distinct
surfaces:

1. a global **Review Center** for every pending approval or question across
   sessions and runs; and
2. an inline **run review** surface that keeps the decision in the context of
   the conversation and event timeline.

The backend already has the core pause/resume mechanism. The next stage must
make that mechanism a product contract: a reviewer must be able to understand
what Vivy wants to do, why it is risky, what exact target will be affected,
what changed since the proposal was created, and what happened after the
decision. A bare approve/deny card is not sufficient for filesystem, Skills,
command, HTTP, and MCP mutations.

The recommended V1 decision set is deliberately small:

- `approve once` — execute the exact server-side proposal;
- `deny` — do not execute and optionally return bounded feedback to the run;
- `answer` — provide user input for a question/elicitation, never as a proxy
  for approval;
- `cancel` — withdraw a pending interaction without authorizing the action.

Generic argument editing, remember-this-decision policies, multi-reviewer
approval, bulk approval, and remote notification channels should remain later
extensions. The UI should reserve space for them without making them part of
the next stage's critical path.

## Research questions and method

This research answers five questions:

1. What does a trustworthy agent HITL lifecycle need beyond a modal?
2. Which decisions should be common across all tools, and which are
   tool-specific?
3. What does the current Vivy implementation already guarantee, and where are
   the product gaps?
4. What UI information architecture best supports a desktop-first local agent?
5. What is the smallest next stage that makes HITL usable and testable?

Evidence came from a static audit of the current Vivy source and event/storage
contracts, the existing Hermes portability research, and current first-party
documentation from LangChain/LangGraph, OpenAI Agents SDK, Microsoft Agent
Framework, MCP, Anthropic Claude Code, Cursor, NIST, and OWASP. Claims about
Vivy below are code observations; claims about external systems link to the
primary source.

## 1. Current Vivy baseline

### What is already real

The current runtime has a stronger foundation than the UI suggests:

| Capability | Evidence in the repository | Assessment |
|---|---|---|
| Durable interrupt/resume | `internal/runtime/service.go`, checkpoint bridge, `Engine.Resume` | Implemented |
| Approval and question are separate | `internal/domain/tool.go`, `internal/domain/question.go`, separate stores and events | Correct foundation |
| Proposal preview | `domain.ToolProposal` and `payloadToolApprovalRequired` include action, target, precondition hash, preview, and risk findings | Implemented but not fully exposed |
| Server-side first-writer-wins | `ApprovalStore.DecideApproval` and `QuestionStore.AnswerQuestion` | Implemented |
| Expiration check | `DecideApproval`/`AnswerQuestion` reject expired items; startup recovery handles expired items | Incomplete for live runs |
| Restart recovery | `Service.Recover`, `rebuildPending`, `rebuildPendingQuestion` | Implemented for supported top-level runs |
| Tool breadth | files, Skills, tasks, network search, HTTP, MCP, sequential thinking, command/process tools | Implemented in the current worktree |
| Workspace and precondition safety | filesystem/Skills backends validate containment, hashes, atomic writes, and bounded payloads | Implemented |
| Event replay | journal-backed run log and JSON-RPC subscription with `after_seq` | Implemented |
| Basic UI action cards | `ui/src/features/runs/view.ts` and `ui/src/app/controller.ts` | Implemented, current-run only |

### Current product gaps

The following gaps are confirmed by the source audit and should drive the next
stage:

1. **The list API is not a review DTO.** `approval/list` returns only id, run,
   tool call, decision, and expiry. The action, target, preview, risk findings,
   precondition hash, and proposal data are only present in the run event and
   are reconstructed by the UI for the currently open run.
2. **The UI has one pending approval slot and one pending question slot.**
   `AppState.pendingApproval` and `pendingQuestion` are selected by matching the
   current run. Pending work from another session cannot be reviewed directly.
3. **There is no durable decision event.** `tool.approval_required` is journaled,
   but the approval decision itself is stored in the row and then the run is
   resumed. A reviewer cannot reconstruct a complete decision audit from the
   event stream alone.
4. **Approval rows lack reviewer/audit metadata.** There is no created time,
   decided time, actor/source, decision reason, or explicit proposal version in
   the public approval result.
5. **Expiration is not a live lifecycle transition.** A pending item can remain
   visible and the run can remain active until a decision is attempted or the
   process restarts. The next stage needs a deterministic expiration path.
6. **The run status does not explain waiting.** The run remains active while the
   UI separately displays a card. A run projection needs `waiting_for` (approval
   or question) without weakening the existing terminal state machine.
7. **The current approval card is argument-centric.** It shows JSON arguments,
   but not a first-class diff, target scope, risk level, precondition status, or
   untrusted-source warning.
8. **Question handling is text-only.** It is sufficient for `ask_user`, but not
   yet a general structured elicitation surface with schema validation, decline,
   and server identity.
9. **The current control plane is single-user local.** That is appropriate for
   this stage, but the protocol should still record a local actor/source so a
   later multi-user or remote reviewer model does not require rewriting the
   audit model.

These gaps do not require replacing Eino or the existing checkpoint bridge.
They require a product projection over the existing durable run and journal
primitives.

## 2. Findings from external patterns

### 2.1 Durable interruption is the primitive, not the UI

LangGraph describes an interrupt as a pause that saves graph state, surfaces a
JSON-serializable payload, waits for external input, and resumes using the same
persistent thread/checkpoint identity. Its documentation also warns that work
before an interrupt can execute again on resume and therefore side effects must
be idempotent or isolated. See [LangGraph interrupts](https://docs.langchain.com/oss/python/langgraph/interrupts).

OpenAI Agents SDK uses the same product shape under different names: an
interruption contains tool name and arguments, the run state is serialized,
the reviewer resolves one or more interruptions, and the original run is
resumed. It explicitly covers local shell, apply-patch, MCP, nested agents,
streaming, and long-running approvals. See [OpenAI Agents HITL](https://openai.github.io/openai-agents-python/human_in_the_loop/)
and [RunState](https://openai.github.io/openai-agents-python/ref/run_state/).

**Implication for Vivy:** the durable object that the UI acts on must be a
server-owned interaction/proposal identity, not a client-side boolean and not
an Eino checkpoint blob. The journal remains the history; the checkpoint only
supports continuation.

### 2.2 Approval, edit, reject, and answer are different decisions

LangChain's current HITL middleware distinguishes `approve`, `edit`, `reject`,
and `respond`; it explicitly says `respond` is acting as the tool/user-input
source and must not be used to deny a side-effecting tool. It also allows the
available decisions to vary by tool policy. See [LangChain HITL middleware](https://docs.langchain.com/oss/python/langchain/human-in-the-loop).

This is the cleanest conceptual split for Vivy:

| Interaction | Human role | Safe V1 action |
|---|---|---|
| Approval | Authorize a proposed side effect | Approve or deny |
| Question | Supply missing information | Answer or cancel |
| Review/edit | Correct a proposal before execution | Defer; specialized later |
| Policy setting | Define future automation | Defer to a separate policy UI |

**Implication for Vivy:** do not let a free-form answer path become an implicit
approval bypass. A question result is data; an approval result is authority.

### 2.3 Review can live inline and in a queue

LangChain's frontend guidance says the same interrupt can be rendered inline in
the transcript, in a review queue, in an admin dashboard, or in a modal. It
also emphasizes that refresh and cross-component review must resume the exact
paused run. See [LangChain frontend HITL](https://docs.langchain.com/oss/python/langchain/frontend/human-in-the-loop).

**Implication for Vivy:** use two projections of one interaction:

- inline, so the conversation explains why Vivy paused;
- global queue, so the user can find and review work after changing sessions or
  returning to the app later.

The queue is not a second source of truth. It is a query over durable pending
interactions.

### 2.4 Risk is multidimensional

Microsoft's safety guidance recommends considering side effects, data
sensitivity, reversibility, and scope of impact when deciding which tools need
approval. It also treats model output and tool data as untrusted and calls out
indirect prompt injection. See [Microsoft Agent Framework safety](https://learn.microsoft.com/en-us/agent-framework/concepts/agents/safety).

This is more useful than a simple read/write flag. Vivy should show at least:

- effect: read, local mutation, process, network, remote mutation;
- reversibility: reversible, recoverable, irreversible, unknown;
- scope: one path, workspace, many files, external system;
- data sensitivity: ordinary, sensitive, credential-adjacent;
- trust: local validated, remote/untrusted, model-generated;
- precondition: unchanged, changed, unavailable.

The policy engine may collapse these dimensions into a decision, but the UI
must preserve the evidence that led to the decision.

### 2.5 MCP adds an explicit elicitation boundary

MCP Elicitation standardizes a server asking the client to collect additional
structured information. It says the client controls the user interaction, the
server must not use elicitation for sensitive information, and the UI should
identify the requesting server, allow review/modification, and provide decline
and cancel. See [MCP Elicitation](https://modelcontextprotocol.io/specification/2025-06-18/client/elicitation).

**Implication for Vivy:** a future MCP elicitation request belongs in the
question family, with a visible server/source boundary. An MCP tool call that
can mutate state remains an approval family item even if the server also asks
for data.

### 2.6 Permission modes reduce prompt fatigue, but are policy, not approval

Claude Code documents separate permission modes for default prompts,
accepting edits, planning, and bypassing permissions; it also distinguishes
read-only tools, shell commands, and file modifications. See [Claude Code CLI and permission modes](https://code.claude.com/docs/en/cli-usage).

Cursor's current Auto-review mode similarly combines allowlists, sandboxable
actions, and a classifier before falling back to user approval. See [Cursor Auto-review](https://cursor.com/changelog/auto-review).

**Implication for Vivy:** “approve once”, “allow in this session”, and “allow
in this workspace” are useful future policy controls, but they must never
rewrite the individual approval record. Each executed effect still needs an
auditable proposal and decision. The next stage should expose only approve
once to keep the security model legible.

### 2.7 Human roles and accessibility must be explicit

NIST AI RMF treats governance as continuous and calls for clearly defined human
roles and responsibilities for human-AI configurations and oversight. See
[NIST AI RMF Core](https://airc.nist.gov/airmf-resources/airmf/5-sec-core/).

For a local single-user product this means the role can initially be
`local_user`, but the domain should still record who/what decided and from
which surface. The UI must not rely on color alone, must expose focusable
actions, and must announce newly pending work to assistive technology.

## 3. Proposed HITL domain model

### 3.1 Interaction state machine

```text
                 ┌───────────────┐
                 │   proposed    │
                 └──────┬────────┘
                        │ persist proposal + checkpoint
                        v
                 ┌───────────────┐
                 │ awaiting_user │
                 └──┬──────┬───┬─┘
                    │      │   │
       approve/answer│      │   └──cancel/expire
                    │      │
                    v      v
              approved  denied/cancelled/expired
                    │
                    v
                 executing
                    │
             ┌──────┴─────────┐
             v                v
          succeeded       failed/stale
```

The persisted interaction is separate from the run status. A run may remain
non-terminal while `waiting_for=approval|question`; the UI should derive that
attention state from the interaction projection. This keeps existing run
terminal invariants intact while making waiting visible.

### 3.2 Review object

The next public DTO should contain the following stable fields. `proposal` is
immutable after creation; an edited proposal, if supported later, must create a
new revision/hash rather than mutate history.

```text
ReviewItem
  id
  kind: approval | question
  status: pending | submitting | approved | denied | answered |
         cancelled | expired | stale | executing | succeeded | failed
  session_id, run_id, tool_call_id
  source: vivy | mcp:<server> | child:<run>
  created_at, expires_at, decided_at
  actor/source
  policy_profile, policy_hash
  proposal (approval only)
    action, target, precondition_hash, preview, risk_findings
    args/data_redacted, effect, reversibility, scope, trust
  question (question only)
    prompt, schema, server_name, sensitive=false
  decision_reason (optional bounded text)
  stale_reason/error (optional bounded text)
```

The raw checkpoint and secrets are not part of this DTO. `proposal.data` is
used by the server to resume and must be treated as sensitive internal state;
the review surface receives a redacted/rendered view.

### 3.3 Required durable events

The existing `tool.approval_required` and `user.question_required` events stay
backward-compatible. The next contract should add or derive the following
events:

| Event | Purpose |
|---|---|
| `tool.approval_decided` | record approve/deny, actor/source, and decision time |
| `user.question_cancelled` | distinguish cancel from unanswered/failed |
| `user.question_expired` | make timeout visible and replayable |
| `tool.approval_expired` | make timeout visible and replayable |
| `tool.proposal_stale` | report precondition/hash mismatch without pretending execution occurred |
| `tool.execution_started` / existing `tool.started` | tie resume to actual effect execution |
| existing `tool.finished` | report result or bounded failure |

If adding event names immediately would create unnecessary contract churn, a
first implementation may persist a generalized `interaction.resolved`
projection and expose it through the review API. The invariant is the same:
the decision must be reconstructible independently of volatile UI state.

### 3.4 Expiration and cancellation

Expiration must be an active transition, not merely a check in the response
handler. The next stage should provide a bounded sweeper or equivalent
deadline mechanism that:

1. marks the interaction expired exactly once;
2. resumes or closes the run with a deterministic human-timeout outcome;
3. publishes the transition to connected clients;
4. leaves the audit trail and proposal intact;
5. makes a late decision return a conflict/expired error without executing.

For a first desktop release, `human_timeout` as a structured failure cause is
clearer than pretending timeout was user cancellation. The exact run terminal
mapping can be decided during implementation, but it must not leave a live run
stuck indefinitely.

## 4. Tool risk policy for the next stage

| Tool family | Default interaction | Review payload | V1 rationale |
|---|---|---|---|
| notes/tasks read, list, search, sequential thinking | auto | result/source only | no side effect |
| file read/search | auto | path scope + trust marker | read-only but output is untrusted |
| file write/patch | approval | unified diff, target, precondition, protected-path warnings | exact human review is possible |
| Skills list/view | auto | provenance and trust marker | read-only discovery |
| Skill create/update/delete/rollback | approval | diff, revision, provenance, rollback hint | durable local mutation |
| network search / HTTP GET/HEAD | auto if allowlisted | URL, host, credentials policy, output trust | read-only external access |
| HTTP mutation | approval | method, host, redacted headers/body summary, idempotency/reversibility | external side effect |
| MCP list tools | auto | server identity and capabilities | discovery |
| MCP call | approval by default if effect is unknown | server, tool, arguments, trust, remote effect warning | remote semantics are not known locally |
| command/execute/process | approval | exact argv, cwd, environment policy, timeout, output trust | process side effect |
| child run | approval when it can reach effectful tools | child scope, policy snapshot, workspace, budget | delegated authority must be visible |
| browser use | excluded | none | explicit product decision |
| GraphTool | test/conformance only | test fixture output | future Vivy-specific capability, not production tool |

The key decision is that `readonly=false` is necessary but not sufficient for
the UI. Every proposal should carry normalized risk facts so a reviewer is not
forced to infer them from a tool name.

## 5. UI information architecture

### 5.1 Primary navigation

The desktop-first shell should have three conceptual areas:

```text
Vivy shell
├── Sessions
├── Needs review  (global count; approvals + questions)
├── Current conversation
│   └── inline interaction card when this run is waiting
└── Run inspector
    ├── Overview
    ├── Review
    └── Activity / events
```

`Needs review` is a first-class destination, not merely a badge that opens the
current run. Selecting an item navigates to the owning session/run and opens a
review detail view without losing the queue context.

### 5.2 Review Center list

Each row should answer “what needs my attention?” without opening it:

- kind badge: Approval or Question;
- short action/tool label;
- target/source (`workspace/...`, `mcp server/tool`, command, or question
  origin);
- session title and run age;
- risk label and the strongest risk finding;
- expiry countdown or expired state;
- current status and connection-independent freshness.

Sorting defaults to risk first, then nearest expiry, then creation time. Filters
should include kind, session, tool family, risk level, and status. Bulk actions
are intentionally absent in V1.

### 5.3 Approval detail

The detail surface is a review document, not an argument dump:

1. **Decision header:** “Vivy wants to …”, tool, source, risk, expiry.
2. **Impact summary:** exact target, effect type, scope, reversibility,
   precondition status.
3. **Preview:** diff viewer for files/Skills; structured request summary for
   HTTP/MCP; exact command/working directory for process tools.
4. **Arguments:** human-readable fields first, raw JSON collapsed and redacted.
5. **Warnings:** untrusted content, remote server, credential boundary,
   protected path, stale/precondition mismatch.
6. **Run context:** link to conversation and event timeline.
7. **Actions:** Approve once, Deny, optional bounded reason. If the proposal is
   stale, replace approval with “Refresh proposal”/“Cannot approve stale
   action”.

The approve button must never imply that the UI itself performed the effect;
after submitting, the state transitions to “Decision recorded — executing”
and then to the actual result.

### 5.4 Question detail

Questions use a separate visual treatment and language:

- identify Vivy or the MCP server asking;
- show the prompt and expected fields/schema;
- make privacy and sensitivity explicit;
- allow review/edit before submit;
- provide Answer and Cancel/Decline;
- never label Answer as Approve.

The initial UI may render text questions only. The data contract should allow a
bounded JSON Schema so structured MCP elicitation can be added without a second
interaction model.

### 5.5 State and error UX

Every interactive surface needs these states:

| State | UI behavior |
|---|---|
| pending | actionable review/answer controls |
| submitting | disable duplicate actions, retain payload |
| approved/answered | show recorded decision and wait for execution |
| denied/cancelled | show outcome and return to transcript |
| expired | read-only details, explain timeout, no action button |
| stale | highlight changed target/hash, require a new proposal |
| failed | show bounded error and link to run event |
| disconnected | preserve details, show reconnect state, do not assume result |
| resolved elsewhere | mark read-only after refresh/replay |

The UI must handle a second reviewer or a second tab winning the first-writer
race. A 409 is a state synchronization event, not a generic red error.

## 6. Next-stage target: HITL Review Center and UI Foundations

### P0 — must ship

1. **Interaction contract:** review DTO, lifecycle states, source/actor, expiry,
   decision timestamp/reason, proposal version/hash, and redacted rendering.
2. **Queue API:** list/get/respond for approvals and questions across all runs;
   include session/run metadata and complete review payload.
3. **Decision lifecycle:** durable decision/expiry/cancel events or an equivalent
   replayable projection; live expiration; late-decision conflict semantics.
4. **Review Center UI:** global queue, filters, risk/expiry indicators, detail
   view, deep-link to session/run, reconnect-safe refresh.
5. **Inline review UI:** upgrade the current run card to use the same detail
   component and proposal data as the queue.
6. **File/Skills review:** diff-first renderer with precondition and provenance
   warnings, plus stale proposal handling.
7. **Non-file review renderers:** exact command view, HTTP/MCP structured view,
   and untrusted-output/source warning.
8. **Question baseline:** distinct text question surface with cancel/expiry and
   a schema-ready wire shape.
9. **Verification:** Go contract/integration tests, SQLite restart/expiry/race
   tests, Playwright flows for queue → detail → decision → resumed result, and
   keyboard/accessibility smoke checks.

### P1 — after the baseline proves stable

- specialized edit/revise flow for file/Skill proposals;
- scoped “allow again” policy with explicit scope and audit trail;
- structured MCP elicitation forms;
- interaction history/search and decision analytics;
- reviewer assignment or multi-stage approvals;
- desktop notifications or external channels.

### Explicitly out of scope

- Browser Use and browser automation dependencies;
- GraphTool as a production capability (test/conformance only);
- blanket “approve all” or silent auto-approval;
- remote multi-tenant identity/RBAC;
- Slack/email approval adapters;
- replacing the Eino checkpoint bridge or the journal-first architecture.

## 7. Acceptance gates for the next stage

The stage is complete only when all of the following are observable:

- A pending approval created in session A is discoverable and reviewable while
  session B is open.
- Refreshing the page, reconnecting the JSON-RPC stream, or restarting Vivy
  does not lose the proposal or duplicate the effect.
- File and Skills approvals show the same server-generated diff the executor
  will apply; a changed precondition cannot be approved as if it were current.
- A denied approval produces a durable decision and a bounded model-visible
  rejection outcome; it never executes the side effect.
- An approved item shows decision recorded, execution started, and final result
  as separate states.
- A question answer cannot authorize an effectful tool, and a question can be
  cancelled or expire without leaving the run indefinitely active.
- A concurrent/late decision is rendered as resolved elsewhere, stale, or
  expired—not as an ambiguous failure.
- No secret, raw credential, checkpoint blob, or unredacted sensitive payload
  is exposed in the queue, event stream, browser storage, or screenshots.
- The UI remains usable with keyboard navigation, visible focus, semantic
  labels, and non-color status cues.
- The same review object can be rendered inline and in the global queue without
  two divergent decision paths.

## 8. Decisions still requiring product sign-off

These are bounded choices for the next planning checkpoint, not blockers to
the research:

1. **Timeout outcome:** should human timeout mark the run `failed` with a new
   `human_timeout` cause, or use a new non-terminal interaction outcome first?
2. **Feedback on deny:** should V1 collect an optional denial reason, or keep
   deny one-click and let the user send a separate message?
3. **Desktop posture:** current recommendation is desktop-first responsive web,
   with the queue available at the same local origin as the chat.
4. **Visual direction:** behavior and information architecture are ready to
   freeze; colors, typography, and illustration style should be selected in the
   UI design sprint rather than guessed in this research document.
5. **Edit timing:** recommendation is no generic edit in P0; decide whether a
   file-specific “revise proposal” belongs in P1.

## Sources

- [LangGraph interrupts](https://docs.langchain.com/oss/python/langgraph/interrupts)
- [LangChain HITL middleware](https://docs.langchain.com/oss/python/langchain/human-in-the-loop)
- [LangChain frontend HITL](https://docs.langchain.com/oss/python/langchain/frontend/human-in-the-loop)
- [OpenAI Agents SDK HITL](https://openai.github.io/openai-agents-python/human_in_the_loop/)
- [OpenAI Agents SDK RunState](https://openai.github.io/openai-agents-python/ref/run_state/)
- [MCP Elicitation](https://modelcontextprotocol.io/specification/2025-06-18/client/elicitation)
- [Microsoft Agent Framework safety](https://learn.microsoft.com/en-us/agent-framework/concepts/agents/safety)
- [Microsoft Agent Framework tool approval](https://learn.microsoft.com/en-us/agent-framework/agents/tools/tool-approval)
- [Claude Code CLI and permission modes](https://code.claude.com/docs/en/cli-usage)
- [Cursor Auto-review](https://cursor.com/changelog/auto-review)
- [NIST AI RMF Core](https://airc.nist.gov/airmf-resources/airmf/5-sec-core/)
- [OWASP Top 10 for LLM Applications](https://owasp.org/www-project-top-10-for-large-language-model-applications/)

