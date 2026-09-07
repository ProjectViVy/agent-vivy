# Acceptance — TUI-MD-STREAM-CACHE

How a human can tell the fix landed (VIVY CODE fullscreen TUI, long streaming
answer, e.g. "写一篇分多节、含代码块与列表的长文"):

1. **Streaming output looks unchanged where it matters**
   - While the answer streams, headings/paragraphs/code blocks render with
     the same glamour styling as before; the growing bubble updates per tick.
   - The cut strategy is invisible by design: blocks are glued with a single
     blank line, and no doubled margins or lost trailing paragraphs appear.
   - After the stream completes, the bubble re-renders once through the
     normal full-render path (the final appearance is the established one).
2. **Long streams stop re-rendering the whole document per tick**
   - Observable as reduced per-tick latency on very long answers (the heavy
     glamour pass only covers the trailing segment once a safe blank-line
     boundary exists).
   - Content with hazards (an unclosed fenced code block, a table, a loose
     list continuation) still streams correctly — the cache simply stays
     unused and the full render runs, exactly as before.
3. **No markdown constructs are split mid-stream**
   - A fenced code block opened mid-answer is never cut into two rendered
     halves; syntax highlighting of the open fence stays coherent until the
     fence closes.

Unit-level proof is in `summary.md`; the render contract (same visible
content, same fallback on render errors) is pinned by
`sdk/tui/view/streaming_markdown_test.go` plus the existing
`markdown_test.go`/`view_test.go` render suites.
