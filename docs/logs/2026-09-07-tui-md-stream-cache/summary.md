# TUI-MD-STREAM-CACHE — 流式 Markdown 的 stable-prefix 增量渲染

## What changed

Ports crush's `internal/ui/chat/streaming_markdown.go` stable-prefix strategy
(board row `TUI-MD-STREAM-CACHE`; read-only reference
`.workspace/crush/internal/ui/chat/streaming_markdown.go`) into the shared
Vivy renderer:

- **New `sdk/tui/view/streaming_markdown.go`**
  - `streamMarkdownRender(id, source, width, quiet)` renders a growing
    streaming bubble by reusing the glamour render of a cached "stable
    prefix"; each flush only re-renders the trailing delta.
  - The stable/trailing cut sits immediately after a blank line where no
    markdown construct can be open. Boundary validation rejects: odd fence
    parity (open fenced code), HTML block openers, link reference
    definitions, loose-list continuation paragraphs, setext underlines on
    either side of the cut, and any last line that opens a construct (table
    pipe, block quote, list marker, indented code). Any doubt falls back to
    a full render and leaves the cache untouched.
  - Entries are keyed by (message id, quiet), bounded at 8 with wholesale
    eviction, reset on width change or non-prefix rewrite. Cumulative
    fence/list state makes candidate validation O(delta) per flush.
  - Two concatenated glamour renders are not byte-identical to one full
    render (glue joins fragments with a single blank line after trimming
    margins) — accepted during streaming, exactly as in crush; the final
    non-streaming render is the cached full render via `messageMarkdownCache`.
- **`sdk/tui/view/markdown.go`**
  - `renderMarkdown` split into the lock-holding wrapper and
    `renderMarkdownLocked`; the streaming path holds `mdRenderMu` across the
    whole prefix+trailing sequence so no other render interleaves with the
    shared goldmark state.
- **`sdk/tui/view/render.go`**
  - `renderMessageBody` now takes the message; `message.Streaming` routes
    through `streamMarkdownRender` while completed messages keep the exact
    previous full-render path.

## Tests

- `sdk/tui/view/streaming_markdown_test.go`
  - boundary promotion across flushes + boundaryless flush keeps the cache;
  - open fence never cut (no prefix seeded), closed fence boundary resumes;
  - width change and mid-stream rewrite reset/reseed the entry;
  - hazard table for `findSafeMarkdownBoundary` (open fence, setext, loose
    list continuation, link ref, HTML block, table row);
  - a `Streaming: true` message renders through the cache with markdown
    content intact.

## Explicitly not done

- The paused-offset message anchor, per-history render caching, and the
  loading epoch remain open under `TUI-VIEWPORT-N1-OPEN`.
- Tool-card markdown/chroma/diff routing remains open under
  `TUI-MD-TOOL-RESULTS`.
- No `git push` was performed.
