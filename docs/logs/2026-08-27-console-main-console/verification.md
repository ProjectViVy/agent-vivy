# Verification — 2026-08-27 console Main Console

Commands run from the repo root (`C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy`).

## JS syntax

```text
node --check studio/dsh-vivy-console/client.js   # exit 0
```

## Deployed copy

- Installed profile copy re-synced and hash-verified identical:
  `data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-console/`
  (`client.js`, `package.json`, `README.md`).
- No host-half change → **no Studio restart needed**; the client bundle is
  served fresh per request, so the Main Console appears after a browser refresh.

## Gate: `just ci`

```text
just ci 2>&1 | Tee-Object -FilePath data/studio-home/just-ci-console-main-console.log
```

Result: **exit 0** (full gate: fmt-check, vet, test, headless-compile, ui-ci).

## Host-API regression (unchanged surface)

The one-click actions call the same routes verified in the previous
iteration's live smoke (`/vivy-console/api/start|stop|restart` +
`/frontend/start|stop|restart`); no route changed in this iteration
(`git diff --stat` for `index.js` is empty). The 2026-08-27
`console-single-dev-server` smoke (22 checks, SMOKE ALL PASS) remains the
host-behaviour proof.

## Interaction check (user view)

After a browser refresh at `http://127.0.0.1:3090`, the 「Vivy Console」 tab
shows two sections Main Console / Logs; the Main Console shows the overall state line,
one-click buttons, and the backend/frontend status cards.
