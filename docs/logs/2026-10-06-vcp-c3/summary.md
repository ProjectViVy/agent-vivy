# VCP C3 — GUI session tree page and byte-verified export

Story: `docs/superpowers/plans/vivy-code-parity/C3-gui-session.md`

## What landed

- **`exports/read` RPC** (`internal/rpc/exports.go`): the verified-read path for export downloads. `exports/read {name, expected_digest?}` reads a file under the wired `ControlDeps.ExportsDir` (`<dataRoot>/exports`), returning `{name, digest, size, data_base64}` up to 32 MiB. `validateExportFileName` enforces a bare basename (`[a-zA-Z0-9._-]`, no separators, `.`/`..`/non-clean rejected). A supplied `expected_digest` that mismatches the file's SHA-256 fails `CodeNotFound` ("export changed since the digest was bound"). Advertised as capability `exports.read` only when `ExportsDir` is set; wired in `internal/app/app.go`.
- **`ExportResult` digest fields** (`internal/runtime/session_copy.go`): `Name`, `SHA256` (hex over the written bytes), and `Size` now accompany `Path` and `MessageCount`, so a client can bind a verified read to the artifact it just produced.
- **New Module `vivy/session-tree`** (`plugins/coding/session-tree/`): UI-only module contributing one `std/ui-extension@v1` provider (`vivy.session-tree.page`) with zero grants, declaration `vivy-module.yaml` hash-pinned, en/zh `i18n/catalog.json`, and module tests asserting descriptor/declaration parity.
- **Session tree page** (`ui/session-tree/src/`): registers `defineUIRoute` `/session-tree` plus a `vivy` nav item (`workflow` icon, order 45). The page subscribes to `host.store` for the active session, loads `host.api.sessionTree()`, and renders `flattenTree` (same DFS semantics as the TUI: roots = parentless-or-missing-parent, children by `created_at`, cycle-guarded, orphans appended flat). Rows indent by depth, show a GitFork glyph on forks and a `(current)` badge; click selects, double-click calls `selectSession` + `router.navigate({to:'/'})`.
- **Toolbar actions**: Refresh, Clone (`session/clone` → select), Export HTML, Import (hidden file input). Export runs `exportSession` → `readExport(name, sha256)` → base64 decode → **browser-side `sha256Hex` verification** against both the export result and the read digest → Blob download. Any digest mismatch surfaces `verifyFailed` instead of downloading.
- **Face contract** (`sdk/ui/src/module.ts`): `FaceSessionTree`/`Node`/`Edge`, `FaceSessionCloneResult`, `FaceSessionImportResult`, `FaceSessionExportResult`, `FaceExportReadResult`; `FaceClientAPI` gained `sessionTree`, `cloneSession`, `importSession`, `exportSession`, `readExport`; `FaceStoreState` gained `selectSession` (mirrors the real store). `ui/src/lib/api.ts` maps the five RPC verbs.
- **Wiring**: root `go.mod` require+replace, `repoSourceDirs`, `generate-default` externals, `recipes/default.vivy.yml` (`modules`, `order.std/ui-extension@v1`, `ui.extensions`), regenerated `zz_default.go`, staged `ui/src/generated`, `vitest.config.ts` include for the module's tests, `default-generation.expected.json` gains `vivy/session-tree`.

## Boundaries held

The export file is served only through the verified transfer path (`exports/read` + digest binding + client-side SHA-256 check); no HTML is inlined into the page and no filesystem path is trusted. `/share` remains deferred (O4). The page consumes C1's read model only — `flattenTree` is a layout function, not a second graph builder.
