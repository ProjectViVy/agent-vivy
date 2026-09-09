# verification — remove subscriptions plugin

## Commands run

```text
git -C studio rm -r dsh-plugin-subscriptions        # tree removed, staged
git -C studio commit -m "chore(plugins): drop dsh-plugin-subscriptions snapshot"
git -C studio status --short                        # clean
git -C studio log --oneline -3                      # 5b6cb4e on top of b7de607

Remove-Item -Recurse -Force data/studio-home/plugins/subscriptions
Test-Path data/studio-home/plugins/subscriptions    # False

# live Studio (127.0.0.1:3090), read-only probes:
GET /                                 -> title "Vivy Studio";
                                       boot entries contain no "subscription",
                                       no "@studio" anywhere in boot JSON
GET /dsh-plugin-hub/installed         -> installed/versions/paths/loaded/dshCapable
                                       contain none of dsh-plugin-subscriptions

rg -n "dsh-plugin-subscriptions" studio (excluding node_modules)  # 0 matches
```

## `just ci` gate

`just ci` was run when this deliverable was complete and **failed at `fmt-check`**;
all failures came from pre-existing dirty areas in the root tree (partial compaction
code from another parallel lane):

```text
internal\app\compaction.go
internal\rpc\control.go
internal\storage\sqlite\compaction.go
internal\storage\postgres\compaction.go
internal\runtime\compaction_service.go
internal\runtime\compaction_policy.go
internal\runtime\compaction_middleware.go
```

These files (`internal/app`, `internal/rpc`, `internal/storage/*`,
`internal/runtime/compaction_*`) have no overlap with this deliverable, which changed
only the `studio/` submodule (an independent git repository) and `data/studio-home/`
(gitignored Studio scratch). Per process, the failure is recorded here: it was not
introduced by this change and belongs to the root tree's pre-existing uncommitted lane
state. `just ci` can be fully green after that lane closes. This deliverable touched no
Go/UI source files.

## Studio-side verification conclusion

- After the submodule removal commit, `git -C studio status` is clean.
- The profile layer (package.json bundles / deps, `vivy-source-plugins.json`,
  `gro.ngilp-hsd-versions.json`, node_modules, cordis) was rechecked and contains no
  trace of the plugin.
- The running app's load manifest and the Hub's installed endpoint both contain no such
  plugin; the server does not need a restart.
