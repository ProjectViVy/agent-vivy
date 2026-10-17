# Verification — provider environment/selection seam hardening

Worktree: `C:\Users\com01\Desktop\agent-vivy-provider-env-fix`, branch
`fix/provider-env-selection-hardening` from `origin/main` `5715950d`.
Toolchain: go1.26.4 windows/amd64, node v24.19.0, pnpm 10.11.0.

`ui/dist` is not committed (`.gitignore` ignores `/ui/dist/*`) and
`ui/embed.go` embeds `all:dist`, so every Go command for `internal/app`,
`internal/rpc` and `ui` needs the directory to exist. A **compile-only stub**
`ui/dist/index.html` (gitignored, never committed) was used for those commands.
The real UI bundle could not be produced in this environment — see below.

## Blocked here: the UI production build (`ui-ci`)

`pnpm build` (in `ui/`) fails before emitting `dist/index.html`:

```
error during build:
[vite:esbuild-transpile] remove C:\Users\<user>\AppData\Local\Temp\esbuild-<hash>: Access is denied.
```

Evidence that this is environmental and predates the change:

- It reproduces on the **pristine worktree** (at `origin/main`, before any source
  edit in this branch).
- `vite:esbuild-transpile` is a `renderChunk` plugin, so it runs on whole chunks.
  A minimal repro isolates the trigger without the repository at all:
  `vite.transformWithEsbuild(<1 MB string>, 'big.ts', {loader:'ts'})` fails with
  the same message, while the same call on a small input — and 64 parallel small
  transforms — succeeds.
- `esbuild` itself is healthy: `esbuild.transform` and
  `esbuild.build({write:false})` both succeed, and Windows Defender real-time
  protection reports `False`.
- A security product is still active: `HipsTray` exited but `HipsDaemon`
  (Huorong / 火绒) kept running, and `fltmc filters` returned
  `Access is denied` (`0x80070005`). The file esbuild failed to remove is
  deletable by PowerShell *after* the failure, i.e. a transient hold on a
  freshly written file — the signature of a file-system filter.
- Redirecting `TEMP`/`TMP` to a fresh local directory changes the reported path
  and still fails, so this is not a stale-temp-file or path issue.

Consequence: **`just ci` cannot complete in this environment** (`ui-ci` →
`ui-core` → `pnpm build`). Its other recipes were run individually, and the UI
gates that do not need the production build were run directly. To close the gap,
run `just ci` where Huorong's daemon is fully stopped or after a reboot; no Go
result below depends on it.

## Reproductions on the pristine tree (before this change)

1. **A — ambiguous environment freeze.** With only `OPENAI_API_KEY` set and
   `config.providers.active: deepseek`, `ModelResolver.Current()` returned
   `provider=aihubmix model=claude-sonnet-4 ready=true frozen=true`. `aihubmix`
   and `openai` both declare `env_key: OPENAI_API_KEY`, and `catalog.Vendors()`
   is name-sorted, so `hits[0]` was the gateway. The session was then frozen, so
   Settings could not correct it.
2. **B1 — a registered provider's key written into a vendor variable.** A
   registry entry with `display_name: openai` on `https://gateway.example/v1`
   made `applySettingsEnv` write `OPENAI_API_KEY="sk-my-gateway"` — the
   operator's gateway key into the embedded `openai` vendor's variable.
3. **C — a default outside the model list.** `settings.Load` accepted
   `default_model: ghost-model` with `models: [real-model]`, and
   `resolveCustomSelection` then adopted `ghost-model` as the selection model,
   which the `Settings → 模型` list cannot render.
4. **B2 — `VIVY_API_BASE` was never withdrawn.** No path cleared it after
   `applySettingsEnv` published it, so a settings write could not undo its own
   address for that process.
5. **D — unbounded discovery.** `ModelListClient.List` bounded the response body
   in bytes but not the number of accepted ids, and had no id-length bound.

## Commands run after the change

All from the worktree root unless stated.

| Command | Result |
|---|---|
| `gofmt -l internal cmd` | empty (no unformatted file) |
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `go test -count=1 ./internal/provider/...` | `ok` 4.7s — includes the three new discovery tests |
| `go test -count=1 ./internal/app/settings/...` | `ok` 3.4s — includes the two new registry tests |
| `go test -count=1 -run 'TestResolver\|TestApplySettingsEnv\|TestApplyBaseURLEnv\|TestResolveStoredSelection\|TestModelResolver' ./internal/app/` | `ok` 1.1s — 33 tests pass, including the five new freeze tests and both new env tests |
| `go test -count=1 -p 1 -timeout 35m -skip '^TestMemoryLoop' ./...` | see below — one package fails, for the UI-build reason |
| `go test -count=1 -timeout 35m -skip '^TestMemoryLoop' ./internal/app/...` | `ok` 312.9s (app), `ok` 3.6s (settings) |
| `go test -count=1 -p 1 ./internal/provider/... ./internal/rpc/...` | `ok` 8.4s (provider), `ok` 312.7s (rpc) |
| `cd ui; pnpm typecheck` | exit 0 |
| `cd ui; pnpm test` | 92 files / 692 tests passed |
| `cd ui; pnpm build` | **blocked** (environment, above) |

### Full-suite result

`go test -count=1 -p 1 -timeout 35m -skip '^TestMemoryLoop' ./...` → exit 1,
with **exactly one failing package**:

```
FAIL	agent-vivy/sdk/internal/conformance	339.001s
    frontend_v1_test.go:273: sdk: build Web UI with generated Assembly: exit status 1: vite v7.3.6 building client environment for production...
    [vite:esbuild-transpile] remove C:\...\Temp\esbuild-<hash>: Access is denied.
    reproduction_test.go:41: checked-in conformance results differ from executed suites
```

That package shells out to `vite build`, so it fails for the same blocker
recorded at the top of this file — the checked-in conformance digest then
mismatches because the UI suites could not execute. `agent-vivy/ui` and every
other package reported `ok`. The same run was repeated per affected tree with
the repository's own flags (table above) and all four are green, so the conformance
failure is not attributable to this change.

Observation (pre-existing, outside the gate): running `internal/app` *without*
`-skip '^TestMemoryLoop'` fails `TestMemoryLoopActivityFailurePreservesSourceAndArchiveRetry`
and then reaches the Go default 10-minute timeout — which is presumably why
`just test` skips that prefix (`docs/TODO.md` `CI-APP-CONCURRENT-TIMEOUT` covers
the surrounding instability). No memory or loop code is touched here.


### Regression demonstration

With the Fix C implementation removed but its tests kept
(`git stash push -- internal/app/settings/settings.go`):

```
--- FAIL: TestProviderRegistryReconcilesDefaultModelIntoModels (0.02s)
    settings_test.go:1331: models after load = [real-model], want [real-model ghost-model]
--- FAIL: TestUpsertProviderReconcilesDefaultModelIntoModels (0.00s)
```

`git stash pop` restores the fix and both tests pass. For A, B and D the
pre-change behaviour is the reproduction list above; the new tests reference
identifiers that only exist after the change (`FrozenAmbiguity`,
`MaxModelListEntries`, `appliedBaseURL`), so they cannot compile against the
pre-change code.

## Real-path smoke

The real composition root was started twice from this worktree
(`go run ./cmd/vivy`, an untracked `config.yaml` pointing at
`127.0.0.1:8788` and a worktree-local `data/smoke*`; both were deleted
afterwards). The environment was the exact reproduction case: `OPENAI_API_KEY`
set, no `VIVY_PROVIDER`, `config.providers.active: deepseek`.

Run 1 — ambiguous environment:

```
{"level":"WARN","source":{"function":"agent-vivy/internal/app.NewWithAssembly","file":".../internal/app/app.go","line":391},"msg":"environment provider selection is ambiguous","detail":"environment credentials OPENAI_API_KEY are set for several providers (aihubmix, openai); set config.providers.active or VIVY_PROVIDER to choose one, so the settings document stays in effect"}
{"level":"INFO","source":{"function":"agent-vivy/internal/app.(*App).Run.func1","file":".../internal/app/app.go","line":2275},"msg":"vivy starting","addr":"127.0.0.1:8788"}
```

The process started normally (not frozen to `aihubmix`) and said why. Before the
change the same environment froze the session to `aihubmix` with no diagnostic.

Run 2 — the same environment plus `VIVY_PROVIDER=aihubmix`:

```
{"level":"INFO",... "msg":"vivy starting","addr":"127.0.0.1:8788"}
```

No ambiguity warning: the explicit choice is honoured and the session freezes to
the named vendor, exactly as documented.

Limits of this smoke: it exercises process startup, the environment decision and
the operator-visible warning. It does not drive a run, the Face, or the settings
write RPCs, and no browser was available (see above). A full run against a live
provider was not attempted: no provider credential for a real endpoint was
configured, and `OPENAI_API_KEY=sk-smoke-ambiguous` is a placeholder.

## Temporary artifacts created for verification (not committed)

- `ui/dist/index.html` — gitignored compile stub so the Go packages that embed
  the UI can build. The real `pnpm build` output replaces it; delete it once a
  UI build succeeds in this worktree.
- `config.yaml`, `data/smoke`, `data/smoke2` — smoke run scratch; removed.


## Not run, and why

- `just ci` as a whole, and `just ui-ci`: blocked by the UI production build
  (above). Every other recipe it chains was run individually or is listed here.
- `scripts/test-memory-loop.ps1` (the second half of `just test`): excluded, as
  this change touches no memory, loop, Eino or runtime code.
- Browser acceptance at `http://127.0.0.1:3015`: this session has no browser
  tooling, and the split pair currently on `:3015`/`:8787` belongs to another
  active lane (one writer per tree, `AGENTS.md`). The real composition root was
  exercised over its RPC interface instead (above).
