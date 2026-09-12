# PLG-P7 internal moduleization acceptance

- [x] Every required internal Port has exactly one canonical T1 Provider.
- [x] Public Recipes and T2/T3 Modules cannot provide or override `core/*`
  authority.
- [x] LoopDriver and ModelHost compose through internal Modules without a
  second Engine or model broker.
- [x] Storage and checkpoint composition preserve Journal schema, terminal
  uniqueness, durability, recovery, and Eino adapter boundaries.
- [x] Credential resolution is scoped and non-enumerable, and secrets do not
  enter Manifests, Journals, errors, or provider-cache keys.
- [x] Sandbox composition preserves final Policy, approval, cancellation,
  process cleanup, audit, and ToolHost governance.
- [x] Optional Hosts are selected conditionally and omitted capabilities are
  absent from minimal Assembly output.
- [x] Lifecycle startup failure rolls back in reverse order, preserves causes,
  and shutdown is reverse-order and idempotent.
- [x] Automated conformance prevents second L0 Service, Journal, Policy, RPC,
  ChannelHost, FaceHost, ActionHost, or ToolHost owners and enforces the Eino
  import quarantine.
- [x] The exact `just ci` gate and a real default pack/Inspect smoke pass.
- [x] No PLG-P8 SCX-integration or PLG-P9 release-conformance claim is made.
