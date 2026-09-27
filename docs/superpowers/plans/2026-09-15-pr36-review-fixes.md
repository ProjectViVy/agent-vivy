# PR #36 Review Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make PR #36 mergeable and correct its channel plugin boundaries, durable-delivery recovery, run-event ordering, and session cleanup behavior.

**Architecture:** Keep transport details inside plugins and policy/durability inside the host. Resolve optional capabilities through the provider capability target, persist outbound identity before a run can emit events, and make recovery depend on an actually started channel. Preserve main's workspace migration while assigning new monotonic migration and conformance IDs to the channel work.

**Tech Stack:** Go, SQLite, PostgreSQL, TypeScript, GitHub Actions

**Spec:** `docs/architecture/VIVY-CHANNEL-PACK.md`

## Global constraints

- Add a regression test before each production fix and observe it fail.
- Keep plugin packages free of imports from `agent-vivy/internal/*`.
- Do not discard existing PR or `main` behavior while resolving conflicts.
- Run focused tests first, then the full repository verification required by CI.
- Update generated conformance evidence only from verified final sources.

### Task 1: Reconcile `main`

- Merge `main` into `feat/channel-tier1`.
- Keep workspace as SQLite migration 23 / PostgreSQL schema 21 / CN-27.
- Move channel deliveries to SQLite migration 24 / PostgreSQL schema 22 / CN-28.
- Move channel retention to CN-29.
- Combine overlapping RPC declarations and generated evidence sources without dropping either feature.

### Task 2: Preserve media capabilities through the app wrapper

- Add a production-shaped regression test using a provider instance that implements `MediaSender`.
- Verify the test fails because `providerChannel` hides the optional interface.
- Resolve `MediaSender` through `CapabilityTarget` and rerun the focused test.

### Task 3: Make delivery recovery safe

- Add tests for a durable row whose plugin is absent and for a compiled but non-started channel.
- Verify current recovery panics or consumes retries.
- Store channel identity independently from the live channel object.
- Rearm and enqueue only when the named channel is present in the started set; otherwise leave the row open.

### Task 4: Register outbound targets before runtime events

- Add a deterministic fast-terminal-run regression test.
- Verify the terminal event can arrive before target registration.
- Add a runtime preparation hook invoked after run creation and before the drive goroutine starts.
- Persist and register the target in that hook; abort startup if durable persistence fails.

### Task 5: Delete channel deliveries with sessions

- Extend storage conformance coverage to create a delivery, delete the session, and assert the delivery is gone.
- Verify SQLite and PostgreSQL fail before the fix.
- Add explicit channel-delivery deletion to both session deletion transactions.

### Task 6: Verify and refresh evidence

- Run focused package tests after each fix.
- Run the repository's full lint, unit, integration, conformance, and UI checks.
- Regenerate conformance evidence hashes from the final source tree.
- Confirm the PR is conflict-free and all required checks pass.
