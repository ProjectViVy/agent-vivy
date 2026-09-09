# Verification — 2026-08-27 console Packaging & Version page

Commands run from the repo root (`C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy`).

## JS syntax

```text
node --check studio/dsh-vivy-console/index.js   # exit 0
node --check studio/dsh-vivy-console/client.js  # exit 0
```

## Live smoke of the restored lifecycle host (isolation, mock ctx)

`data/studio-home/vivy-console/smoke-lifecycle.mjs` imports the edited
`index.js`, drives `/vivy-console/api/*` against the real `vivy-studio.exe`
and the Studio ledger, then checks the dev loop is untouched.
Run: `node data/studio-home/vivy-console/smoke-lifecycle.mjs` →
**exit 0, SMOKE ALL PASS (17 checks)**:

| Check | Result |
| --- | --- |
| `/resolve` reports `studio` (vivy-studio.exe path) + dev fields | PASS |
| `list generations` / `evals` / `worktrees` OK (JSON arrays; worktree pinned) | PASS |
| bogus ledger kind fails with message | PASS |
| release without confirm refused before spawn (NG-25 guard) | PASS |
| unknown action refused | PASS |
| job machinery end-to-end: `inspect` on a bogus target spawns, streams output, terminates | PASS |
| unknown job id → literal `Job not found` | PASS |
| dev regression: `/status` + `/logs` OK; lifecycle did **not** start backend or frontend | PASS |

The CLI contract was also probed directly:
`vivy-studio.exe --worktree . list generations|evals` (JSON arrays) and
`--worktree . workspace list` (`{"worktrees":[…]}`), matching
`lifecycleList`'s parsing.

## Ledger hygiene

Verification only read the Studio ledger; the single spawned job was a safe
`inspect` on a nonexistent target (non-mutating). No pack/eval/release/
install/rollback ran.

## Deployed copy + restart

- Installed profile copy re-synced and hash-verified:
  `data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-console/`
  (`index.js`, `client.js`, `package.json`, `README.md`).
- Host routes changed → Studio restarted detached
  (`data/studio-home/restart-studio.ps1`); the listener at
  `http://127.0.0.1:3090` came back up. Refresh the browser to see the new
  「Packaging & Version」 tab.

## Gate: `just ci`

```text
just ci 2>&1 | Tee-Object -FilePath data/studio-home/just-ci-console-package-page.log
```

Result: **exit 0** (fmt-check, vet, test, headless-compile, ui-ci).
