# Serialize cognitive durable state transactions

Policy CAS, accepted-source notification and admission all update the same SQLite snapshot. The previous save helper re-read a fresh version while writing an older whole-state value. Concurrent writers could both accept the same policy revision, erase accepted SourceHigh, or erase an admitted workflow identity/window.

Serialize those bounded read/modify/write transactions on the existing Service, separate from loop-lifecycle and model execution locks. Carry the version obtained with the state value into SnapshotStore.Put instead of re-reading it. Workflow/model execution stays asynchronous on the existing Eino/native child/INOFY path; no new scheduler, authority or store exists.

Three controlled concurrency tests pause a real SQLite snapshot read: competing policy CAS, accepted input during policy save and workflow admission during policy save. Another test overlaps twelve manual triggers and an automatic input wake, asserting one admitted window and retained identity. Scripted Domain/model responses only exercise the controller boundary; these are not fake-backend product acceptance proofs.
