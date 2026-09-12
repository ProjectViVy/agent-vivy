# PR #21 integration summary

## Outcome

PR #21's PLG-P3 governance closure was reviewed against `main`, and its
approval authority remains on the existing Service, Journal, Policy,
ToolHost, and Eino checkpoint path. The integration pass fixed one durable
approval cleanup race exposed by GitHub CI.

## Changes

- Added a deterministic regression that cancels a run synchronously when its
  `tool.approval_required` event is published, before the run can enter the
  in-memory pending map.
- Closed the exact durable approval through the existing
  `ApprovalLifecycleStore` when cancellation wins that suspension window.
- Preserved event order: approval required, approval cancelled, then one run
  terminal. No alternate runtime, approval store, or execution path was added.

## Eino capability check

The PLG-P3 branch uses pinned Eino `StatefulInterrupt`, `GetInterruptState`,
and `GetResumeContext` through `internal/runtime`. The cleanup fix is Vivy's
durable lifecycle around that checkpoint and does not add or replace an Eino
capability.

## Explicitly not changed

- PLG-P7 internal Module composition in PR #22.
- Public Port versions, Recipe selection, or generated Assembly wiring.
- Tenant journals or Studio state.
