# DSH: tool-call row mechanics

Source: `.workspace/deepseek-harness/deepseek-harness` @ `0d1f50007f9bca3f52b06e1c3074fa14d5fb0720`. Paths below are relative to that root; every citation is `path:line`.

## Where it lives

- Row chrome: `packages/client/ui-tool/src/client/tool/components/ToolRow.tsx:111` — the only generic row renderer; every variant and every tool-supplied card funnels through it.
- Tree/root composition: `packages/client/ui-tool/src/client/tool/ToolCallTree.tsx:101`.
- Fallback card + variant icons: `.../tool/toolviews/GenericToolCard.tsx:31`.
- Pure row model: `.../tool/models/tool-call-model.ts:237` (`toolRowModel`, plus `classifyTool:92`, `formatToolBody:216`, `resultText:134`).
- Shared chrome primitive: `packages/client/ui-primitives/src/DisclosureRow.tsx:33`; status mark `.../StateDot.tsx:22`; geometry `.../components/ToolRow.module.css`.
- Row input contract: `block: ToolCallBlock` from `@deepseek-ai/dsh-client-ui-chat/client`, defined at `packages/client/ui-conversation/src/client/contract/records.ts:280`.

## Lifecycle and states

`ToolRowState` is a 4-arm union (`tool-call-model.ts:21`): `'running' | 'ok' | 'error' | 'stopped'`. Derivation is entirely from the frozen slice (`tool-call-model.ts:239-243`):

```
const done = 'kind' in block
const state: ToolRowState = !done ? 'running'
  : block.error?.code === 'interrupted' ? 'stopped'
    : block.isError ? 'error' : 'ok'
```

"Running" is literally "the block is a `RunningToolCall`" (no `kind` discriminant, `records.ts:265`); "stopped" is an error whose code is `interrupted`; denial has no arm of its own.

- Leading slot by state (`ToolRow.tsx:90-96`): `error` → `<StateDot state="error" />` (red, `ui-primitives/src/StateDot.module.css:43-45`); `stopped` → `<StateDot state="warning" />` (amber, `:39-41`); otherwise the caller's `icon`.
- Running is **not** a spinner and **not** a dot: the variant icon stays and a CSS glare band sweeps the row. `ToolRow.module.css:23-43`: `.root[data-state='running'] .row::after { ... animation: dsh-tool-row-sweep 2.6s ease-out infinite }`; test comment `ToolRow.tsx:343` "running keeps the icon (row sweep carries the signal)".
- The only words are accessibility text (`ToolRow.tsx:102-109`, rendered at `:198`): `row.running` / `row.failed` / `row.stopped` into `.visuallyHidden`; EN `Running` / `Failed` / `Stopped` (`packages/client/ui-conversation/src/client/locales.ts:252-254`).
- **No success check/cross glyph exists.** `ok` renders the variant icon only and `stateStatus` returns `null` (`ToolRow.tsx:107`).
- **Timing is absent.** Neither `time` nor `callTime` is rendered anywhere in `ui-tool/src` (grep `callTime|duration` over that tree returns only prose). `ToolResultNode.callTime` is documented "used for call-row duration" (`records.ts:165-166`) but this package never consumes it.
- Denial/abort:
  - Abort/interruption → `stopped`, warning dot, `row.stopped`, summary unchanged.
  - Auto-review denial → a settled `isError` result with `name === 'AutoReviewDeniedError' && code === 'AUTO_REVIEW_DENIED'` (`tool-call-model.ts:118-125`). It renders as `state: 'error'` with a *replacement* collapsed summary (`GenericToolCard.tsx:33-35,57-60`): args body forced to `null`, `output = autoReview.output`, `errorSummary = autoReview.summary`. Keys `tool.autoReviewRejected` / `tool.autoReviewNotExecuted` / `tool.autoReviewReasonFallback` (`models/auto-review-denial.ts:31-35`; EN `locales.ts:275-277`). The stored reason is normalized for display only (trim; `[\r\n\u2028\u2029]+` → space).
- Failure always replaces, never supplements, the summary (`ToolRow.tsx:164-166`): `const failureLine = state === 'error' ? errorSummary ?? null : null`, `const summaryText = failureLine ?? terminalBody?.description ?? summary`. A failure line also drops the trailing suffix (`:175`) and the open-file link (`:179`; test `tests/tool-row.client.spec.tsx:441-454`).

## Input/output disclosure

Header content in DOM order inside one 24px flex line (`DisclosureRow.tsx:74-100`, `DisclosureRow.module.css:16-23`): `[16px leading] gap6 [title] gap8 [summary FILL truncate]`.

- **Title** is a locale key, never the raw tool name: `title={t(model.titleKey)}` (`GenericToolCard.tsx:53`), keys `tool.title.search|read|bash|write|edit|code|generic` (`tool-call-model.ts:32-36`); EN `Search`, `Read`, `Bash`, `Write`, `Edit`, `Code`, `Tool call` (`locales.ts:258-264`). Tool-owned overrides exist for `pwsh`, `read_image`, `cordis_*` (`tool-call-model.ts:77-85`).
- **Summary** is args-derived and single-line (`tool-call-model.ts:180-194`). Per-variant key preference (`:169-177`): `bash: ['description','command']`, `read: ['path','file_path','url']`, `search: ['query','pattern','url']`, `write`/`edit: ['path','file_path']`, `code: ['description']`, `others: []`. Then first string value, then `firstLine()` at the first `\n`; non-JSON args fall back to the raw string; empty args fall back to the `callId` (`:244-246`). For `others` the wire name is prefixed into the summary slot: `` `${toolName} · ${base}` `` (`:250-252`).
- **Target/path**: `filePath` is derived only for variants `read|write|edit` and only from `path`/`file_path` (`tool-call-model.ts:197-208`) — never `url`. Display relativizes to session `cwd` then abbreviates a POSIX home to `~` (`:246`); the value handed to the open callback stays authored.
- **Full input** is never in the collapsed row. Expanded it is pretty-printed JSON via `formatToolBody` (`tool-call-model.ts:216-227`: `JSON.stringify(parsed, null, 2)`, raw string when unparseable), formatted *only while expanded* (memoized on `open`, `ToolRow.tsx:159-162`; asserted by counting `JSON.stringify` calls, `tests/tool-row.client.spec.tsx:321-341`).
- **Full output** is the flattened result text (`tool-call-model.ts:134-144`): text blocks verbatim, other block shapes pretty JSON, and for an empty failed result `"${name}: ${code}"`. It renders in the expanded card under an `OUT` gutter label (`ToolRow.tsx:310-317`).
- **Inline, not a side panel**: "Every card is read in place in the call tree; there is no second, full-height presentation of a selected call" (`packages/client/ui-tool/README.md:65`).
- Generic body geometry (`ToolRow.module.css:212-232`): `.ioCard` with an `IN`/`OUT` two-column grid whose `.ioSection`s are independently `max-height: 150px; overflow-y: auto`. The `code` variant instead renders a `CodeBlock` in `.bodyScroll` capped at `max-height: 260px` (`:203-206`, `ToolRow.tsx:294-298`) and contributes no input body (`ToolRow.tsx:194-195`). When a card exists it wins over IN/OUT (`ToolRow.tsx:156-162,238-321`).
- **Truncation is a card-level head/tail fold, not a text cut**: diff keeps `CHAT_DIFF_MAX_LINES = 9` (`models/diff-card-model.ts:7`), read `= 8` (`read-card-model.ts:17`), search `= 8` (`search-card-model.ts:12`); terminal passes `maxLines={Infinity}` and scrolls (`ToolRow.tsx:243-248`). The primitives compute `headTailCap(total, maxLines, expanded)` (`ui-primitives/src/head-tail-cap.ts:23-26`) and expose a control labelled `diff.expandRest` / `read.expandRest` / `search.expandRest` / `terminal.expandRest` (EN `… {count} more lines`, `locales.ts:282,286,294,322`). A diff row also prints `+{added} -{removed}` in the collapsed line via `diffTotals` (`ToolRow.tsx:170-174`, `.diffStat` `ToolRow.module.css:113-119`), and a capped search keeps a recovery footer with its "Full … stored at …" locator (`ToolRow.tsx:283-288`). Independent flags: `web.sourcesTruncated` / `web.contentTruncated` (`ui-primitives/src/WebBlock.tsx:167,183`).

## Trees, sub-calls, concurrency

`ToolCallTree` renders exactly one root block plus its recursive children: `const block = node.data.root` (`ToolCallTree.tsx:105`).

- Every node, at any depth, goes through the same atomic dispatch `ToolCall` (`ToolCallTree.tsx:15-53`); recursion is `ToolCallBranch` (`:55-93`). README: "sends the root and children at every depth through the same atomic dispatch path, without subscribing to a separate parent-to-children map" (`README.md:60`).
- Per-node DOM contract (`ToolCallTree.tsx:38-43`): `data-chat-anchor-key={`call:${callId}`}` and `data-chat-call-id={callId}` (paging/selection).
- Children wrapper (`ToolCallTree.tsx:74-90`, `ToolCallTree.module.css:5-12`) renders only when `block.subCalls.length > 0`, as `<div className={css.subCalls} data-subcalls>`; indentation is `margin: 4px 0 2px 22px; padding-left: 8px; border-left: 0.5px solid var(--dsw-alias-border-l2)` — one 22px step plus a hairline rail per level.
- **Ordering** is the wire's dispatch order; nothing sorts or groups it. `records.ts:171-172`: "Child calls owned by this call, in dispatch order" (`subCalls: readonly ToolCallBlock[]`). The tree test pins the exact parentage walk `parent → parent:code:1 → unrelated:ptc:7` (`tests/tool-call-tree.client.spec.tsx:68-94`).
- **Parallel calls** are plain siblings in `subCalls`: no concurrency badge, brace, lane, or count on the generic row. (The only parallel-active count in the design is a *todo-row* `summarySuffix`, `ToolRow.tsx:36-43` — not a tree feature.)
- Each child is an independent row with its own state: a running child wears the same sweep as a native in-flight row and an `isError` child the same error dot (`tests/chat-ptc-subcalls.client.spec.tsx:236-263` and `:208-217`).
- A running `run_code` root nests its so-far dispatches under the running parent row (`tests/chat-ptc-subcalls.client.spec.tsx:236-248`).
- Depth is bounded by the client store, not the view: `MAX_TOOL_CALL_TREE_DEPTH = 256` (`ui-chat/src/client/model/tool-call-tree.ts:14`), with cycle/edge rejection (`:163-198`).
- README limitation: "The Host excludes `run_code` from PTC mode program bindings — production events produce one dispatch level; the recursive Runtime/UI contract supports nesting" (`README.md:106`).

## View selection and fallback

One rule: atomic tool views are keyed by **wire tool name**, dispatched through the keyed slot with a render-site fallback (`ToolCallTree.tsx:44-49`):

```
autoReviewDenied
  ? <GenericToolCard {...owner} t={t} />
  : renderSlot('tool.call.toolview', owner, {
      entryKey: toolName,
      fallback: <GenericToolCard {...owner} t={t} />,
    })
```

- The slot is declared by the parent Chat node it renders into (`src/client/apply.ts:33-41`): the `conversation.chat.node` / `tool-call` registration carries `children: { 'tool.call.toolview': { kind: 'keyed', scope: 'session' } }`.
- Slot declaration: `contract/slots.ts:26` — `'tool.call.toolview': { kind: 'keyed'; scope: 'session'; owner: ToolCallOwnerProps }`, with "the key domain is open (any wire tool name…) so there is no compile-time key set to pick from and a typo simply never renders" and "A key the shipped composition already covers is replaced, not shared; an unclaimed key falls back to the generic tool row".
- Registration (`README.md:34-40`, exercised at `tests/toolview-slot.client.spec.tsx:284-331`): `ctx.slots.inject('tool.call.toolview', () => ctx.slots.register({ name, key: '<wire tool name>' }, View))`. `inject` waits for the *declaration*, so a registrant may mount before `ui-tool`; the contribution is dropped when the declaration collapses and leaves with the caller's fiber.
- Keyed resolution lives in the renderer (`ui-renderer/src/client/scoped-slots.tsx:806-813`): find the entry whose `options.key === opts.entryKey`; if none and the cell is unoccupied, render `opts.fallback`. A keyed hit replaces the fallback without remounting the view ("per-key version tick", `tests/toolview-slot.client.spec.tsx:231-248`); a duplicate key throws at load (`:250-257`, `ui-slots/src/index.ts:844-848`).
- **Denial pre-empts the keyed slot**: `toolRowModel(...).autoReviewDenial !== null` is checked *before* dispatch, so no business registration can hide a denial (`ToolCallTree.tsx:34-49`; `tests/toolview-slot.client.spec.tsx:151-185`).
- **Fallback content** (`GenericToolCard.tsx`): `classifyTool(toolName)` → variant (`tool-call-model.ts:92-94`, table `:47-74`), variant icon (`GenericToolCard.tsx:16-24`), then `toolRowModel` for title/summary/args/output/filePath plus cards tried in order `terminalCardModel`, `readCardModel`, `diffCardModel`, `searchCardModel`, `webCardModel` (`:36-40`). An unknown name is `others` → title `tool.title.generic` (`Tool call`) and a `toolName · summary` row (`tests/tool-row.client.spec.tsx:508-515`). A single-file tool never exposes an args body (`GenericToolCard.tsx:46-58`).
- Plain-React mimicry (behaviour only): a `Map<string, ComponentType<ToolRowOwner>>` keyed by wire name, `const View = registry.get(block.name) ?? GenericToolCard` at render time, and a fallback that classifies the name into a variant and derives title/summary/path. No keyed slot, no `renderSlot`, no Cordis.

## Interaction inventory

| Affordance | Citation | Label / key |
|---|---|---|
| Expand/collapse | `DisclosureRow.tsx:50,74-82` | none (chevron); `role="button"`, `tabIndex=0`, `aria-expanded`, whole row clickable, Enter/Space only (`:55-59`); non-expandable rows render a passive span, no button (`tests/tool-row.client.spec.tsx:354-359`); gate `ToolRow.tsx:157-158` |
| Collapsed hover preview | `DisclosureRow.tsx:60-70`, `DisclosureRow.module.css:71-77` | none — icon fades out, chevron fades in |
| Copy (code body) | `ToolRow.tsx:296` | `copy` / `copied` (EN `Copy`/`Copied`, `packages/client/locale/src/locales/en.ts:8-9`) |
| Copy (cards) | `models/primitive-labels.ts:33,51,75` | `copy` / `copied` |
| Expand truncated card | `primitive-labels.ts:38,56,81`, `terminal-card-model.ts:30` | `*.expandRest`, `*.expandAria`, `*.collapseAria`, `collapse`/`expand` (`locales.ts:281-286`) |
| Open file | `ToolRow.tsx:179-192,216-224` | none — the path text is the control (dotted-underline `.fileLink`, `ToolRow.module.css:126-151`); only when `filePath` **and** `onOpenFile` are set and the row is not a failure; passes `{ line }` when `filePathLine` is set; `stopPropagation` so it never toggles |
| Inspect / jump to trajectory | `ToolRow.tsx:322-331`, `ToolCallTree.tsx:32` | `row.inspect` (`Inspect` / `查看`); hover/focus-revealed pill with `IconInspectOutline12`, only inside the expanded body and only when the owner passes `inspect`; fires `inspectCall(callId)` |
| Error detail | `ToolRow.tsx:165,227,310-317` | `row.output` (`OUT`): first line replaces the summary, full error-coloured text (`data-error`) shows expanded |
| Retry | — | **Absent** in `ui-tool` (grep `retry` in `src` returns nothing); the conversation-level retry node is unrelated |
| Jump to approval | — | **Absent**; no approval/permission affordance, only `sandbox_permissions`/`justification` arg *validation* (`models/raw-tool-call.ts:58-64`) |

Localized title/state keys live in the `conversation` namespace (`src/client/locale.ts:2` → `CONVERSATION_NS = 'conversation'`), used because the `tool-call` Chat Node registration declares `locale: NS` (`apply.ts:33-41`); README records this reuse as a known limitation (`README.md:108`).

## Client data contract

The row consumes one frozen union, never a live subscription:

- `ToolCallBlock = RunningToolCall | ToolResultNode` (`ui-conversation/src/client/contract/records.ts:280`).
- `RunningToolCall` (`records.ts:265-277`): `callId`, `parentCallId?`, `name`, `argsRaw`, `turn`, `step`, `time`, `subCalls`.
- `ToolResultNode` (`records.ts:155-173`): `kind: 'tool-result'`, `seq`, `time`, `callId`, `parentCallId?`, `call: { name, argsRaw } | null` (null when window truncation left the head outside), `callTime: number | null`, `content: readonly ContentBlock[]`, `isError`, `error?: { name, code, reason? }`, `meta?`, `subCalls`.
- Tree owner props (`ui-tool/src/client/contract/slots.ts:100-103`): `ToolTreeProps = PropsRuntime<'conversation.chat.node','tool-call'> & PropsRenderSlots<'tool.call.toolview'> & PropsLocale<'conversation'> & InjectFace<ToolHostInfoInjected>`; fed with `node.data.root` and `cwd`, `openFile`, `inspectCall`, `loadImage`, `useHostInfo`, `t` (`ToolCallTree.tsx:101-104`).
- Atomic-view owner currency: `ToolCallOwnerProps` (`slots.ts:55-81`) — `callId`, `toolName`, `block`, `cwd?`, `home?`, `openFile(path, options?)`, `loadImage`, `inspect?`; built at the dispatch site including `inspect: () => { inspectCall(callId) }` (`ToolCallTree.tsx:24-33`).
- Row-level selected shape: `ToolRowModel` (`tool-call-model.ts:97-116`) — `variant`, `titleKey`, `summary`, `filePath`, `bodyRaw`, `output`, `errorSummary`, `autoReviewDenial`, `state`. Helpers: `classifyTool:92`, `toolRowModel:237`, `formatToolBody:216`, `resultText:134`, `parsedToolCall` (`models/raw-tool-call.ts:17`), `singleResultText` (`:46`).
- `ToolRowProps` (`ToolRow.tsx:28-88`) deliberately carries pre-derived strings (`title`, `summary`, `bodyRaw`, `output`, `errorSummary`) plus optional card models — the chrome primitive knows nothing about tools.
- Home facts arrive through a hook, not an injected value: `ToolHostInfoInjected.hooks.hostInfo: HostObservable<RemoteHostFacts>` (`slots.ts:87-97`), selected as `useHostInfo(info => info.home)` (`ToolCallTree.tsx:104`); the JSDoc explains a plain injected value would freeze at first render (`slots.ts:89-94`).
- Session `cwd` and the `openFile`/`inspectCall`/`loadImage` callbacks come from the Chat node owner (`ui-chat/src/client/contract/slots.ts:81,84-93`).

## Implementer notes

Keep (behaviour worth reproducing in a plain React + Tailwind app):

1. **One row component, many variants.** Variant from the wire name via a small lookup (`tool-call-model.ts:47-94`) over a fixed variant union (`:18`) driving icon, title key, and summary key preference; unknown names land on a styled fallback (`others` + `tool.title.generic`), not an unstyled row.
2. **A pure row model.** `toolRowModel(toolName, block, cwd, home)` is a total function over immutable data producing every display string (`tool-call-model.ts:237-269`). Port that first; the JSX then becomes trivial.
3. **State from data, not timers.** `running` = no result yet; `error` = `isError`; `stopped` = `error.code === 'interrupted'`. No client-side duration, no optimistic success.
4. **Header line contract**: 24px row, `[leading 16px][title][2px dot][summary ellipsized]` (`ToolRow.module.css:1-7`, `DisclosureRow.module.css:16-23`). The summary is the only flexible, clipping element; a count that must survive needs its own non-shrinking trailing fragment (`ToolRow.tsx:232-234`).
5. **Running is a row-wide subtle animation, not a spinner**, leaving the tool icon in place (`ToolRow.module.css:23-43`); error/stopped swap the leading icon for a coloured dot *and* emit a visually-hidden text label (`ToolRow.tsx:98-109,198`).
6. **Failure replaces the summary**: first error line collapsed, full text expanded, error colour, no file link, no suffix (`ToolRow.tsx:164-179`).
7. **Expansion is local `useState` on the row** (`ToolRow.tsx:137,176-178`) with a lazily computed body, and the collapsed summary stays visible while open (`keepContentWhenOpen`, `:209`).
8. **Card-over-text precedence** with one `card = askQuestion ?? terminal ?? diff ?? read ?? image ?? search ?? web` chain (`ToolRow.tsx:156`) and args-body suppression for single-file tools (`GenericToolCard.tsx:58`).
9. **Truncation UX**: head/tail folding with an explicit "N more lines" expander and exact `shown / total` counts (`head-tail-cap.ts:23-26`, `locales.ts:283-289`).
10. **Recursive sub-calls** at one fixed indent step with a hairline rail, dispatch order preserved, per-child state (`ToolCallTree.module.css:5-12`).
11. **View selection as a keyed lookup with a render-site fallback**, plus "a typed override can never mask a denial" (denial identity checked before the lookup, `ToolCallTree.tsx:34-49`).

Do not copy (DSH-plugin-specific, inert outside its runtime):

- `ctx.slots.register` / `slots.inject` / the `children` authorization table and `SlotMap` declaration merging; the keyed-cell shadowing model, per-key version ticks, and the `deadCell()` crash face (`ui-renderer/src/client/scoped-slots.tsx:799-812`). Reproduce the *behaviour* with a `Map`.
- The four-shares props derivation and synthesized `use<Name>` hooks (`ui-slots/src/index.ts:474-489`), and the `HostObservable`+selector mechanism for `home` (`slots.ts:87-97`) — pass plain props/strings.
- Cordis registrant boilerplate and package-private test hooks (`bash-sample.tsx:156-163`, `data-sample="bash"`).
- `data-chat-anchor-key` / `data-chat-call-id` are this chat surface's paging/selection contract (`ToolCallTree.tsx:41-42`); add an analogous anchor only if your own selector needs it.
- `--dsw-*` tokens and CSS-module class names (`ToolRow.module.css` throughout): re-express as Tailwind utilities; the geometry and colour *roles* are the part worth keeping.
- The DSH terminal/read/diff/search/spill model validation rules (`models/*-card-model.ts`) describe DSH result envelopes and persisted metadata; port them only if your tools emit the same formats.
