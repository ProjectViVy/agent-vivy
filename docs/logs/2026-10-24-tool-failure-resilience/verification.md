# Verification — tool-failure resilience

Host: macOS (darwin/arm64), Go pinned by `GOTOOLCHAIN=go1.26.4` (the ambient Homebrew
Go 1.27.1 breaks `golang.org/x/net v0.54.0` — `http2.TrailerPrefix` is excluded under
`//go:build !(go1.27 && !http2legacy)`). Lane:
`.worktrees/tool-failure-continuation` on `fix/tool-failure-continuation` from
`stable` @ `9697dd0e`.

## Run and green

```text
gofmt -w <changed files>                       # no output; gofmt -l internal schemas docs is empty
GOTOOLCHAIN=go1.26.4 go vet ./internal/runtime/... ./internal/app/... \
    ./internal/storage/... ./internal/tools/... ./internal/domain/...
                                               # clean
GOTOOLCHAIN=go1.26.4 go build ./...            # clean (after copying ui/dist into the worktree)
GOTOOLCHAIN=go1.26.4 go test -p 1 -count=1 ./internal/...
                                               # all packages ok; internal/runtime 67.0s,
                                               # internal/app 18.8s, internal/rpc 19.1s
GOTOOLCHAIN=go1.26.4 go test -p 1 -count=1 ./internal/runtime/
                                               # ok (second full run after the last edits)
git diff --check                               # clean
```

New/updated tests, all deterministic (no live network; every proxy case uses an
`httptest` proxy plus an injected resolver):

- `internal/runtime/web_network_test.go` — fetch through an HTTP proxy succeeds and the
  proxy sees the absolute-form request; a `NO_PROXY=example.com` entry leaves a fetch of
  `http://other.test/` proxied (`TestWebFetchProxiesHostOutsideNoProxy`, which fails
  against the previous probe-derived allow set: the probe host matched `NO_PROXY`, the
  allow set came back empty, and the proxy's own loopback address was then refused as a
  private target); a private target behind the same proxy is refused with zero proxy
  hits; a redirect into a private host is refused and the hop is never sent.
- `internal/runtime/download_test.go` — `TestDownloadStreamsThroughConfiguredProxy`.
- `internal/runtime/tool_failure_test.go` — `TestModelLegToolFailureFallback`
  (plain error → `tool_error`; `*url.Error` / `net.Error` / per-call timeout →
  `network_failed`; typed `ArgError` keeps `invalid_arguments`; parent cancellation,
  `context.Canceled`, native interrupt and refusal sentinels stay unclassified) and
  `TestToolFailureTransportErrorBecomesToolResult`, which replaces the old
  `...StaysFatal`: the run completes, the row is `recoverable/tool_error/unknown`, the
  tool ran once, and the next model turn's input carries the diagnostic. The typed
  §5 table (`TestToolFailureClassification`) is unchanged, so the allowlist contract
  still holds; `internal/runtime/skills_backend_test.go` was not weakened.
- `internal/runtime/tooladapter_test.go` — an unselected tool now returns a refused
  result with zero invocations; `TestToolAdapterHookDenyRefusesCall` covers a hook
  `deny`.
- `internal/runtime/hooks_test.go` — a deny asserts `ErrHookDenied`; timeout/malformed
  envelopes still assert `ErrHookBlocked`.
- `internal/runtime/http_request_test.go` — the host-allowlist denial is still outside
  the typed classifier and now satisfies `errors.Is(err, ErrPolicyDenied)`.
- `internal/runtime/toolmount_session_test.go`, `internal/runtime/work_control_tools_test.go`
  — updated to the new refusal contract: the hidden-tool leak and the unadmitted
  `report_goal` produce `refused/policy_denied/not_executed`, the Goal is untouched,
  and the run completes instead of failing.
- `internal/runtime/child_oneshot_test.go` — a journaled `child.failed{message,detail}`
  composes into `message + "\nCause: " + detail`; the payload validates against
  `schemas/events/payloads/child.failed.json`, and a 1025-byte `detail` is rejected.
- `internal/storage/tool_operations_test.go` — `network_failed` and `tool_error` are
  accepted reasons.

## Not run, with reasons

- `just wiring-ci`, `just kernel-ci`, `just d02-ci` — **unrun**. These require an
  independent Linux-filesystem checkout with its own pinned Laputa; macOS cannot
  certify them (see `.agents/skills/vivy-advance-dev-env`). Left to the owner.
- `./sdk/...` pack/conformance tests — **environment-blocked in this worktree, not
  code-related**: `sdk/internal/assembly` fails on a missing
  `ui/node_modules/.bin/tsc` (the worktree has no installed UI toolchain), and
  `sdk/host/v1` / `sdk/internal` / `sdk/internal/conformance` fail inside
  `go mod tidy` because module downloads are blocked on this host
  (`proxy.golang.org ... i/o timeout`). The same `sdk/internal/assembly` package passes
  on the root `stable` tree, which has a warm toolchain. No file in this delivery
  touches `sdk/`, `ui/`, or the pack path.
- Real-path browser smoke at `http://127.0.0.1:3015` — **pending**. The split pair
  needs `ui/node_modules` in the lane; the steps are written out in `acceptance.md`
  for the owner's retest (proxy fetch, delegated child fetch, broken-proxy
  `network_failed` continuation, private-target refusal, `child.failed.detail`).
- PostgreSQL conformance, Windows native owner checks — out of scope for this lane.

## Dependency note

`golang.org/x/net v0.54.0` moved from the indirect block to the direct require block
because `internal/runtime/web_network.go` now imports
`golang.org/x/net/http/httpproxy`. `go mod tidy` could not run (blocked module
downloads), so the promotion was made by hand and `go build ./...` was re-verified.
