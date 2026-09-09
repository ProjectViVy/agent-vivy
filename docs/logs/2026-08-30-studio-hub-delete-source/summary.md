# 2026-08-30 — Studio Plugin Hub uninstall loop + automatic source commits

Deliverable: VIVY-STUDIO-PLUGIN-HUB (the `dsh-plugin-hub` submodule under `studio/`)
now has a complete "delete/uninstall" loop plus automatic commits for Vivy source
plugins.

## What changed

### Uninstall deletes the source directory (complete loop)

Previously, uninstalling a Vivy source-installed plugin removed it only from the profile
`package.json` and registry (`vivy-source-plugins.json`), **leaving** the
`studio/<slug>/` source directory behind (the comments and documentation at the time
both said "avoid accidentally deleting local changes"). This now changes to:

- After successful removal from the profile/registry, `removeVivySourcePlugin()` deletes
  the entire `studio/<slug>/` source directory (`deleteVivySourcePluginDir`).
- Clone directories contain a nested `.git`: run `git status --porcelain` before
  deletion and explicitly warn in task output when uncommitted changes exist (the
  changes are deleted with the directory).
- Locate the directory using the registry `localPath` first, falling back for old
  installs to the `file:` spec in profile dependencies (`resolveVivySourceDir`); accept
  only paths inside the `studio/` submodule worktree, preventing deletion outside the
  source tree.

### Automatic commits (vivySourceAutoCommit)

New setting `vivySourceAutoCommit` (enabled by default, server + client + Settings panel):

- After installing / updating / uninstalling a source plugin, automatically `git commit`
  changes under that plugin's path in `studio/` into the `vivy-studio` submodule
  (`commitVivySourceChange`).
- **Path restriction**: `git add -- <plugin-path>`; never stage/commit other submodule
  changes. No related changes (exit 0) creates no empty commit; a non-git repository or
  commit failure records only a warning and does not block install/uninstall; explicitly
  use `-c commit.gpgsign=false` to avoid unattended signing prompts.
- After deleting a directory during uninstall, automatically commit the deletion
  (`chore(hub): remove plugin <name> source`).

### Client

- The uninstall confirmation dialog shows an amber warning for source-installed plugins:
  it will also delete the `studio/<slug>` source directory (server `/installed` adds
  `vivySourcePaths` → `InstalledItem.vivySourcePath` → `UninstallModal`).
- Add an "Automatic source-plugin commits" toggle to Settings (with zh/en copy).
- Add a `useShell` parameter to `runCommand`; add `runGit` (native spawn, not cmd). Fix
  Windows failures where shell word-splitting broke arguments containing spaces (commit
  messages / paths with spaces); clone / pull / status also switch to `runGit`.

### Adjacent fixes (pre-existing submodule breakage, not introduced here)

- `tests/catalog.test.ts`: remove the reference to the removed API `installCommandOf`
  and its test case (the sealed build has no such export, so typecheck necessarily
  failed).
- `tests/install-target.test.ts`: the `update` verb no longer strips the command (the
  sealed build supports only `add`); align the assertion with the actual contract.

## What was explicitly not done

- Do not commit/backup uncommitted changes in the nested repository before uninstall —
  the dialog makes the deletion risk clear, task output warns about it, and the user
  explicitly requested source deletion.
- Do not change the installation mechanism (the clone flow retains the nested `.git` so
  `git pull` remains usable).
- Do not push or change the host root repository's `studio` gitlink (the host has another
  lane in progress; submodule changes were committed on an independent branch and the
  host lane will bump the gitlink when it closes).
- Do not perform a real network install/delete exercise in the running Studio (it needs
  external network access and would pollute the current profile registry); local
  temporary directories and a temporary git repository cover the loop in unit tests;
  see verification.md for runtime smoke.

## Files touched (submodule `studio/dsh-plugin-hub`)

- `src/server/services/install/vivy-source.ts` — core: deletion + automatic commit
- `src/server/services/settings.ts` — `vivySourceAutoCommit`
- `src/server/http/routes.ts` — Settings allowlist + `/installed` vivySourcePaths
- `src/client/hooks/useSettings.ts`, `components/views/SettingsView.tsx`,
  `locales.ts`, `data/host.ts`, `logic/installed.ts`,
  `hooks/useCatalog.ts`, `components/modals/modals.tsx`,
  `components/PluginHubSection.tsx`
- `tests/vivy-source.test.ts` (6 new cases), `tests/catalog.test.ts`,
  `tests/install-target.test.ts` (fix existing breakage)
- `VIVY-INTEGRATION.md` — behavior documentation sync
