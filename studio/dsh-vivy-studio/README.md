# dsh-vivy-studio

First-party DSH shell skin for the independent **Vivy Studio** app.

- Product name: Vivy Studio (title, sidebar wordmark, welcome)
- Theme: Fluorite (concert-hall paper / night ink, teal accent)
- Loaded as a sealed `vivy-studio` profile bundle — not a `--patch` overlay
- Host `webServer.tapIndex` injects CSS + title script. Do not insert nodes into React trees.
- Built-in DSH themes ship empty token maps, so the stylesheet is the live palette. `node check-tokens.mjs` diffs official `--dsw-alias-*` / `--dsw-specific-*`.

See `docs/architecture/VIVY-STUDIO.md` §9.1.

```powershell
# from agent-vivy/
.\launch-vivy-studio.ps1
# dsh --profile vivy-studio --port 3090
```
