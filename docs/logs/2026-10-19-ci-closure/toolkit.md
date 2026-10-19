# S3: Portable Toolkit subprocess fixtures

## Scope and reproduced cause

- Isolated lane: `ci-repair-toolkit`, based on `e46d332362608062b44509810628e66bce97a712`; no delivery commit or publication is authorized.
- Broad content search located the executable POSIX fixtures in `internal/runtime/bundle_hooks_matrix_test.go`, specifically `TestToolkitCapturedHookFailureMatrix`. Searches included `internal/bundles`, Toolkit modules, SDK tests, and app/runtime tests, not just Toolkit-named files.
- The matrix uses real `BundleHookDispatcher` execution and exact captured consent, but its commands use `cat`, `printf`, POSIX redirection, and `sleep`. Production `runHookProcessScoped` correctly chooses `cmd /c` on Windows and `sh -c` elsewhere. The fixture, not the production shell selection, owns the portability defect.
- All repairs remain in the assigned runtime Toolkit-test boundary; no scope expansion or production/runtime/owner-guard change is required. Existing policy, approval, exact-consent, revocation, environment-isolation, and terminal-run authority assertions must remain intact.
- Eino capability check: test-only subprocess fixtures; no LLM/ADK/runtime capability or adapter change is proposed.

## Red evidence

All Go commands use process-local `GOTOOLCHAIN=go1.26.4`, `GOPROXY=https://proxy.golang.org,direct`, `GIT_CONFIG_GLOBAL=/dev/null`, and human author/committer environment (`大湿 <1136523089@qq.com>`). No Git configuration is edited.

A temporary Python `go test -exec` wrapper outside the checkout creates a temporary directory containing only an `sh` symlink, replaces the test process's `PATH` with that directory, and runs the real compiled test binary. This retains the production platform shell while removing external `cat`, `printf`, and `sleep`; shell-built-in `printf` remains available on Linux, so this is not native Windows emulation.

```text
go test -exec '/usr/bin/python3 /tmp/vivy-toolkit-shell-only.py' ./internal/runtime -run '^TestToolkitCapturedHookFailureMatrix$' -count=1 -v
```

1. Unmodified baseline: FAIL, exit 1. `allow` fails at line 65 with `sh: 1: cat: not found`. The existing timeout case incorrectly passes on a missing executable rather than an actual timeout.
2. Before replacing any command, strengthen failure-reason assertions for exit 1, invalid updated input, and timeout. Rerun the same command: FAIL, exit 1. `allow` still fails; `other_exit_fails_closed` reports `cat: not found` instead of `hook exited 1`; `timeout_fails_closed` reports `cat: not found` / `sleep: not found` instead of `timed out`. This demonstrates why merely asserting a blocked result can mask a broken fixture.
3. The first helper-process run of the same command failed only its literal `timed out` expectation: the existing runner reports the context-killed process as `hook exited -1` on this path. Production error mapping was not changed. An intermediate caller-deadline assertion passed all eight cases, and temporarily restoring `cat >/dev/null; sleep 2` for the timeout case made it fail with `context: <nil>` plus missing-utility errors. The final design below is stronger: it tests the declared Hook timeout itself, not an outer caller deadline.
4. With the final timeout marker design, temporarily mutate only the owning `newBundleHookFixture` in `internal/runtime/bundle_hooks_test.go:45` from `TimeoutMs: 1000` to `3000`. Run the command below: FAIL, exit 1. The child exits successfully before either deadline, so the dispatcher returns `Decision:allow`, `<nil>`, where the matrix requires `ErrHookBlocked`. Restore `1000` immediately; no mutation remains in the patch. This proves the final case detects a missing/overlong declared Hook deadline rather than passing on generic subprocess failure.

```text
go test -exec '/usr/bin/python3 /tmp/vivy-toolkit-shell-only.py' ./internal/runtime -run '^TestToolkitCapturedHookFailureMatrix$/timeout_fails_closed' -count=1 -v
```

## Implementation and green evidence

- Only `internal/runtime/bundle_hooks_matrix_test.go` changes executable test code. It reuses `os.Executable` as a real child through the unchanged production platform shell (`cmd /c` or `sh -c`), with explicit executable quoting. No POSIX command utility or Bash fallback is required.
- The child reads and validates the complete real stdin payload (event, run, tool, arguments) and resolved `HOOK_ALLOWED` binding before producing any outcome. It emits real plain-text/JSON stdout, exit-2 stderr, exit 1, invalid input, or timeout behavior. Existing decision, rewrite, consent, and blocked-error assertions remain; additional failure-cause assertions reject broken fixtures.
- The timeout child writes `timeout-fixture-waiting` to stderr after input validation, waits deliberately for two seconds, then exits successfully. The unchanged fixture declares a one-second Hook timeout; the matrix's enclosing chain budget is four seconds, so it cannot stand in for that declaration. A real Hook timeout must kill the child and preserve its stderr marker. This timer is the explicit timeout stimulus, not synchronization or a sleep to hide a race.
- The child alone sets `GORACE=atexit_sleep_ms=0` to remove the race detector's post-exit delay from the one-second Hook execution budget. Race detection is not disabled and race reports still fail the matrix.
- App compilation uses only a private copy of the already-built immutable `memory-ci-closure/agent-vivy/ui/dist` at this lane's `ui/dist`. No UI rebuild or shared mutable `node_modules` is used.

All commands below passed with exit 0. Focused normal/race runtime checks were rerun after restoring the timeout mutation and finalizing the fixture.

| Command | Result |
| --- | --- |
| `go test -exec '/usr/bin/python3 /tmp/vivy-toolkit-shell-only.py' ./internal/runtime -run '^TestToolkitCapturedHookFailureMatrix$' -count=1 -v` | Final marker design: all eight cases PASS, package 1.920s; no external `cat`, `printf`, or `sleep` available. |
| `go test ./internal/bundles ./internal/modules/toolkits ./internal/runtime -run 'Test(Toolkit\|BundleHook\|BundleMCPConsent\|ScriptHook)' -count=1` | Final restored source: PASS, packages 1.488s / 0.004s / 3.333s. |
| `go test ./internal/bundles ./internal/modules/toolkits -count=1` | Full owning Toolkit packages PASS, 1.458s / 0.004s. Fixture commits inherit the required human author and committer environment. |
| `go test ./internal/app -run TestToolkit -count=1` | PASS, 0.961s; includes human-origin policy/approval cases, real Service.Run consent capture, lifecycle/recovery, and `TestToolkitPreviewAfterViewingTerminalRun`. |
| `go test -race -exec '/usr/bin/python3 /tmp/vivy-toolkit-shell-only.py' ./internal/runtime -run 'Test(Toolkit\|BundleHook\|BundleMCPConsent\|ScriptHook)' -count=1` | Final restored source: PASS, 22.268s, no race report, shell-only PATH. |
| `go test -race ./internal/app -run TestToolkit -count=1` | PASS, 8.731s, no race report; policy/approval and terminal authority assertions remain green. |
| `go vet ./internal/runtime` | PASS, no diagnostics on final source. |
| `go vet ./internal/bundles ./internal/modules/toolkits` | PASS, no diagnostics. |
| `gofmt -w internal/runtime/bundle_hooks_matrix_test.go`; `gofmt -l internal/runtime/bundle_hooks_matrix_test.go` | Formatting applied; final list is empty. |
| `go test -c -o "/tmp/vivy toolkit's runtime.test" ./internal/runtime` followed by `python3 /tmp/vivy-toolkit-shell-only.py "/tmp/vivy toolkit's runtime.test" -test.run='^TestToolkitCapturedHookFailureMatrix$' -test.count=1 -test.v` | PASS, all eight cases, 1.90s; executable path has both whitespace and an apostrophe. |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c -o /tmp/vivy-toolkit-runtime-windows.test.exe ./internal/runtime` | PASS: Windows test binary cross-compiles from final source. This is not native Windows execution. |

The shell-only wrapper was created outside the repository. Its full reproducible contents are:

```python
#!/usr/bin/env python3
"""Execute a compiled Toolkit test with the platform shell but no POSIX utilities."""
import os
import shutil
import subprocess
import sys
import tempfile

with tempfile.TemporaryDirectory(prefix="vivy-toolkit-shell-only-") as root:
    shell = shutil.which("sh")
    if not shell:
        raise RuntimeError("sh is required for the Linux shell-only fixture check")
    os.symlink(shell, os.path.join(root, "sh"))
    env = os.environ.copy()
    env["PATH"] = root
    print("Executing Toolkit tests with PATH containing only sh (no cat/printf/sleep)", flush=True)
    sys.exit(subprocess.call(sys.argv[1:], env=env))
```

## Handoff

Export only the changed matrix and this log with explicit `git add -N` for the new log, then `git diff --binary`. Delivery patch: `/home/ubuntu/memory-write-model/ci-closure-results/toolkit.patch`. No delivery commit is created. `git diff --check` passes; HEAD remains baseline `e46d332362608062b44509810628e66bce97a712`. The original fixture helper, production subprocess runner, owner guard, dependency pins, SDK code, and app authority tests are unchanged.

## Untested boundaries

- This worker runs on Linux, not a native Windows host. Native Windows execution and aggregate acceptance belong to S6.
- Full `just ci`, unrelated repairs, PostgreSQL, dependency-pin changes, SDK packing, and UI behavior are outside this Story's focused acceptance.
- No production `data/`, journals, `~/.vivy`, original-checkout edits, shared mutable Laputa, pushes, PRs, merges, deployments, or delivery commits are used.
