# Vivy chat presentation today (inventory for the DSH comparison)

Scope: `ui/` (the Vivy web UI served at `127.0.0.1:3015`) plus the projection
code it reads. Verified by reading the files cited below; no runtime probing
beyond the earlier live smoke.

## Transcript shell

| Concern | Vivy today | Where |
| --- | --- | --- |
| Container | `ScrollArea` + `max-w-4xl` column, one entry per message | `ui/src/components/chat/ChatView.tsx:90-98` |
| Ordering | flat `messages[]` from `session/messages`, then the streaming placeholder | `ui/src/components/chat/ChatView.tsx:94-95` |
| Turn / step grouping | none in chat; the store keeps `run_id` per message but the view never groups by it | `ui/src/components/chat/ChatView.tsx:94`, `ui/src/lib/api.ts:58` |
| Turn navigation / jump to turn | none | — |
| Load earlier / paging | none (whole list at once) | — |
| Back-to-bottom button | none | — |
| Turn-level activity indicator | none in chat; the input area shows a cancel affordance while running | `ui/src/components/chat/ChatInput.tsx` |
| Usage / timing / TPS | none in chat; token stats exist only on the demo dashboard | `ui/src/components/demo/TokenStatsPanel.tsx` |
| Empty state | single centred line | `ui/src/components/chat/ChatView.tsx:93` |

## Message rendering

| Concern | Vivy today | Where |
| --- | --- | --- |
| User message | right-aligned primary bubble, attachments as `<img>`, hover actions (copy / edit→rewind) | `ui/src/components/chat/MessageBubble.tsx:139-163` |
| Assistant message | left bubble, markdown body (`remark-gfm` + typography plugin since 2026-09-19) | `ui/src/components/chat/MessageBubble.tsx:181-185` |
| Reasoning | a collapsed `<details>` **only on the streaming placeholder**; plain text, no markdown, disappears once the run completes and the projected messages replace the placeholder | `ui/src/components/chat/MessageBubble.tsx:185`, `ui/src/components/chat/ChatView.tsx:95`, `ui/src/lib/store.ts:225` |
| Assistant tool-call step | rendered as nothing (empty projected row was hidden on 2026-09-19) | `ui/src/components/chat/MessageBubble.tsx:134` |
| Tool result | one bordered card per tool result: label `工具结果` + raw result text; file-mutation JSON gets a `DiffView` plus a collapsible raw payload | `ui/src/components/chat/MessageBubble.tsx:66-88` |
| Tool identity in the transcript | **not shown**: no tool name, no arguments, no duration, no status; the wire type carries only `role: 'tool'` + `content` | `ui/src/lib/api.ts:58` |
| Assistant footer | timestamp + copy / regenerate / rewind / fork | `ui/src/components/chat/MessageBubble.tsx:186-201` |
| Errors | run-level `RecoverableError` above the composer | `ui/src/components/chat/ChatView.tsx:96-97` |

## Data actually available client-side

| Fact | Available? | Where |
| --- | --- | --- |
| Live run event stream (`seq`, `type`, `payload`), incl. `tool.started`, `tool.finished` (`tool_name`, `tool_call_id`, `result`, `error`), `model.delta`, `model.reasoning_delta` | yes, while a run is active or replayed via `run/log` | `ui/src/lib/store.ts:53,218-246`, `ui/src/lib/api.ts:66,271` |
| Fold of those events into UI rows | only `streamingText` / `streamingReasoning` accumulation; tool events are appended to `runEvents` and otherwise unused (except `tool.finished` → reload todos) | `ui/src/lib/store.ts:221-246` |
| Tool name / call id / args / result / duration / error per call, for history | yes, in the `trajectory/session` projection (`Text`=tool name, `CallID`, `InputDetail`=args JSON, `OutputDetail`, `Result`, `IsError`, `TimeSeconds`, `Group`=`Step N`, `Turn`, `Tokens`) | `internal/runtime/trajectory.go:44-62,299-311` |
| Reasoning text for history | **no**: `model.reasoning_delta` is never projected (neither `message_projector.go` nor `trajectory.go` reads it) | `internal/runtime/message_projector.go:92-140`, `internal/domain/event.go:12` |
| Token usage / TTFT / per-request timing | partial: `trajectory/session` carries per-request `usage` and per-record `time_seconds`; TTFT is not computed | `internal/runtime/trajectory.go:64-79` |
| Tool-row card models (diff / read / search / terminal / web / image) | diff only, derived from the tool result JSON by `parseToolResultDiff` | `ui/src/lib/diff.ts:121`, `ui/src/components/chat/MessageBubble.tsx:68` |

## The trajectory surface is the closest existing analog

Vivy already ports DSH's trajectory model, and it is the only place with
per-step tool structure.

- Types explicitly ported from DSH `packages/client/ui-trajectory`:
  `ui/src/components/trajectory/trajectory-types.ts:1-6`.
- It already declares DSH fields Vivy never populates on real data:
  `thinkingDetail` and `ttftMs` (`trajectory-types.ts:62,68`) are **read** by the
  detail panel and the timeline (`TrajectoryDetailPanel.tsx:48-49,192-199`,
  `TrajectoryTimeline.tsx:47-49,218-220`) but only ever set by the demo fixture
  (`trajectory-demo-data.ts:118-364`) — the wire mapping fills neither
  (`trajectory-session.ts:22-42`), and the Go projection emits no such fields.
- The assistant-summary fold (assistant row + N following tool rows → one
  `assistantSummary` with `toolCount`) already exists:
  `ui/src/components/trajectory/trajectory-utils.ts:215,239-252`.

## Where the transcript and the trajectory disagree

The chat transcript and the trajectory panel are two independent projections of
the same run: `session/messages` (per-message, no tool identity, no reasoning)
and `trajectory/session` (per-turn/step records, tool identity, no reasoning).
Anything the chat shows about tools today comes from the raw result *text*
embedded in a `role: 'tool'` message, not from the structured record.
