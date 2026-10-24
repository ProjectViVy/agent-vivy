# C1 — Session tree / clone / import / export (storage + RPC)

**Goal:** `session/tree`, `session/clone`, `session/import`, `session/export` over existing Journal/truncation infrastructure.
**Epic:** C. **Requirements:** RQ-SESS.
**Spec:** [VCP-D1](../../specs/2026-10-06-vivy-code-parity-design.md) §5.4. **Baseline:** `f34f3ce`.

## Scope

**Files:** `internal/storage` (tree read model — both backends + conformance), `internal/runtime` (clone/import/export services), `internal/rpc` (4 methods), `domain` types. Migration only if fork-link data is insufficient (fork links already land via `session_truncations.fork_session_id` — verify first; if a `forked_from` column on sessions is cleaner, take the next free migration number at execution time).

## Tasks

- [ ] Verify fork provenance is fully derivable from `session_truncations` + sessions table (check `CommitSessionFork` writes). If not, add migration.
- [ ] `SessionTree(ctx)` → nodes (id, title, created_at, last_active, parent fork point message id) + edges (from, to, kind=fork). Bounded: cap node count, stable ordering.
- [ ] `CloneSession(ctx, sessionID)` → new session id; copies the **visible view** (post-fold messages) into a fresh session; provenance event `session.cloned_from`.
- [ ] `ImportSession(ctx, jsonl)` → parse pi-style JSONL entries (role/content/tool_call/tool_result shapes), map to message rows, unknown kinds counted in result.skipped; reject malformed first line.
- [ ] `ExportSession(ctx, sessionID, format)` → standalone HTML string/file (inline styles, no external refs, messages + tool calls + usage footer); written to instance `exports/` dir; RPC returns path.
- [ ] RPC wiring + param validation + bounds (export size cap, import line cap).
- [ ] Tests: tree of A→fork→rewind→fork renders expected edges; clone preserves visible view and not hidden markers; import round-trips a pi JSONL fixture (write a small fixture under testdata); export contains message bodies and no external network refs.
- [ ] `go test ./internal/storage ./internal/runtime ./internal/rpc -run 'Tree|Clone|Import|Export'`; `just ci`.
- [ ] Commit `feat(session): tree read model, clone, JSONL import, HTML export`.

## Boundary

No UI (C2/C3). No `/share` upload (O4). Import is a *new* session — never merges into an existing one. Export HTML is trusted-readonly (escape content; CSP `default-src 'none'`).

## Acceptance

The four methods round-trip a real session end-to-end; conformance suite covers tree read on both backends; migration (if any) is dual-dialect.
