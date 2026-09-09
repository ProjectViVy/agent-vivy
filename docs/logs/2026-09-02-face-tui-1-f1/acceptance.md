# Acceptance — FACE-TUI-1 F1

## How to verify it (human acceptance perspective)

1. **Complete one approval-requiring conversation in-process, without opening any port.**
   ```
   go test ./internal/app -run TestLoopbackControlCompletesApprovedConversation -v
   ```
   Pass = in a `WithoutGateway()` composition (no embed, mux, origin policy, or listener),
   an ordinary Go client uses `App.DialControl` and the same RPC method set as the web face
   to create a session, start a turn, receive approval, approve it, wait for the run to
   finish, and see zero `write_note` errors in the journal with the final text in the
   session messages.

2. **Without a UI, approval does not silently self-heal; cancellation is the durable exit.**
   ```
   go test ./internal/app -run TestGatewaylessRunWithoutFaceCancelsDurably -v
   ```
   Pass = after approval suspends, the run stays suspended; send `run/cancel` through the
   in-process control plane, the run settles in `cancelled`, and the process's `Run` exits
   cleanly.

3. **The headless composition really no longer listens.**
   `TestLoopbackControl*` asserts `a.httpServer == nil`; `vivy_headless` (the headless-compile
   step in CI) now composes through `RunHeadless → WithoutGateway()`, so there is no
   in-process `http.Server`.

4. **The web face is unchanged.**
   The default composition (without the new option) is byte-for-byte equivalent to before
   the change — the embedded-UI path and origin-policy test
   (`TestRPCBootstrapRoutePrecedesUIShell`) in `just ci` are all green.

## Boundaries

- The TUI organ itself (`faces/tui`), the pack `face:` key, and the FaceHost SDK contract
  belong to later slices and are outside this slice's acceptance scope.
- The approval round uses `write_note`: read-only tools being automatically allowed through
  the approval gate is existing design (`runtime/policy.go`), not a defect in this slice.
