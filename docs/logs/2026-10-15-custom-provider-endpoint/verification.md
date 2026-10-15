# Verification

All commands run from the repository root on 2026-10-15.

## 1. The defect is reproduced by a test

`internal/app/provider_selection_test.go`, case
`a registered custom provider resolves to its own endpoint`, pins the reported
shape: a document whose `provider` is an adapter, whose `base_url` no embedded
vendor declares, and whose `providers` list carries the registry entry.

Before the fix the case could only assert the defect
(`ok=false`, `vendor="Acme LLM"`); after it, the case asserts the registry
endpoint is carried (`ok=true`, adapter + address + model from the entry) and
`TestRegisteredCustomProviderRunsFromItsOwnEndpoint` asserts the live resolver
reports `provider="WXYG"`, `Ready`, and the registry key.

`internal/provider/provider_test.go:TestResolvingChatModelRunsARegisteredEndpoint`
drives the run path with an empty catalog and asserts the request reaches
`/chat/completions` on the registered address — the exact call that used to
return `provider "WXYG": no embedded vendor data` — and that a deferred adapter
still fails closed (`ErrAdapterDeferred` from
`Catalog.RefForSelection`, `modelhost.ErrProfileNotFound` from the compiled
host).

## 2. Package tests

```text
go vet ./internal/provider/... ./internal/app/...      exit 0
go test ./internal/provider/... -count=1               ok  agent-vivy/internal/provider  7.832s
go test ./internal/app/ -run 'TestRegisteredCustomProviderRunsFromItsOwnEndpoint|TestResolveStoredSelection|TestResolver|TestApplySettingsEnv' -count=1 -v
                                                       ok  agent-vivy/internal/app  0.773s (22 tests)
```

The whole `internal/app` package was also run in full; one run failed and one
run stalled, both on pre-existing, unrelated flakiness documented on the board:

| Observation | Assessment |
|---|---|
| `TestMemoryLoopActivityFailurePreservesSourceAndArchiveRetry` fails with `TempDir RemoveAll cleanup: unlinkat ...memory.sqlite3: The process cannot access the file because it is being used by another process` | Pre-existing Windows SQLite handle-release flake; the change under test touches no memory-loop code |
| A full `./internal/app/` run reaches the package timeout | Matches the open board row `CI-APP-CONCURRENT-TIMEOUT` (an isolated `-run` selection passes in under a second); reproduced on the **clean stashed tree** as well, so it is not this change |

Neither was hidden: both were reproduced with the working tree stashed
(`git stash push -u -- internal/`) before being attributed.

## 3. Real path against the reported gateway

A temporary, build-tag-isolated probe (`-tags customproviderlive`, deleted after
use; it read the key from the user's own settings document at runtime and never
wrote it anywhere) loaded the reported data directory
(`.workspace/dev-home/settings.yaml`) and drove the production resolver and chat
model:

```text
stored provider="openai-completions" model="DeepSeek-V4.1-Flash" base_url="https://api.wxyungu.com/v1" entry="WXYG"
live provider="WXYG" model="DeepSeek-V4.1-Flash" endpoint=https://api.wxyungu.com/v1
upstream reply: "OK"
--- PASS: TestLiveCustomProvider (0.47s)
```

This is the end-to-end evidence: the selection the user had stored now resolves
to the registered endpoint, the registry credential resolves, and the model
answers from the gateway instead of failing inside the run.

## 4. Repository gate

`just ci` (`ensure-laputa bootstrap-test fmt-check ui-ci vet test diva-recall-test
headless-compile plugin-ci`).

One host-environment failure had to be worked around: `pnpm build` (inside
`ui-ci`) failed with
`[vite:esbuild-transpile] remove C:\Users\com01\AppData\Local\Temp\esbuild-<hash>: Access is denied`,
twice, at different hashes. Re-running with `TMP`/`TEMP` pointing inside the
workspace built cleanly in 20.52s. That is an esbuild temp-file cleanup problem
in this machine's global `%TEMP%`, not a repository defect; the gate was re-run
with the redirected temp directory.

```text
just ci (TMP/TEMP redirected)   exit 1 — see the attribution table below
```

Phase results:

| Phase | Result |
|---|---|
| `ensure-laputa`, `bootstrap-test` | pass |
| `fmt-check` | pass (`gofmt -l` on every changed file lists nothing; `git diff --check` clean) |
| `ui-ci` (typecheck, 692 vitest tests, i18n conformance, build) | pass |
| `vet ./...` | pass |
| `test` | `internal/app` **ok 79.378s**; failing packages are `sdk/codeclient`, `sdk/internal`, `sdk/internal/assembly`, `sdk/internal/conformance` |
| `diva-recall-test`, `headless-compile`, `plugin-ci` | not reached (`test` failed first) |

## 5. Attribution of the gate failures

Every failing package was re-run with the working tree stashed
(`git stash push -u -- internal/`) and failed identically on the clean tree, so
none of them is caused by this change:

| Failure | Cause | Clean-tree check |
|---|---|---|
| `sdk/internal`, `sdk/internal/assembly`, `sdk/internal/conformance` (10 of 12 `sdk/internal` failures) | `symlink ...: A required privilege is not held by the client` — this Windows token has no `SeCreateSymbolicLinkPrivilege` (`whoami /priv` lists none); the pack/UI-seal tests link `ui/node_modules` into an isolated boundary | `go test ./sdk/internal/assembly` → same `TestHashSourceTreeRejectsSymbolicLinks` failure |
| `sdk/internal` (`TestPackAndInspectCheckedInFullUIGateRecipe`) | `open C:\Users\com01\AppData\Local\go-build\...: Access is denied` — the machine's Go build cache path is not writable from this process | same class, same tree |
| `sdk/codeclient` (`TestClientAgainstRealVivyCode`) | `handshake get_state: codeclient: protocol stream ended` — the SDK client against a built `vivy-code.exe` | reproduced on the clean tree: `FAIL agent-vivy/sdk/codeclient 15.993s` with the same message |
| `pnpm build` inside `ui-ci`, first run | `[vite:esbuild-transpile] remove C:\Users\...\Temp\esbuild-<hash>: Access is denied` (twice, different hashes) | host `%TEMP%` cleanup problem, not repository state; the build succeeds with `TMP`/`TEMP` inside the workspace (20.52s) |

## 6. Not run

- `just ui-e2e` — outside `just ci` (board row `CI-E2E-NOT-IN-GATE`) and
  unaffected: no UI, RPC or wire shape changed.
- The three phases after `test` — `test` failed first, for the reasons above.

## 7. Source digest

`internal/sourcehash` hashes every file under `internal/`, so the edits in
`internal/app` and `internal/provider` move the digest of the six
`internal`-rooted conformance suites (`std/tool@v1 vivy/protected-tools`,
`std/tool-world@v1 vivy/mcp-host`, `std/provider-profile@v1`,
`std/context-source@v1`, `std/skill-source@v1`, `std/observer/run@v1
scx/reference-fixtures`). Ordinary `just ci` strips source identity before
comparing, so it cannot report a stale value; the refresh is explicit:

```text
VIVY_RELEASE_CONFORMANCE=1 go test ./sdk/internal/conformance \
  -run '^TestCheckedInProviderConformanceMatchesExecutedSuites$' -count=1
```

First run: `release source digest for vivy/context-source =
1acae684959aac7850437e258069fa2612128b5eceddf44dae7b554933e3d5a9, checked-in
evidence covers aea33bd278880e519754504b1d3fe20a7bf645a60d5210396580f4d4e3e57abc`.

`sdk/internal/assembly/conformance_results.json` was updated (5 occurrences, the
five `internal`-rooted `providerId` rows). Re-run: **no
`release source digest` message remains**, i.e. every checked-in digest equals
the digest computed from the live tree; only the two symlink-blocked UI pack
suites still fail.
