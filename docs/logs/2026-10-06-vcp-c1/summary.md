# VCP C1 — session tree, clone, JSONL import, HTML export

Story: `docs/superpowers/plans/vivy-code-parity/C1-session-tree-storage.md`

## What landed

- **`session/tree`** (`SessionTree` in `internal/runtime/session_copy.go`): bounded read model over the existing `session_truncations` fork provenance — nodes = newest 500 sessions (durable activity order), edges = `reason=fork` markers whose endpoints both resolve. Child nodes carry `parent_session_id` + `fork_point_message_id`.
- **`session/clone`** (`CloneSession` in `rewind_service.go`): the fork machinery extracted into `commitSessionFork`; clone pins the cutoff at the effective-view tail and journals `session.cloned_from` (new `EventSessionClonedFrom`) instead of `session.forked`. Empty sources clone cleanly via `commitEmptySessionCopy` (no fork-point anchors exist).
- **`session/import`** (`ImportSession`): parses pi's session JSONL (v3 `SessionHeader` first line + parentId-chained entries). `message` entries map onto Vivy row shapes — user text/images (image blocks become attachments), assistant text + one row per `toolCall` block (+i ms created_at bump keeps transcript order under `ORDER BY created_at, id`), `toolResult` → tool rows. Thinking blocks and all other entry types count into `result.skipped`; malformed first line → `ErrImportMalformed` → `InvalidParams`. Commits atomically through `CommitSessionFork` with a `session.imported` provenance event. Caps: 8 MiB body, 10k lines, 5k rows.
- **`session/export`** (`ExportSession` + `ExportDir` dep): renders the visible view to a standalone HTML file in `<dataRoot>/exports/` — inline styles, `default-src 'none'; style-src 'unsafe-inline'` CSP, all dynamic text escaped, footer with per-role counts and timestamp. Non-`html` formats rejected.
- **Storage**: `TruncationStore.ListSessionForkLinks` (all fork/forked-from markers, insertion order) implemented on sqlite + postgres, conformance-covered inside CN-21. No migration — fork provenance was already derivable from the 020/025 schema.
- **RPC-mode face** (`sdk/tui/face/rpc.go`): `clone` now calls the kernel `session/clone` (was the fork-at-tail shim); `export_html` → `session/export`; `get_tree` → `session/tree`; `get_entries` → `session/messages` projection; all four flipped to `available: true` in `get_commands`.

## Boundaries held

No UI changes (C2/C3). No `/share` upload (O4). Import always creates a new session, never merges. Export is trusted-readonly HTML.
