# VC-1f — UI diff rendering (unified/split + statistics + Review Center)

Date: 2026-08-31 · Branch: `feat/vc1a-bash-tool` (worktree `agent-vivy-vc0`)

## What changed

The server half (go-udiff, standard unified diff) and UI half (a custom renderer) are both part of this delivery:

- **Server**: `boundedDiff` in `internal/runtime/filesystem_backend.go` now uses
  `go-udiff` (`github.com/aymanbagabas/go-udiff`, MIT, item 2 in the self-developed inventory) to output
  standard unified diff (3 context lines, `--- a/…`/`+++ b/…` headers, `@@ -l,c +l,c @@` hunks),
  retaining the 32 KiB `maxDiffBytes` truncation cap (appending a `[diff truncated]` marker line after truncation).
  Five call sites (WriteFile / PatchFile / MultiPatchFile / PrepareWriteFile /
  PreparePatchFile / PrepareMultiPatchFile) were upgraded automatically; the old single-hunk pseudo-diff
  generator was deleted. go-udiff was promoted from an indirect to a direct dependency in `go.mod`.
  Two Go test diff assertions were updated in sync (real unified-diff context lines carry a ` ` prefix).
- **UI parser**: `ui/src/lib/diff.ts` — lenient unified-diff parsing: it does not trust the line counts
  declared by hunk headers (truncation can cut off a trailing hunk), and advances line numbers by line type only;
  it tolerates pseudo-diff (bare `@@`, context without a prefix), `\ No newline`, and `[diff truncated]` markers;
  it returns null when text lacks file headers or a hunk (the caller falls back to plain text). It also adds
  `diffSplitRows` (pair deletion/addition blocks by index for split rendering) and `parseToolResultDiff` (extract diff/path
  from FileMutationResult JSON).
- **UI component**: `ui/src/components/ui/DiffView.tsx` — presentation aligned with Crush
  (D10 decision, behavioral alignment under FSL-1.1-MIT, zero code copying, custom renderer):
  `+N −M` statistics, unified/split mode toggle, line numbers + add/delete coloring (emerald/rose),
  hunk header lines, truncation notice, and a `max-h-96` scrolling container; on parse failure it falls back to plain-text `<pre>`.
- **Integration point 1 (Review Center)**: `ApprovalsView.tsx` approval-detail `preview` uses DiffView when
  `looksLikeDiff()` identifies a diff, otherwise retaining the original plain-text `<pre>`
  (non-diff approvals such as bash are unaffected). This also closes the FACE-TUI-2 "approval diff highlighting" item.
- **Integration point 2 (chat bubble)**: the `MessageBubble.tsx` tool-result branch renders
  "tool result + path + DiffView + collapsed raw JSON" when content is FileMutationResult JSON with a non-empty `diff` field;
  other tool results (bash/grep/read, etc.) remain unchanged.
- **i18n**: added `diff.{unified,split,truncated,statsLabel}` and
  `chat.toolResultRaw`, synchronized zh/en (covered by dictionary-parity tests).
- **Tests**: `ui/src/lib/diff.test.ts` (parsing/pairing/truncation/tool-result extraction, 12 cases),
  `ui/src/components/ui/DiffView.test.tsx` (2 SSR-render smoke cases).

## Explicitly not done

- LSP/diagnostic diff coloring and syntax highlighting (Crush uses tree-sitter highlighting; this is reserved for a later decision).
- Stale-read protection for `filetracker`/`file_versions` (RB-1, pending O1..O6 decisions; untouched).
- Old pseudo-diffs in historical data (bare hunk variants without `--- a/` headers) can also be rendered by the lenient parser,
  but no migration was performed—old messages remain readable as-is.
- MessageBubble editing / reverting / forking remain placeholders (outside this card).
