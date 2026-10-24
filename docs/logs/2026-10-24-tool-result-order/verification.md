# Verification

Toolchain: `GOTOOLCHAIN=go1.26.4`, `GOPROXY=https://goproxy.cn,direct`, serial
`-p 1`. Host: macOS (arm64).

## Unit tests

```text
go test -count=1 -p 1 ./internal/runtime/ -run 'TestPairToolTurns|TestStabilizeToolTurns|TestBuildRunContext'
ok  agent-vivy/internal/runtime
```

New cases:

- `TestPairToolTurnsOrdersResultsByCall` — calls t1/t2 with results journaled
  t2 then t1; the projection emits t1's result, then t2's.
- `TestPairToolTurnsOrdersResultsByCallAfterSteer` — the same group with a
  steer written between results; output is t1 result, t2 result, steer.

## Package tests (merged tree)

```text
go test -count=1 -p 1 ./internal/tools/...
ok  agent-vivy/internal/tools

go test -count=1 -p 1 ./internal/runtime
ok  agent-vivy/internal/runtime   66.982s
```

One earlier invocation of `./internal/tools/... ./internal/runtime` failed
inside `internal/runtime`; the log tail did not capture the failing test name
and two subsequent standalone runs of the same package passed. Recorded as an
unidentified flake, not as green evidence for that first run.

```text
gofmt -l internal/runtime internal/tools   # empty
git diff --check                           # clean
```

## Real path

The split dev pair was restarted from the merged tree
(`bash scripts/dev.sh --no-browser`, home `.workspace/dev/home`):

- `127.0.0.1:8787/rpc/bootstrap` → 200
- Vite `127.0.0.1:3015` → ready

Before the fix, the same session failed every retry with
`400 ... tool call result does not follow tool call (2013)`; after the restart
the user is re-running that session.

## Not run

`just wiring-ci`, `just kernel-ci` and full `just ci` require a Linux checkout
and Laputa; they are not runnable on this macOS host. The Linux kernel gate
remains the landing gate for this change.
