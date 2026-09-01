# Verification

Commands run from the worktree root (`agent-vivy-vc0`), 2026-09-01:

- `gofmt -l` on all touched files → clean (after `gofmt -w`).
- `go vet ./internal/tools/ ./internal/pluginhost/ ./internal/runtime/ ./internal/app/ ./sdk/...` → ok.
- `go test ./internal/tools/ ./internal/pluginhost/ ./internal/runtime/` → ok
  (tools 1.420s, pluginhost 0.112s, runtime 87.694s).
- `cd plugins/lsp && go vet ./... && go test -race ./...` → ok 1.111s
  (module-internal gate per vivy-plugin-five; plugins/lsp is a nested module
  outside the `just ci` scan, same reason as slices 1-3).
- `just ci` → **CI_EXIT=0** (kernel gate re-run because internal/ changed:
  full Go suite + importlint + 195 UI tests + vite build).
- `go build -o vivy-sdk.exe ./sdk && vivy-sdk verify plugins/lsp` → ok.
- `vivy-sdk pack --with lsp` → **gen_c569df92d8fbfc1f** (vivy.exe with lsp
  linked; the observer is a plugin capability, not a tool, so the manifest
  tool list is unchanged: the five lsp_* tools).

## Test evidence chain

- `TestWriteToolAttachesDiagnostics` / `TestPatchToolAttachesDiagnostics`:
  mutation result JSON carries the `diagnostics` field; source receives the
  touched path.
- `TestMutationToolsSkipDiagnosticsWhenUnchangedOrUnwired`: Changed=false
  skips the source; unwired ops and silent sources leave the result
  unchanged (no field).
- `TestFilesystemBackendWriteDiagnosticsForwarding`: backend forwards to
  the wired source, nil when unwired.
- `TestDiagnosticBridgeCollectsToolWorldObservers` / `...NilAndEmptyInputs`:
  tool-world observers collected in order, channel-seam and non-observers
  skipped, blank lines dropped, nil bridge/paths safe.
- `TestObserveWriteBackfillsDiagnostics` (in-module, full Env.Spawn pipe
  path with the fake LSP server): backfill returns the published
  diagnostic for the .go file, skips .txt without spawning a server,
  reuses the server on the next mutation and reports nothing on the clean
  publish. `TestFormatDiagnosticLinesCapsAndCounts`: 30-line cap + "... N
  more".

## Smoke exceptions (recorded, same as slices 1-3)

- gopls / typescript-language-server / pyright-langserver / rust-analyzer
  are not installed on this machine → a real-server end-to-end run is not
  demonstrated. Substitute evidence: the fake-server e2e above exercises
  the full spawn → handshake → didOpen/didChange → publish path. Manual
  live steps are in acceptance.md.
- The write-approval UI linkage is unchanged (backfill is post-approval in
  the same run) and needs no separate demo; no UI change → no `:3015`
  browser smoke for this slice.
