# summary — Remove dsh-plugin-subscriptions from Vivy Studio

## What changed

Completely remove `dsh-plugin-subscriptions` (referred to by the user as
`@studio/dsh-plugin-subscriptions`) from Vivy Studio.

| Item | Status |
|---|---|
| `studio/dsh-plugin-subscriptions/` (community plugin snapshot pinned inside the vivy-studio submodule) | Removed, commit `5b6cb4e` (vivy-studio repository) |
| The `dsh-plugin-subscriptions/` table row in `studio/README.md` | Removed |
| `data/studio-home/plugins/subscriptions/` (plugin runtime data: `auth.json` / `models.json` / `proxy.json`) | Removed (Studio's own scratch, not committed) |
| `package.json` / bundles / `vivy-source-plugins.json` / `gro.ngilp-hsd-versions.json` / node_modules under `data/studio-home/profiles/vivy-studio/` | Already clean (Plugin Hub had previously uninstalled it; `hub.log`: `Uninstall succeeded v1ki/dsh-plugin-subscriptions`); no residue found in this review |
| Running Studio app (`dsh --profile vivy-studio --port 3090`) | Rechecked `window.__DSH_BOOT__` and `/dsh-plugin-hub/installed`; no subscriptions entry, and the app had not loaded the plugin |

The host repository changed only one gitlink (the `studio` submodule pointer), plus this
iteration record.

## Why

The user required that the plugin no longer exist in Studio. It had already been
uninstalled from the profile, but the source snapshot remained pinned in the vivy-studio
submodule and listed in `studio/README.md`, while the runtime data directory also
remained. This change closes both gaps so removal holds at every layer: source tree,
profile, and runtime data.

## Explicitly not done

- Do not alter historical mentions in `docs/logs/` from 2026-08-29 / 2026-08-30
  (historical facts are retained).
- Do not clean the Plugin Hub catalog cache (`cache/catalog-plugins-zh.json` is a
  remote-catalog mirror containing ecosystem-available plugins, not installation state;
  plugin removal does not include changing the catalog).
- Do not restart the running Studio server: the app never loaded this plugin (it was
  already uninstalled from the profile); deleting the source snapshot affects only
  future vivy-source installation paths, so a restart is unnecessary.
- Do not push any commit (push requires explicit authorization). Commits remain local.
