# Issue 39 recovery architecture revision

The owner requested an architecture update after review of branch `bbcbd10`. D15 preserves Eino scheduling and Service/Journal ownership, moves the minimum durable operation/claim/result foundation into ORCH-01 before integrated G0, and requires explicit blocking of unknown effects. The prior double-call probe does not establish crash replay or Eino incompatibility. Historical evidence is unchanged; the current gate is BLOCKED on missing implementation and integration evidence.

Synchronized architecture, Story index, ORCH-01/02/06/08, execution plan and backlog. Unexecuted Tasks 5-14 and final acceptance checks are open. No runtime, test code, schema or user-facing behavior was changed. No commit/push or release is part of this documentation edit.
