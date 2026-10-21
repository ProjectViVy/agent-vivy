-- Offline target-copy fragment. Bind ?1 to the conversion timestamp.
-- Execute with expire_baseline_approvals.sql and the other owners' pending
-- interaction settlement inside ONE transaction. Never run against a source.
-- Keep generations as historical private bytes; disable only Eino pointers.
DELETE FROM checkpoints WHERE substr(id, 1, 5) = 'ckpt-';

UPDATE questions
SET status = 'expired', answered_at = ?1, actor = 'upgrade_conversion',
    decision_reason = CAST('upgrade_recovery_required' AS BLOB)
WHERE status = 'pending'
  AND run_id IN (
      SELECT id FROM runs
      WHERE status IN ('accepted', 'queued', 'active')
        AND NOT EXISTS (
            SELECT 1 FROM run_events WHERE run_events.run_id = runs.id
              AND type IN ('run.completed', 'run.failed', 'run.cancelled',
                           'child.completed', 'child.failed', 'child.cancelled')
        )
  );

INSERT INTO run_events (run_id, seq, type, created_at, payload_version, payload)
SELECT r.id, COALESCE((SELECT MAX(seq) FROM run_events WHERE run_id = r.id), 0) + 1,
       CASE WHEN r.kind = 'child' THEN CASE WHEN r.status = 'active' THEN 'child.failed' ELSE 'child.cancelled' END
            WHEN r.status = 'active' THEN 'run.failed' ELSE 'run.cancelled' END,
       ?1, 1, CAST(
           CASE WHEN r.status = 'active' THEN
             '{"cause_category":"internal_error","message":"upgrade_recovery_required","outcome":"upgrade_recovery_required"}'
           ELSE '{"reason":"upgrade_recovery_required","outcome":"upgrade_recovery_required"}' END
           AS BLOB)
FROM runs r
WHERE r.status IN ('accepted', 'queued', 'active')
  AND NOT EXISTS (
      SELECT 1 FROM run_events WHERE run_id = r.id
        AND type IN ('run.completed', 'run.failed', 'run.cancelled',
                     'child.completed', 'child.failed', 'child.cancelled')
  );

UPDATE runs
SET status = CASE WHEN status = 'active' THEN 'failed' ELSE 'cancelled' END
WHERE status IN ('accepted', 'queued', 'active')
  AND EXISTS (
      SELECT 1 FROM run_events WHERE run_id = runs.id AND created_at = ?1
        AND type IN ('run.failed', 'run.cancelled', 'child.failed', 'child.cancelled')
        AND json_extract(CAST(payload AS TEXT), '$.outcome') = 'upgrade_recovery_required'
  );
