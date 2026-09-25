# Verification

Initial PG-6 code revision: `1aca64f937c462f3558cf6b51c591a4ae703028b`. The configured author and committer are the human contributor `mastwet`.

Focused checks passed:

```text
go test ./internal/app -run '^TestPlanGoalIntegratedRoundLimitBlocksDurably$' -count=3 -timeout=150s
go test ./sdk/internal/conformance -run '^TestCheckedInProviderConformanceMatchesExecutedSuites$' -count=1 -timeout=10m
go run ./sdk/internal/cmd/source-hash internal ''
git diff --check
```

The source-hash command returned `c37564d5b82325c5d3a1d2e1664a7423410470ad8e6ff841a07deaafd2365d6b`; exactly five internal-rooted `sourceSha256` entries were refreshed. The cap test passed three runs, and the conformance producer test passed in 172.202 seconds. Earlier PG-6 test work also exercised a missing-`submit_plan` mutation and a `max_rounds` mutation; both failed the intended assertions before restoration.

Final `just ci` on the integration worktree ran with Go on `PATH` and `CI=true`, and exited **0** on 2026-09-25. The format check, UI typecheck, 424 UI tests, Vite build, i18n checks, Go vet, full Go tests, headless compile, and all plugin/face module checks passed. Relevant package durations were `internal/app` 311.878 seconds, `internal/rpc` 500.021 seconds, `internal/runtime` 673.837 seconds, `sdk/internal` 790.959 seconds, and `sdk/internal/conformance` 363.369 seconds. Vite emitted its existing chunk-size warning.

Intermediate checks are not counted as final evidence: one `just ci` attempt at an earlier PG-6 revision failed when the existing `TestModelWorkIdentityUsesEinoCallIDForDistinctCallsAndRetries` exceeded a 5-second wait under full-suite load; it passed three isolated reruns and a later full gate passed. Interrupted command sessions yielded no final exit result. A subsequent attempt paused at pnpm's interactive reinstall prompt and was stopped; setting `CI=true` resolved that prompt for the final passing gate.

Independent PG-6 follow-up review passed after the restarted round-limit Work projection assertion was added. A final read-only branch review found no new actionable code defect in the PG-6 integration delta; reviewers did not claim to rerun CI.

Unverified by user direction: live PostgreSQL migration/parity (`just test-postgres`) and Docker checks were deferred to the next phase; no PostgreSQL DSN/service was configured. Browser acceptance belongs to the user and was not run by the agent. The live-provider coding walkthrough was not run because credentials were not configured. These outcomes do not complete PG-6 or release downstream Stories.

## PR CI follow-up

The first GitHub Actions run for PR #57 passed `ui ci` and `full UI browser smoke` but failed `backend ci` in the existing `TestVC1Walkthrough`: the Windows runner did not have `rg` on `PATH`. The test now uses `git grep --no-index` against its disposable workspace file, preserving the Bash-tool and on-disk verification while depending only on Git, already required for the checkout. The focused walkthrough passed three consecutive local runs:

```text
go test ./internal/runtime -run '^TestVC1Walkthrough$' -count=3 -timeout=2m
```

The changed internal source digest is `621fcd68049603c59f6824d8d5be8fa074870e0030dcde0aa115ad268e58716b`; the five internal-rooted conformance rows were refreshed. `TestCheckedInProviderConformanceMatchesExecutedSuites` passed in 153.586 seconds. The subsequent complete `just ci` passed with exit code 0 on the resulting code and digest: 424 UI tests, UI build/typecheck/i18n, Go vet/full tests, headless compile, and plugin/face checks. The GitHub Actions rerun for the follow-up push is reported separately in the PR checks and is not claimed by this local result.
