# DSH: markdown and message furniture

Reference draft for the DSH web client. Citations are relative to the DSH source root
`.workspace/deepseek-harness/deepseek-harness` (rev `0d1f50007f9bca3f52b06e1c3074fa14d5fb0720`).
Read-only research; nothing in that tree was modified.

## Pipeline and plugins

There is **no remark/rehype pipeline**. `packages/client/ui-primitives/src/markdown/render.tsx:1-5` —
*"Direct mdast→React markdown renderer. Replaces the react-markdown / remark-rehype pipeline with one
switch over parsed nodes so streaming can cache frozen blocks as React elements"*. No `remark-*`,
`rehype-*`, `react-markdown`, `rehype-sanitize`, Prism or highlight.js entry exists in the manifest.

Two grammars, one per rendering arm (`.../markdown/parse.ts:26-31`, `:39-43`). The streaming arm is
`fromMarkdown(text, { extensions: [gfm(), cjkFriendlyStrong()], mdastExtensions: [gfmFromMarkdown()] })`;
the settled arm is `fromMarkdown(text, { extensions: [gfm(), cjkFriendlyStrong(), mathCompatibility(), math()], mdastExtensions: [gfmFromMarkdown(), mathFromMarkdown()] })`.
The first, `parseGfm`, carries "no math, so incomplete TeX never flashes KaTeX errors mid-stream"
(`parse.ts:20-22`); the second adds it (`parse.ts:33-38`).

- **GFM** (`gfm()` + `gfmFromMarkdown()`) supplies tables, strikethrough (`delete` → `<del>`,
  `render.tsx:290-291`), task lists (`render.tsx:414-416`, `:432-440`), autolink literals (a fixture
  token has "no scheme, so GFM does not autolink it", `apps/web/tests/markdown-wide-table.e2e.ts:76`)
  and footnotes (`render.tsx:348-349`, `:624-684`).
- **`cjkFriendlyStrong()`** is a local micromark text extension on the asterisk
  (`.../markdown/cjkFriendlyStrong.ts:66-74`, `:81-83`).
- **`mathCompatibility()`** adds `\(...\)`, `\[...\]` and same-line `$$...$$` onto
  `micromark-extension-math`'s vocabulary (`.../markdown/mathCompatibility.ts:318-338`, `:347-349`).
- Raw HTML never reaches the DOM: "No HTML parser enters the pipeline: raw HTML stays literal text"
  (`render.tsx:324-326`); unmapped merge-extensible node types "render nothing" (`render.tsx:355-359`).

Dependencies (`packages/client/ui-primitives/package.json` `devDependencies` — browser-only
third-party implementations belong there, `packages/client/AGENTS.md` rule 5):
`mdast-util-from-markdown ^2.0.3`, `mdast-util-gfm ^3.1.0`, `mdast-util-math ^3.0.0`,
`micromark-extension-gfm ^3.0.0`, `micromark-extension-math ^3.1.0`,
`micromark-core-commonmark ^2.0.3`, `micromark-util-sanitize-uri ^2.0.1`,
`micromark-util-character ^2.1.1`, `micromark-util-classify-character ^2.0.1`,
`micromark-util-symbol ^2.0.1`, `micromark-util-types ^2.0.2`, `micromark-factory-space ^2.0.1`,
`katex ^0.16.47`, `shiki ^4.3.1`, `@shikijs/langs ^4.3.1`, `clsx ^2.0.0`, `@types/mdast ^4.0.4`.
Public exports: `MarkdownText`, `CodeBlock`, `JsonBlock`, `extractMarkdownPlainText`
(`packages/client/ui-primitives/src/index.ts:66-72`).

`packages/client/ui-renderer` is **not** in this pipeline: it mounts the assembled app and makes "the
sole context-level `renderSlot('root')` call" (`ui-renderer/src/client/app.tsx:19-21`,
`ui-renderer/README.md:32`); blocks and code come from `ui-primitives`.

## Tables

Column count decides the layout (`render.tsx:462-492`): `const wide = columns >= 4 && context.inBlockquote !== true`,
and the wrapper is `className={clsx(css.tableScroll, wide ? 'md-table-wide' : css.tableFill)}` with
`tabIndex={wide ? 0 : undefined}`. The wide block "keeps the table at natural width and exposes the stable `md-table-wide` hook so a
hosting layout (the chat transcript) can widen it past the message column" (`render.tsx:466-471`); the
explicit `tabIndex` keeps it keyboard-reachable because resting `overflow-x: hidden` drops Chromium's
implicit scroller focusability (`render.tsx:473-476`). Cell alignment is an inline style mirroring
hast (`render.tsx:508-514`).

`MarkdownText.module.css`:

- `.tableScroll { max-width: 100%; overflow-x: auto; overscroll-behavior-x: contain; }` (`:190-194`)
- `.tableScroll:global(.md-table-wide) { overflow-x: hidden; padding-bottom: var(--dsh-scrollbar-width, 8px); }` (`:206-209`)
- reveal on hover/focus: `overflow-x: scroll; padding-bottom: 0;` (`:211-215`) — the toggle is
  `overflow-x` itself because "Chromium never repaints state-conditioned scrollbar STYLES" (`:196-205`)
- natural width `.tableScroll table { border-collapse: collapse; width: max-content; max-width: max-content; }` (`:227-231`)
- fill `.tableFill table { width: 100%; max-width: none; }` (`:237-240`)
- `th`/`td` `padding: 10px 16px; max-width: min(30vw, 320px); min-width: 100px` (`:242-260`), first cell
  `padding-left: 0`, last `padding-right: 0` (`:262-269`)

Breakout lives in the host (`ui-chat/.../AssistantMarkdown.module.css:33-43`):
`--dsh-table-spare: max(0px, calc((100cqw - var(--dsh-chat-content-width)) / 2))`,
`width: calc(100% + var(--dsh-table-lead) + var(--dsh-table-spare)); max-width: none;` plus matching
negative `margin-left` and `padding-left`; the container query comes from
`container-type: inline-size` (`ui-chat/.../ChatView.module.css:20`). E2E pins fill/long-cell
`overflow <= 1`, wide `overflow > 1`, `wideHook` true only at ≥4 columns, breakout only above 748px,
and hover `'hidden 8px'` → `'scroll 0px'` (`apps/web/tests/markdown-wide-table.e2e.ts:330-351`,
`:394-401`, keyboard scroll `:362-375`).

## Code

**Inline code** (`render.tsx:292-323`): line endings become spaces; a token that is *exactly* an
absolute HTTP(S) URL keeps code chrome and gains a safe anchor (`:295-301`, `:554-563`); a
file-mention token becomes a `<button>` (`:305-321`); everything else stays inert `<code>`. Styling is
`:not(pre) > code` with `font-size: 0.875em !important` in a 6px-radius chip
(`MarkdownText.module.css:161-172`).

**Fenced blocks** (`renderCode`, `render.tsx:363-399`): an empty fence keeps a stock
`<pre><code class="language-…">` for parity (`:365-372`); the info string truncates at the first
non-word char, `const lang = /^[\w-]+/.exec(language)?.[0]` (`:373-375`); a settled ```` ```math ````
fence renders as display TeX (`:376-380`); otherwise
`<CodeBlock code={`${node.value}\n`} lang={lang} streaming={context.streaming} … />` (`:381-398`).

`CodeBlock` (`.../markdown/CodeBlock.tsx`): header banner with info string plus copy button
(`:187-196`, stable hooks `data-code-block-banner` / `data-code-block-content`); copy reads the
rendered `<pre>` text and flips to the copied label for 1000 ms (`:151-163`); `lineNumbers` is opt-in,
default false (`:31`, `:67`), drawn as a CSS-counter gutter (`CodeBlock.module.css:107-130`); the body
wraps by default — `white-space: pre-wrap; word-break: break-all;` (`CodeBlock.module.css:78-90`).

**Highlighting** is shiki, not Prism (`.../markdown/highlight.ts:1-19`): a synchronous core on the
JavaScript regex engine ("no oniguruma WASM") with a CSS-variables theme whose colors live in the
theme package's `--shiki-*` sheets (`:144-149`). Boot grammars are TypeScript, shell and JSON
(`LANGS = [langTs, langBash, langJson]`, `:42`); a 24-entry `LAZY_GRAMMARS` map dynamic-imports
python, rust, yaml, markdown, … on first use (`:53-77`), with an alias table covering fence aliases
and file-extension hints (`:90-133`). Unknown or not-yet-loaded languages fall back to plain text,
never an error (`:17-18`, `:272-277`); `useViewportHighlighting` defers work until the block
intersects the viewport (`useViewportHighlighting.ts:55-72`).

## Links, images, paths, CJK

**Links.** The allowlist is a protocol switch, not a library: http/https/mailto pass, everything else
— including relative and unparsable destinations — returns `''` and the anchor collapses to its
children (`render.tsx:45-60`, `:529-543`). External links get
`target="_blank" rel="noopener noreferrer"` (`:537`) and link text is prefixed with a category
`LinkIcon` (`:539`, `src/LinkIcon.tsx:22`, `:35-54`). Fragment anchors fail the allowlist, so footnotes
render as bare `<sup>` numbers with plain-text back-reference markers (`render.tsx:633-635`,
`:645-684`) — in-page footnote links are **not** implemented. E2E pins `href`/`target`/`rel` and that
`curl <url>` and `javascript:` stay inert (`apps/web/tests/markdown-inline-code-links.e2e.ts:120-136`).

**Images.** Absolute HTTP(S) only (`render.tsx:62-70`). A local destination renders only when the owner
supplies a `MarkdownPathImages` vocabulary (`:147-155`); chat maps absolute POSIX paths to
`` `${origin}/api/file?path=${encodeURIComponent(value)}` `` (`ui-chat/.../AssistantMarkdown.tsx:22-26`,
`:56-59`). Without a resolvable source the image degrades to an italic alt-text `<span>`
(`render.tsx:565-571`); a failed load also falls back to `alt || destination` (`:574-588`). Attributes
and CSS: `loading="lazy" decoding="async" referrerPolicy="no-referrer"`,
`max-width: 100%; border-radius: 8px; object-fit: contain` (`:578-586`, `MarkdownText.module.css:290-299`).
E2E pins that attribute set plus alt fallbacks for missing/oversized/outside-workspace files
(`apps/web/tests/markdown-images.e2e.ts:227-266`). Zoom/lightbox is **not** in the markdown renderer;
the original-size lightbox belongs to the attachment gallery
(`apps/web/tests/image-display.expected.e2e.ts:69-78`).

**File paths.** An inline-code token becomes a file affordance only through an owner resolver — "the
renderer never guesses at what looks like a path" (`render.tsx:157-170`) — rendered as
`<code><button title={mention.title} aria-label={mention.label}>` with a leading icon (`:305-321`).
Mentions are suppressed inside an anchor (`:305`, `:188`) and are settled-render only.

**CJK.** `cjkFriendlyStrong` lets asterisk strong close after punctuation when CJK prose continues
without whitespace: `cjkStrongClose = markerCount >= 2 && unicodePunctuation(previous) && isCjkCharacter(code)`
(`.../markdown/cjkFriendlyStrong.ts:55-58`) over Han, Hiragana, Katakana, Hangul, Bopomofo (`:9-15`).
Eight `**…**`-adjacent cases are pinned in `apps/web/tests/markdown-cjk-strong.e2e.ts:25-34`.

**Sanitization** is structural: no HTML parser, protocol allowlist, `normalizeUri` on parsed destinations, and "KaTeX runs without trusted commands" (`render.tsx:7-12`, `:547`).

## Math

KaTeX `^0.16.47`, imported statically — **not lazy**: `import katex from 'katex'`
(`.../markdown/katex.tsx:20`), plus `import 'katex/dist/katex.min.css'` (`MarkdownText.tsx:23`). The
three-arm failure chain replicating rehype-katex (`katex.tsx:66-89`): strict render with
`throwOnError: true`, then `strict: 'ignore', throwOnError: false`, then a manual
`<span className="katex-error" style={{ color: '#cc0000' }} title={String(error)}>` (`:74-86`).
Output is an HTML string parsed by the browser's own `DOMParser` and mapped onto React elements (`:88-89`) because React 18 has no MathML support (`:11-15`). Display math scrolls:
`.markdown :global(.katex-display) { max-width: 100%; overflow-x: auto; overflow-y: hidden; }`
(`MarkdownText.module.css:179-183`). While streaming TeX stays literal so partial formulae cannot
flash errors (`render.tsx:376-380`, `MarkdownText.tsx:151-155`). E2E asserts 6 `.katex`, 2
`.katex-display`, 0 `.katex-error` for a fixture using `$…$`, `\(…\)`, `\[…\]`, `$$…$$`
(`apps/web/tests/math-rendering.e2e.ts:54-58`, `:122-124`).

## Streaming stability

All but the trailing two blocks freeze: `const UNSTABLE_TAIL_BLOCKS = 2` ("the second-to-last is
retained as safety margin", `.../markdown/incremental.ts:33-38`), and only the tail slice is
re-parsed (`:336-358`). Render keys are absolute source offsets, so a block crossing the freeze
boundary reconciles instead of remounting (`:40-50`, `:70-73`). An unclosed top-level fence gets a
second frontier — only the last completed line plus the current partial line re-enter the grammar
(`:98-118`, `:195-301`). `update(text)` memoizes by exact text and bumps `generation` (clearing
caches) on non-append input (`:305-324`).

`MarkdownText` is `memo`'d and keeps a `StreamingRenderer` in a ref that returns the cached array when
`text === this.lastText` (`MarkdownText.tsx:85-86`, `:167-187`); frozen elements and the footnote/
reference state they consumed are carried forward, with a copy handed to the tail each frame
(`:96-141`). `labels`, `fileMentions` and `pathImages` must be reference-stable — "a new identity
discards the streaming render cache mid-message" — and the vocabularies apply to "settled renders
only" (`:150-162`); chat passes a `useMemo`'d label object and image vocabulary
(`ui-chat/.../AssistantMarkdown.tsx:52`, `:56-59`).

Fence highlighting is incremental too: `StreamingHighlightSession` resumes from saved shiki grammar
state and reports only newly completed lines plus a mutable tail (`highlight.ts:340-444`), and
`CodeBlock` seals completed lines into fixed 32-line React groups so later chunks reconcile only the
growing group (`CodeBlock.tsx:52-53`, `:125-144`); an unchanged fence keeps that DOM across settlement (`:99-109`).

Documented deviation: a reference-style link or footnote whose definition sits on the other side of
the freeze boundary renders literally until the settled full parse self-heals it
(`incremental.ts:25-28`, `MarkdownText.tsx:8-11`, `ui-primitives/README.md` Known Limitations).
## Message furniture

Content and furniture are separate components. `AssistantMarkdown` renders only the block sequence —
`div.root > div.body` (16px gap) plus the interrupted marker
`<span className={css.stopped}>{t('message.stopped')}</span>`
(`ui-chat/.../AssistantMarkdown.tsx:134-141`); footer spacing is a host stylesheet rule
(`AssistantMarkdown.module.css:9-21`, `:52-70`: `.actions { margin-top: 16px; margin-left: -6px; }`).

Per-message actions sit in the **turn tail**, not inside the markdown (`ui-chat/.../TurnTailNodeView.tsx:37-67`): a `div` carrying `data-turn-tail={data.turn}` and `data-actions-reveal={isLatestTurn ? 'always' : 'hover'}` wraps the turn-tail slot and then
`<MessageIconActions text={assistantText(closing.blocks)} time={closing.time} clock="end" onBranch={() => { forkAt(closing.finalNode.seq) }} branchUnavailable={data.branchUnavailable || hasLaterChatNode} extraActions={assistantActions} usageAction={…} t={t} />`.

Slot order inside `MessageIconActions` is fixed (`MessageIconActions.tsx:82-112`): clock (start side
only) → **copy** → `extraActions` (plugin slot) → **branch** → `usageAction` → clock (end side).

| Affordance | Where | Labels |
|---|---|---|
| copy | `MessageIconActions.tsx:85-89` | `t('copy')` / `t('copied')` (`packages/client/locale/src/locales/en.ts:8-9`), check-glyph swap for 1000 ms (`:71-74`) |
| branch / fork | `MessageIconActions.tsx:91-109`, `onBranch` → `forkAt` (`apply.ts:158-164`) | `message.branch`, `message.branchUnavailable` (`ui-chat/src/client/locale.ts:182-183`); unavailable uses `aria-disabled` + a visually-hidden reason because "Native disabled buttons do not deliver the hover/focus events Tooltip needs" (`:93-109`) |
| like / dislike | `ui-message-feedback`, the `feedback` entry of `conversation.chat.assistant-actions` (`ui-message-feedback/README.md:42`, `src/client/index.ts:76-77`), rendered between copy and branch | `action.like` / `action.dislike` (`ui-message-feedback/src/client/locales.ts:39-42`) |
| timestamp | same row, `clock="start"` for user messages (`MessageItem.tsx:327-335`), `clock="end"` for assistant | `formatMessageClock` yields `HH:mm`, a date template, or `clock.ymd` (`chat/message-chrome.ts:79-103`) |
| per-turn usage + turn-time pills | `usageAction` (`TurnTailNodeView.tsx:52-64`, `chat/TurnUsagePanel.tsx:41-70`) | `message.turnUsage.*` (`ui-chat/src/client/locale.ts:197-206`) |

Assistant actions render only when the closing node carries a `messageId`: "Interruption-frozen
partials carry no messageId, so they address no durable message and contribute no per-message actions"
(`TurnTailNodeView.tsx:31-36`). User bubbles get the same row under the bubble (`MessageItem.tsx:226`,
`:320-338`). **No edit or regenerate affordance exists**: grep for `regenerate`/`editMessage` across
`packages/client` returns nothing.

`StatsPills` is **session-level, under the composer** — registered on `conversation.composer.dock`
(`ui-chat/src/client/apply.ts:171-174`) — with two pills: a gauge pill (turns, steps, tokens/s)
opening the session-statistics dialog and a database pill (total tokens, cache hit) opening the
token-usage dialog (`StatsPills.tsx:138-233`, `:236-315`). Values ride the `sessionStats` /
`tokenUsage` projections with a window-scoped fold fallback (`:317-332`); the row is hidden when no
steps and no billed tokens exist (`:332`), and `data-composer-stats` lets the composer tighten its
clearance (`:333-336`).

## Implementer notes

**Portable, keep:** the block-freeze scheme with source-offset keys (a per-chunk full re-parse is
quadratic in reply length, `incremental.ts:1-14`); the two-grammar split (streaming math is what
flashes KaTeX errors); protocol allowlists plus "no HTML parser" as the whole sanitization story
(dropping relative links and fragment anchors is intended); a scroll wrapper *and* a ≥4-column wide
variant for tables, where `md-table-wide` is a cross-package contract between `ui-primitives` and the
chat layout; lazy grammar loading with an external-store load counter so a plain-text fence upgrades
when its grammar lands (`highlight.ts:197-260`); memoized label/vocabulary props, without which
streaming caching is silently defeated.

**DSH-internal, do not copy blindly:** `--dsw-*`/`--dsh-*` tokens, CSS-Module class hashes,
`md-code-block`/`md-table-wide`/`data-code-block-*` hooks, and the container-query breakout depend on
the DSH theme and chat column (`AssistantMarkdown.module.css:33-43`, `ChatView.module.css:20`); the
slot system and the Cordis-free label-prop discipline (zero `ctx`, all copy through `t`) are DSH
architecture (`ui-primitives/README.md`, `packages/client/AGENTS.md`); the DOM-parity ceremony
(`wrapBlockChildren` newline interleaving, `/^[\w-]+/` language truncation, the synthetic trailing
newline into `CodeBlock`) only matches a replaced hast pipeline and can be dropped in a greenfield app.

**New dependencies a plain React + Tailwind app would need:** `mdast-util-from-markdown` +
`micromark-extension-gfm` + `mdast-util-gfm` (or the remark route DSH abandoned), plus
`micromark-extension-math` + `mdast-util-math` + `katex`, `shiki` + `@shikijs/langs`,
`micromark-util-sanitize-uri`, `clsx`. The two local micromark extensions (`cjkFriendlyStrong`,
`mathCompatibility`) would have to be ported or dropped — the CJK workaround is what makes
`**注意：**内容` render as bold, and no published plugin for it was found.
