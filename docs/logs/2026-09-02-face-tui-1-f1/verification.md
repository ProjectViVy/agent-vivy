# Verification — FACE-TUI-1 F1

## New tests (`internal/app/facehost_test.go`)

```
go test ./internal/app -run 'TestLoopbackControlCompletesApprovedConversation|TestGatewaylessRunWithoutFaceCancelsDurably' -count=1 -v
--- PASS: TestLoopbackControlCompletesApprovedConversation (2.28s)
--- PASS: TestGatewaylessRunWithoutFaceCancelsDurably (1.27s)

go test ./internal/app -run 'TestLoopbackControlCompletesApprovedConversation|TestGatewaylessRunWithoutFaceCancelsDurably' -count=3 -race
ok  agent-vivy/internal/app  11.682s
```

The first test covers the F1 success criterion: in a composition with no embed or listener,
an in-process JSON-RPC client follows the web face's method chain (initialize → session/create
→ turn/start → approval/list → approval/respond(approved) → run/get completed), with zero
`tool.finished` errors in the journal and the final text in session/messages. The second
test covers the no-UI approval failure path: approval suspends without a response →
run/cancel receives cancelled → gateway-less `App.Run` returns cleanly.

## Static + package

```
gofmt -l internal/app/          # clean after gofmt -w facehost_test.go
go vet ./...                    # clean
go build ./...                  # clean
go test ./internal/app -count=1 # ok
```

## just ci

```
( just ci > /tmp/ci-facehost-f1.log 2>&1; echo "CI-EXIT:$?" >> /tmp/ci-facehost-f1.log )
CI-EXIT:0   # fmt-check, ui-ci, vet, test, headless-compile, plugin-ci — 0 FAIL lines
```

The headless-compile step also proves that the `vivy_headless` composition still compiles
under `WithoutGateway()`.

## Real-path note (smoke)

- F1 is a composition-layer capability with no visible UI change — browser smoke at `:3015`
  is not applicable (the web-face path is untouched; the embedded-UI tests in `just ci` and
  `TestRPCBootstrapRoutePrecedesUIShell` regression-pin the default gateway composition).
- Executable-surface smoke = the composition tests themselves: `DialControl` uses the real
  `controlHandler` + real journal + real sqlite storage + scripted model endpoint (the frozen
  ENV session points `VIVY_API_BASE` at a local SSE server), which is the future real driver
  path for the headless face.
- A regression found and fixed during development: the zero-value
  `appOptions.gateway` caused `httpServer == nil` in the default composition and was caught
  by the existing `TestRPCBootstrapRoutePrecedesUIShell` (full-package test). The initial
  run only covered the new tests and missed it; the full-package run remains the process
  precedent.
