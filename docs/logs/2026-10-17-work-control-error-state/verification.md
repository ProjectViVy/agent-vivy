# Verification — 2026-10-17-work-control-error-state

All commands ran from the repository root unless noted. The running dev pair
(`vivy-backend` on 127.0.0.1:8787, Vite on 127.0.0.1:3015) was left running and
untouched.

## 1. Reproduction and fix, at the real transport level

A Node script speaks the same JSON-RPC WebSocket the browser uses
(`/rpc/bootstrap` → socket → `initialize`), then replays
`store.selectSession`: bind the connection to session A (`session/get A`), then
fire `session/messages B`, `session/todos B` and `session/work/get B`
concurrently.

Before — against the dev backend still running the old code (origin
`http://127.0.0.1:3015`):

```
session/messages -> ok
session/todos  -> ok
session/work/get -> ERROR -32009: session is not bound to this connection
session/work/subscribe -> ok
plan/enter for an unselected session -> ERROR -32009: session is not bound to this connection
```

After — against a control plane built from this change, isolated on
`127.0.0.1:8788` with `VIVY_USER_HOME` in `%TEMP%` and `VIVY_ADDR` overridden,
so no production journal was touched:

```
session/messages -> ok
session/todos  -> ok
session/work/get -> ok
session/work/subscribe -> ok (work_sub_6cd8c891888e9af4)
plan/enter for an unselected session -> ERROR -32009: session is not bound to this connection
```

The isolated backend was stopped and its data root deleted afterwards;
`127.0.0.1:8787` still belongs to the original process (pid 13428).

## 2. Go gates

```
go test ./internal/rpc/ -run 'TestWorkReadEstablishesPeerSessionBinding|TestPlanEnterUsesAuthenticatedPeerSessionBinding' -count=1
ok  	agent-vivy/internal/rpc	1.985s

go test ./internal/rpc/ -count=1
ok  	agent-vivy/internal/rpc	163.007s

gofmt -l internal/rpc/work_control.go internal/rpc/work_subscription.go internal/rpc/plan_goal_identity_test.go
(no output)

just fmt-check   -> exit 0
just vet         -> exit 0
just test        -> see below
```

## 3. UI gates

```
cd ui; pnpm vitest run src/components/chat/WorkControlBar.test.tsx src/components/chat/ChatView.test.tsx src/lib/failure.test.ts
Test Files  3 passed (3)
Tests       25 passed (25)
```

`just i18n-check` (its `ui-core` dependency runs `pnpm install`,
`pnpm typecheck`, `pnpm test`, `pnpm build`):

```
pnpm typecheck  -> passed
pnpm test       -> Test Files 93 passed (93) / Tests 701 passed (701)
pnpm build      -> FAILED in vite:esbuild-transpile: "remove
                   C:\Users\...\Temp\esbuild-<hash>: Access is denied."
i18n completeness / cross-face checks -> not reached by the recipe
```

The two i18n scripts were run directly instead (they only need the TypeScript
parser from `ui/node_modules`):

```
node scripts/check-i18n-completeness.js
PASS: en=1573 keys / 161 placeholders; zh=1573 keys / 161 placeholders; runtime copy audit clean.   exit 0

node --test scripts/check-i18n-cross-face.test.js
pass 8 / fail 0   exit 0

node scripts/check-i18n-cross-face.js
PASS: 14 shared semantic units; Web and TUI en/zh projections and arguments conform.   exit 0
```

An intermediate run reported `Unclassified Web key: chatInput.thinkingModeDefault /
chatInput.thinkingUnsupported` — two keys added by the in-flight
composer-thinking lane in the same working tree, not by this change; that lane
closed them in `scripts/i18n-cross-face-contract.json` while this session was
running, and the final run above is clean. `workControl.error`, which this
change deletes, and `errors.workControlTitle`, which it adds, were moved
together in the same contract file and produced no findings at any point.

### The `pnpm build` failure is environmental, not caused by this change

The failing step is esbuild removing its own temporary transform input. A
standalone script with no relation to this change reproduces it in this
session's shell, with `TEMP` pointing at `%TEMP%`, at the workspace, and at the
Desktop parent directory:

```
node probe.cjs
  require('…/.pnpm/esbuild@0.28.2/node_modules/esbuild')
    .transform(<large TS source>, { loader: 'ts', target: 'es2020' })
transform ERR: remove C:\...\esbuild-<hash>: Access is denied.
```

`Remove-Item` on files created by this shell in the same directories succeeds,
so the denial belongs to the esbuild child process's file handling in this
environment. The build did emit `ui/dist/assets/index-*.js` before failing, and
the emitted bundle contains the new surface (`Work state unavailable` and
`errors.workControlTitle` found in `ui/dist/assets/index-BhbuAj61.js`), so the
production path did compile the change.

Not verified: whether `pnpm build` and therefore `just ci` fail the same way in
a plain user terminal on this host. One `pnpm build` there settles it.

## 4. Full Go suite

```
just test   (go test -p 1 -timeout 35m ./... ; then scripts/test-memory-loop.ps1)
```

Every package relevant to this change passed, including `internal/rpc`
(159.975s) and `internal/app` (166.041s).

The failures are environmental, not attributable to this change:

- `sdk/internal` (13 tests), `sdk/internal/conformance` (1 test) and
  `sdk/generation` (1 test): all fail building the Web UI with the same
  `[vite:esbuild-transpile] remove …: Access is denied` from §3 (26 occurrences
  in the run). `scripts/test-memory-loop.ps1` was not reached.
- `FAIL agent-vivy/ui [build failed]` plus
  `removal_conformance_test.go:157: open ..\..\.workspace\tmp\build-temp\go-build…:
  The system cannot find the file specified`: transient races with a concurrent
  `just test` / `pnpm build` running in the same tree from the in-flight
  composer-thinking lane (`ui/e2e/thinking-gate.spec.ts`,
  `ui/src/components/chat/ChatInput.thinking.test.tsx` and the
  `chatInput.thinking*` keys appeared in the working tree while this run was in
  flight). `go vet ./ui` re-run alone exits 0.

## 5. Dev-loop transform

`GET http://127.0.0.1:3015/src/components/chat/WorkControlBar.tsx` returns 200
and the transformed module contains `WorkControlFailure`,
`RecoverableFailure` and `titleKey: "errors.workControlTitle"`, so the running
Vite dev server compiles the change. The backend on :8787 still serves the old
Go code, so the interface at :3015 shows the new error presentation only after
the dev pair is restarted.

Not verified: a click-through of the failure panel in a real browser. No
browser automation is available in this session; the panel was checked through
`WorkControlBar.test.tsx` (happy path, control-plane explanation, suppression
next to a failed conversation load) and the transport-level run above.
