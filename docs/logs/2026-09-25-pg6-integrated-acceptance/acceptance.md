# Acceptance

Run the deterministic app-level suite:

```text
go test ./internal/app -run '^TestPlanGoalIntegrated' -count=1
```

It verifies the exact saved Plan and review origin, explicit Goal approval, two ordinary runs, a successful execute event used as completion evidence, stable completion after reopen, and read-only write denial. Additional cases verify pending review recovery, durable round-limit blocking and its restarted RPC projection, in-flight pause and reopen, and migration of a version-23 SQLite database through `app.New`.

For the user's browser acceptance, the split app at `http://127.0.0.1:3015` should show the submitted Plan without changing its Markdown, require explicit approval before the Goal starts, show later automatic runs and their evidence, and refresh activation correctly after reconnect. No browser result is recorded here. PostgreSQL parity and the live-provider coding walkthrough also remain open, so PG-6 and downstream Stories remain blocked.
