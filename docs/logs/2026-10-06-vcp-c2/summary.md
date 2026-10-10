# VCP C2 — TUI session tree and portability commands

Story: `docs/superpowers/plans/vivy-code-parity/C2-tui-session-cmds.md`

## What landed

- **Command catalog** (`sdk/tui/command`): seven new specs — `/tree`, `/clone [title]`, `/import <path>`, `/export`, `/copy`, `/bug`, `/debug` — with en/zh descriptions, arg validation, and cross-face contract entries.
- **`/tree`** (`sdk/tui/view/tree.go` + `render.go`): navigable overlay over C1's `session/tree` read model. `flattenTree` lays nodes out depth-first under `parent_session_id`/`fork` edges (roots = parentless-or-missing-parent nodes, children sorted by `created_at`, cycle-guarded, orphans appended flat). Arrows/`ctrl+p/n` move, Enter calls `SelectSession`, Esc closes. The face only renders the kernel graph — no second graph builder.
- **`/clone`** → `session/clone` RPC, then the fork-style post-switch in `applyCommandResult` (unmarshal `session_id`, append to the list, `loadSessionCmd`). Same pattern covers **`/import`**, which reads the local JSONL file client-side and posts it to `session/import`.
- **`/export`** → `session/export` (html); the overlay shows the kernel path via a new `vivy.tui.result.exported` formatter instead of raw JSON. **`/import`** shows `imported N (M skipped) → session`.
- **`/copy`**: last non-empty assistant message → OSC 52 clipboard write to stdout; terminals ignoring the sequence drop it harmlessly. Empty history → `copyEmpty` error.
- **`/debug`** → `diagnostics/logs` (runtime, limit 80) rendered as `HH:MM:SS LEVEL component message` lines via a `debug` case in `FormatResultWithTranslator`.
- **`/bug`** → new **`diagnostics/bundle`** RPC (`internal/rpc/diagnostics.go`): writes a markdown report — buildinfo.Version, session_id, generated_at, bounded redacted runtime-log tail (200 records) — into the new `DiagnosticsBundleDir` dep wired to `<dataRoot>/exports`. Advertised as `diagnostics.bundle` capability only when the dir is set.
- **Surface contract**: `SessionTreeProvider` (+ `TreeNode`/`TreeEdge`/`TreeMsg`) added to `surface.Driver`; `Live.SessionTree()` fetches and unmarshals the RPC result.

## Boundaries held

`/share` excluded (O4). Tree navigator consumes C1's read model only. `/import` never merges — always a new session. `/bug` output is a local file, no upload.
