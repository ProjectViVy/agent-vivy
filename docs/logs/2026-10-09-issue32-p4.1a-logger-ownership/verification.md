# P4.1a verification

## Baseline red

The focused regressions first showed that `WithLogger` did not exist,
`dailyFile.Write` reopened after Close, host close left profile A as the
process default, and a failed startup did not exercise the host-owned sink.

## Green

Commands run with Go 1.26.4 (`GOTOOLCHAIN=local`):

```text
go test ./internal/app ./internal/logging ./sdk/host/v1 -run 'Test(AppUsesInjectedLoggerAtComposition|HostLoggerProfileReopen|DailyFileCloseIsPermanent|HostFailedOpenRestoresOwnedLogger|HostFailedOpenClosesOwnedSinkAndReleasesSlot)' -count=1 PASS
go test ./internal/app -count=1 PASS
go test -race ./internal/logging ./sdk/host/v1 -count=1 PASS
git diff --check PASS
```

Host tests use the real sealed-generation fixture. Profile A is closed before
profile B opens; profile B composition and marker records stay in B's directory,
and writing through A's retained logger leaves A's closed file size unchanged.
The injected embedded-startup failure logs after `logging.Setup`, verifies the
host slot is available for a second Open, and checks that a later process
default is preserved. `dailyFile` separately verifies `io.ErrClosedPipe` and
that a date rollover cannot create a new file after Close.

`pnpm --dir ui build` could not run: the checkout's `ui/node_modules` directory
contains no installed packages and the configured `registry.npmmirror.com`
could not be reached. To compile the gateway-less Go tests, a temporary
`ui/dist/.keep` satisfied the embed pattern; it was removed after testing. No
UI source or generated UI files changed. Native Windows file-handle behavior
remains for P7's platform acceptance.
