# Task 5 report — authenticated Module Action RPC

Date: 2026-09-11
Branch: `feat/plugin-v1-p6`
Scope: PLG-P6 Task 5 only. Tasks 6 and 7 were not started.

## Design record

- `module.action.invoke` is the only RPC method for Module Control Actions.
  The handler advertises it only when the composition-owned `ActionHost` has
  a non-empty sealed action inventory; it does not register provider-defined
  paths or methods.
- `Peer` carries an opaque caller established by the authenticated transport.
  The WebSocket adapter binds the already-validated handshake token after the
  upgrade, and `Peer` copies that caller (plus the server-attested Face
  identity) into each request context. Browser JSON cannot populate either
  value. The app composition passes its one ActionHost to the Control handler.
- RPC parameters are strict and bounded: exactly `module_id`, `action_id`,
  and `input`; duplicate or unknown fields, invalid IDs, malformed JSON,
  inputs over 1 MiB, and JSON nesting over 32 levels are rejected before the
  Host is called. Approval, Trust, authority, Grant, session, Generation,
  instance, and caller claims are not accepted as request fields.
- The server passes the authenticated caller and the exact requested owner/
  action to `ActionHost.Invoke`. The Host remains authoritative for active
  Generation, instance, provider binding, effective Grants, schemas, policy,
  timeout/cancellation, audit, and Secret redaction. RPC errors use a small
  fixed vocabulary and never forward provider, audit, panic, or Secret text.
  Results are defensively copied, valid JSON, and bounded before serialization.
- `sdk/ui/src/action-client.ts` exposes a typed client and factory for the
  fixed method. It only validates identifiers and JSON-serializes bounded
  input (size/depth); it makes no approval, Trust, Grant, session, or other
  authority decision. Transport results and errors are returned to the
  caller. The SDK root export and package subpath expose the client while
  preserving existing exports.

## TDD evidence

### RED

The required focused tests were written before the production RPC/client
implementation. The initial failures were:

```text
PATH=/workspace/scratch/82edc4b9648e/toolchains/go1.26.4/bin:$PATH GOPROXY=https://goproxy.cn,direct GOFLAGS=-buildvcs=false go test ./internal/rpc -run ModuleAction -count=1
undefined: ControlDeps.ActionHost
undefined: WithAuthenticatedCaller

cd sdk/ui && pnpm exec vitest run src/action-client.test.ts
Failed to resolve import "./action-client"
```

The RED tests cover the authenticated caller/peer path, spoofed Module ID,
forged authority fields, missing caller, oversized/deep/invalid input,
unknown action and plugin-defined methods, cancellation/timeout, and
provider-error redaction. The SDK tests cover the fixed wire method and
shape, bounded serialization, cyclic input, no local authority fields, and
transport error preservation.

### GREEN

Focused checks after implementation:

```text
PATH=/workspace/scratch/82edc4b9648e/toolchains/go1.26.4/bin:$PATH GOPROXY=https://goproxy.cn,direct GOFLAGS=-buildvcs=false go test ./internal/rpc -run ModuleAction -count=1
ok   agent-vivy/internal/rpc

cd sdk/ui && pnpm exec vitest run src/action-client.test.ts src/module.test.ts
2 files passed; 27 tests passed

cd sdk/ui && pnpm exec tsc --noEmit
passed

git diff --check
passed
```

## Verification deferred to the P6 completion lane

The coding-first request deferred broad verification until all P6 coding is
complete. Run these exact gates from the final P6 tree:

```text
just ci

PATH=/workspace/scratch/82edc4b9648e/toolchains/go1.26.4/bin:$PATH GOPROXY=https://goproxy.cn,direct GOFLAGS=-buildvcs=false go test ./...
PATH=/workspace/scratch/82edc4b9648e/toolchains/go1.26.4/bin:$PATH GOPROXY=https://goproxy.cn,direct GOFLAGS=-buildvcs=false go vet ./...

cd sdk/ui && pnpm test && pnpm typecheck
cd ui && pnpm test && pnpm typecheck && pnpm build
```

In this Linux runner, `just` is not installed, so the repository `just ci`
entrypoint cannot be run here. The direct Go commands above retain the
linked-worktree `GOFLAGS=-buildvcs=false` workaround recorded by preflight.

## Files changed

- `internal/rpc/protocol.go`
- `internal/rpc/control.go`
- `internal/rpc/module_action_test.go`
- `internal/rpc/websocket.go` (authenticated WebSocket-to-Peer caller seam)
- `internal/app/app.go` (ActionHost-to-Control handler composition seam)
- `sdk/ui/src/action-client.ts`
- `sdk/ui/src/action-client.test.ts`
- `sdk/ui/src/index.ts`
- `sdk/ui/package.json`

## Known unverified concerns / handoff

- Full repository, UI, build, race, vet, and final P6 conformance gates remain
  intentionally unverified until Tasks 6 and 7 finish. No Task 6 or Task 7
  files were changed.
- The default generated Assembly currently has no Action ProviderSet, so its
  runtime capability correctly omits `module.action.invoke`; a selected
  Generation with Action inventory is required to exercise the endpoint.
- The WebSocket handshake supplies a Face connection identity but not a
  session/run identity. ActionHost bridge actions that require a durable
  session or run therefore remain fail-closed until a later composition seam
  supplies that server-owned context.
