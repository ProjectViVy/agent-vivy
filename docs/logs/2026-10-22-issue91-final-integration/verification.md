# Verification

Phase 1 passed on cd0d4d3f plus the explicit integrated patch (244 source paths).
The three D09 migration test-prefix corrections were present in that tested
patch but omitted from the initial merge staging; a focused follow-up commit
records the same verified test bytes. No production source changed for this
staging correction. The dependency checkpoint is intentionally RED until the
D10/D11 consumer adaptation is integrated.
The independent Debian Linux-filesystem checkout is
`/home/mastwet/work/vivy-issue91-final/agent-vivy`, with its own clean Laputa
ff3936f44ff8cf08c12af2cf698c194cfe474fd3. Go1.26.4 and Node24.21.0 are used.

- RED: TestIntegratedStableMetadataPrefixesRequireFiniteUpgradeWithoutWrites
  rejected accepted stable037 and038 as unsupported_source_format.
- GREEN: after exact catalog-shape classification, the full migration owner
  `go test -tags=vivy_headless ./internal/storage/migrations -count=1 -timeout=5m`
  passed in2.032s. D08's fixture was aligned to its actual pre-change036 so
  retired prompt/checkpoint fields are tested before their removal.
- Windows sandbox WSL enumeration was denied; the approved explicit WSL path
  works. The generic environment analyzer's inaccessible-WSL result is not
  acceptance evidence and no tool installation was performed.
- Final updated source: `just wiring-ci` passed 16 Node and111 Go cases,
  zero skips. `just kernel-ci` passed1871 cases plus real minimal Recipe
  pack/Inspect/boot. Its one existing optional
  `TestBackendGrepRipgrepHonorsGitignore` skip is due to missing rg in WSL.
- Independent `(cd bml && go test ./... -count=1 -timeout=10m)` passed.
- `just d02-ci` passed81 protocol,175 shared Face,1 causal App,13 shutdown
  and9 standalone cases, zero skips. `just d02-web-ci` passed typecheck,
  106 UI cases and2 actual UI artifact packs.
- `just test-postgres` passed142 cases, zero skips, against a disposable
  PostgreSQL16 container with tmpfs storage. The container was removed.
- D09 root identity regressions first reproduced cross-root authority and
  missing-read correction failures. Privacy and nested-parent regressions
  reproduced the independent review findings. Focused runtime/App checks
  then passed; the same selection with `-race` passed (9.337s/1.155s).
- `pnpm --dir ui test`:86 files,680 cases passed. Real split Vite3015 plus
  backend acceptance passed all4 continuity cases (23.9s): exact download,
  changed-file refusal/separate private Run, saved reference after source
  deletion, responsive keyboard actions. A separate real memory scenario
  passed (6.8s): MEMRULES exact save/conflict/refresh and BML2 memory creation.
  Tests used disposable homes, initialized Persona and loopback model responses.
- Independent integration review cleared the original merge and the final
  sandbox/privacy corrections. Review was read-only; tests above were run by
  the integrating lane. This is not either fresh D14 audit.
- `git diff --check` passed. No released SQL001–038 differs from stable.

Native Windows owner acceptance, full-product `just ci`, the assembled finite
upgrade artifact, updated D10/D11 pins and the final frozen-candidate audits
remain separate and are not counted as passes here. No release is claimed.

Task-local full command output lives under `.workspace/verification` in the
independent checkout. No production store was used.
