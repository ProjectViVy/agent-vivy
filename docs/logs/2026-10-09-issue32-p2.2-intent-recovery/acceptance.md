# Acceptance: P2.2

## Local engineering acceptance

C2's local recovery contract is implemented and verified:

- The exact canonical intent exists before workflow admission and survives SQLite close/reopen.
- A restart adopts the existing active Run under the original parent/key; it does not allocate a second workflow.
- An unadmitted intent tied to a terminal supervisor stays fenced instead of rebinding to a replacement parent.
- A deterministic supervisor row created before its snapshot write is adopted if active; a spent terminal ID is skipped.
- A native active workflow whose engine projection is recovery_required or has an unknown node outcome is fenced with no stale ActiveRunID.
- A completed Run settles only its persisted Through. Later input remains above the watermark after completion and another store reopen.
- Only a fully evidenced failure can retry the same Through under the next bounded attempt. Missing workflow projection, effect-stage start, missing model-call finish, lookup error, cancellation, or an ambiguous legacy window remains fenced. A native failed Run whose projection is still settling retains its intent and is rechecked; durable proof for that exact Run can clear a provisional unknown fence.
- Legacy ActiveRunID and attempt are reconstructed from and checked against immutable revision/operation-key data; exhausted retries remain exhausted. Unsupported future schema values reject admission.

## Ruling

`unknown_outcome` remains fenced on manual Trigger as well as automatic wake. A manual trigger cannot prove whether an uncertain external effect committed, so clearing the fence and minting a new operation key could duplicate it. The cost is that a false-positive fence needs a separate deliberate recovery/reset decision; P2.2 does not add that control surface.

## Remaining gates

PostgreSQL conformance and aggregate `just ci` remain pending. P2.3/P2.4 and the remaining phases are open. This task is not product or release accepted.
