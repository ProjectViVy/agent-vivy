# Acceptance — 2026-08-30 Studio Plugin Hub uninstall loop + automatic commits

## How a user can confirm it works

1. **Uninstall deletes the source directory**: in the Vivy Studio Plugin Hub
   (VIVY-STUDIO-PLUGIN-HUB), click "Uninstall" for any **Vivy source-installed** plugin
   in the "Installed" list:
   - The confirmation dialog shows an amber warning: "The source directory
     studio/<plugin-name> will also be deleted…".
   - After confirmation, the task log shows `[vivy-source] deleting source directory
     ...\studio\<plugin-name>` (for a clone with uncommitted changes, a preceding
     `warning: ... has uncommitted changes` line appears).
   - When finished, the plugin directory under `studio/` **no longer exists**
     (`git -C studio status` shows the deletion as pending or committed).
2. **Automatic commit** (enabled by default): after uninstall,
   `git -C studio log -1` shows `chore(hub): remove plugin <name> source`; installing or
   updating a source plugin likewise automatically creates a
   `chore(hub): install/update plugin <name> source` commit. It contains only that plugin
   path, preserving other uncommitted changes in `studio/` exactly.
3. **Toggle**: Plugin Hub → Settings → Updates & Sources shows "Automatic source-plugin
   commits". Turn it off and install/uninstall no longer creates commits (deletion still
   runs; changes wait for a manual `git -C studio commit`).
4. **Non-source plugins are unaffected**: system-directory installations (not
   vivy-source) behave as before, with no source directory to delete and no warning in
   the dialog.

## Environment

- Vivy Studio (DSH harness web, port 3090), profile `vivy-studio`, with
  `vivySourceInstall` enabled (default).
- The uninstall target must be a registry entry in `vivy-source-plugins.json` or a
  `file:` dependency pointing into `studio/`; otherwise the existing system-directory
  uninstall path is used (source is not deleted, as expected).
