# CH-C7a-N1: feishu stop-during-first-connect delivers firstErr

## What changed

- `plugins/feishu/plugin.go` (`supervise`): three early-return paths could
  exit without delivering the first attempt's outcome to Start while
  `first` was still pending — the loop-top `shouldContinue` bail, the
  `ctx.Done` branch of the READY wait, and the post-READY
  `shouldContinue` bail. All three are Stop (or caller-context) races
  during the first connect; a silent exit hangs Start on `firstErr` until
  the caller's context ends (Start watches the parent context, not the
  run context Stop cancels).
- Fix adopts qq's CH-C7a pattern verbatim: an exactly-once `report`
  closure (sent flag; every exit path calls it, later attempts are
  no-ops) plus `stopOutcome` (`ctx.Err()`, falling back to
  "channel stopped while connecting" for the stopped-latched window
  before cancel). The success path's direct `firstErr <- nil` also goes
  through `report` so the delivery contract lives in one place.

## Tests

- `plugin_test.go`: `TestStopDuringFirstConnectReturns` (qq's
  `TestStopDuringFirstHandshakeReturns` template): a `mutedReadyWS`
  wrapper drops the plugin's ready callback so the first attempt blocks
  in the READY wait; Stop lands mid-wait; Start must return an error
  within 2s. Without the fix Start hangs (the final select times out).
- The stop-during-start semantics are now real for both adapters that
  supervised-redial (qq, feishu); the TODO row's "unreachable via Host
  call order" caveat no longer needs to carry the latent risk.

## Not done

- dingtalk: its supervise reports only via its Start-failure return (no
  firstErr handshake — the SDK client's own Start call is the first
  attempt and returns directly), so the shape does not apply.
