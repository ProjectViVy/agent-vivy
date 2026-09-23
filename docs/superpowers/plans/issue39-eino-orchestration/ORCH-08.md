# ORCH-08 — integrated recovery and release acceptance

> **For implementers:** REQUIRED SUB-SKILL: `superpowers:executing-plans` or explicitly assigned `superpowers:subagent-driven-development`.

**Goal:** Verify the native child/workflow product end to end and produce auditable release evidence without masking skipped gates.
**Architecture:** One representative parent → task-masked child follow-up → two-node fanout/join → approval/cancel/restart scenario through public host and UI, both durable SQL backends where supported.
**Tech Stack:** Go, SQLite/Postgres, Eino v0.9.13, JSON-RPC, React, actual dev UI.
**Spec:** [architecture](../../specs/2026-09-23-issue39-eino-orchestration-design.md), G1–G4; [index](index.md) predecessor ORCH-07.
**Review focus:** reconcile graph checkpoint and Vivy Journal after crash, no duplicate effect, policy isolation, parent deletion and clean UI provenance.

## Files and evidence

Test-only changes in `internal/runtime/*test.go`, `internal/app/*test.go`, `internal/rpc/*test.go`, `internal/storage/conformance/*`, `ui/src/lib/*test.ts` and existing UI component tests as evidenced by gaps. NEW `docs/logs/YYYY-MM-DD-issue39-eino-orchestration/{summary,verification,acceptance}.md`; update `docs/TODO.md` §0.1 only for genuine remaining gaps, `docs/COMPLETE.MD` only after accepted completion. This Story must not quietly add unrelated product features to satisfy a fixture.

## Tasks

- [ ] Rebase on current authorized target and compare all decisions in design register with actual product approval. Verify old child/start/get/list/wait/cancel and ordinary parent Service.Run remain compatible. Reject implicit context or persona sharing.
- [ ] Matrix in `verification.md`: two SQL backends; live vs restart; approval allowed/denied/timed out; parent/child/graph cancel; depth/width/budget; same and conflicting operation IDs; corrupt and cross-version checkpoint; parent deletion and session isolation. For each record test name, command, actual result, source commit and whether the case really used Service and Eino. Verify no duplicate child/tool effect under injected crash at the checkpoint/Journal boundary.
- [ ] Run focused packages for storage/runtime/app/RPC/UI, then `just ci`. Use actual `just dev` UI at `http://127.0.0.1:3015` for delegation, follow-up, graph proposal, parallel join, status, interrupt/reload and accessibility/locales. Browser evidence must cite real host responses. Record environment/version and any unavailable gate as blocked rather than passed.
- [ ] Review docs vs shipped behavior and update architecture/index statuses and TODO/COMPLETE only with evidence. Fill `summary.md` with user-visible outcome, `verification.md` with reproducible commands and traces, `acceptance.md` with requirements R1–R7/G0–G4 and remaining risks. Seek human architecture and release review before closing #39.

## Failure and rollback

Stop the rollout if a safety gate fails. Disable public workflow admission while retaining read/status of already persisted Runs, preserve append-only migrations and Journal evidence, and repair with a new reviewed Story. A child-only release is possible only after an explicit scope update to the design and #39; do not silently claim full DAG acceptance. Release handoff references evidence paths, commit and final reviewer decision; absent checks remain visible in the index.
