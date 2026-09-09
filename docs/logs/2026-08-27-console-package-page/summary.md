# Vivy Console — Packaging & Version page (packaging & version management restored)

Date: 2026-08-27
Scope: Vivy Studio overlay (`studio/dsh-vivy-console`), not the Vivy kernel.

## What changed

Per user request, the previously trimmed Studio distribution surface is back
in the console as its own page — **clearly separated from the dev mode**:

1. **「Packaging & Version」 page restored** (was 「Lifecycle」 in the original console,
   removed in the dev-loop-only trim). The client gains a third section
   Main Console / **Packaging & Version** / Logs. The page:
   - states up front that it is the Studio distribution lifecycle executed
     through `vivy-studio.exe` on the pinned worktree and is **completely
     separate from the Main Console dev loop — it never starts or stops any
     backend/frontend process**;
   - ledger chips: generations / evals / releases / installs / events /
     worktrees (JSON table per ledger, Refresh button);
   - action form + buttons: pack / eval / release / reject / install /
     rollback / inspect, with the release human-confirmation checkbox
     (forwards `--actor human --yes` only when checked, NG-25);
   - a live job-output viewer (one concurrent job, polled, streamed output).
2. **Host routes restored** in `index.js`:
   - `/vivy-console/api/lifecycle/list?kind=…` — ledger reads through
     `vivy-studio.exe --worktree <root> list …` / `workspace list`;
   - `/vivy-console/api/lifecycle/run` — spawns one concurrent job
     (`pack/eval/release/reject/install/rollback/inspect` arg builder,
     concurrency limit 1, streamed output, killed on plugin dispose);
   - `/vivy-console/api/lifecycle/jobs/<id>` — job snapshot;
   - `/resolve` now also reports `studio` (vivy-studio.exe path,
     `VIVY_STUDIO` override or root);
   - release without UI confirmation is refused before spawn (keep the
     CLI-side human gate intact).
3. The dev loop (Main Console) is untouched: still the pure-API `vivy_headless`
   backend + Vite dev server, one-click orchestration, unified logs. The
   lifecycle surface is a sibling page, not part of it.

### Files

- `studio/dsh-vivy-console/index.js` — lifecycle host block restored
  (jobs map, `resolveStudioExe`, `lifecycleList/Run`, `jobSnapshot`, routes,
  dispose kills jobs); header comment + route table updated to the two
  concerns.
- `studio/dsh-vivy-console/client.js` — `PackagePane` (Packaging & Version page) added,
  three-section ring; lifecycle CSS restored (`vc-grid`/`vc-field`/`vc-tbl`/
  `vc-empty`).
- `studio/dsh-vivy-console/README.md` / `package.json` — scope, route table,
  page list, and description updated.
- Installed profile copy synced:
  `data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-console/`.

## What was explicitly not done

- No kernel / `internal/` / `cmd/` / `sdk/` changes; `vivy-studio.exe` CLI
  semantics and the Studio ledger are untouched (the console only forwards).
- No action was run that mutates the ledger during verification (lists are
  read-only; the only spawned job was a safe `inspect` on a bogus target,
  which fails without touching anything).
- Pack/eval/release/install/rollback remain human-driven from the page; no
  automation or auto-release added.

## Follow-up (same delivery)

Per user feedback, the tab order is corrected to **Main Console / Logs / Packaging & Version**
(the packaging page comes after the log page). Client-only change
(`SECTIONS` order in `client.js` + README section list); the installed copy
was re-synced — no host change, no Studio restart needed, visible after a
browser refresh. Committed as a small follow-up commit on top of this
deliverable.

## Notes

- The lifecycle page intentionally does not show backend/frontend dev state
  and the Main Console does not show distribution state — the separation the user
  asked for is structural (sibling pages) not just cosmetic.
- Host routes changed, so this iteration needs the detached Studio restart
  after syncing (same no-hot-reload rule as before).
