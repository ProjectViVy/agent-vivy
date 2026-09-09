# Vivy Studio launch fix — dsh-plugin integration naming and build issues

Date: 2026-08-26
Scope: Vivy Studio overlay (`launch-vivy-studio.ps1` + installed profile), not the Vivy kernel.

## Symptom

`launch-vivy-studio.ps1` crashed immediately on startup; dsh reported

```
Error: dsh: plugin tree failed to load: failed to apply loader entry include (cordis:include):
failed to import loader entry dsh-plugin (dsh-plugin): Cannot find package 'dsh-plugin'
imported from ...\data\studio-home\profiles\vivy-studio\
```

`http://127.0.0.1:3090` was unresponsive.

## Root causes (two overlapping issues)

1. **Package-name mismatch**: `studio/dsh-plugin-hub/package.json` declares the
   package name `dsh-plugin` (its `cordis.patch.yml` also registers the loader
   entry as `dsh-plugin`), but `launch-vivy-studio.ps1` used `dsh-plugin-hub` as
   both the dependency key and bundle name. pnpm installed
   `node_modules/dsh-plugin-hub` from the dependency key, while the cordis loader
   resolved `dsh-plugin` → `ERR_MODULE_NOT_FOUND`.

2. **Stale third-party lib build**: fixing the name exposed a second issue—the
   committed `lib/` at the git HEAD of `studio/dsh-plugin-hub` was an outdated,
   incomplete build (only `http/routes.js` and `services/loader.js`, missing
   `services/install/`, `services/profile/`, and so on), while `lib/index.js`
   referenced `./services/install/install.js` → the module was missing at runtime.
   That repository rebuilds `lib` only during `prepack`, and the committed build
   had not kept up with the `src/server` refactor.

## Fix

- `launch-vivy-studio.ps1`: consistently references plugin-hub by its real package
  name `dsh-plugin` (dependency key, `dsh.profile.bundles`, and seal check all
  agree); the seal check now uses an exact quoted match for `"dsh-plugin"` to
  avoid a false match on the `dsh-plugin-hub` substring.
- `studio/dsh-plugin-hub`: rebuilt `lib` from `src/server` with the repository’s
  toolchain (`npm install` + `npm run build:server`), without changing source.
- Synced the installed profile (`data/studio-home/profiles/vivy-studio/`):
  package.json uses the `dsh-plugin` key, and reinstalling
  `node_modules/dsh-plugin` provides the complete lib.

## Not done

- The Vivy kernel, `internal/`, `cmd/`, and `ui/` were not changed.
- The plugin-hub `lib` rebuild was not committed to its repository (third-party,
  and untracked by the parent repository).
- These changes were not committed to git (the debugger + plugin-hub integration
  in the launch script is ongoing; the current delivery owner decides whether to commit).
