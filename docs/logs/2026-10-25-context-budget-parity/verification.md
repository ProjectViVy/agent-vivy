# Verification

All commands ran in the root checkout on `stable`, with the pinned toolchain
(`GOTOOLCHAIN=go1.26.4`). No live provider call was made; the tests use local
fakes and an in-process HTTP server only.

## Formatting and static checks

```text
gofmt -l .            (no output)
go vet ./...          VET_OK
git diff --check      DIFF_OK
```

## Tests

```text
ok  agent-vivy/internal/config            0.108s
ok  agent-vivy/internal/runtime          71.465s
ok  agent-vivy/internal/runtime/compaction 0.304s
ok  agent-vivy/internal/app              20.227s
ok  agent-vivy/internal/app/settings      1.044s
ok  agent-vivy/internal/provider          0.667s
ok  agent-vivy/internal/config            0.115s
```

`internal/config` was run first and on its own because the default changed:
`TestDefaultRuntimeConfig` now pins `MaxContextBytes == 1<<20`, and
`config.example.yaml` is parsed by the existing example-config tests.

New coverage:

- `internal/runtime/child_context_budget_test.go`
  - `TestChildActivationUsesTheEngineContextBudget`: a 300 KiB delegated brief
    is admitted under the engine's 1 MiB budget and rejected with
    `ErrContextBudgetExceeded` under a 256 KiB one — the number that produced
    the reported failure now passes, and the old ceiling still fails closed
    when configured explicitly.
  - `TestChildActivationMailboxRidesTheSameBudget`: with a 512 KiB budget the
    feed includes a 100 KiB pending parent message and stops before a 700 KiB
    one; the oversized message is skipped, not fatal.

Existing child tests continue to pass unchanged (`child_activation`,
`child_retry_binding`, `child_transcript`, and the `internal/app`
sub-agent suites), which pins that removing the child-only ceiling did not
change any other child feed behaviour.

## Build

```text
go build ./...        BUILD_OK
```

## Not run, and why

- **`just ci` / `just kernel-ci`**: full-product gate, not required for a
  default-value and one-constant change; the package set above covers every
  touched owner (`internal/config`, `internal/runtime`, `internal/app`).
- **Split dev pair (`just run` + `pnpm dev`)**: no UI or RPC surface changed.
  The acceptance step that proves the behaviour is a real delegated child run,
  recorded in `acceptance.md` for the owner to perform against a live provider.
- **Race detector / Windows native run**: unchanged concurrency paths; the
  edited code adds no goroutine, channel or shared-state access.
