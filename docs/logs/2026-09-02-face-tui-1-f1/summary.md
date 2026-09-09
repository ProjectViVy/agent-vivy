# FACE-TUI-1 F1 — gateway-less control plane (gateway-less composition)

## What changed

VIVY-FACE-PACK (PR 2 / F1) acceptance criterion: "the launcher/app composition works
without embed; the failure path for approval without a UI is tested." This slice delivers
two items:

1. **Decouple the composition** (`internal/app/app.go`): add `WithoutGateway() AppOption`.
   The gateway branch (browser origin policy, `/rpc` mux, embedded UI shell, and
   `http.Server`) is built only for the default composition; with `WithoutGateway`, the
   process has no listener/embed at all, while the control-plane handler remains in
   `App.control`. `Run`/shutdown are safe when `httpServer == nil`. The `vivy_headless`
   composition (`internal/app/headless.go`) now uses `WithoutGateway()` as well — headless
   previously built an `http.Server` that it never served; under the §7 semantics it now
   does not listen at all.
2. **In-process control plane** (`internal/app/facehost.go`):
   `App.DialControl(ctx, notifications)` uses `net.Pipe` + the existing
   `controlrpc.NewJSONLTransport` + `NewPeer` to start a loopback JSON-RPC pair. The host-side
   handler is the same `controlHandler` reached by the web face over WebSocket, with no
   protocol-layer changes (the protocol layer was transport-independent already).

Tests (`internal/app/facehost_test.go`, app-level composition tests, scripted Anthropic SSE
server + frozen ENV session):

- `TestLoopbackControlCompletesApprovedConversation`: in a composition with no embed or
  listener, `DialControl` completes the same method chain as the web face
  (`initialize → session/create → turn/start → approval/list →
  approval/respond(approved) → run/get(completed)`); the journal asserts zero errors for
  `tool.finished`, and the final text lands in `session/messages`.
- `TestGatewaylessRunWithoutFaceCancelsDurably`: after approval suspends with no face
  response, the run never self-heals; `run/cancel` through the same in-process control plane
  settles the run in `cancelled`, and gateway-less `App.Run` returns nil after context
  cancellation.

## Key findings (facts pinned down in tests)

- Read-only tools are always allowed through the approval gate by
  `EvaluateApprovalPolicy` (`internal/runtime/policy.go`, "readonly or question tools do
  not require approval") — governance-prompt rules do not create a suspension point for
  read-only tools. F1 therefore uses `write_note` for the approval round (non-read-only,
  the existing choice for headless approval tests).
- A manually constructed `config.Config` bypasses defaults: an empty
  `Runtime.WorkspaceRoot` makes the engine's agentsmd loader receive a nil filesystem
  backend and crash; a zero `Tools.Approval.Expiration` makes approval expire immediately.
  Both are explicit in the test config (the real assembly path is covered by config
  defaults).
- The engine calls the model in streaming mode: the scripted server uses `"stream":true`
  to select an Anthropic SSE transcription response, while non-streaming requests still
  return JSON (matching the protocol-level test method in `provider/claude_test.go`).
- Fixed a real regression risk: `appOptions` must explicitly initialize `gateway` to `true`
  (the zero value `false` would remove the HTTP server from every default composition;
  `TestRPCBootstrapRoutePrecedesUIShell` caught it immediately).

## Explicitly not done

- FaceHost / SDK Face contract, the built-in `faces/tui` organ, and the pack `face:` recipe
  key (later FACE-TUI-1 slices).
- F2 headless organization (`vivy run` currently uses `RunHeadless` and does not depend on
  a new face from this slice).
- The four §14 questions (standalone `go.mod` for `faces/`, default face, web/TUI
  cohabitation, and headless approval failure exit) — present them for decision under the
  contract before the F2/F3 slices.
