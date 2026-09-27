# Verification

- RED: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/rpc -run '^TestPublicPlanSubmitRejectsCallerOriginWithoutMutation$' -count=1` failed because the public `plan/submit` handler accepted the request without returning an RPC error.
- GREEN: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/rpc -run '^(TestPublicPlanSubmitRejectsCallerOriginWithoutMutation|TestBuildWorkMutationRejectsModelOnlyPlanSubmit)$' -count=1 -v` passed. The handler test checks the exact method-not-found response and deep equality of pre/post `WorkState` and full `WorkEvent` replay.
- Source digest: `& 'C:\Program Files\Go\bin\go.exe' run ./sdk/internal/cmd/source-hash internal ''` returned `86f8a667b8ee42bb26827a990eac5912d5063a56d977bc255bd6c9134c334552`; the five matching internal conformance entries were refreshed. SDK conformance passed in the full CI run.
- Full gate: with `C:\Program Files\Go\bin` prepended to `PATH`, `just ci` exited 0. This covered Go formatting, UI typecheck and 400 tests, UI build and i18n checks, `go vet ./...`, `go test -timeout 20m ./...`, headless compile, and all plugin/face vet and test gates. The UI build emitted its existing chunk-size advisory; UI tests emitted expected fixture/error-boundary diagnostics.
- `git diff --cached --check` passed.
- Live PostgreSQL conformance was not exercised because `VIVY_POSTGRES_TEST_DSN` was unset. The PostgreSQL package tests included in `just ci` passed.
- Browser smoke was not run: ports 8787 and 3015 were already listening in the shared environment. Those processes were left untouched. This patch has no visible UI behavior change; UI typecheck, tests, and build passed.
