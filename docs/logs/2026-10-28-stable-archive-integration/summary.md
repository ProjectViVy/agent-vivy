# Stable archive integration

## Scope

The owner selected `stable` as the integration target and explicitly excluded `main`. Preserve the existing Mac working changes in `snapshot/mac-uncommitted-20261028-032654` (`f122a72f`) before integrating other branches.

The UI integration merges `feat/ui-overhaul` (`b9fdda15`) while preserving the snapshot's single-primary composer, persisted busy-enter preference, and explicit queued-message flush. Port those behaviors into the decomposed composer instead of restoring the deleted monolithic ChatInput. Replace the removed radio-group primitive with native radio inputs using the semantic token palette.

The original checkout and its index remain untouched. Source deletion is conditional on verified remote preservation and recovery coverage. No production Journal is used for verification.

The second merge integrates `wip/root-lane-snapshot` (`c27a26c4`): incremental Journal-to-message projection, one-shot child projection cleanup migration 048, and interrupted partial-output presentation. Resolve conflicts by retaining newer stable terminal reconciliation and the snapshot's append-only session-view filtering. Keep the new UI message-component decomposition, adding the interrupted marker at its actual assistant owner. Preserve both legacy and explicit-child regression coverage. The incremental cursor is derived state behind the existing projection mutex; the durable Journal remains authoritative. This integrates pre-existing code, not a new orchestration implementation or Eino replacement.
