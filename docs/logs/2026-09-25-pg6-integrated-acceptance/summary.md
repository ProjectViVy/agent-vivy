# PG-6 Deterministic Integrated Acceptance

Added deterministic Plan-to-Goal acceptance tests through the real application, JSON-RPC, pinned Eino/provider path, disposable SQLite database and workspace, and a loopback scripted model server. This delivery changes tests and conformance evidence only; it adds no production runtime path or dependency.

The scenarios cover exact Plan review and human approval, two ordinary Goal runs, a real `go test ./...` execute event linked to completion evidence, completed replay, and read-only write denial. Further app-level cases cover pending review recovery without duplicate execution, finite round-limit exhaustion with the blocked Work projection preserved after restart, pause of an in-flight model request followed by disarmed reopen, and opening a version-23 SQLite database through `app.New`. Existing focused tests cover legacy hard-Plan resume and lower-level transaction and wake races. No app-level fault injector exists for the exact interval between SQLite commit and in-process publication; that interval is not claimed as directly observed here.

PostgreSQL/Docker validation is deferred to the next phase, browser acceptance belongs to the user, and the live-provider coding walkthrough remains unverified because credentials were not configured. These are still PG-6 acceptance gaps, not passes.

No `release.md` is included because this delivery updates a draft PR; it is not a release.
