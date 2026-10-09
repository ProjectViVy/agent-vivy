# Verification — bash redirect classification

- `go test ./internal/tools/` → ok (all `TestClassifyShellScriptTiers` cases,
  including the 21 new redirect cases)
- `go test ./internal/runtime/ -run TestBash -v` → ok, including:
  - `TestBashBackendQuotedRedirectsAndHeredocsWriteWorkspaceFiles`: real bash
    runs `cat > probe.txt <<'EOF'…EOF` and `printf … > 'quoted.txt'` through
    `CommandBackend.Execute`; both files read back from the run workspace
    with content `benchmark payload\n`.
  - `TestBashBackendQuotedRedirectStillCannotEscapeWorkspace`: real bash
    invocations of `> '/tmp/…'`, `> '../…'`, and heredoc into `../…` are all
    denied by the classifier defense-in-depth; neither escape file exists.
- `gofmt -l` over `internal/tools/ internal/runtime/` → clean; `go vet` clean.
- Full `go test -timeout 35m ./...` (via `just ci`) → all packages ok,
  including `internal/runtime` 62.9s, `internal/rpc`, `sdk/internal` 365s.
- `just ci` → green end to end: ensure-laputa, bootstrap-test 6/6, fmt-check,
  ui-ci (pnpm typecheck + vitest + vite build), i18n completeness +
  cross-face, vet, full go test, headless-compile, plugin-ci (all modules).
- Environment notes: VM ran `just ci` under `GIT_CONFIG_GLOBAL=/dev/null`
  (git-manager proxy origin vs `ensure-laputa` expectation); laputa sibling
  checked out at the `laputa-source.lock.json` pin `ff3936f4`; internal
  digest re-pinned after all `internal/` edits were final.
