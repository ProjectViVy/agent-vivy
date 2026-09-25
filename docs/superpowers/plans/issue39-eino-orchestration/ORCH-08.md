# ORCH-08 — integrated recovery and release acceptance

> **For implementers:** REQUIRED SUB-SKILL: `superpowers:executing-plans` or explicitly assigned `superpowers:subagent-driven-development`.

**Goal:** Verify the native child/workflow product end to end and produce auditable release evidence without masking skipped gates.
**Architecture:** Verify R1-R14 through host/UI and both durable SQL backends: authority lineage/intersection, one-shot/continuable modes, required direct mailbox, clean context, parent model, bounded DAG, recovery and lifecycle. G0/G1 remain feasibility gates; planning approval bypasses neither.
**Tech Stack:** Go, SQLite/Postgres, Eino v0.9.13, JSON-RPC, React, actual dev UI.
**Spec:** [architecture](../../specs/2026-09-23-issue39-eino-orchestration-design.md), G1–G4; [index](index.md) predecessor ORCH-07.
**Review focus:** reconcile graph checkpoint and Vivy Journal after crash, no duplicate effect, policy isolation, parent deletion and clean UI provenance.

## Files and evidence

Test-only changes in `internal/runtime/*test.go`, `internal/app/*test.go`, `internal/rpc/*test.go`, `internal/storage/conformance/*`, `ui/src/lib/*test.ts` and existing UI component tests as evidenced by gaps. NEW `docs/logs/YYYY-MM-DD-issue39-eino-orchestration/{summary,verification,acceptance}.md`; update `docs/TODO.md` §0.1 only for genuine remaining gaps, `docs/COMPLETE.MD` only after accepted completion. This Story must not quietly add unrelated product features to satisfy a fixture.

## Tasks

- [ ] Rebase on current authorized target and compare all decisions in design register with actual product approval. Verify old child/start/get/list/wait/cancel and ordinary parent Service.Run remain compatible. Verify selected model/tools stay within parent policy; reject implicit context or persona sharing.
- [ ] Acceptance matrix R1-R14: both SQL backends; current authorizer vs immutable origin lineage, authority intersection, post-origin Run reauthorization and deletion fence (D4); durable direct mailbox identity/order/lifecycle/safe consumption/retry/restart with at-least-once delivery and no exactly-once effect claim (D8); clean task/mail/dependency context only (D10); read-only baseline (D11); parent model and tool narrowing (D12); finite DAG/resource limits (D13); one-shot/continuable distinction (D14); all R1-R7 outcomes. Cover approvals, cancellation, restart, deletion, duplicate/conflicting IDs and unreachable/unconsumed nodes. Record actual command/result/commit and Service/Eino use.
- [ ] Run focused packages for storage/runtime/app/RPC/UI, then `just ci`. Use actual `just dev` UI at `http://127.0.0.1:3015` for delegation, follow-up, graph proposal, parallel join, status, interrupt/reload and accessibility/locales. Browser evidence must cite real host responses. Record environment/version and any unavailable gate as blocked rather than passed.
- [ ] Review docs vs shipped behavior. G0/G1 must pass on evidence; planning approval is not gate approval. Close only after R1-R14 and G0-G4 evidence; deferred/unavailable checks are unverified, never pass. Seek human architecture/release review.

## Failure and rollback

Stop the rollout if a safety gate fails. Disable public workflow admission while retaining read/status of already persisted Runs, preserve append-only migrations and Journal evidence, and repair with a new reviewed Story. A child-only release is possible only after an explicit scope update to the design and #39; do not silently claim full DAG acceptance. Release handoff references evidence paths, commit and final reviewer decision; absent checks remain visible in the index.
