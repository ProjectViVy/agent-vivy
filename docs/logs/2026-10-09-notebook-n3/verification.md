# N3 verification

## Unit/component (vitest, plugin sources)

`cd ui && pnpm exec vitest run '../plugins/vivy-notebook/ui/vivy-notebook/'`

- `api.test.ts` 6/6 — action id + `{operation_key, request}` envelope, caller
  key preserved, `revision_conflict` decodes `current_version`/
  `current_revision_id`, serialized calls contain no
  scope/actor/origin/generated/provenance/source_id fields, replayed receipt
  passthrough, malformed outcome → `outcome_unknown`.
- `editor.test.tsx` 4/4 — `PreservesDraftOnConflict` (stale CAS keeps draft +
  banner + reload/view-current affordances), lost-ack retry reuses one
  operation key and produces one revision, `onDirty(true)` propagation,
  pending → saved transitions.
- `view.test.tsx` 5/5 — `capability_unavailable` renders the unavailable
  view; seeded sections + entry list + open → draft body; create-section
  sends `operation_key: nb-*` + `request.title`; empty section state; dirty
  navigation → discard confirm → leave.

Full suite: `pnpm test` → **613/613** (80 files). `pnpm typecheck` clean.

## Real split-pair e2e (Playwright, packed binaries, disposable state)

Setup (all real, recorded in `ui/playwright.notebook.config.ts` +
`ui/e2e/notebook-backend.ts`):

```
go run ./sdk pack --recipe recipes/default.vivy.yml     --output .workspace/notebook-e2e/default
go run ./sdk pack --recipe recipes/no-notebook.vivy.yml --output .workspace/notebook-e2e/no-notebook
.workspace/notebook-e2e/default/vivy   (VIVY_CONFIG=ui/.e2e-notebook/config.yaml, addr 127.0.0.1:8797, sqlite data_dir, governance full_auto)
cd ui && pnpm dev                      (127.0.0.1:3015, proxies /rpc → :8797)
cd ui && npx playwright test --config playwright.notebook.config.ts
```

Result: **5/5 passed**.

1. Seeded role sections (`section-notes|daily|weekly|monthly`) render on an
   explicit read; no report-generation controls exist anywhere.
2. Full lifecycle in one session: create section → create entry → title +
   markdown save (`已保存`/`saved`) → comment create + resolve + status tab →
   2 immutable revisions + readonly view → move to `section-notes` → delete →
   hidden until show-deleted → restore → export downloads `*.md` +
   `*.sidecar.json` whose markdown matches the saved body and whose provenance
   records `origin: human`.
3. Real backend restart (SIGTERM → respawn same config): section + entry +
   draft body all survive.
4. Two-tab stale CAS: peer tab saves first, our tab's save surfaces the
   conflict banner, keeps the local draft intact, and the
   view-current preview shows the peer body.
5. Chat exclusion + omission: seeded sections readable via
   `module.action.invoke` `vivy.notebook.sections.list`, chat DOM contains no
   notebook ids/content; packed `no-notebook` binary answers `-32601` and the
   route renders `notebook-unavailable`.

Screenshots (in this folder): `n3-editor-saved.png`, `n3-conflict.png`,
`n3-unavailable.png`.

## Backend gate

`go test ./internal/modules/notebook/` green after the comment-status schema
fix (`["open","resolved"]` → `["active","resolved","deleted"]`, matching the
storage layer's written values); `gofmt` clean.

## Whole-repo gate

`GIT_CONFIG_GLOBAL=/dev/null just ci` — all stages green (result in
acceptance.md). The known `TestAppShutdownBounded` TempDir flake documented
in `../2026-10-09-notebook-n2/verification.md` did not appear in this run.

## Notes / deviations

- `module.action.invoke` on a generation without the module returns RPC error
  `-32601 "module action capability is not configured"` (not `-32004`); the
  client maps `-32004`, `-32601`, and the unavailable/not-configured messages
  to `capability_unavailable` — verified live against the packed
  `no-notebook` binary.
- The deleted-entry row opens the detail bar with a Restore affordance; the
  backend `entries.get` on a tombstoned entry does not serve a readonly body,
  so deleted entries are restored from the detail bar rather than previewed.
- Source pin churn: `plugins/vivy-notebook/vivy-module.yaml` `source.sha256`
  was re-pinned via the official stage-ui check after each source change; the
  staged projection under `ui/src/generated/ui/vivy-notebook/` is generated
  and gitignored.
