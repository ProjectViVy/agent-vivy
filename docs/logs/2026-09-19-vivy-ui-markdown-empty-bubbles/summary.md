# Vivy UI: empty assistant bubbles and markdown rendering

Date: 2026-09-19
Owner: Vivy kernel UI (`ui/`)
Trigger: a user screenshot of a live console session showed (1) stray empty
chat bubbles and (2) a markdown table rendered as raw `|` text.

## What changed

1. **Empty assistant bubbles are no longer rendered.**
   `internal/runtime/message_projector.go` projects one `assistant` message per
   `tool.requested` journal event with an empty `content`
   (`EventToolRequested` → `domain.Message{Role: assistant, ToolCallID, ToolName, ToolArgs}`).
   `MessageBubble` rendered that row as a full assistant card: an empty
   `w-fit` bubble plus the entire action bar (timestamp, copy, regenerate,
   rewind, fork). `MessageBubble` now returns `null` for a non-user message
   with blank content that is neither streaming nor carrying reasoning.
   User messages are exempt (attachment-only turns have empty text).

2. **Markdown is rendered as markdown.**
   - `remark-gfm` is wired into every chat-bubble `ReactMarkdown` through a
     new local `MarkdownBody` component, so GFM tables, strikethrough, task
     lists and autolinks parse instead of showing their source syntax. The two
     call sites (user and assistant bubbles) now share one body component.
   - `ui/src/styles.css` loads `@plugin "@tailwindcss/typography"`. Every
     `.prose` surface already asked for typography (`prose prose-sm
     dark:prose-invert`), but without the plugin the class was unknown and
     Tailwind emitted no rules: paragraph spacing, list markers, inline-code
     chips and table borders were all missing.
   - The plugin paints its own grey body colour, which turned the primary
     (user) bubble's white text grey. `.prose-inherit` in `ui/src/styles.css`
     re-points the prose colour variables at `currentColor` for that bubble,
     and `MarkdownBody` takes a `className` so only the user bubble opts in.
     A fenced code block on the coloured bubble gets a translucent white
     background instead of the plugin's dark one.

Dependencies added to `ui/package.json` (+ `pnpm-lock.yaml`): `remark-gfm@4.0.1`,
`@tailwindcss/typography@0.5.20`.

## Files

- `ui/src/components/chat/MessageBubble.tsx` — empty-message guard,
  `MarkdownBody`, `remarkPlugins`
- `ui/src/styles.css` — typography plugin
- `ui/package.json`, `ui/pnpm-lock.yaml` — new dependencies
- `ui/src/components/chat/MessageBubble.test.tsx` — new unit tests

## Explicitly not done

- **Tool-call detail (issue 3 of the same report) is out of scope by the
  user's instruction.** Tool result cards still show only `工具结果` plus the
  raw result envelope; the tool name and arguments are still not surfaced.
  Note for that work: the UI wire type `Message` (`ui/src/lib/api.ts`) does not
  yet carry `tool_call_id` / `tool_name` / `tool_args`, though the kernel
  projection produces them — surfacing them needs a wire field, not only a UI
  change.
- `ChannelTutorialModal` still renders markdown without `remark-gfm`. It shows
  short channel setup prose with no tables today; left alone to keep the change
  scoped.
- No change to the kernel projection: the empty assistant rows are legitimate
  Journal projection output (they carry the tool call), and the fix belongs in
  presentation.
