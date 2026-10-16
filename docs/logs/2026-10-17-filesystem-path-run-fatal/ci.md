# `just ci` record — 2026-10-17

Tree: `feat/issue-58-unified-context` root working tree, with the fix in
`internal/runtime/filesystem_backend.go` plus the two test files.

## Result

| Recipe | Result |
| --- | --- |
| `ensure-laputa` | ok |
| `bootstrap-test` | ok |
| `fmt-check` | ok |
| `ui-ci` (i18n-check → ui-core: typecheck, 692 vitest tests, `vite build`) | **ok** |
| `vet` | ok |
| `test` (`go test -p 1 -timeout 35m ./...`) | **FAIL — environment, not this change** |
| `diva-recall-test`, `headless-compile`, `plugin-ci` | not reached (the `test` recipe exits first) |

`agent-vivy/internal/runtime` — the only package this change touches behaviourally
— passed inside that run (`ok agent-vivy/internal/runtime 386.999s`), and
`internal/app`, `internal/tools`, `internal/storage/sqlite` and the rest of the
kernel passed as well.

## Environment failures in `test` (all pre-existing)

1. **Go build cache is not deletable/readable by the test process.**
   `internal\app\app.go:65:2: open C:\Users\com01\AppData\Local\go-build\<hash>-a: Access is denied`
   → `internal/studiocore TestEvalSpawnsCandidateItself`, `sdk/codeclient
   TestClientAgainstRealVivyCode`, `sdk/internal/assembly [setup failed]` and the
   `sdk/internal/conformance` suites that shell out to it.
2. **Symlink privilege is not held** ("A required privilege is not held by the
   client"): Windows Developer Mode is off, so every test that links
   `ui/node_modules` into an isolated build boundary, and
   `sdk/internal/assembly TestHashSourceTreeRejectsSymbolicLinks`, fails.
3. **esbuild cannot remove its temp file in `%TEMP%`**
   (`vite:esbuild-transpile remove C:\Users\...\Temp\esbuild-<hash>: Access is
   denied`) — this is why the first `just ci` attempt died in `ui-core`
   (`vite build`), not in any Go step.

The same class of failure is recorded in the pre-change CI log from the previous
day, `.workspace/tmp/just-ci-merged-2.log` (2026-10-16, before this change):
`required privilege is not held by the client` for the same
`sdk/internal/frontend_v1_test.go` / `failure_matrix_execution_test.go` cases,
and `open C:\Users\com01\AppData\Local\go-build\33\33ec...-d: Access is denied`.
That log also ends with `error: Recipe 'test' failed on line 39`.

Workaround used to get past (1) and (3) for this record: run `just ci` with
`TEMP`/`TMP` (and later `GOCACHE`) pointed inside the workspace. That is a run
setting only — no repository file was changed for it.

With `TEMP` **and** `GOCACHE` relocated, a targeted rerun shows the split
exactly:

```
go test ./sdk/internal ./internal/studiocore ./sdk/codeclient -count=1
# ok   agent-vivy/internal/studiocore 34.458s
# ok   agent-vivy/sdk/codeclient      27.706s
# FAIL agent-vivy/sdk/internal       162.442s   ← only "A required privilege is not held by the client"
```

So (1) disappears under the relocated build cache and (2) — the machine's
missing Windows symlink privilege — is the only irreducible failure. (2) cannot
be worked around without the machine-level symlink privilege / Developer Mode.

## Targeted verification of this change

```
go vet ./internal/runtime/                              # clean
go test ./internal/runtime/ -run 'TestEinoFilesystemBackend|TestFilesystemPathMistakeRecoversInsideRun|TestToolFailure' -count=1
                                                        # PASS
go test ./sdk/internal/assembly -count=1                # only TestHashSourceTreeRejectsSymbolicLinks fails (privilege)
VIVY_RELEASE_CONFORMANCE=1 go test ./sdk/internal/conformance -run '^TestCheckedInProviderConformanceMatchesExecutedSuites$'
                                                        # no "release source digest" mismatch; only the full-ui
                                                        # symlink-privilege failures remain
```

## Conformance digest re-pin

Editing `internal/` moves `internal/sourcehash.Tree`'s identity, so the five
`internal`-rooted `sourceSha256` rows in
`sdk/internal/assembly/conformance_results.json` were re-pinned in this change:

```
go run ./sdk/internal/cmd/source-hash internal 1acae684959aac7850437e258069fa2612128b5eceddf44dae7b554933e3d5a9
# 1acae684… → 982fc2a3dee710532fcab219c6e9b93f3a47a3fbaec9bdb8115985c9d1ba899b (5 rows)
```

The declared-digest placeholder was passed explicitly because PowerShell drops an
empty-string argument; the old value does not occur anywhere under `internal/`,
so the result equals `HashSourceTree("internal", "")`. Re-running the helper with
the new value as the declared digest returns the same `982fc2a3…`, which is
exactly the comparison `TestCheckedInProviderConformanceMatchesExecutedSuites`
performs — and that test reported no digest mismatch.

Superseded the same day by the follow-up cut in this same working tree, which
edits `internal/` again and re-pins the five rows `982fc2a3…` → `04f63283…`
(see [`../2026-10-17-tool-argument-run-fatal/ci.md`](../2026-10-17-tool-argument-run-fatal/ci.md)).
The interim value is kept here because it was the identity of this cut's tree.
