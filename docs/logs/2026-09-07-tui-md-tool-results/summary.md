# TUI-MD-TOOL-RESULTS — 工具卡片正文分流 Markdown/chroma/diff

## What changed

Ports crush's tool-result content routing (board row `TUI-MD-TOOL-RESULTS`;
read-only reference
`.workspace/crush/internal/ui/chat/tool_result_content.go` + `generic.go`)
into `sdk/tui/view/render.go`:

- **New classifier `toolResultContent`** picks one of four kinds for a tool
  card body:
  1. `toolResultJSON` — body starts with `{`/`[` and re-indents cleanly via
     `json.Indent` (layout-only, byte preserving: oversized integers are not
     float64-round-tripped).
  2. `toolResultDiff` — `isUnifiedDiffContent`, a line-prefix port of crush's
     `diffdetect` (git header + file header, or a `@@` hunk + file header)
     plus a Vivy-compat rule that keeps hunk-only preview bodies routing to
     the diff path. The line-prefix rule avoids the previous substring check
     misclassifying markdown lists (`- item`) as diff deletions.
  3. `toolResultMarkdown` — the crush sniff patterns (`# `, `**`, ` ``` `,
     `- `, `1. `, `> `, `---`, …).
  4. `toolResultPlain` — previous `wrapText` behaviour.
- **New `renderToolBodyLines`** renders by kind:
  - JSON/markdown become fenced code blocks (```` ```json ````/```` ```markdown
    ````) routed through the existing `renderMarkdown` pipeline with
    `quiet=false`, because the normal markdown style carries the chroma
    config while the quiet style does not — chroma highlighting comes for
    free through the shared renderer instead of crush's separate
    `SyntaxHighlight` machinery. Rendered lines are truncated to
    `contentWidth` with `ansi.Truncate` rather than re-wrapped (chroma
    output must not be re-flowed).
  - Diff goes through the existing `renderDiffBody` colouring after
    `wrapText`.
  - Plain keeps `wrapText`.
- `renderToolWithOptions` routes its body block through `renderToolBodyLines`.
  The 8-line `compactToolLines` cap and `ctrl+o` expansion behaviour are
  unchanged; `renderDiffBody` itself is unchanged (its guard is still relied
  on by direct test callers).

## Tests

- `sdk/tui/view/tool_results_test.go`
  - classification table (JSON object/array, invalid JSON, unified diff,
    git diff, hunk-only, markdown heading/list, plain output);
  - JSON re-indent is byte-preserving (20-digit integer survives) and
    indents nested objects;
  - `renderToolBodyLines` routing: plain has no chroma escapes, markdown
    body carries chroma 256-color foreground escapes and keeps content,
    diff body is diff-coloured rather than fenced;
  - full tool card with a markdown result renders through the code path and
    keeps its content.

## Explicitly not done

- No `git push` was performed.
- Crush's per-path output width split (body vs diff width) collapses into the
  single Vivy `contentWidth`; no behaviour regression is visible at current
  card widths.
