---
id: durable
type: module-design
status: draft
title: durable (`plugins/vivy-durable`)
parent: architecture
depends-on: [gateway, laputa, python, basics, process]
---

## Responsibility

One persistent task lifecycle around Rig's execution path: tasks with an admitted
generation, durable state written at commit barriers, control operations that a
running turn observes, timed wakes that a live host delivers, asynchronous Recap
context, and AutoDream as the internal evolution role.

## Boundary

- It owns durability, not execution: the turn runs on Rig's stage machinery
  (`CommitBarrier`, `PersistOutcomes`), and this crate decides what survives a
  crash and how a resumed agent learns what it was doing.
- Every sibling it needs is an optional feature, so a recipe decides whether the
  edge exists: `runtime` (state and lifecycle), `assistant-context` (Recap, with
  laputa and basics), `wake`, `autodream` (with python).
- `d0-probe`, `checkpoint-probe` and `rig-fork-probe` are acceptance probes for the
  Rig checkpoint APIs. They are not runtime capability; `rig-fork-probe` only
  re-characterizes a local fork.

## Invariants

- **It targets ordinary process crashes, and says so.** OS-crash and power-loss
  durability are neither implemented nor promised; the storage layer reports a
  state as healthy or not and never presents an unverified one as recovered.
- Persisted state is keyed by capability (`vivy.durable.v1`/`v2`/`v3`), so a build
  with fewer features does not read state a richer build wrote.
- A result is persisted only after its stage commits; interrupted work is not
  replayed blindly.
- Background work (Recap, Dream) yields to foreground input, and a detached Dream
  job is cancellable through the front.
- The key is written with the checkpoint APIs of the local Rig fork named in the
  handoff, not by the ordinary upstream pin.
- Existing sessions and executables from earlier species are preserved rather than
  migrated; new sessions are the D4 ones.

## Limitations

- Requires the unpublished local Rig checkpoint fork via `--rig-path`; upstream
  alone does not provide the persistence interfaces.
- Print mode saves no wait: a deadline needs a live terminal or desktop host.
- Long-term memory uses explicit tools only; idle reflection is off by default.
- The assistant does not yet delegate coding sessions or subagents.
