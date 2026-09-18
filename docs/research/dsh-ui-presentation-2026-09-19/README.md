# DSH UI presentation reference — thinking, tool calls, writes (research dossier)

Purpose: Vivy's chat surface should stop being "bubbles + raw tool result cards".
This dossier records how DeepSeek Harness (the reference harness Vivy already
ports pieces of) presents reasoning, tool calls, file writes and turn process,
then states the concrete gap against Vivy today and a phased plan to close it.

Source pin: `.workspace/deepseek-harness/deepseek-harness` @
`0d1f50007f9bca3f52b06e1c3074fa14d5fb0720` (2026-09-15). Read-only: nothing in
that tree was modified.

Method: four parallel source-reading streams (each cited `path:line`) plus
first-hand reading of the transcript shell, the tool-row component, the shared
disclosure primitive, the `ui-chat` locale dictionary and the DSH cookbook. The
Vivy side was inventoried from `ui/` and the Go projections it reads.

## Files

| File | Content |
| --- | --- |
| `stream-a-reasoning.md` | Reasoning ("Think") row + the turn/process fold machinery |
| `stream-b-tool-rows.md` | Tool-call row mechanics: states, I/O disclosure, trees, view selection |
| `stream-c-tool-families.md` | Per-family cards: diff/write, read, search, shell, web, todo, question |
| `stream-d-markdown.md` | Markdown pipeline, code, tables, math, streaming stability, message furniture |
| `vivy-current-presentation.md` | What Vivy renders today, with citations |

## 1. What DSH renders (condensed)

### 1.1 Transcript anatomy

The transcript is a flat, ordered list of typed nodes; each node renders through
one keyed seat (`ui-chat/src/client/chat/ChatView.tsx:222-235,784-808`,
`ChatNodeSeat.tsx:38-149`). Node kinds include user, steering, context,
system-prompt, assistant-step, command, compaction, model-retry, turn-error,
turn-max-tokens, **turn-process** (synthetic), turn-tail, unknown
(`register-node-renderers.ts:18-56`).

Three structural facts matter more than any single component:

1. **A turn has a process and an answer.** Reasoning, tool calls and injected
   context are "process"; the last reply-bearing assistant step is the answer
   (`conversation-nodes/turn-process.ts:117-122,158-167`).
2. **While the turn streams, everything is visible in flow order.** The
   synthetic process controller renders `null` until the turn closes
   (`TurnProcessNodeView.tsx:11`, `ChatNodeSeat.tsx:62-68`).
3. **When the turn closes, the process folds** behind one row labelled from
   counts — `N tool calls · N messages · N subagents`, or `Thought for a while` /
   `已思考` when all counts are zero (`TurnProcessNodeView.tsx:13-40`,
   `locale.ts:62-69,173-180`) — the folded rows becoming `hidden="until-found"`
   and the answer row tightening from a 16px to an 8px gap
   (`ChatNodeSeat.tsx:98`, `ChatView.module.css:60-64`). `turn-error` and
   `turn-max-tokens` never fold (`contract/turn-process.ts:20-33`).

The whole turn also carries a live status while running: `深度求索中...` /
`Deep diving...`, which only reveals the elapsed clock after 15 s
(`ChatView.tsx:168-201`, `locale.ts:25,136`).

### 1.2 Reasoning: a collapsed "Think" disclosure, one per block

Reasoning is not a bubble and not a node: it is a `reasoning` block inside the
assistant step, rendered as a `ReasoningRow` disclosure (`AssistantMarkdown.tsx:85-95`).

- **Collapsed by default, always** — streaming, settled and interrupted alike;
  nothing auto-expands (`ReasoningRow.tsx:29`, pinned by
  `reasoning-row.client.spec.tsx:65-77`).
- **Collapsed summary is live data, not copy**: the *latest* line while running
  (right-aligned, allowed to overflow so the newest tokens stay visible), the
  *first* line when settled (ellipsised); `**` markers are stripped from the
  summary only (`ReasoningRow.tsx:8-17,30`).
- Title is `Think` / `思考` (`ReasoningRow.tsx:46`, `locale.ts:59,170`); while
  running there is an SR-only `运行中`/`Running` plus a 2.6 s gradient sweep that
  stops under `prefers-reduced-motion` (`ReasoningRow.tsx:39`,
  `ReasoningRow.module.css:32-51,117-121`).
- Expanded body is plain text (`white-space: pre-wrap`), indented 22px under the
  title, and its header is **sticky** while open so a long chain cannot bury the
  collapse toggle (`ReasoningRow.module.css:25-30,107-115`).
- **No per-reasoning timer and no token count** — time/usage live in the turn
  tail, not on the reasoning row.

### 1.3 One contract for every tool call

`ToolRow` is the only row renderer; every tool funnels through it
(`ui-tool/src/client/tool/components/ToolRow.tsx:111-336`):

| Element | Contract |
| --- | --- |
| Chrome | one 24px `DisclosureRow`: `[16px leading][title][2px dot][summary, ellipsised, fills]` (`DisclosureRow.module.css:16-23,79-84`) |
| Title | a locale key per variant — Read/Search/Bash/Write/Edit/Code/Tool call — never the raw tool name (`tool-call-model.ts:32-36`) |
| Summary | derived from args: variant-specific key preference (`bash: description→command`, `read/write/edit: path`, `search: query/pattern`), first line only, path relativised to the session cwd then `~`-abbreviated (`tool-call-model.ts:169-208,237-269`) |
| State | `running \| ok \| error \| stopped`, derived **only** from the frozen block (`tool-call-model.ts:239-243`): no result yet → running; `error.code === 'interrupted'` → stopped; `isError` → error |
| Running | not a spinner: the tool icon stays and a 2.6 s glare band sweeps the row (`ToolRow.module.css:23-43`) |
| Error / stopped | the icon is replaced by a red/amber `StateDot`, and the error's **first line replaces the summary** (never supplements it); suffix and file link are dropped (`ToolRow.tsx:90-96,164-179`) |
| Expanded body | exactly one card, in priority order `askQuestion ?? terminal ?? diff ?? read ?? image ?? search ?? web`, else a code block or an `IN`/`OUT` card with independently capped, scrolling sections (`ToolRow.tsx:156,193-321`, `ToolRow.module.css:212-246`) |
| Truncation | one shared head/tail fold (`hidden = total - maxLines`, `head = ceil(max/2)`), chat caps at **9 diff / 8 read / 8 search** rows vs 16 in panels; terminal opts out with `maxLines={Infinity}` and scrolls (`head-tail-cap.ts:23-27`, `diff-card-model.ts:7`, `read-card-model.ts:17`, `search-card-model.ts:12`, `ToolRow.tsx:245-280`) |
| Sub-calls | recursive at every depth through the same dispatch, one 22px indent + hairline rail per level, wire dispatch order, per-child state; parallel siblings are unlabelled (`ToolCallTree.tsx:55-93`, `ToolCallTree.module.css:5-12`) |
| Selection | keyed by wire tool name with a render-site generic fallback; an Auto-review **denial bypasses the keyed lookup** so no business view can hide it (`ToolCallTree.tsx:34-49`) |
| Affordances | whole-row toggle (Enter/Space, `aria-expanded`), hover icon→chevron cross-fade, the path itself is the "open file" button (with a 1-based line), a hover **Inspect** pill into the trajectory view, copy, "N more lines" expander. No retry, no jump-to-approval, and **no duration anywhere** (`ToolRow.tsx:102-109,179-192,232-234,296,322-331`) |

### 1.4 Per-family cards (the part that makes the transcript readable)

- **write / edit** → unified diff only, 3 context lines per side
  (`structuredPatch(..., { context: 3, maxEditLength: 256 })`), rows
  `path | del | add | context | gap` with `- `/`+ ` drawn in CSS so the sign
  survives greyscale; `+N -M` rides the collapsed row and the expanded footer;
  a create is reconstructed from the write args when `diffs: []`; an errored
  mutation shows no card at all (`DiffBlock.tsx:71-131,222`,
  `diff-card-model.ts:6-7,70-113`).
- **read** → path summary is the openable link (landing on `offset`), card shows
  a `显示 N / M 行` window banner, real line-number gutter, shiki highlighting by
  language hint, head/tail fold at 8 lines (`ReadBlock.tsx:103-150`,
  `read-card-model.ts:17,31-71`).
- **read_image** → in-card label + gallery through a child slot + the result's
  own envelope line (`image/png image, 1496x260 px`), never the raw attachment
  object (`ToolRow.tsx:264-272`).
- **grep / glob** → grouped per file with that file's match count, per-file
  expand, `显示 N / M 匹配 · K 文件`, a "never present a capped result as
  complete" banner, and a recovery locator line when the host spilled
  (`SearchBlock.tsx:34-40,143-155,200-240`).
- **bash / terminal** → prompt line with resolved cwd, output only once settled
  (no live streaming in the transcript), exit code/signal parsed from the
  result's trailing `[exit code: N]` / `[killed by signal: X]` marker and shown
  as a pill, one run-state dot, in-card scroll instead of folding; abort =
  stopped + amber dot; background jobs render in a **session-header popover**,
  not the transcript (`terminal-card-model.ts:86-97,205-236,265-311`,
  `ui-jobs/src/client/JobListAction.tsx:44-54`).
- **web_search / web_fetch** → provider answer as markdown above a numbered
  source list (title → hostname → raw URL fallback, snippet, date), `http(s)`
  only as anchors, `HTTP 200` for a fetch (`WebBlock.tsx:74-186`).
- **todo_write** → one-line `1/3 已完成 · <first in-progress>` summary, not a
  checklist (`todo-row.tsx:39-44`).
- **ask_user_question** → read-only transcript card pairing questions with
  answers by id; the interactive form is a different package that takes over the
  composer (`AskQuestionCard.tsx:11-39`, `ui-user-questions`).

### 1.5 Markdown and message furniture

- DSH abandoned `react-markdown`/remark for a direct mdast→React switch so
  streaming can freeze completed blocks as React elements
  (`ui-primitives/src/markdown/render.tsx:1-5`); **two grammars**: streaming =
  `gfm() + cjkFriendlyStrong()`, settled adds `mathCompatibility() + math()`
  (`parse.ts:26-43`).
- Tables: ≥4 columns gets a natural-width, keyboard-reachable scroll wrapper
  that reveals on hover/focus, and the chat breaks it out of the message column
  via a container query (`render.tsx:462-492`, `MarkdownText.module.css:190-231`,
  `AssistantMarkdown.module.css:33-43`).
- Code: shiki with 3 boot grammars + 24 lazy ones, banner + copy button, opt-in
  line numbers, wrap-by-default; unknown language = plain text
  (`CodeBlock.tsx:187-196`, `highlight.ts:42-133`).
- Links/images pass a protocol allowlist; raw HTML renders as literal text (no
  HTML parser in the pipeline); CJK needs a local `cjkFriendlyStrong` micromark
  extension so `**注意：**内容` closes strong after punctuation
  (`render.tsx:1-12,45-70`, `cjkFriendlyStrong.ts:55-58`).
- Math is KaTeX, statically imported, with a 3-arm failure chain; TeX is
  deliberately absent from the streaming grammar so partial formulae never flash
  errors (`katex.tsx:20,66-89`, `parse.ts:20-22`).
- Furniture: per-message actions live in the **turn tail** with a fixed order —
  clock → copy → plugin extra actions (like/dislike) → branch → usage/time pills
  (`MessageIconActions.tsx:82-112`, `TurnTailNodeView.tsx:37-67`). There is no
  edit and no regenerate affordance in DSH. Session-level stats pills sit under
  the composer (`StatsPills.tsx:138-233`).

## 2. Vivy today

Full inventory in `vivy-current-presentation.md`. Summary of what matters:

- The transcript is a flat message list; tool results are full-width cards with
  a `工具结果` label and raw JSON/text, and the tool-call step itself renders as
  nothing (`ui/src/components/chat/ChatView.tsx:90-98`,
  `ui/src/components/chat/MessageBubble.tsx:66-88,131-134`).
- Reasoning is a collapsed `<details>` that exists **only while the run
  streams**, then disappears when the projected messages replace the streaming
  placeholder (`MessageBubble.tsx:185`, `lib/store.ts:225`).
- The wire type carries no tool identity at all: `Message` is
  `{id, run_id, role, content, created_at, attachments?, provenance?}`
  (`ui/src/lib/api.ts:58`).
- The **data exists**: the assistant row the kernel projects for each
  `tool.requested` already carries `ToolCallID` / `ToolName` / `ToolArgs`
  (`internal/runtime/message_projector.go:114-118`), and `model.reasoning_delta`
  events exist in the Journal (`internal/domain/event.go:12`) — neither reaches
  the UI.
- A parallel, DSH-derived projection already ships in the trajectory panel and
  carries tool name, call id, args, result, error, duration, step group and
  tokens (`internal/runtime/trajectory.go:44-62,285-311`), but the wire mapping
  (`ui/src/components/trajectory/trajectory-session.ts:22-42`) never fills
  `thinkingDetail` / `ttftMs` / the tool name even though the panel and timeline
  read them (`TrajectoryDetailPanel.tsx:48-49,192-199`,
  `TrajectoryTimeline.tsx:47-49`), so those parts are demo-fixture-only today.

## 3. Gap table

| # | Element | DSH | Vivy | Impact |
| --- | --- | --- | --- | --- |
| 1 | Tool identity in the transcript | title + args-derived summary + path link per call | nothing (raw result card) | **high** — you cannot tell which tool ran |
| 2 | Tool state | `running/ok/error/stopped`, derived from the frozen block | none | **high** — no in-flight or failed signal |
| 3 | Tool result rendering | one card per family, capped and folded | diff only (`DiffView`), everything else raw text | **high** — the transcript is a JSON dump |
| 4 | Turn process fold | one row (`N tool calls · N messages`) after the turn closes | every tool result is a permanent card | **high** — long turns bury the answer |
| 5 | Reasoning | collapsed Think row, kept after the turn, live last line | `<details>` only while streaming, then gone | **high** — thinking is unreadable afterwards |
| 6 | Turn tail | usage tokens, run time, TPS, TTFT, copy, branch | timestamp + copy/regenerate/rewind/fork | medium |
| 7 | Errors / retry | retry countdown row, turn-error row, max-tokens notice, stopped chip | one run-level error banner | medium |
| 8 | Tool-row affordances | open file at a line, Inspect → trajectory | none | medium (Vivy has the trajectory panel and the files panel) |
| 9 | Approvals | composer takeover, never a transcript row | separate approvals view + `ReviewCard` | medium — a waiting tool has no row state |
| 10 | Markdown | GFM + math + CJK strong + shiki + wide-table breakout | GFM + typography (2026-09-19); no highlighting, no math, CJK `**注意：**` limitation, no footnote links | medium |
| 11 | Navigation | turn rail, load earlier, back-to-bottom | none (single scroll area) | low until sessions get long |
| 12 | Accessibility of state | SR-only status text, `role=button`/`aria-expanded` on every row, reduced-motion | native `<details>` only | medium |

## 4. Recommended alignment plan

Framing rule from the repo: Vivy stays one runtime and one source of truth per
fact. This is a presentation plan; only Phase 0 touches the kernel, and only to
*widen an existing projection*.

### Phase 0 — decide the transcript's tool/reasoning data home (kernel + wire)

Decide once, then derive everything else from it. Two candidates already exist:

- **(a) The message projection** (`session/messages`). The rows already carry
  `ToolCallID/ToolName/ToolArgs`; exposing them
  (`internal/rpc/control.go` message DTO + `ui/src/lib/api.ts:58`) preserves
  transcript ordering exactly and needs no new fold. Reasoning would need one
  more field on the projected assistant message (fold
  `EventModelReasoningDelta` alongside the text deltas in
  `internal/runtime/message_projector.go:90-140`), which loses block interleaving
  but is enough for a DSH-style Think row.
- **(b) The trajectory projection** (`trajectory/session`). Already DSH-shaped
  (step group, call id, args, result, duration, tokens) but coarser: it has no
  message identity, so the chat would have to re-derive interleaving.

Recommendation: **(a) for the transcript** (ordering is the transcript's job),
keeping (b) as the "Inspect" destination it already is. Live runs render from
`runEvents`, which the store already collects (`ui/src/lib/store.ts:218-246`).
Reasoning should be kept after the run only if we accept the projection change;
otherwise ship live-only reasoning first and add the field second.

### Phase 1 — transcript rows (UI only)

- New pure fold `ui/src/components/chat/transcript-rows.ts`: messages +
  `runEvents` → typed rows (`user | assistant | reasoning | tool | toolTail |
  error`), grouped by `run_id` with a `Step N` label like the trajectory fold.
- New `ToolRow.tsx` implementing the DSH contract in Tailwind: 24px disclosure,
  variant title from the tool name, args-derived single-line summary, state dot,
  `+N -M` suffix for diffs, error line replacing the summary, expandable body.
- New `ReasoningRow.tsx`: collapsed by default, live last line while streaming,
  first line when settled, sticky header when open, sweep under
  `prefers-reduced-motion`, `**` stripped from the summary only.
- Body cards: reuse the existing `DiffView` for file mutations (already fed by
  `parseToolResultDiff`, `ui/src/lib/diff.ts:121`), add an `IN`/`OUT` card and a
  terminal/text card with the shared head/tail fold
  (`hidden = total - maxLines`, `head = ceil(max/2)`; chat caps 9 diff / 8 read /
  8 search).
- Keep `MessageBubble` for the user and assistant prose bodies only; its action
  bar becomes the row-level tail.

### Phase 2 — turn structure

- Fold a completed turn's process behind one row (`已思考 · N 次工具调用`),
  expandable, with a 标准/紧凑 setting like DSH's
  (`settings.transcript.normal|compact`). This is the single biggest readability
  win and needs no new data beyond Phase 1.
- Turn tail: run time, TPS and token usage from `trajectory/session`'s per-request
  `usage`/`started_at`/`completed_at` (`internal/runtime/trajectory.go:64-79`);
  TTFT needs a new field (DSH shows it; Vivy's port already declares `ttftMs`).
- Row-level link into the existing trajectory panel ("查看调用"), and open-file
  through the existing workspace read API where a path is known.

### Phase 3 — markdown and code polish

- `**注意：**内容` (CJK strong after punctuation) does not bold today; DSH needed
  a custom micromark extension for it. Either port that extension or accept the
  limitation explicitly.
- Fenced code in chat has no highlighting; `highlight.js` is already a dependency
  used by `FilesPanel` only — reusing it in the markdown body is the cheap route
  (shiki would be a new dependency with a larger payload).
- Math: not present; defer until asked (KaTeX is a real dependency and a real
  maintenance surface).

### Explicitly not copying

- The Cordis slot/register/inject machinery, the four-shares props and the
  keyed-cell renderer (a `Record<toolName, Component>` plus a default is the
  plain-React equivalent).
- CSS-Module class names and `--dsw-*`/`--dsh-*` tokens: keep the geometry,
  colour roles and behaviour; re-express in Tailwind utilities and Vivy's theme
  tokens.
- DSH's mdast→React renderer rewrite. Its motivation is streaming-cache
  performance; Vivy's `react-markdown` + `remark-gfm` (+ typography plugin) is
  adequate until streaming markdown actually janks.
- DSH's absence of edit/regenerate: Vivy's Journal-backed rewind/edit/fork are a
  product feature and stay.

### Architecture / Eino check

No agent-loop, model, tool-registry, prompt, checkpoint or multi-agent behaviour
is proposed. Phase 0 adds fields to an existing Journal-derived projection (the
same events `internal/runtime/trajectory.go` already folds); no new Eino surface
is involved and the `internal/runtime`/`internal/provider` import quarantine is
untouched. Phase 1-3 are React/Tailwind presentation over the store.

## 5. Decisions needed before implementation

1. Phase 0 (a) vs (b) — tool rows from `session/messages` (recommended) or from
   `trajectory/session`?
2. Is reasoning kept after a run completes (one extra projected field), or
   live-only for now?
3. Turn process fold default: always fold completed turns, or expose the
   DSH-style 标准/紧凑 setting?
4. Does a waiting-for-approval tool get a transcript row state, or does the
   approvals view stay the only place?
5. Scope now: Phase 1 only, or Phase 1+2 as one delivery?

## 6. Verified claims

The stream drafts were spot-checked against source before integration: the
markdown renderer header and the two micromark grammars
(`render.tsx:1-12`, `parse.ts:26-43`), the chat card caps
(`diff-card-model.ts:7`, `read-card-model.ts:17`, `search-card-model.ts:12`,
`ToolRow.tsx:245-280`), the state derivation and the denial bypass
(`tool-call-model.ts:237-243`, `ToolCallTree.tsx:34-49`), the absence of any
duration rendering in `ui-tool/src`, the reasoning row's sticky header and 2.6 s
sweep (`ReasoningRow.module.css:25-51`), and the bash row's production
registration (`ui-tool/src/client/apply.ts:43`).
