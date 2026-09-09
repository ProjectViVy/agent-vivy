# CH-C8 — Same-Binary Subprocess (DEFERRED Note, Not a Work Order)

## 1. Identity

| | |
|---|---|
| ID | CH-C8 |
| Status | **DEFERRED** — Not part of this phase close |
| Dependency | At least M-CH2 (Host ABI written across a process boundary) |
| Contract | §6 third-layer offloading, C8 |

**Do not claim this file for implementation** until a separate capability proposal is opened.

## 2. Goal (Future)

The `vivy channel --name telegram` subprocess runs the adapter. The Host supervises it. A crash = `channel_lost`, not species death. Killing one ear does not stop Journal.

## 3. Current State

Since C3, the contract has been written across a process boundary (Inbound goes only through Env), but the first slice calls in-process. The worker sub-run's argv pattern can be reused.

## 4. Target Structure

Same binary, different processes. Not a second body (NG-11). Not a `.dll`. Env.PublishInbound uses IPC (specific protocol to be decided later).

## 5–8. Out of Scope

Do not invent custom IPC prematurely in C4–C7. Do not use an external `telegram.exe`.

## 9. Risk

Splitting processes too early will distort the ABI template. Make the split after the text loop is stable.

## 10. Handoff

Write the real ten-section work-start PLAN only after an independent capability proposal is approved.
