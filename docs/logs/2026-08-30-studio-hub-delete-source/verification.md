# Verification — 2026-08-30 Studio Plugin Hub uninstall loop + automatic commits

Worktree: `C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy`
(`studio/` submodule HEAD 5b6cb4e; all changes in this iteration landed inside the
`dsh-plugin-hub` submodule).

## Unit tests / typecheck / build

```text
cd studio/dsh-plugin-hub

# Dependencies (this subtree had no node_modules; install devDeps first; reuse the global
# pnpm store, with no external network dependency)
pnpm install --prefer-offline
# -> resolved 112, added 46, done in 5s

# Typecheck (three tsconfig files: client / server / test)
npm run typecheck        # -> all green (before fixing the pre-existing break:
                         #    catalog.test.ts referenced removed installCommandOf and
                         #    reported TS2305; fixed)

# Full tests (node --test, Node v24.14.1, native .ts type stripping)
npm test                 # -> 56 pass / 0 fail
#    six new cases in tests/vivy-source.test.ts:
#    resolve path (registry priority / file: fallback / out-of-bounds rejection)
#    commitVivySourceChange (path restriction / no empty commit / skip non-repository)
#    deleteVivySourcePluginDir (deletion / dirty-repository warning / missing directory)
#    uninstall loop integration (delete directory + commit trace + unrelated changes
#    unaffected)
#    pre-existing break fixed: align update assertion in install-target.test.ts
#    sealed contract (only add strips the command; update/remove return unchanged)

# Build (run equivalent build-script steps manually on Windows; npm run build's rm -rf
# is unavailable in cmd)
Remove-Item -Recurse -Force lib
npx tsc -p tsconfig.server.json      # exit 0
npx tsdown                           # client/client.js 360.47 kB, ok
node scripts/tools/normalize-client-banner.mjs  # banner ok
# Spot-check the compiled artifact: lib/services/install/vivy-source.js contains
# commitVivySourceChange / deleteVivySourcePluginDir / vivySourceAutoCommit /
# sourceDeleted — confirm the new logic is in the bundle
```

## Runtime smoke (Plugin Hub loaded in the running Studio)

- Refresh the profile copy: run `pnpm install` under
  `data/studio-home/profiles/vivy-studio` (file: copy updates
  `node_modules/dsh-plugin`, including new lib + client).
- Per vivy-studio's no-hot-reload rule, **restart Studio in a separate process** (the
  `restart-studio.ps1` method avoids self-termination inside the tree); after port 3090
  recovers:
  - `GET /dsh-plugin-hub/settings` returns `vivySourceAutoCommit: true`.
  - `GET /dsh-plugin-hub/installed` returns the `vivySourcePaths` field.
  - The Web UI Plugin Hub loads normally and Settings shows the "Automatic source-plugin
    commits" toggle.

## Not run (reason recorded)

- `just ci`: that gate covers the host Go/UI repository; this deliverable is entirely in
  the `studio/` submodule (independent git repository and gate: typecheck + test + build
  are all green).
- Real network install → uninstall exercise: it would require cloning a third-party
  repository over the network and writing the current Studio profile registry (polluting
  the user environment). Deterministic unit tests using temporary directories and a
  temporary git repository cover the core loop without network access.

## Defects / boundary notes

- When a nested clone repository (clone flow) with uncommitted changes is uninstalled,
  the changes are deleted with the directory (with a warning) — a product decision, not
  a defect.
- Automatic commits also create commits on a submodule detached HEAD (local trace), do
  not push, and leave branch organization to the user.
