# Notes — open items found while doing this work

Not fixed in this change; recorded here rather than in `docs/TODO.md` §0.1
because another lane currently has `docs/TODO.md` modified in the shared root
tree, and two lanes must not write the same file. Move these to the board when
that lane lands.

## 1. A fresh `studio/` checkout cannot boot until `dsh-plugin-hub` is built

`launch-vivy-studio.ps1` fails on a clean checkout with

```text
dsh: plugin tree failed to load: ... Cannot find module
  ...\profiles\vivy-studio\node_modules\dsh-plugin\lib\index.js
```

`studio/dsh-plugin-hub` ships TypeScript sources only: `lib/` (tsc server output)
and `client/` (tsdown output) are in its `.gitignore` and **were never committed**
(`git ls-files dsh-plugin-hub/lib` is empty at every revision, including
`chore/dsh-0.2-plugins`). Neither `scripts/ensure-studio.ps1` nor
`launch-vivy-studio.ps1` builds it.

Working remedy used here, per `studio/dsh-plugin-hub/VIVY-INTEGRATION.md`:

```powershell
cd studio/dsh-plugin-hub
npm ci
Remove-Item -Recurse -Force lib, client
npx tsc -p tsconfig.server.json
npx tsdown
node scripts/tools/normalize-client-banner.mjs
```

Upstream fix belongs in `ProjectViVy/vivy-studio`: either build in the bootstrap
step or ship the built artifacts. Installing `dsh-plugin@1.3.5` from npm instead
is **wrong** — that is the unsealed upstream hub (ads, self-update), not
`VIVY-STUDIO-PLUGIN-HUB`.

## 2. `launch-vivy-studio.ps1` has a dead first-run branch

`launch-vivy-studio.ps1:123-135` writes the profile manifest and then re-reads
the file it just wrote to compute `$needSeal`; the manifest always contains all
four bundle names, so `$needSeal` is always `false` and the intended first-run
`dsh plugin --profile vivy-studio add ...` branch never executes. Every start
therefore takes the `pnpm install` path only.

## 3. Dead git proxy breaks submodule fetches

This machine has `http.proxy` / `https.proxy = http://127.0.0.1:7892` in git
config, and nothing listens on that port. `git submodule update --init` and
`just ensure-studio` fail until the proxy is running or the setting is removed.
Workarounds used here, none of which persist a config change:

```powershell
git -c http.proxy= -c https.proxy= submodule update --init --recursive -- studio
git -c http.proxy= -c https.proxy= -C studio push origin main
```

The same dead endpoint is why the plugin hub's catalog route answers 502 while
the machine has working direct access to GitHub.

## 4. Skin edit trap

`data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-studio` is a
junction into a pnpm virtual-store copy whose files are **hard links** to
`studio/dsh-vivy-studio/`. Editor-style writes (temp file + rename) replace the
inode and silently leave the loaded copy stale; always copy the edited files
into the `.pnpm` path and verify SHA parity, then restart the server.
