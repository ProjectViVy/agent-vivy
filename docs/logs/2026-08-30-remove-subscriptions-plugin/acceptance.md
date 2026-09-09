# acceptance — remove subscriptions plugin

## How a person can confirm it

1. The `studio/` submodule no longer contains `dsh-plugin-subscriptions/`:
   `git -C studio ls-files | Select-String subscriptions` has no output, and the
   "What lives here" table in `studio/README.md` has one fewer row.
2. Open Vivy Studio (`http://127.0.0.1:3090`, refreshing if necessary); the Plugin Hub /
   dsh-plugin "Installed" list contains neither Subscriptions nor the subscriptions
   plugin. Then check repository Settings for no Subscriptions login entry (the plugin
   provided Settings → Subscriptions and a subscriptions provider).
3. Studio runtime data no longer contains traces of the plugin:
   `data/studio-home/plugins/subscriptions/` does not exist (auth/models/proxy data was
   removed).
4. Future "GitHub plugin installation" no longer mixes a new snapshot with the old
   snapshot when writing into `studio/`; reinstalling `v1ki/dsh-plugin-subscriptions`
   requires a completely new installation flow.

## Things that did not happen (also part of acceptance)

- No commit was pushed (push requires explicit authorization).
- `data/vivy.db` / `data/demo/` / `data/workspaces/` were not touched (air gap).
- The running Studio server was not restarted (it had not loaded the plugin).
