# A2A-01 acceptance notes

Story outcome (E1 / R1): nonempty optional Channel task contract with
correct Module identity and no regression to existing providers.

- The public seam is exactly design §5.1 — verbatim declarations, no
  aliases, no validation/persistence policy smuggled into SDK helpers.
- `ChannelModuleIDs` is generated from resolved descriptors (single
  source), keyed by provider ID; the wrapper's `ModuleID()` now returns
  the sealed Module identity (`vivy/telegram`) instead of the provider
  display ID (`vivy.telegram`) — the RF2 smell is closed.
- `taskGrantedChannelHost` forwards the five TaskHost methods and the
  discovery reader only when both the wrapped capability and the
  `channel.a2a` grant exist, re-checking the grant on every call; a
  typed-nil Host is never invoked.
- Missing/ambiguous Module mapping is a build/validation error at both
  surfaces (generator, app validation); ordinary Channels keep the
  preexisting capability set.
- Explicitly NOT claimed: production TaskHost, HTTP listener, resume
  cursor — those belong to A2A-02..05. Inspect still distinguishes
  declared/granted/wired/started truth.
- just ci red = sole known main regression `TestClientAgainstRealVivyCode`
  (PR #37 pending merge); documented, not introduced here.
