# Verification

## Targeted checks

- `go test ./sdk/tui/command ./sdk/tui/view` — PASS.
- `go test ./internal/tui ./internal/rpc ./internal/runtime` — PASS.
- `cd faces/tui; go test ./...` — PASS.
- Coverage includes catalog validation, bundle/custom endpoint identity, unrelated-settings preservation, runtime startup fencing and cleanup, candidate deduplication/current ordering, filtering, terminal-safe labels, URL non-rendering, busy/queue/read-only/capability guards, stale result rejection, non-dismissible in-flight selection, built-in/packed RPC parity, and post-selection sidebar refresh scheduling.

## Product gate

- Final code-stable `just ci` — PASS: formatting, UI typecheck, 201 UI tests,
  production UI build, Go vet/full tests, headless compilation, and every
  plugin/face vet+test slice passed.
- `just vivy-code` and `vivy-code.exe --help` — PASS; the independent
  headless-tagged executable built and printed the private-instance VIVY CODE
  contract. The generated executable was moved out of the worktree root into
  ignored `.workspace/build-artifacts/` after verification.

## Real path

- A native Windows PTY launch was attempted with isolated
  `VIVY_USER_HOME=.workspace/model-picker-smoke-home`, but this execution host
  failed before process startup with `CreateProcessW` OS error `-1073283067`
  (`FormatMessageW` error 317). The smoke home was not created and no
  interactive pass is claimed. The real Bubble Tea transition path is covered
  by shared/built-in/packed tests, while executable construction and its
  non-interactive `--help` path passed.

## Review

- Three GPT-5.6-LUNA MAX specialists reviewed runtime/RPC safety, Crush interaction semantics, and built-in/packed parity. Their merge-blocking findings were addressed: in-flight Esc race, terminal label injection, global-scope wording, stale terminal cleanup, config-default ghost endpoints, complete pre-baked bundle models, and sidebar refresh verification.
