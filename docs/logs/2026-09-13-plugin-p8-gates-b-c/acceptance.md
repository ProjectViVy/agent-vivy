# PLG-P8 Gates B/C acceptance

Date: 2026-09-13

- [x] Gate A is already merged and its frozen authority/Port map remains the
  controlling contract.
- [x] A real SCX reference Module is selected through a sealed Recipe and
  generated Assembly; no runtime plugin discovery or hidden authority is added.
- [x] ContextHost enforces scope, versions, exact resource resolution,
  cancellation, expiry, deterministic Treatment, provenance, redaction, and
  required/optional byte budgets.
- [x] Immutable Context View identity covers Source, content, version,
  provenance, and Treatment.
- [x] Runtime prepares the initial model input through ContextHost and commits
  the View in a versioned event that restart/resume paths recover.
- [x] ObserverHost receives only committed subscribed events, applies a sealed
  allowed-field projection and structural secret redaction, and advances its
  cursor only on an accepted/completed receipt.
- [x] Ambiguous acknowledgement and outage/reconnect produce one logical SCX
  update; restart recovers pending completed/cancelled terminal delivery.
- [x] Legacy closed v1 request/terminal payload shapes remain valid while the
  additive SCX projection is explicitly versioned v2.
- [x] Default and minimal Recipes remain buildable; module removal changes the
  Generation and physically omits the SCX Module and generated bindings.
- [x] Two SCX rebuilds are byte/identity deterministic and `inspect-artifact`
  verifies the sealed policies and source hashes.
- [x] The packed SCX binary boots through its generated Assembly.
- [x] Studio installs the prior and candidate releases, rolls back to the prior
  sealed Generation, and preserves the tenant Journal sentinel byte-for-byte.
- [x] Focused race checks, full Go/UI CI equivalents, standalone Module checks,
  and independent review pass.
- [x] Original SCX stage IDs are not invented; completion is recorded against
  stable Gates A/B/C under the owner's explicit authorization.
- [x] PLG-P8 is complete. Broader PLG-P9 remains open and unclaimed.
