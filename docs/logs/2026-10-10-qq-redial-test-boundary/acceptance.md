# Acceptance

The reconnect test still proves that a resumable close carries the original
session ID and sequence, an unresumable close triggers a fresh Identify, a
cannot-identify close ends the supervisor after exactly three attempts, and
Stop remains bounded and idempotent.

Assertions now observe completion of these operations, rather than availability
of a freshly allocated client or passage of a fixed sleep. Twenty ordinary and
twenty race-enabled repetitions pass with the complete plugin suite.

## Module boundary

- Module `vivy/qq` is the existing trusted T2 source at `repo:plugins/qq`, based
  on `cc024efc`; the parent refreshes its final source pin after integration.
- It provides `vivy.qq` on supported `std/channel@v1`, consumed solely by
  ChannelHost (`0..n` providers). Runtime policy and grants retain authority.
- Existing grants are `channel.poll`, `secret.read`, and constrained
  `net.client`; all fixture traffic uses in-memory fakes or loopback HTTP.
- Generation lifecycle, supervisor failure behavior, SDK, Provider, Consumer,
  Failure Model, and Inspect contracts are unchanged. Existing executable
  conformance artifacts are refreshed by the parent against the final tree.
- This maintenance change preserves the completed channel lifecycle requirement
  in `PLG-P2-default-generation-parity.md` and repairs its failing QQ test.
