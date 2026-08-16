# Studio Lifecycle Closure Acceptance

Date: 2026-08-16

| Acceptance item | Evidence | Result |
|---|---|---|
| Studio runs `vivy-sdk pack` itself (ST-5) | `studiocore.Pack` execs the sdk binary and records the Generation (built) in `data/studio-home/studio.db`; `TestRecordGenerationRecordsPackOutput` | PASS |
| Studio spawns candidate EXE for eval, independent data dir (ST-5) | `studiocore.Eval` → `eval.Launch`; candidate booted from a real `./cmd/vivy` build; EvalRun recorded in the Studio ledger; candidate journal under `data/studio-home/evals/` | PASS |
| Live species process zero participation (ST-5) | No `evals/start`/`promotions/promote` RPC used; `assertTenantUntouched` | PASS |
| Release is human-gated (ST-7) | `Release` refuses any actor other than `human`; CLI requires `--actor human --yes` (NG-25) | PASS |
| Install writes daily location; next launch is the new body (ST-7) | `Install` copies released EXE + `install.json` into the target; round-trip test checks the installed hash equals the released generation | PASS |
| No live-process hot-swap (ST-7) | install/rollback never start/kill a process; file-only writes (NG-3) | PASS |
| Rollback restores previous Release (ST-8) | `Rollback` restores the Studio snapshot of the previous release; `install.rolled_back` event recorded | PASS |
| Tenant Journal untouched (ST-8) | Sentinels for `data/vivy.db` / `data/workspaces/` unchanged across install and rollback; `ErrBlockedTarget` for targets inside the worktree | PASS |
| `just ci` green | fmt-check + vet + test pass after all changes | PASS |

No ST-5/ST-7/ST-8 acceptance item remains open. S9 (frozen eval suite) and
auto-release rules are explicitly deferred.
